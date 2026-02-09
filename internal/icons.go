package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Constants for icon management
const (
	IconsDir        = "./icons"
	iconManifestURL = "https://cdn.jsdelivr.net/gh/homarr-labs/dashboard-icons@main/tree.json"
	IconCDNBase     = "https://cdn.jsdelivr.net/gh/homarr-labs/dashboard-icons"
	iconCacheTTL    = 24 * time.Hour
	maxUploadSize   = 5 << 20 // 5MB
)

var (
	// Icon manifest cache
	iconManifest    []string
	iconManifestMu  sync.RWMutex
	iconManifestAge time.Time

	// Allowed icon upload extensions
	allowedIconExts = map[string]bool{
		".svg":  true,
		".png":  true,
		".jpg":  true,
		".jpeg": true,
		".gif":  true,
		".webp": true,
	}
)

// Dashboard icons types

type iconManifestResponse struct {
	Icons []string `json:"icons"`
	Total int      `json:"total"`
}

type iconDownloadRequest struct {
	Name   string `json:"name"`
	Format string `json:"format"`
}

func HandleIconSearch(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, http.MethodGet) {
		return
	}

	query := strings.ToLower(r.URL.Query().Get("q"))

	// Get or refresh icon manifest
	icons, err := getIconManifest()
	if err != nil {
		log.Printf("Failed to fetch icon manifest: %v", err)
		http.Error(w, "Failed to fetch icon list", http.StatusInternalServerError)
		return
	}

	// Filter icons by query
	var filtered []string
	for _, icon := range icons {
		if query == "" || strings.Contains(strings.ToLower(icon), query) {
			filtered = append(filtered, icon)
		}
	}

	WriteJSON(w, iconManifestResponse{
		Icons: filtered,
		Total: len(filtered),
	})
}

func getIconManifest() ([]string, error) {
	iconManifestMu.RLock()
	if len(iconManifest) > 0 && time.Since(iconManifestAge) < iconCacheTTL {
		defer iconManifestMu.RUnlock()
		return iconManifest, nil
	}
	iconManifestMu.RUnlock()

	// Fetch fresh manifest
	resp, err := http.Get(iconManifestURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to fetch manifest: HTTP %d", resp.StatusCode)
	}

	var treeData map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&treeData); err != nil {
		return nil, err
	}

	// Extract icon names from the tree.json structure
	// The structure has "svg" and "png" keys with arrays of filenames
	var icons []string
	iconSet := make(map[string]bool)

	if svgList, ok := treeData["svg"].([]interface{}); ok {
		for _, item := range svgList {
			if name, ok := item.(string); ok {
				// Remove .svg extension
				name = strings.TrimSuffix(name, ".svg")
				if !iconSet[name] {
					iconSet[name] = true
					icons = append(icons, name)
				}
			}
		}
	}

	// Cache the result
	iconManifestMu.Lock()
	iconManifest = icons
	iconManifestAge = time.Now()
	iconManifestMu.Unlock()

	return icons, nil
}

func HandleIconDownload(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, http.MethodPost) {
		return
	}

	var req iconDownloadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "Missing icon name", http.StatusBadRequest)
		return
	}

	// Default to svg format
	format := req.Format
	if format == "" {
		format = "svg"
	}
	if format != "svg" && format != "png" {
		http.Error(w, "Invalid format (must be svg or png)", http.StatusBadRequest)
		return
	}

	if err := EnsureDir(IconsDir); err != nil {
		http.Error(w, "Failed to create icons directory", http.StatusInternalServerError)
		return
	}

	// Check if icon already exists
	iconFileName := req.Name + "." + format
	iconPath := filepath.Join(IconsDir, iconFileName)
	if _, err := os.Stat(iconPath); err == nil {
		// Icon already exists
		WriteJSONStatusPath(w, "ok", "/icons/"+iconFileName)
		return
	}

	// Download icon from CDN
	iconURL := fmt.Sprintf("%s/%s/%s.%s", IconCDNBase, format, req.Name, format)
	if err := DownloadFile(iconURL, iconPath); err != nil {
		log.Printf("Failed to download icon '%s': %v", req.Name, err)
		http.Error(w, "Failed to download icon", http.StatusBadGateway)
		return
	}

	log.Printf("✓ Downloaded icon: %s", iconFileName)
	WriteJSONStatusPath(w, "ok", "/icons/"+iconFileName)
}

func HandleIconUpload(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, http.MethodPost) {
		return
	}

	// Limit upload size
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)

	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		http.Error(w, "File too large (max 5MB)", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("icon")
	if err != nil {
		http.Error(w, "Missing icon file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Validate file type by extension
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !allowedIconExts[ext] {
		http.Error(w, "Invalid file type (allowed: svg, png, jpg, jpeg, gif, webp)", http.StatusBadRequest)
		return
	}

	if err := EnsureDir(IconsDir); err != nil {
		http.Error(w, "Failed to create icons directory", http.StatusInternalServerError)
		return
	}

	// Sanitize filename
	safeName := SanitizeFilename(strings.TrimSuffix(filepath.Base(header.Filename), ext))
	if safeName == "" {
		safeName = "icon"
	}

	// Generate unique filename
	iconFileName, iconPath := UniqueFilename(IconsDir, safeName, ext)

	// Create and save the file
	dst, err := os.Create(iconPath)
	if err != nil {
		http.Error(w, "Failed to save icon", http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		http.Error(w, "Failed to save icon", http.StatusInternalServerError)
		return
	}

	log.Printf("✓ Uploaded icon: %s", iconFileName)
	WriteJSONStatusPath(w, "ok", "/icons/"+iconFileName)
}

// SanitizeFilename removes unsafe characters from a filename
func SanitizeFilename(name string) string {
	safe := regexp.MustCompile(`[^a-zA-Z0-9_-]`).ReplaceAllString(name, "-")
	safe = regexp.MustCompile(`-+`).ReplaceAllString(safe, "-")
	return strings.Trim(safe, "-")
}

// UniqueFilename generates a unique filename in the given directory
func UniqueFilename(dir, baseName, ext string) (string, string) {
	fileName := baseName + ext
	filePath := filepath.Join(dir, fileName)
	counter := 1
	for {
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			return fileName, filePath
		}
		fileName = fmt.Sprintf("%s-%d%s", baseName, counter, ext)
		filePath = filepath.Join(dir, fileName)
		counter++
	}
}
