package tests

import (
	"os"
	"path/filepath"
	"testing"

	"statis/internal"
)

func TestEnsureDir(t *testing.T) {
	tmpDir := t.TempDir()
	testPath := filepath.Join(tmpDir, "nested", "dirs", "here")

	// Directory shouldn't exist yet
	if _, err := os.Stat(testPath); !os.IsNotExist(err) {
		t.Fatal("Test directory already exists")
	}

	// Create it
	err := internal.EnsureDir(testPath)
	if err != nil {
		t.Fatalf("EnsureDir failed: %v", err)
	}

	// Should exist now
	info, err := os.Stat(testPath)
	if err != nil {
		t.Fatalf("Directory was not created: %v", err)
	}

	if !info.IsDir() {
		t.Error("Created path is not a directory")
	}

	// Calling again should not error
	err = internal.EnsureDir(testPath)
	if err != nil {
		t.Errorf("EnsureDir on existing dir failed: %v", err)
	}
}

func TestDownloadFile(t *testing.T) {
	// Skip this test in CI or when network unavailable
	if os.Getenv("SKIP_NETWORK_TESTS") != "" {
		t.Skip("Skipping network test")
	}

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	// Download a small file from a reliable source
	err := internal.DownloadFile("https://httpbin.org/robots.txt", testFile)
	if err != nil {
		t.Fatalf("DownloadFile failed: %v", err)
	}

	// Check file exists
	info, err := os.Stat(testFile)
	if err != nil {
		t.Fatalf("Downloaded file not found: %v", err)
	}

	if info.Size() == 0 {
		t.Error("Downloaded file is empty")
	}
}

func TestDownloadFileInvalidURL(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	err := internal.DownloadFile("https://this-domain-does-not-exist-12345.com/file.txt", testFile)
	if err == nil {
		t.Error("Expected error for invalid URL, got nil")
	}
}
