package main

import (
	"embed"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v3"
)

//go:embed static templates
var embeddedFS embed.FS

// Config represents the main configuration structure
type Config struct {
	Title    string    `yaml:"title" json:"title"`
	Subtitle string    `yaml:"subtitle" json:"subtitle"`
	Theme    Theme     `yaml:"theme" json:"theme"`
	Layout   Layout    `yaml:"layout" json:"layout"`
	Services []Section `yaml:"services" json:"services"`
	Widgets  []Widget  `yaml:"widgets" json:"widgets"`
}

type Layout struct {
	WidgetColumns  int `yaml:"widget_columns" json:"widget_columns"`
	ServiceColumns int `yaml:"service_columns" json:"service_columns"`
	CardsPerRow    int `yaml:"cards_per_row" json:"cards_per_row"`
}

type Theme struct {
	PrimaryColor    string `yaml:"primary_color" json:"primary_color"`
	SecondaryColor  string `yaml:"secondary_color" json:"secondary_color"`
	BackgroundColor string `yaml:"background_color" json:"background_color"`
	CardColor       string `yaml:"card_color" json:"card_color"`
	TextColor       string `yaml:"text_color" json:"text_color"`
	FontFamily      string `yaml:"font_family" json:"font_family"`
}

type Section struct {
	Name  string `yaml:"name" json:"name"`
	Items []Item `yaml:"items" json:"items"`
}

type Item struct {
	Name        string `yaml:"name" json:"name"`
	URL         string `yaml:"url" json:"url"`
	Icon        string `yaml:"icon" json:"icon"`           // URL to icon image
	IconText    string `yaml:"icon_text" json:"icon_text"` // Emoji or text fallback
	Description string `yaml:"description" json:"description"`
	Target      string `yaml:"target" json:"target"` // _blank, _self, etc.
}

type Widget struct {
	Type   string            `yaml:"type" json:"type"` // uptime-kuma, iframe, weather, etc.
	Title  string            `yaml:"title" json:"title"`
	Config map[string]string `yaml:"config" json:"config"` // Widget-specific config
}

var (
	config     Config
	configPath string
	configMu   sync.RWMutex
	templates  *template.Template
)

func main() {
	// Determine config path
	configPath = os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "./config.yaml"
	}

	// Load configuration
	if err := loadConfig(); err != nil {
		log.Printf("Warning: Could not load config from %s: %v", configPath, err)
		log.Println("Using default configuration")
		config = getDefaultConfig()
		// Save default config
		saveConfig()
	}

	// Parse templates with custom functions
	funcMap := template.FuncMap{
		"normalizeFont": func(fontName string) string {
			return strings.ReplaceAll(fontName, " ", "-")
		},
		"colName": func(n int) string {
			names := []string{"", "one", "two", "three", "four", "five", "six", "seven", "eight", "nine", "ten", "eleven", "twelve"}
			if n >= 1 && n <= 12 {
				return names[n]
			}
			return "four" // default
		},
	}
	var err error
	templates, err = template.New("").Funcs(funcMap).ParseFS(embeddedFS, "templates/*.html")
	if err != nil {
		log.Fatalf("Failed to parse templates: %v", err)
	}

	// Start file watcher in background
	go watchConfigFile()

	// Setup routes
	mux := http.NewServeMux()

	// Static files
	staticFS, _ := fs.Sub(embeddedFS, "static")
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	// Fonts directory (for downloaded Google Fonts)
	mux.Handle("/fonts/", http.StripPrefix("/fonts/", http.FileServer(http.Dir("./fonts"))))

	// Pages
	mux.HandleFunc("/", handleIndex)
	mux.HandleFunc("/settings", handleSettings)

	// API endpoints
	mux.HandleFunc("/api/config", handleAPIConfig)
	mux.HandleFunc("/api/widget/uptime-kuma", handleUptimeKumaProxy)
	mux.HandleFunc("/api/widget/system-stats", handleSystemStats)
	mux.HandleFunc("/api/widget/rss", handleRSSWidget)
	mux.HandleFunc("/api/fonts/clear", handleClearFonts)

	// Get port from environment or default to 8080
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("🏠 Statis starting on http://localhost:%s", port)
	log.Printf("📁 Config file: %s", configPath)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}

