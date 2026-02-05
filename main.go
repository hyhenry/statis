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
	Icon        string `yaml:"icon" json:"icon"`                 // URL to icon image
	IconName    string `yaml:"icon_name" json:"icon_name"`       // Dashboard icon name (e.g., "opnsense", "portainer-dark") - auto-downloads on config load
	IconText    string `yaml:"icon_text" json:"icon_text"`       // Emoji or text fallback
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

	// Icon manifest cache
	iconManifest    []string
	iconManifestMu  sync.RWMutex
	iconManifestAge time.Time

	// Skip next fsnotify reload (to avoid infinite loop when we save after processing)
	skipNextReload bool
	skipReloadMu   sync.Mutex

	// Predefined web-safe fonts (no download needed)
	predefinedFonts = map[string]bool{
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
)

// Constants
const (
	iconsDir        = "./icons"
	fontsDir        = "./fonts"
	iconManifestURL = "https://cdn.jsdelivr.net/gh/homarr-labs/dashboard-icons@main/tree.json"
	iconCDNBase     = "https://cdn.jsdelivr.net/gh/homarr-labs/dashboard-icons"
	iconCacheTTL    = 24 * time.Hour
	maxUploadSize   = 5 << 20 // 5MB
)

// HTTP response helpers

func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func writeJSONStatus(w http.ResponseWriter, status string) {
	writeJSON(w, map[string]string{"status": status})
}

func writeJSONStatusPath(w http.ResponseWriter, status, path string) {
	writeJSON(w, map[string]string{"status": status, "path": path})
}

func requireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method != method {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return false
	}
	return true
}

// Directory helpers

func ensureDir(path string) error {
	return os.MkdirAll(path, 0755)
}

// Config parsing helpers

func parseConfig(data []byte) (Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	// Apply defaults
	if cfg.Theme.SecondaryColor == "" {
		cfg.Theme.SecondaryColor = cfg.Theme.PrimaryColor
	}
	return cfg, nil
}

// Template rendering helper

func renderTemplate(w http.ResponseWriter, name string) {
	configMu.RLock()
	defer configMu.RUnlock()

	if err := templates.ExecuteTemplate(w, name, config); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// File download helper

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

// Font helpers

func isPredefinedFont(fontName string) bool {
	return predefinedFonts[fontName]
}

func normalizeFont(fontName string) string {
	return strings.ReplaceAll(fontName, " ", "-")
}

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

	// Icons directory (for downloaded dashboard icons)
	mux.Handle("/icons/", http.StripPrefix("/icons/", http.FileServer(http.Dir("./icons"))))

	// Pages
	mux.HandleFunc("/", handleIndex)
	mux.HandleFunc("/settings", handleSettings)

	// API endpoints
	mux.HandleFunc("/api/config", handleAPIConfig)
	mux.HandleFunc("/api/widget/uptime-kuma", handleUptimeKumaProxy)
	mux.HandleFunc("/api/widget/system-stats", handleSystemStats)
	mux.HandleFunc("/api/widget/rss", handleRSSWidget)
	mux.HandleFunc("/api/assets/clean-unused", handleCleanUnusedAssets)
	mux.HandleFunc("/api/icons/search", handleIconSearch)
	mux.HandleFunc("/api/icons/download", handleIconDownload)
	mux.HandleFunc("/api/icons/upload", handleIconUpload)

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

	cfg, err := parseConfig(data)
	if err != nil {
		return err
	}
	config = cfg

	// Process icon_name fields and download icons if needed
	if processIconNames(&config) {
		// Save updated config (with populated icon paths) - must release lock first
		configMu.Unlock()
		if err := saveConfigQuiet(); err != nil {
			log.Printf("Warning: Failed to save config after processing icons: %v", err)
		}
		configMu.Lock()
	}

	return nil
}

func saveConfig() error {
	configMu.Lock()
	defer configMu.Unlock()

	// Ensure directory exists
	if err := ensureDir(filepath.Dir(configPath)); err != nil {
		return err
	}

	// Download Google Font if needed
	if config.Theme.FontFamily != "" {
		if err := downloadGoogleFont(config.Theme.FontFamily); err != nil {
			log.Printf("Warning: Failed to download font '%s': %v", config.Theme.FontFamily, err)
		}
	}

	// Process icon_name fields and download icons before saving
	processIconNames(&config)

	data, err := yaml.Marshal(&config)
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}

