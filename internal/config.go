package internal

import (
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v3"
)

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
	Favicon         string `yaml:"favicon" json:"favicon"`           // Path to favicon image (auto-populated if favicon_name is set)
	FaviconName     string `yaml:"favicon_name" json:"favicon_name"` // Dashboard icon name for favicon - auto-downloads on config load
}

type Section struct {
	Name  string `yaml:"name" json:"name"`
	Items []Item `yaml:"items" json:"items"`
}

type Item struct {
	Name        string `yaml:"name" json:"name"`
	URL         string `yaml:"url" json:"url"`
	Icon        string `yaml:"icon" json:"icon"`           // URL to icon image
	IconName    string `yaml:"icon_name" json:"icon_name"` // Dashboard icon name (e.g., "opnsense", "portainer-dark") - auto-downloads on config load
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
	AppConfig  Config
	ConfigPath string
	ConfigMu   sync.RWMutex

	// Skip next fsnotify reload (to avoid infinite loop when we save after processing)
	skipNextReload bool
	skipReloadMu   sync.Mutex

	// PredefinedFonts - web-safe fonts (no download needed)
	PredefinedFonts = map[string]bool{
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

// ParseConfig parses YAML config data into a Config struct
func ParseConfig(data []byte) (Config, error) {
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

func LoadConfig() error {
	ConfigMu.Lock()
	defer ConfigMu.Unlock()

	data, err := os.ReadFile(ConfigPath)
	if err != nil {
		return err
	}

	cfg, err := ParseConfig(data)
	if err != nil {
		return err
	}
	AppConfig = cfg

	// Process icon_name fields and download icons if needed
	if ProcessIconNames(&AppConfig) {
		// Save updated config (with populated icon paths) - must release lock first
		ConfigMu.Unlock()
		if err := saveConfigQuiet(); err != nil {
			log.Printf("Warning: Failed to save config after processing icons: %v", err)
		}
		ConfigMu.Lock()
	}

	return nil
}

func SaveConfig() error {
	ConfigMu.Lock()
	defer ConfigMu.Unlock()

	// Ensure directory exists
	if err := EnsureDir(filepath.Dir(ConfigPath)); err != nil {
		return err
	}

	// Download Google Font if needed
	if AppConfig.Theme.FontFamily != "" {
		if err := DownloadGoogleFont(AppConfig.Theme.FontFamily); err != nil {
			log.Printf("Warning: Failed to download font '%s': %v", AppConfig.Theme.FontFamily, err)
		}
	}

	// Process icon_name fields and download icons before saving
	ProcessIconNames(&AppConfig)

	data, err := yaml.Marshal(&AppConfig)
	if err != nil {
		return err
	}

	return os.WriteFile(ConfigPath, data, 0644)
}

// saveConfigQuiet saves the config without re-processing icons (used after ProcessIconNames already ran)
// Sets skip flag to prevent fsnotify from triggering another reload
func saveConfigQuiet() error {
	ConfigMu.Lock()
	defer ConfigMu.Unlock()

	// Set skip flag before writing
	skipReloadMu.Lock()
	skipNextReload = true
	skipReloadMu.Unlock()

	data, err := yaml.Marshal(&AppConfig)
	if err != nil {
		return err
	}

	if err := os.WriteFile(ConfigPath, data, 0644); err != nil {
		return err
	}

	log.Printf("✓ Config saved with updated icon paths")
	return nil
}

func reloadConfig() error {
	data, err := os.ReadFile(ConfigPath)
	if err != nil {
		return err
	}

	newConfig, err := ParseConfig(data)
	if err != nil {
		return err
	}

	// Download Google Font if needed
	if newConfig.Theme.FontFamily != "" {
		if err := DownloadGoogleFont(newConfig.Theme.FontFamily); err != nil {
			log.Printf("Warning: Failed to download font '%s': %v", newConfig.Theme.FontFamily, err)
		}
	}

	// Process icon_name fields and download icons if needed
	iconsChanged := ProcessIconNames(&newConfig)

	// Apply the new config
	ConfigMu.Lock()
	AppConfig = newConfig
	ConfigMu.Unlock()

	log.Printf("✓ Config reloaded from %s", ConfigPath)

	// If icons were processed, save the updated config (with populated icon paths)
	if iconsChanged {
		if err := saveConfigQuiet(); err != nil {
			log.Printf("Warning: Failed to save config after processing icons: %v", err)
		}
	}

	return nil
}

func WatchConfigFile() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("Failed to create file watcher: %v", err)
		return
	}
	defer watcher.Close()

	// Add config file to watcher
	err = watcher.Add(ConfigPath)
	if err != nil {
		log.Printf("Failed to watch config file: %v", err)
		return
	}

	log.Printf("👀 Watching %s for changes", ConfigPath)

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

func GetDefaultConfig() Config {
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