func loadConfig() error {
	configMu.Lock()
	defer configMu.Unlock()

	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	if err := yaml.Unmarshal(data, &config); err != nil {
		return err
	}

	if config.Theme.SecondaryColor == "" {
		config.Theme.SecondaryColor = config.Theme.PrimaryColor
	}

	return nil
}

func saveConfig() error {
	configMu.Lock()
	defer configMu.Unlock()

	// Ensure directory exists
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Download Google Font if needed
	if config.Theme.FontFamily != "" {
		if err := downloadGoogleFont(config.Theme.FontFamily); err != nil {
			log.Printf("Warning: Failed to download font '%s': %v", config.Theme.FontFamily, err)
		}
	}

	data, err := yaml.Marshal(&config)
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}

func reloadConfig() error {
	// Read the file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	// Unmarshal into temporary config for validation
	var newConfig Config
	if err := yaml.Unmarshal(data, &newConfig); err != nil {
		return err
	}

	if newConfig.Theme.SecondaryColor == "" {
		newConfig.Theme.SecondaryColor = newConfig.Theme.PrimaryColor
	}

	// Download Google Font if needed
	if newConfig.Theme.FontFamily != "" {
		if err := downloadGoogleFont(newConfig.Theme.FontFamily); err != nil {
			log.Printf("Warning: Failed to download font '%s': %v", newConfig.Theme.FontFamily, err)
		}
	}

	// Apply the new config
	configMu.Lock()
	config = newConfig
	configMu.Unlock()

	log.Printf("✓ Config reloaded from %s", configPath)
	return nil
}

func watchConfigFile() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("Failed to create file watcher: %v", err)
		return
	}
	defer watcher.Close()

	// Add config file to watcher
	err = watcher.Add(configPath)
	if err != nil {
		log.Printf("Failed to watch config file: %v", err)
		return
	}

	log.Printf("👀 Watching %s for changes", configPath)

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			// Handle write and create events (some editors use rename/create on save)
			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
				log.Printf("📝 Config file changed, reloading...")
				if err := reloadConfig(); err != nil {
					log.Printf("❌ Failed to reload config: %v", err)
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Printf("File watcher error: %v", err)
		}
	}
}

