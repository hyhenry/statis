package tests

import (
	"os"
	"path/filepath"
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
