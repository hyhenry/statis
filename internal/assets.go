package internal

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Constants for asset management
const (
	FontsDir = "./fonts"
)

// Font helpers

func IsPredefinedFont(fontName string) bool {
	return PredefinedFonts[fontName]
}

func NormalizeFont(fontName string) string {
	return strings.ReplaceAll(fontName, " ", "-")
}

// Asset cleanup

type cleanupResult struct {
	FontsRemoved int      `json:"fonts_removed"`
	IconsRemoved int      `json:"icons_removed"`
	FilesRemoved []string `json:"files_removed"`
}

func HandleCleanUnusedAssets(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, http.MethodDelete) {
		return
	}

	ConfigMu.RLock()
	currentFont := AppConfig.Theme.FontFamily
	usedIcons := GetUsedIcons()
	ConfigMu.RUnlock()

	result := cleanupResult{
		FilesRemoved: []string{},
	}

	// Clean unused fonts
	if entries, err := os.ReadDir(FontsDir); err == nil {
		normalizedFont := NormalizeFont(currentFont)
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			// Keep files that start with the current font name (e.g., "Inter.css", "Inter_xxx.woff2")
			if currentFont != "" && !IsPredefinedFont(currentFont) && strings.HasPrefix(name, normalizedFont) {
				continue
			}
			// Remove unused font file
			filePath := filepath.Join(FontsDir, name)
			if err := os.Remove(filePath); err == nil {
				result.FontsRemoved++
				result.FilesRemoved = append(result.FilesRemoved, "fonts/"+name)
				log.Printf("Removed unused font: %s", name)
			}
		}
	}

	// Clean unused icons
	if entries, err := os.ReadDir(IconsDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			iconPath := "/icons/" + name
			if !usedIcons[iconPath] {
				// Remove unused icon file
				filePath := filepath.Join(IconsDir, name)
				if err := os.Remove(filePath); err == nil {
					result.IconsRemoved++
					result.FilesRemoved = append(result.FilesRemoved, "icons/"+name)
					log.Printf("Removed unused icon: %s", name)
				}
			}
		}
	}

	log.Printf("✓ Cleaned up %d fonts and %d icons", result.FontsRemoved, result.IconsRemoved)
	WriteJSON(w, result)
}

// GetUsedIcons returns a set of icon paths that are currently referenced in the config
func GetUsedIcons() map[string]bool {
	used := make(map[string]bool)
	for _, section := range AppConfig.Services {
		for _, item := range section.Items {
			if item.Icon != "" && strings.HasPrefix(item.Icon, "/icons/") {
				used[item.Icon] = true
			}
			// Also include icons that would be generated from icon_name
			if item.IconName != "" {
				used["/icons/"+item.IconName+".svg"] = true
			}
		}
	}
	// Include favicon if it's from the icons directory
	if AppConfig.Theme.Favicon != "" && strings.HasPrefix(AppConfig.Theme.Favicon, "/icons/") {
		used[AppConfig.Theme.Favicon] = true
	}
	if AppConfig.Theme.FaviconName != "" {
		used["/icons/"+AppConfig.Theme.FaviconName+".svg"] = true
	}
	return used
}