func getDefaultConfig() Config {
	return Config{
		Title:    "Statis",
		Subtitle: "Welcome home",
		Theme: Theme{
			PrimaryColor:    "#33C3F0",
			SecondaryColor:  "#33C3F0",
			BackgroundColor: "#1a1a2e",
			CardColor:       "#16213e",
			TextColor:       "#eaeaea",
			FontFamily:      "system",
		},
		Layout: Layout{
			WidgetColumns:  4,
			ServiceColumns: 8,
			CardsPerRow:    3,
		},
		Services: []Section{
			{
				Name: "Infrastructure",
				Items: []Item{
					{Name: "Proxmox", URL: "https://proxmox.local:8006", IconText: "🖥️", Description: "Hypervisor"},
					{Name: "TrueNAS", URL: "https://truenas.local", IconText: "💾", Description: "Storage"},
					{Name: "Portainer", URL: "https://portainer.local:9443", IconText: "🐳", Description: "Containers"},
				},
			},
			{
				Name: "Media",
				Items: []Item{
					{Name: "Plex", URL: "https://plex.local:32400", IconText: "🎬", Description: "Media Server"},
					{Name: "Jellyfin", URL: "https://jellyfin.local:8096", IconText: "📺", Description: "Media Server"},
					{Name: "Sonarr", URL: "https://sonarr.local:8989", IconText: "📡", Description: "TV Shows"},
				},
			},
			{
				Name: "Monitoring",
				Items: []Item{
					{Name: "Uptime Kuma", URL: "https://uptime.local:3001", IconText: "📊", Description: "Status Page"},
					{Name: "Grafana", URL: "https://grafana.local:3000", IconText: "📈", Description: "Dashboards"},
					{Name: "Prometheus", URL: "https://prometheus.local:9090", IconText: "🔥", Description: "Metrics"},
				},
			},
		},
		Widgets: []Widget{
			{
				Type:  "uptime-kuma",
				Title: "Service Status",
				Config: map[string]string{
					"url":  "https://uptime.local:3001",
					"slug": "servers",
				},
			},
			{
				Type:  "uptime-kuma",
				Title: "Service Status",
				Config: map[string]string{
					"url":  "https://uptime.local:3001",
					"slug": "services",
				},
			},
		},
	}
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	configMu.RLock()
	defer configMu.RUnlock()

	if err := templates.ExecuteTemplate(w, "index.html", config); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func handleSettings(w http.ResponseWriter, r *http.Request) {
	configMu.RLock()
	defer configMu.RUnlock()

	if err := templates.ExecuteTemplate(w, "settings.html", config); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func handleAPIConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		configMu.RLock()
		defer configMu.RUnlock()
		json.NewEncoder(w).Encode(config)

	case http.MethodPut:
		var newConfig Config
		if err := json.NewDecoder(r.Body).Decode(&newConfig); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		configMu.Lock()
		config = newConfig
		configMu.Unlock()

		if err := saveConfig(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleUptimeKumaProxy(w http.ResponseWriter, r *http.Request) {
	// This proxies requests to Uptime Kuma's status page API
	// Fetches both monitor list and heartbeat data, then merges them

	kumaURL := r.URL.Query().Get("url")
	slug := r.URL.Query().Get("slug")

	if kumaURL == "" || slug == "" {
		http.Error(w, "Missing url or slug parameter", http.StatusBadRequest)
		return
	}

	// Fetch monitor list (names, groups, etc.)
	statusURL := kumaURL + "/api/status-page/" + slug
	statusResp, err := http.Get(statusURL)
	if err != nil {
		http.Error(w, "Failed to fetch status page", http.StatusBadGateway)
		return
	}
	defer statusResp.Body.Close()

	var statusData map[string]interface{}
	if err := json.NewDecoder(statusResp.Body).Decode(&statusData); err != nil {
		http.Error(w, "Failed to decode status page", http.StatusInternalServerError)
		return
	}

	// Fetch heartbeat data
	heartbeatURL := kumaURL + "/api/status-page/heartbeat/" + slug
	heartbeatResp, err := http.Get(heartbeatURL)
	if err != nil {
		http.Error(w, "Failed to fetch heartbeat data", http.StatusBadGateway)
		return
	}
	defer heartbeatResp.Body.Close()

	var heartbeatData map[string]interface{}
	if err := json.NewDecoder(heartbeatResp.Body).Decode(&heartbeatData); err != nil {
		http.Error(w, "Failed to decode heartbeat data", http.StatusInternalServerError)
		return
	}

	// Merge heartbeat data into status data
	statusData["heartbeatList"] = heartbeatData["heartbeatList"]
	statusData["uptimeList"] = heartbeatData["uptimeList"]

	// Return merged data
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(statusData)
}

type cpuTimes struct {
	Total uint64
	Idle  uint64
}

type systemStatsResponse struct {
	CPU    cpuStats    `json:"cpu"`
	Memory memoryStats `json:"memory"`
}

type cpuStats struct {
	UsagePercent float64 `json:"usage_percent"`
}

type memoryStats struct {
	TotalBytes     uint64  `json:"total_bytes"`
	AvailableBytes uint64  `json:"available_bytes"`
	UsedBytes      uint64  `json:"used_bytes"`
	UsedPercent    float64 `json:"used_percent"`
}

func handleSystemStats(w http.ResponseWriter, r *http.Request) {
	if runtime.GOOS != "linux" {
		http.Error(w, "system stats supported on linux only", http.StatusNotImplemented)
		return
	}

	cpuUsage, err := getCPUUsagePercent()
	if err != nil {
		http.Error(w, "failed to read cpu usage", http.StatusInternalServerError)
		return
	}

	memTotal, memAvailable, err := getMemoryStats()
	if err != nil {
		http.Error(w, "failed to read memory stats", http.StatusInternalServerError)
		return
	}

	used := memTotal - memAvailable
	usedPercent := 0.0
	if memTotal > 0 {
		usedPercent = (float64(used) / float64(memTotal)) * 100
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(systemStatsResponse{
		CPU: cpuStats{
			UsagePercent: cpuUsage,
		},
		Memory: memoryStats{
			TotalBytes:     memTotal,
			AvailableBytes: memAvailable,
			UsedBytes:      used,
			UsedPercent:    usedPercent,
		},
	})
}

func getCPUUsagePercent() (float64, error) {
	first, err := readCPUTimes()
	if err != nil {
		return 0, err
	}

	time.Sleep(200 * time.Millisecond)

	second, err := readCPUTimes()
	if err != nil {
		return 0, err
	}

	deltaTotal := second.Total - first.Total
	deltaIdle := second.Idle - first.Idle
	if deltaTotal == 0 {
		return 0, nil
	}

	usage := (float64(deltaTotal-deltaIdle) / float64(deltaTotal)) * 100
	return usage, nil
}

func readCPUTimes() (cpuTimes, error) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return cpuTimes{}, err
	}

	lines := strings.Split(string(data), "\n")
	if len(lines) == 0 {
		return cpuTimes{}, fmt.Errorf("empty /proc/stat")
	}

	fields := strings.Fields(lines[0])
	if len(fields) < 5 {
		return cpuTimes{}, fmt.Errorf("unexpected /proc/stat format")
	}

	var total uint64
	var idle uint64
	for i := 1; i < len(fields); i++ {
		value, err := strconv.ParseUint(fields[i], 10, 64)
		if err != nil {
			return cpuTimes{}, err
		}
		total += value
		if i == 4 || i == 5 { // idle + iowait
			idle += value
		}
	}

	return cpuTimes{Total: total, Idle: idle}, nil
}

func getMemoryStats() (uint64, uint64, error) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, 0, err
	}

	var totalKB uint64
	var availableKB uint64
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		switch fields[0] {
		case "MemTotal:":
			value, err := strconv.ParseUint(fields[1], 10, 64)
			if err != nil {
				return 0, 0, err
			}
			totalKB = value
		case "MemAvailable:":
			value, err := strconv.ParseUint(fields[1], 10, 64)
			if err != nil {
				return 0, 0, err
			}
			availableKB = value
		}
	}

	if totalKB == 0 {
		return 0, 0, fmt.Errorf("MemTotal not found")
	}
	if availableKB == 0 {
		return 0, 0, fmt.Errorf("MemAvailable not found")
	}

	return totalKB * 1024, availableKB * 1024, nil
}