// saveConfigQuiet saves the config without re-processing icons (used after processIconNames already ran)
// Sets skip flag to prevent fsnotify from triggering another reload
func saveConfigQuiet() error {
	configMu.Lock()
	defer configMu.Unlock()

	// Set skip flag before writing
	skipReloadMu.Lock()
	skipNextReload = true
	skipReloadMu.Unlock()

	data, err := yaml.Marshal(&config)
	if err != nil {
		return err
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return err
	}

	log.Printf("✓ Config saved with updated icon paths")
	return nil
}

func reloadConfig() error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	newConfig, err := parseConfig(data)
	if err != nil {
		return err
	}

	// Download Google Font if needed
	if newConfig.Theme.FontFamily != "" {
		if err := downloadGoogleFont(newConfig.Theme.FontFamily); err != nil {
			log.Printf("Warning: Failed to download font '%s': %v", newConfig.Theme.FontFamily, err)
		}
	}

	// Process icon_name fields and download icons if needed
	iconsChanged := processIconNames(&newConfig)

	// Apply the new config
	configMu.Lock()
	config = newConfig
	configMu.Unlock()

	log.Printf("✓ Config reloaded from %s", configPath)

	// If icons were processed, save the updated config (with populated icon paths)
	if iconsChanged {
		if err := saveConfigQuiet(); err != nil {
			log.Printf("Warning: Failed to save config after processing icons: %v", err)
		}
	}

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
				// Check if we should skip this reload (triggered by our own save)
				skipReloadMu.Lock()
				shouldSkip := skipNextReload
				skipNextReload = false
				skipReloadMu.Unlock()

				if shouldSkip {
					log.Printf("📝 Config file changed (by us), skipping reload")
					continue
				}

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
	renderTemplate(w, "index.html")
}

func handleSettings(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "settings.html")
}

func handleAPIConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		configMu.RLock()
		defer configMu.RUnlock()
		writeJSON(w, config)

	case http.MethodPut:
		var newConfig Config
		if err := json.NewDecoder(r.Body).Decode(&newConfig); err != nil {
			log.Printf("❌ Failed to decode config JSON: %v", err)
			http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
			return
		}

		configMu.Lock()
		config = newConfig
		configMu.Unlock()

		if err := saveConfig(); err != nil {
			log.Printf("❌ Failed to save config: %v", err)
			http.Error(w, "Failed to save: "+err.Error(), http.StatusInternalServerError)
			return
		}

		log.Printf("✓ Config saved successfully")
		writeJSONStatus(w, "ok")

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

	writeJSON(w, statusData)
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

	writeJSON(w, systemStatsResponse{
		CPU:    cpuStats{UsagePercent: cpuUsage},
		Memory: memoryStats{TotalBytes: memTotal, AvailableBytes: memAvailable, UsedBytes: used, UsedPercent: usedPercent},
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
	Title          string         `xml:"title"`
	Link           string         `xml:"link"`
	Description    string         `xml:"description"`
	PubDate        string         `xml:"pubDate"`
	Enclosure      rssEnclosure   `xml:"enclosure"`
	MediaContent   []mediaContent `xml:"http://search.yahoo.com/mrss/ content"`
	MediaThumbnail []mediaContent `xml:"http://search.yahoo.com/mrss/ thumbnail"`
}

type rssEnclosure struct {
	URL  string `xml:"url,attr"`
	Type string `xml:"type,attr"`
}

type mediaContent struct {
	URL  string `xml:"url,attr"`
	Type string `xml:"type,attr"`
}

// Atom feed types for parsing
type atomFeed struct {
	Entries []atomEntry `xml:"entry"`
}

type atomEntry struct {
	Title   string     `xml:"title"`
	Links   []atomLink `xml:"link"`
	Summary string     `xml:"summary"`
	Content string     `xml:"content"`
	Updated string     `xml:"updated"`
}

type atomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
	Type string `xml:"type,attr"`
}

type rssResponse struct {
	Items []rssResponseItem `json:"items"`
}

type rssResponseItem struct {
	Title       string `json:"title"`
	Link        string `json:"link"`
	Description string `json:"description"`
	PubDate     string `json:"pub_date"`
	Image       string `json:"image"`
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

	items, err := parseFeed(body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, rssResponse{Items: items})
}

// parseFeed parses RSS 2.0 or Atom feed data
func parseFeed(body []byte) ([]rssResponseItem, error) {
	// Try RSS 2.0 first
	var rss rssFeed
	if err := xml.Unmarshal(body, &rss); err == nil && len(rss.Channel.Items) > 0 {
		return parseRSSItems(rss.Channel.Items), nil
	}

	// Try Atom
	var atom atomFeed
	if err := xml.Unmarshal(body, &atom); err == nil && len(atom.Entries) > 0 {
		return parseAtomEntries(atom.Entries), nil
	}

	return nil, fmt.Errorf("failed to parse feed (not valid RSS or Atom)")
}