// ProcessIconNames downloads icons for any items that have icon_name set.
// Also processes favicon_name in the theme.
// Returns true if any config changes were made (icon paths updated).
func ProcessIconNames(cfg *Config) bool {
	if err := EnsureDir(IconsDir); err != nil {
		log.Printf("Warning: Failed to create icons directory: %v", err)
		return false
	}

	changed := false

	// Process service item icons
	for i := range cfg.Services {
		for j := range cfg.Services[i].Items {
			item := &cfg.Services[i].Items[j]
			if item.IconName == "" {
				continue
			}

			iconFileName := item.IconName + ".svg"
			iconPath := filepath.Join(IconsDir, iconFileName)
			localURL := "/icons/" + iconFileName

			// Check if file exists
			if _, err := os.Stat(iconPath); err == nil {
				if item.Icon != localURL {
					item.Icon = localURL
					changed = true
					log.Printf("✓ Updated icon path for '%s' to match icon_name: %s", item.Name, item.IconName)
				}
				continue
			}

			// Download from CDN
			iconURL := fmt.Sprintf("%s/svg/%s.svg", IconCDNBase, item.IconName)
			if err := DownloadFile(iconURL, iconPath); err != nil {
				log.Printf("Warning: Failed to download icon '%s': %v", item.IconName, err)
				continue
			}

			item.Icon = localURL
			changed = true
			log.Printf("✓ Downloaded icon for '%s': %s", item.Name, item.IconName)
		}
	}

	// Process favicon_name
	if cfg.Theme.FaviconName != "" {
		iconFileName := cfg.Theme.FaviconName + ".svg"
		iconPath := filepath.Join(IconsDir, iconFileName)
		localURL := "/icons/" + iconFileName

		// Check if file exists
		if _, err := os.Stat(iconPath); err == nil {
			if cfg.Theme.Favicon != localURL {
				cfg.Theme.Favicon = localURL
				changed = true
				log.Printf("✓ Updated favicon path to match favicon_name: %s", cfg.Theme.FaviconName)
			}
		} else {
			// Download from CDN
			iconURL := fmt.Sprintf("%s/svg/%s.svg", IconCDNBase, cfg.Theme.FaviconName)
			if err := DownloadFile(iconURL, iconPath); err != nil {
				log.Printf("Warning: Failed to download favicon icon '%s': %v", cfg.Theme.FaviconName, err)
			} else {
				cfg.Theme.Favicon = localURL
				changed = true
				log.Printf("✓ Downloaded favicon icon: %s", cfg.Theme.FaviconName)
			}
		}
	}

	return changed
}

// DownloadGoogleFont downloads a Google Font and saves it locally with offline font files
func DownloadGoogleFont(fontName string) error {
	if fontName == "" || IsPredefinedFont(fontName) {
		return nil
	}

	if err := EnsureDir(FontsDir); err != nil {
		return err
	}

	normalizedName := NormalizeFont(fontName)
	fontPath := filepath.Join(FontsDir, normalizedName+".css")

	// Check if font already exists
	if _, err := os.Stat(fontPath); err == nil {
		log.Printf("Font '%s' already downloaded", fontName)
		return nil
	}

	// Fetch font CSS from Google Fonts API
	cssContent, err := fetchGoogleFontCSS(fontName)
	if err != nil {
		return err
	}

	// Download font files and update CSS to reference local copies
	modifiedCSS, err := downloadFontFilesAndUpdateCSS(cssContent, normalizedName, FontsDir)
	if err != nil {
		log.Printf("Warning: Failed to download font files for '%s': %v", fontName, err)
		modifiedCSS = cssContent // Use original CSS as fallback
	}

	if err := os.WriteFile(fontPath, []byte(modifiedCSS), 0644); err != nil {
		return err
	}

	log.Printf("✓ Downloaded Google Font: %s", fontName)
	return nil
}

// fetchGoogleFontCSS fetches CSS from Google Fonts API
func fetchGoogleFontCSS(fontName string) (string, error) {
	fontURL := "https://fonts.googleapis.com/css2?family=" + url.QueryEscape(fontName) + ":wght@300;400;500;600;700&display=swap"
	req, err := http.NewRequest("GET", fontURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("font not found: HTTP %d (check font name spelling)", resp.StatusCode)
	}

	cssBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(cssBytes), nil
}

// downloadFontFilesAndUpdateCSS downloads font files and updates CSS to reference local copies
func downloadFontFilesAndUpdateCSS(cssContent, fontName, fontsDir string) (string, error) {
	// Regex to find url(...) in CSS
	// Matches: url(https://fonts.gstatic.com/...)
	urlRegex := `url\((https://[^)]+)\)`
	re := regexp.MustCompile(urlRegex)

	matches := re.FindAllStringSubmatch(cssContent, -1)
	if len(matches) == 0 {
		return cssContent, nil // No URLs found
	}

	modifiedCSS := cssContent
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}

		fontFileURL := match[1]

		// Extract filename from URL
		urlParts := filepath.Base(fontFileURL)
		// Create a safe filename
		fontFileName := fontName + "_" + urlParts
		fontFilePath := filepath.Join(fontsDir, fontFileName)

		// Download the font file if it doesn't exist
		if _, err := os.Stat(fontFilePath); os.IsNotExist(err) {
			if err := DownloadFile(fontFileURL, fontFilePath); err != nil {
				log.Printf("Warning: Failed to download font file %s: %v", fontFileURL, err)
				continue
			}
		}

		// Replace the Google URL with local path
		localURL := "/fonts/" + fontFileName
		modifiedCSS = strings.Replace(modifiedCSS, fontFileURL, localURL, 1)
	}

	return modifiedCSS, nil
}