// RSS feed types for parsing
type rssFeed struct {
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Items []rssItem `xml:"item"`
}

type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
}

// Atom feed types for parsing
type atomFeed struct {
	Entries []atomEntry `xml:"entry"`
}

type atomEntry struct {
	Title   string    `xml:"title"`
	Link    atomLink  `xml:"link"`
	Summary string    `xml:"summary"`
	Content string    `xml:"content"`
	Updated string    `xml:"updated"`
}

type atomLink struct {
	Href string `xml:"href,attr"`
}

type rssResponse struct {
	Items []rssResponseItem `json:"items"`
}

type rssResponseItem struct {
	Title       string `json:"title"`
	Link        string `json:"link"`
	Description string `json:"description"`
	PubDate     string `json:"pub_date"`
}

func handleRSSWidget(w http.ResponseWriter, r *http.Request) {
	feedURL := r.URL.Query().Get("url")
	if feedURL == "" {
		http.Error(w, "Missing url parameter", http.StatusBadRequest)
		return
	}

	// Fetch the RSS feed
	resp, err := http.Get(feedURL)
	if err != nil {
		http.Error(w, "Failed to fetch RSS feed", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		http.Error(w, "RSS feed returned error", http.StatusBadGateway)
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "Failed to read RSS feed", http.StatusInternalServerError)
		return
	}

	var items []rssResponseItem

	// Try parsing as RSS 2.0 first
	var rss rssFeed
	if err := xml.Unmarshal(body, &rss); err == nil && len(rss.Channel.Items) > 0 {
		for _, item := range rss.Channel.Items {
			items = append(items, rssResponseItem{
				Title:       item.Title,
				Link:        item.Link,
				Description: stripHTMLTags(item.Description),
				PubDate:     item.PubDate,
			})
		}
	} else {
		// Try parsing as Atom
		var atom atomFeed
		if err := xml.Unmarshal(body, &atom); err == nil && len(atom.Entries) > 0 {
			for _, entry := range atom.Entries {
				desc := entry.Summary
				if desc == "" {
					desc = entry.Content
				}
				items = append(items, rssResponseItem{
					Title:       entry.Title,
					Link:        entry.Link.Href,
					Description: stripHTMLTags(desc),
					PubDate:     entry.Updated,
				})
			}
		} else {
			http.Error(w, "Failed to parse feed (not valid RSS or Atom)", http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rssResponse{Items: items})
}

// stripHTMLTags removes HTML tags from a string
func stripHTMLTags(s string) string {
	re := regexp.MustCompile(`<[^>]*>`)
	return re.ReplaceAllString(s, "")
}

func handleClearFonts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Delete the fonts directory
	fontsDir := "./fonts"
	if err := os.RemoveAll(fontsDir); err != nil {
		log.Printf("Failed to clear fonts: %v", err)
		http.Error(w, "Failed to clear fonts", http.StatusInternalServerError)
		return
	}

	log.Printf("✓ All custom fonts cleared")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// downloadGoogleFont downloads a Google Font and saves it locally with offline font files
func downloadGoogleFont(fontName string) error {
	if fontName == "" || isPredefinedFont(fontName) {
		return nil // No download needed for predefined fonts
	}

	// Create fonts directory if it doesn't exist
	fontsDir := "./fonts"
	if err := os.MkdirAll(fontsDir, 0755); err != nil {
		return err
	}

	// Normalize the filename (replace spaces with hyphens for valid URLs)
	normalizedName := strings.ReplaceAll(fontName, " ", "-")

	// Check if font already exists
	fontPath := filepath.Join(fontsDir, normalizedName+".css")
	if _, err := os.Stat(fontPath); err == nil {
		log.Printf("Font '%s' already downloaded", fontName)
		return nil // Font already exists
	}

	// Fetch font CSS from Google Fonts API
	// Use multiple weights for better coverage
	// Important: Use a User-Agent header to get woff2 format URLs
	// URL-encode the font name to handle spaces and special characters
	encodedFontName := url.QueryEscape(fontName)
	fontURL := "https://fonts.googleapis.com/css2?family=" + encodedFontName + ":wght@300;400;500;600;700&display=swap"
	req, err := http.NewRequest("GET", fontURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("font not found: HTTP %d (check font name spelling)", resp.StatusCode)
	}

	// Read CSS content
	cssBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	cssContent := string(cssBytes)

	// Download all font files referenced in the CSS and replace URLs
	modifiedCSS, err := downloadFontFilesAndUpdateCSS(cssContent, normalizedName, fontsDir)
	if err != nil {
		log.Printf("Warning: Failed to download font files for '%s': %v", fontName, err)
		// Still save the original CSS as fallback
		modifiedCSS = cssContent
	}

	// Save modified CSS file
	if err := os.WriteFile(fontPath, []byte(modifiedCSS), 0644); err != nil {
		return err
	}

	log.Printf("✓ Downloaded Google Font: %s", fontName)
	return nil
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
			if err := downloadFile(fontFileURL, fontFilePath); err != nil {
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

// downloadFile downloads a file from a URL and saves it locally
func downloadFile(url, filepath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

// isPredefinedFont checks if a font is one of the predefined web-safe fonts
func isPredefinedFont(fontName string) bool {
	predefinedFonts := map[string]bool{
		"system":          true,
		"Arial":           true,
		"Helvetica":       true,
		"Georgia":         true,
		"Times New Roman": true,
		"Courier New":     true,
		"Verdana":         true,
		"Trebuchet MS":    true,
		"Impact":          true,
	}
	return predefinedFonts[fontName]
}
