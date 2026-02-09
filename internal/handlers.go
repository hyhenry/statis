package internal

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
)

// Templates holds the parsed templates (set by main.go)
var Templates *template.Template

// RenderTemplate renders a template with the current config
func RenderTemplate(w http.ResponseWriter, name string) {
	ConfigMu.RLock()
	defer ConfigMu.RUnlock()

	if err := Templates.ExecuteTemplate(w, name, AppConfig); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func HandleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	RenderTemplate(w, "index.html")
}

func HandleSettings(w http.ResponseWriter, r *http.Request) {
	RenderTemplate(w, "settings.html")
}

func HandleAPIConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		ConfigMu.RLock()
		defer ConfigMu.RUnlock()
		WriteJSON(w, AppConfig)

	case http.MethodPut:
		var newConfig Config
		if err := json.NewDecoder(r.Body).Decode(&newConfig); err != nil {
			log.Printf("❌ Failed to decode config JSON: %v", err)
			http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
			return
		}

		ConfigMu.Lock()
		AppConfig = newConfig
		ConfigMu.Unlock()

		if err := SaveConfig(); err != nil {
			log.Printf("❌ Failed to save config: %v", err)
			http.Error(w, "Failed to save: "+err.Error(), http.StatusInternalServerError)
			return
		}

		log.Printf("✓ Config saved successfully")
		WriteJSONStatus(w, "ok")

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