func parseRSSItems(items []rssItem) []rssResponseItem {
	result := make([]rssResponseItem, 0, len(items))
	for _, item := range items {
		result = append(result, rssResponseItem{
			Title:       item.Title,
			Link:        item.Link,
			Description: stripHTMLTags(item.Description),
			PubDate:     item.PubDate,
			Image:       extractRSSImage(item),
		})
	}
	return result
}

func parseAtomEntries(entries []atomEntry) []rssResponseItem {
	result := make([]rssResponseItem, 0, len(entries))
	for _, entry := range entries {
		desc := entry.Summary
		if desc == "" {
			desc = entry.Content
		}
		imageURL := pickAtomImage(entry.Links)
		if imageURL == "" {
			imageURL = extractFirstImageURL(desc)
		}
		result = append(result, rssResponseItem{
			Title:       entry.Title,
			Link:        pickAtomLink(entry.Links),
			Description: stripHTMLTags(desc),
			PubDate:     entry.Updated,
			Image:       imageURL,
		})
	}
	return result
}

// extractRSSImage extracts image URL from RSS item using multiple sources
func extractRSSImage(item rssItem) string {
	if item.Enclosure.URL != "" && strings.HasPrefix(item.Enclosure.Type, "image/") {
		return item.Enclosure.URL
	}
	if url := findFirstMediaURL(item.MediaContent); url != "" {
		return url
	}
	if url := findFirstMediaURL(item.MediaThumbnail); url != "" {
		return url
	}
	return extractFirstImageURL(item.Description)
}

// findFirstMediaURL returns the first non-empty URL from media content slice
func findFirstMediaURL(media []mediaContent) string {
	for _, m := range media {
		if m.URL != "" {
			return m.URL
		}
	}
	return ""
}

func pickAtomLink(links []atomLink) string {
	for _, link := range links {
		if link.Href == "" {
			continue
		}
		if link.Rel == "" || link.Rel == "alternate" {
			return link.Href
		}
	}
	if len(links) > 0 {
		return links[0].Href
	}
	return ""
}

func pickAtomImage(links []atomLink) string {
	for _, link := range links {
		if link.Href == "" {
			continue
		}
		if link.Rel == "enclosure" && strings.HasPrefix(link.Type, "image/") {
			return link.Href
		}
	}
	return ""
}

func extractFirstImageURL(s string) string {
	re := regexp.MustCompile(`(?i)<img[^>]+src=["']([^"']+)["']`)
	matches := re.FindStringSubmatch(s)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// stripHTMLTags removes HTML tags from a string
func stripHTMLTags(s string) string {
	re := regexp.MustCompile(`<[^>]*>`)
	return re.ReplaceAllString(s, "")
}

type cleanupResult struct {
	FontsRemoved int      `json:"fonts_removed"`
	IconsRemoved int      `json:"icons_removed"`
	FilesRemoved []string `json:"files_removed"`
}

func handleCleanUnusedAssets(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodDelete) {
		return
	}

	configMu.RLock()
	currentFont := config.Theme.FontFamily
	usedIcons := getUsedIcons()
	configMu.RUnlock()

	result := cleanupResult{
		FilesRemoved: []string{},
	}

	// Clean unused fonts
	if entries, err := os.ReadDir(fontsDir); err == nil {
		normalizedFont := normalizeFont(currentFont)
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			// Keep files that start with the current font name (e.g., "Inter.css", "Inter_xxx.woff2")
			if currentFont != "" && !isPredefinedFont(currentFont) && strings.HasPrefix(name, normalizedFont) {
				continue
			}
			// Remove unused font file
			filePath := filepath.Join(fontsDir, name)
			if err := os.Remove(filePath); err == nil {
				result.FontsRemoved++
				result.FilesRemoved = append(result.FilesRemoved, "fonts/"+name)
				log.Printf("Removed unused font: %s", name)
			}
		}
	}

	// Clean unused icons
	if entries, err := os.ReadDir(iconsDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			iconPath := "/icons/" + name
			if !usedIcons[iconPath] {
				// Remove unused icon file
				filePath := filepath.Join(iconsDir, name)
				if err := os.Remove(filePath); err == nil {
					result.IconsRemoved++
					result.FilesRemoved = append(result.FilesRemoved, "icons/"+name)
					log.Printf("Removed unused icon: %s", name)
				}
			}
		}
	}

	log.Printf("✓ Cleaned up %d fonts and %d icons", result.FontsRemoved, result.IconsRemoved)
	writeJSON(w, result)
}

