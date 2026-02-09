package internal

import (
	"encoding/json"
	"net/http"
)

// HTTP response helpers

func WriteJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func WriteJSONStatus(w http.ResponseWriter, status string) {
	WriteJSON(w, map[string]string{"status": status})
}

func WriteJSONStatusPath(w http.ResponseWriter, status, path string) {
	WriteJSON(w, map[string]string{"status": status, "path": path})
}

func RequireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method != method {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return false
	}
	return true
}
