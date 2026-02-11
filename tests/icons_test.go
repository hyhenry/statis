package tests

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"statis/internal"
)

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{"with spaces", "with-spaces"},
		{"special!@#chars", "special-chars"},
		{"multiple---dashes", "multiple-dashes"},
		{"---leading-trailing---", "leading-trailing"},
		{"123numbers", "123numbers"},
		{"under_score", "under_score"},
		{"MixedCase", "MixedCase"},
		{"", ""},
	}

	for _, tc := range tests {
		result := internal.SanitizeFilename(tc.input)
		if result != tc.expected {
			t.Errorf("SanitizeFilename(%q) = %q, expected %q", tc.input, result, tc.expected)
		}
	}
}

func TestUniqueFilename(t *testing.T) {
	// Create a temporary directory
	tmpDir := t.TempDir()

	// Test when file doesn't exist
	fileName, filePath := internal.UniqueFilename(tmpDir, "test", ".svg")
	if fileName != "test.svg" {
		t.Errorf("Expected 'test.svg', got '%s'", fileName)
	}
	expectedPath := filepath.Join(tmpDir, "test.svg")
	if filePath != expectedPath {
		t.Errorf("Expected path '%s', got '%s'", expectedPath, filePath)
	}

	// Create the file
	if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Now it should return a unique name
	fileName2, filePath2 := internal.UniqueFilename(tmpDir, "test", ".svg")
	if fileName2 != "test-1.svg" {
		t.Errorf("Expected 'test-1.svg', got '%s'", fileName2)
	}
	expectedPath2 := filepath.Join(tmpDir, "test-1.svg")
	if filePath2 != expectedPath2 {
		t.Errorf("Expected path '%s', got '%s'", expectedPath2, filePath2)
	}

	// Create that file too
	if err := os.WriteFile(filePath2, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Should increment again
	fileName3, _ := internal.UniqueFilename(tmpDir, "test", ".svg")
	if fileName3 != "test-2.svg" {
		t.Errorf("Expected 'test-2.svg', got '%s'", fileName3)
	}
}

// --- Icon Handler Tests ---

func TestHandleIconSearch_InvalidMethod(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/icons/search", nil)
	recorder := httptest.NewRecorder()

	internal.HandleIconSearch(recorder, req)

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", recorder.Code)
	}
}

func TestHandleIconDownload_InvalidMethod(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/icons/download", nil)
	recorder := httptest.NewRecorder()

	internal.HandleIconDownload(recorder, req)

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", recorder.Code)
	}
}

func TestHandleIconDownload_MissingName(t *testing.T) {
	body := bytes.NewReader([]byte(`{"format": "svg"}`))
	req := httptest.NewRequest(http.MethodPost, "/api/icons/download", body)
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	internal.HandleIconDownload(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", recorder.Code)
	}
}

func TestHandleIconDownload_InvalidJSON(t *testing.T) {
	body := bytes.NewReader([]byte(`not valid json`))
	req := httptest.NewRequest(http.MethodPost, "/api/icons/download", body)
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	internal.HandleIconDownload(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", recorder.Code)
	}
}

func TestHandleIconDownload_InvalidFormat(t *testing.T) {
	body := bytes.NewReader([]byte(`{"name": "test", "format": "gif"}`))
	req := httptest.NewRequest(http.MethodPost, "/api/icons/download", body)
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	internal.HandleIconDownload(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", recorder.Code)
	}

	if !strings.Contains(recorder.Body.String(), "Invalid format") {
		t.Errorf("Expected 'Invalid format' error, got: %s", recorder.Body.String())
	}
}

func TestHandleIconDownload_DefaultFormat(t *testing.T) {
	// Create temp icons directory
	tmpDir := t.TempDir()
	iconsDir := filepath.Join(tmpDir, "icons")
	if err := os.MkdirAll(iconsDir, 0755); err != nil {
		t.Fatalf("Failed to create icons dir: %v", err)
	}

	// Create a test icon file
	testIconPath := filepath.Join(iconsDir, "test-icon.svg")
	if err := os.WriteFile(testIconPath, []byte("<svg></svg>"), 0644); err != nil {
		t.Fatalf("Failed to create test icon: %v", err)
	}

	// Request without format should default to svg
	body := bytes.NewReader([]byte(`{"name": "test-icon"}`))
	req := httptest.NewRequest(http.MethodPost, "/api/icons/download", body)
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	// Temporarily change working directory
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	internal.HandleIconDownload(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var result map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if result["path"] != "/icons/test-icon.svg" {
		t.Errorf("Expected path '/icons/test-icon.svg', got '%s'", result["path"])
	}
}

func TestHandleIconUpload_InvalidMethod(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/icons/upload", nil)
	recorder := httptest.NewRecorder()

	internal.HandleIconUpload(recorder, req)

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", recorder.Code)
	}
}