// getUsedIcons returns a set of icon paths that are currently referenced in the config
func getUsedIcons() map[string]bool {
	used := make(map[string]bool)
	for _, section := range config.Services {
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
	return used
}

// Dashboard icons types

type iconManifestResponse struct {
	Icons []string `json:"icons"`
	Total int      `json:"total"`
}

type iconDownloadRequest struct {
	Name   string `json:"name"`
	Format string `json:"format"`
}

func handleIconSearch(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
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

	writeJSON(w, iconManifestResponse{
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

func handleIconDownload(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
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

	if err := ensureDir(iconsDir); err != nil {
		http.Error(w, "Failed to create icons directory", http.StatusInternalServerError)
		return
	}

	// Check if icon already exists
	iconFileName := req.Name + "." + format
	iconPath := filepath.Join(iconsDir, iconFileName)
	if _, err := os.Stat(iconPath); err == nil {
		// Icon already exists
		writeJSONStatusPath(w, "ok", "/icons/"+iconFileName)
		return
	}

	// Download icon from CDN
	iconURL := fmt.Sprintf("%s/%s/%s.%s", iconCDNBase, format, req.Name, format)
	if err := downloadFile(iconURL, iconPath); err != nil {
		log.Printf("Failed to download icon '%s': %v", req.Name, err)
		http.Error(w, "Failed to download icon", http.StatusBadGateway)
		return
	}

	log.Printf("✓ Downloaded icon: %s", iconFileName)
	writeJSONStatusPath(w, "ok", "/icons/"+iconFileName)
}

// Allowed icon upload extensions
var allowedIconExts = map[string]bool{
	".svg":  true,
	".png":  true,
	".jpg":  true,
	".jpeg": true,
	".gif":  true,
	".webp": true,
}

func handleIconUpload(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
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

	if err := ensureDir(iconsDir); err != nil {
		http.Error(w, "Failed to create icons directory", http.StatusInternalServerError)
		return
	}

	// Sanitize filename
	safeName := sanitizeFilename(strings.TrimSuffix(filepath.Base(header.Filename), ext))
	if safeName == "" {
		safeName = "icon"
	}

	// Generate unique filename
	iconFileName, iconPath := uniqueFilename(iconsDir, safeName, ext)

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
	writeJSONStatusPath(w, "ok", "/icons/"+iconFileName)
}

// sanitizeFilename removes unsafe characters from a filename
func sanitizeFilename(name string) string {
	safe := regexp.MustCompile(`[^a-zA-Z0-9_-]`).ReplaceAllString(name, "-")
	safe = regexp.MustCompile(`-+`).ReplaceAllString(safe, "-")
	return strings.Trim(safe, "-")
}

// uniqueFilename generates a unique filename in the given directory
func uniqueFilename(dir, baseName, ext string) (string, string) {
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

// processIconNames downloads icons for any items that have icon_name set.
// Returns true if any config changes were made (icon paths updated).
func processIconNames(cfg *Config) bool {
	if err := ensureDir(iconsDir); err != nil {
		log.Printf("Warning: Failed to create icons directory: %v", err)
		return false
	}

	changed := false
	for i := range cfg.Services {
		for j := range cfg.Services[i].Items {
			item := &cfg.Services[i].Items[j]
			if item.IconName == "" {
				continue
			}

			iconFileName := item.IconName + ".svg"
			iconPath := filepath.Join(iconsDir, iconFileName)
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
			iconURL := fmt.Sprintf("%s/svg/%s.svg", iconCDNBase, item.IconName)
			if err := downloadFile(iconURL, iconPath); err != nil {
				log.Printf("Warning: Failed to download icon '%s': %v", item.IconName, err)
				continue
			}

			item.Icon = localURL
			changed = true
			log.Printf("✓ Downloaded icon for '%s': %s", item.Name, item.IconName)
		}
	}
	return changed
}

// downloadGoogleFont downloads a Google Font and saves it locally with offline font files
func downloadGoogleFont(fontName string) error {
	if fontName == "" || isPredefinedFont(fontName) {
		return nil
	}

	if err := ensureDir(fontsDir); err != nil {
		return err
	}

	normalizedName := normalizeFont(fontName)
	fontPath := filepath.Join(fontsDir, normalizedName+".css")

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
	modifiedCSS, err := downloadFontFilesAndUpdateCSS(cssContent, normalizedName, fontsDir)
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
