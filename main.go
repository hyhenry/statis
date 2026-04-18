package main

import (
	"embed"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"

	"statis/internal"
)

//go:embed static templates
var embeddedFS embed.FS

func main() {
	// Determine config path
	internal.ConfigPath = os.Getenv("CONFIG_PATH")
	if internal.ConfigPath == "" {
		internal.ConfigPath = "./config.yaml"
	}

	// Load configuration
	if err := internal.LoadConfig(); err != nil {
		log.Printf("Warning: Could not load config from %s: %v", internal.ConfigPath, err)
		log.Println("Using default configuration")
		internal.AppConfig = internal.GetDefaultConfig()
		// Save default config
		internal.SaveConfig()
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
	internal.Templates, err = template.New("").Funcs(funcMap).ParseFS(embeddedFS, "templates/*.html")
	if err != nil {
		log.Fatalf("Failed to parse templates: %v", err)
	}

	// Start file watcher in background
	go internal.WatchConfigFile()

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
	mux.HandleFunc("/", internal.HandleIndex)
	mux.HandleFunc("/settings", internal.HandleSettings)

	// API endpoints
	mux.HandleFunc("/api/config", internal.HandleAPIConfig)
	mux.HandleFunc("/api/widget/uptime-kuma", internal.HandleUptimeKumaProxy)
	mux.HandleFunc("/api/widget/system-stats", internal.HandleSystemStats)
	mux.HandleFunc("/api/widget/rss", internal.HandleRSSWidget)
	mux.HandleFunc("/api/widget/truenas-scale", internal.HandleTrueNASSCALEWidget)
	mux.HandleFunc("/api/assets/clean-unused", internal.HandleCleanUnusedAssets)
	mux.HandleFunc("/api/icons/search", internal.HandleIconSearch)
	mux.HandleFunc("/api/icons/download", internal.HandleIconDownload)
	mux.HandleFunc("/api/icons/upload", internal.HandleIconUpload)

	// Get port from environment or default to 8080
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("🏠 Statis starting on http://localhost:%s", port)
	log.Printf("📁 Config file: %s", internal.ConfigPath)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
