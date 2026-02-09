package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"statis/internal"
)

func TestWriteJSON(t *testing.T) {
	recorder := httptest.NewRecorder()

	data := map[string]string{"key": "value"}
	internal.WriteJSON(recorder, data)

	// Check content type
	contentType := recorder.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type 'application/json', got '%s'", contentType)
	}

	// Check body
	var result map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse JSON response: %v", err)
	}

	if result["key"] != "value" {
		t.Errorf("Expected key='value', got '%s'", result["key"])
	}
}

func TestWriteJSONStatus(t *testing.T) {
	recorder := httptest.NewRecorder()

	internal.WriteJSONStatus(recorder, "ok")

	var result map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse JSON response: %v", err)
	}

	if result["status"] != "ok" {
		t.Errorf("Expected status='ok', got '%s'", result["status"])
	}
}

func TestWriteJSONStatusPath(t *testing.T) {
	recorder := httptest.NewRecorder()

	internal.WriteJSONStatusPath(recorder, "ok", "/icons/test.svg")

	var result map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse JSON response: %v", err)
	}

	if result["status"] != "ok" {
		t.Errorf("Expected status='ok', got '%s'", result["status"])
	}
	if result["path"] != "/icons/test.svg" {
		t.Errorf("Expected path='/icons/test.svg', got '%s'", result["path"])
	}
}

func TestRequireMethod(t *testing.T) {
	tests := []struct {
		requestMethod  string
		requiredMethod string
		shouldPass     bool
	}{
		{"GET", "GET", true},
		{"POST", "POST", true},
		{"GET", "POST", false},
		{"POST", "GET", false},
		{"DELETE", "DELETE", true},
		{"PUT", "PUT", true},
	}

	for _, tc := range tests {
		req := httptest.NewRequest(tc.requestMethod, "/test", nil)
		recorder := httptest.NewRecorder()

		result := internal.RequireMethod(recorder, req, tc.requiredMethod)

		if result != tc.shouldPass {
			t.Errorf("RequireMethod(%s, %s) = %v, expected %v",
				tc.requestMethod, tc.requiredMethod, result, tc.shouldPass)
		}

		if !tc.shouldPass && recorder.Code != http.StatusMethodNotAllowed {
			t.Errorf("Expected status 405, got %d", recorder.Code)
		}
	}
}
