package main

import (
	"embed"
	"encoding/json"
	"html/template"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"

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
	Services []Section `yaml:"services" json:"services"`
	Widgets  []Widget  `yaml:"widgets" json:"widgets"`
}

type Theme struct {
	PrimaryColor    string `yaml:"primary_color" json:"primary_color"`
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
	Icon        string `yaml:"icon" json:"icon"`                   // URL to icon image
	IconText    string `yaml:"icon_text" json:"icon_text"`         // Emoji or text fallback
	Description string `yaml:"description" json:"description"`
	Target      string `yaml:"target" json:"target"`               // _blank, _self, etc.
}

type Widget struct {
	Type   string            `yaml:"type" json:"type"`     // uptime-kuma, iframe, weather, etc.
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

	// Parse templates
	var err error
	templates, err = template.ParseFS(embeddedFS, "templates/*.html")
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

	// Get port from environment or default to 8080
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("🏠 Homelab Dashboard starting on http://localhost:%s", port)
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

	return yaml.Unmarshal(data, &config)
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
		Title:    "Homelab Dashboard",
		Subtitle: "Welcome home",
		Theme: Theme{
			PrimaryColor:    "#33C3F0",
			BackgroundColor: "#1a1a2e",
			CardColor:       "#16213e",
			TextColor:       "#eaeaea",
			FontFamily:      "system",
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

// downloadGoogleFont downloads a Google Font and saves it locally
func downloadGoogleFont(fontName string) error {
	if fontName == "" || isPredefinedFont(fontName) {
		return nil // No download needed for predefined fonts
	}

	// Create fonts directory if it doesn't exist
	fontsDir := "./fonts"
	if err := os.MkdirAll(fontsDir, 0755); err != nil {
		return err
	}

	// Check if font already exists
	fontPath := filepath.Join(fontsDir, fontName+".css")
	if _, err := os.Stat(fontPath); err == nil {
		log.Printf("Font '%s' already downloaded", fontName)
		return nil // Font already exists
	}

	// Fetch font CSS from Google Fonts API
	// Use multiple weights for better coverage
	url := "https://fonts.googleapis.com/css2?family=" + fontName + ":wght@300;400;500;600;700&display=swap"
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil // Font might not exist, skip silently
	}

	// Read CSS content
	cssBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// Save CSS file
	if err := os.WriteFile(fontPath, cssBytes, 0644); err != nil {
		return err
	}

	log.Printf("✓ Downloaded Google Font: %s", fontName)
	return nil
}

// isPredefinedFont checks if a font is one of the predefined web-safe fonts
func isPredefinedFont(fontName string) bool {
	predefinedFonts := map[string]bool{
		"system":         true,
		"Arial":          true,
		"Helvetica":      true,
		"Georgia":        true,
		"Times New Roman": true,
		"Courier New":    true,
		"Verdana":        true,
		"Trebuchet MS":   true,
		"Impact":         true,
	}
	return predefinedFonts[fontName]
}
