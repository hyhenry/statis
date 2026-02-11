package tests

import (
	"bytes"
	"encoding/json"
	"html/template"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"statis/internal"
)

func TestHandleIndex_ValidPath(t *testing.T) {
	// Setup templates
	tmpl := template.Must(template.New("index.html").Parse(`<!DOCTYPE html><html><body>{{.Title}}</body></html>`))
	internal.Templates = tmpl

	// Setup config
	internal.AppConfig = internal.Config{
		Title: "Test Dashboard",
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	recorder := httptest.NewRecorder()

	internal.HandleIndex(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", recorder.Code)
	}

	body := recorder.Body.String()
	if body == "" {
		t.Error("Expected non-empty response body")
	}
}

func TestHandleIndex_InvalidPath(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/invalid-path", nil)
	recorder := httptest.NewRecorder()

	internal.HandleIndex(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", recorder.Code)
	}
}

func TestHandleIndex_NestedPath(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/some/nested/path", nil)
	recorder := httptest.NewRecorder()

	internal.HandleIndex(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", recorder.Code)
	}
}

func TestHandleSettings(t *testing.T) {
	// Setup templates
	tmpl := template.Must(template.New("settings.html").Parse(`<!DOCTYPE html><html><body>Settings: {{.Title}}</body></html>`))
	internal.Templates = tmpl

	// Setup config
	internal.AppConfig = internal.Config{
		Title: "Test Dashboard",
	}

	req := httptest.NewRequest(http.MethodGet, "/settings", nil)
	recorder := httptest.NewRecorder()

	internal.HandleSettings(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", recorder.Code)
	}

	body := recorder.Body.String()
	if body == "" {
		t.Error("Expected non-empty response body")
	}
}

func TestHandleAPIConfig_Get(t *testing.T) {
	// Setup config
	internal.AppConfig = internal.Config{
		Title:    "API Test",
		Subtitle: "Test subtitle",
		Theme: internal.Theme{
			PrimaryColor: "#FF0000",
		},
		Layout: internal.Layout{
			WidgetColumns:  4,
			ServiceColumns: 8,
			CardsPerRow:    3,
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	recorder := httptest.NewRecorder()

	internal.HandleAPIConfig(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", recorder.Code)
	}

	// Check content type
	contentType := recorder.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type 'application/json', got '%s'", contentType)
	}

	// Parse response
	var cfg internal.Config
	if err := json.Unmarshal(recorder.Body.Bytes(), &cfg); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}

	if cfg.Title != "API Test" {
		t.Errorf("Expected title 'API Test', got '%s'", cfg.Title)
	}

	if cfg.Theme.PrimaryColor != "#FF0000" {
		t.Errorf("Expected primary color '#FF0000', got '%s'", cfg.Theme.PrimaryColor)
	}
}

func TestHandleAPIConfig_Put_Valid(t *testing.T) {
	// Create temp directory for config
	tmpDir := t.TempDir()
	testConfigPath := filepath.Join(tmpDir, "test_config.yaml")

	// Save original and restore after test
	origConfigPath := internal.ConfigPath
	internal.ConfigPath = testConfigPath
	defer func() { internal.ConfigPath = origConfigPath }()

	// Prepare request body
	newConfig := internal.Config{
		Title:    "Updated Title",
		Subtitle: "Updated subtitle",
		Theme: internal.Theme{
			PrimaryColor:    "#00FF00",
			SecondaryColor:  "#00FF00",
			BackgroundColor: "#000000",
			CardColor:       "#111111",
			TextColor:       "#FFFFFF",
			FontFamily:      "system",
		},
		Layout: internal.Layout{
			WidgetColumns:  3,
			ServiceColumns: 9,
			CardsPerRow:    2,
		},
	}

	body, _ := json.Marshal(newConfig)
	req := httptest.NewRequest(http.MethodPut, "/api/config", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	internal.HandleAPIConfig(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", recorder.Code)
	}

	// Check response
	var result map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if result["status"] != "ok" {
		t.Errorf("Expected status 'ok', got '%s'", result["status"])
	}

	// Verify config was saved
	if _, err := os.Stat(testConfigPath); os.IsNotExist(err) {
		t.Error("Config file was not created")
	}
}

func TestHandleAPIConfig_Put_InvalidJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPut, "/api/config", bytes.NewReader([]byte("not valid json")))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	internal.HandleAPIConfig(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", recorder.Code)
	}
}

func TestHandleAPIConfig_Put_EmptyBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPut, "/api/config", bytes.NewReader([]byte("")))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	internal.HandleAPIConfig(recorder, req)

	// Empty body should fail JSON parsing
	if recorder.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", recorder.Code)
	}
}

func TestHandleAPIConfig_InvalidMethod(t *testing.T) {
	methods := []string{http.MethodPost, http.MethodDelete, http.MethodPatch}

	for _, method := range methods {
		req := httptest.NewRequest(method, "/api/config", nil)
		recorder := httptest.NewRecorder()

		internal.HandleAPIConfig(recorder, req)

		if recorder.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s: Expected status 405, got %d", method, recorder.Code)
		}
	}
}

func TestRenderTemplate_Success(t *testing.T) {
	// Setup template
	tmpl := template.Must(template.New("test.html").Parse(`Title: {{.Title}}, Subtitle: {{.Subtitle}}`))
	internal.Templates = tmpl

	// Setup config
	internal.AppConfig = internal.Config{
		Title:    "Render Test",
		Subtitle: "Template Works",
	}

	recorder := httptest.NewRecorder()
	internal.RenderTemplate(recorder, "test.html")

	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", recorder.Code)
	}

	expected := "Title: Render Test, Subtitle: Template Works"
	if recorder.Body.String() != expected {
		t.Errorf("Expected body '%s', got '%s'", expected, recorder.Body.String())
	}
}

func TestRenderTemplate_TemplateError(t *testing.T) {
	// Setup template that will fail execution (accessing non-existent field)
	tmpl := template.Must(template.New("bad.html").Parse(`{{.NonExistentMethod}}`))
	internal.Templates = tmpl

	internal.AppConfig = internal.Config{}

	recorder := httptest.NewRecorder()
	internal.RenderTemplate(recorder, "bad.html")

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", recorder.Code)
	}
}

func TestRenderTemplate_MissingTemplate(t *testing.T) {
	// Setup with a valid template that isn't the one we request
	tmpl := template.Must(template.New("exists.html").Parse(`exists`))
	internal.Templates = tmpl

	internal.AppConfig = internal.Config{}

	recorder := httptest.NewRecorder()
	internal.RenderTemplate(recorder, "missing.html")

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", recorder.Code)
	}
}

func TestHandleAPIConfig_ConcurrentReads(t *testing.T) {
	// Setup config
	internal.AppConfig = internal.Config{
		Title: "Concurrent Test",
	}

	// Run multiple concurrent GET requests
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
			recorder := httptest.NewRecorder()
			internal.HandleAPIConfig(recorder, req)

			if recorder.Code != http.StatusOK {
				t.Errorf("Expected status 200, got %d", recorder.Code)
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}