func TestHandleIconUpload_MissingFile(t *testing.T) {
	// Create empty multipart form
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/icons/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	recorder := httptest.NewRecorder()

	internal.HandleIconUpload(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", recorder.Code)
	}
}

func TestHandleIconUpload_InvalidFileType(t *testing.T) {
	// Create multipart form with invalid file type
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile("icon", "test.txt")
	if err != nil {
		t.Fatalf("Failed to create form file: %v", err)
	}
	io.WriteString(part, "text content")
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/icons/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	recorder := httptest.NewRecorder()

	internal.HandleIconUpload(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", recorder.Code)
	}

	if !strings.Contains(recorder.Body.String(), "Invalid file type") {
		t.Errorf("Expected 'Invalid file type' error, got: %s", recorder.Body.String())
	}
}

func TestHandleIconUpload_ValidSVG(t *testing.T) {
	// Create temp directory for icons
	tmpDir := t.TempDir()

	// Change working directory temporarily
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Create multipart form with valid SVG
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile("icon", "my-icon.svg")
	if err != nil {
		t.Fatalf("Failed to create form file: %v", err)
	}
	io.WriteString(part, "<svg xmlns=\"http://www.w3.org/2000/svg\"></svg>")
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/icons/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	recorder := httptest.NewRecorder()

	internal.HandleIconUpload(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var result map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if result["status"] != "ok" {
		t.Errorf("Expected status 'ok', got '%s'", result["status"])
	}

	if !strings.HasPrefix(result["path"], "/icons/") {
		t.Errorf("Expected path to start with '/icons/', got '%s'", result["path"])
	}

	// Verify file was created
	iconPath := filepath.Join(tmpDir, "icons", strings.TrimPrefix(result["path"], "/icons/"))
	if _, err := os.Stat(iconPath); os.IsNotExist(err) {
		t.Error("Icon file was not created")
	}
}

func TestHandleIconUpload_ValidPNG(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile("icon", "test.png")
	if err != nil {
		t.Fatalf("Failed to create form file: %v", err)
	}
	// Write minimal PNG header
	io.WriteString(part, "\x89PNG\r\n\x1a\n")
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/icons/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	recorder := httptest.NewRecorder()

	internal.HandleIconUpload(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestHandleIconUpload_SanitizesFilename(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	// Use filename with special characters
	part, err := writer.CreateFormFile("icon", "my icon!@#$.svg")
	if err != nil {
		t.Fatalf("Failed to create form file: %v", err)
	}
	io.WriteString(part, "<svg></svg>")
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/icons/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	recorder := httptest.NewRecorder()

	internal.HandleIconUpload(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", recorder.Code)
	}

	var result map[string]string
	json.Unmarshal(recorder.Body.Bytes(), &result)

	// Filename should be sanitized
	if strings.Contains(result["path"], "!") || strings.Contains(result["path"], "@") {
		t.Errorf("Filename was not sanitized: %s", result["path"])
	}
}

// --- Additional SanitizeFilename tests ---

func TestSanitizeFilename_Unicode(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"日本語", ""},
		{"emoji🎉icon", "emoji-icon"},
		{"café", "caf"},
		{"naïve", "na-ve"},
	}

	for _, tc := range tests {
		result := internal.SanitizeFilename(tc.input)
		if result != tc.expected {
			t.Errorf("SanitizeFilename(%q) = %q, expected %q", tc.input, result, tc.expected)
		}
	}
}

func TestUniqueFilename_ManyCollisions(t *testing.T) {
	tmpDir := t.TempDir()

	// Create files test.svg, test-1.svg, test-2.svg, ... test-9.svg
	os.WriteFile(filepath.Join(tmpDir, "test.svg"), []byte(""), 0644)
	for i := 1; i <= 9; i++ {
		fileName := filepath.Join(tmpDir, "test-"+string(rune('0'+i))+".svg")
		os.WriteFile(fileName, []byte(""), 0644)
	}

	fileName, _ := internal.UniqueFilename(tmpDir, "test", ".svg")
	// Should find a unique name (test-10.svg)
	if fileName == "test.svg" {
		t.Error("Should not return test.svg as it exists")
	}
}
