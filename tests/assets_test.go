package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"statis/internal"
)

func TestIsPredefinedFont(t *testing.T) {
	predefined := []string{"system", "Arial", "Helvetica", "Georgia", "Times New Roman", "Courier New", "Verdana", "Trebuchet MS", "Impact"}
	notPredefined := []string{"Roboto", "Open Sans", "Lato", "Inter", "custom-font"}

	for _, font := range predefined {
		if !internal.IsPredefinedFont(font) {
			t.Errorf("Expected '%s' to be predefined", font)
		}
	}

	for _, font := range notPredefined {
		if internal.IsPredefinedFont(font) {
			t.Errorf("Expected '%s' to NOT be predefined", font)
		}
	}
}

func TestNormalizeFont(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Open Sans", "Open-Sans"},
		{"Roboto", "Roboto"},
		{"Times New Roman", "Times-New-Roman"},
		{"Noto Sans JP", "Noto-Sans-JP"},
		{"", ""},
		{"Already-Normalized", "Already-Normalized"},
	}

	for _, tc := range tests {
		result := internal.NormalizeFont(tc.input)
		if result != tc.expected {
			t.Errorf("NormalizeFont(%q) = %q, expected %q", tc.input, result, tc.expected)
		}
	}
}

func TestGetUsedIcons(t *testing.T) {
	// Save original config
	origConfig := internal.AppConfig

	// Set test config
	internal.AppConfig = internal.Config{
		Theme: internal.Theme{
			Favicon:     "/icons/favicon.svg",
			FaviconName: "custom-favicon",
		},
		Services: []internal.Section{
			{
				Name: "Test",
				Items: []internal.Item{
					{Name: "Service1", Icon: "/icons/service1.svg", IconName: ""},
					{Name: "Service2", Icon: "", IconName: "service2"},
					{Name: "Service3", Icon: "https://external.com/icon.png", IconName: ""},
				},
			},
		},
	}
	defer func() { internal.AppConfig = origConfig }()

	used := internal.GetUsedIcons()

	// Should include favicon
	if !used["/icons/favicon.svg"] {
		t.Error("Expected favicon.svg to be in used icons")
	}

	// Should include favicon_name generated path
	if !used["/icons/custom-favicon.svg"] {
		t.Error("Expected custom-favicon.svg to be in used icons")
	}

	// Should include service1 icon
	if !used["/icons/service1.svg"] {
		t.Error("Expected service1.svg to be in used icons")
	}

	// Should include service2 icon_name generated path
	if !used["/icons/service2.svg"] {
		t.Error("Expected service2.svg to be in used icons")
	}

	// Should NOT include external URL
	if used["https://external.com/icon.png"] {
		t.Error("External URLs should not be in used icons")
	}
}

// --- Asset Handler Tests ---

func TestHandleCleanUnusedAssets_InvalidMethod(t *testing.T) {
	methods := []string{http.MethodGet, http.MethodPost, http.MethodPut}

	for _, method := range methods {
		req := httptest.NewRequest(method, "/api/assets/clean-unused", nil)
		recorder := httptest.NewRecorder()

		internal.HandleCleanUnusedAssets(recorder, req)

		if recorder.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s: Expected status 405, got %d", method, recorder.Code)
		}
	}
}

func TestHandleCleanUnusedAssets_EmptyDirectories(t *testing.T) {
	// Create temp directory and change to it
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Setup empty config (no icons or fonts in use)
	origConfig := internal.AppConfig
	internal.AppConfig = internal.Config{
		Theme: internal.Theme{
			FontFamily: "system", // predefined font
		},
		Services: []internal.Section{},
	}
	defer func() { internal.AppConfig = origConfig }()

	req := httptest.NewRequest(http.MethodDelete, "/api/assets/clean-unused", nil)
	recorder := httptest.NewRecorder()

	internal.HandleCleanUnusedAssets(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", recorder.Code)
	}

	var result struct {
		FontsRemoved int      `json:"fonts_removed"`
		IconsRemoved int      `json:"icons_removed"`
		FilesRemoved []string `json:"files_removed"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if result.FontsRemoved != 0 {
		t.Errorf("Expected 0 fonts removed, got %d", result.FontsRemoved)
	}
	if result.IconsRemoved != 0 {
		t.Errorf("Expected 0 icons removed, got %d", result.IconsRemoved)
	}
}

func TestHandleCleanUnusedAssets_CleansUnusedFonts(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Create fonts directory with unused fonts
	fontsDir := filepath.Join(tmpDir, "fonts")
	os.MkdirAll(fontsDir, 0755)
	os.WriteFile(filepath.Join(fontsDir, "UnusedFont.css"), []byte("/* unused */"), 0644)
	os.WriteFile(filepath.Join(fontsDir, "UnusedFont_regular.woff2"), []byte("font"), 0644)

	// Setup config with different font
	origConfig := internal.AppConfig
	internal.AppConfig = internal.Config{
		Theme: internal.Theme{
			FontFamily: "system", // predefined, so all fonts in dir are unused
		},
		Services: []internal.Section{},
	}
	defer func() { internal.AppConfig = origConfig }()

	req := httptest.NewRequest(http.MethodDelete, "/api/assets/clean-unused", nil)
	recorder := httptest.NewRecorder()

	internal.HandleCleanUnusedAssets(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", recorder.Code)
	}

	var result struct {
		FontsRemoved int      `json:"fonts_removed"`
		IconsRemoved int      `json:"icons_removed"`
		FilesRemoved []string `json:"files_removed"`
	}
	json.Unmarshal(recorder.Body.Bytes(), &result)

	if result.FontsRemoved != 2 {
		t.Errorf("Expected 2 fonts removed, got %d", result.FontsRemoved)
	}

	// Verify files were deleted
	if _, err := os.Stat(filepath.Join(fontsDir, "UnusedFont.css")); !os.IsNotExist(err) {
		t.Error("UnusedFont.css should have been deleted")
	}
}

func TestHandleCleanUnusedAssets_KeepsUsedFont(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Create fonts directory
	fontsDir := filepath.Join(tmpDir, "fonts")
	os.MkdirAll(fontsDir, 0755)
	os.WriteFile(filepath.Join(fontsDir, "Inter.css"), []byte("/* Inter */"), 0644)
	os.WriteFile(filepath.Join(fontsDir, "Inter_regular.woff2"), []byte("font"), 0644)
	os.WriteFile(filepath.Join(fontsDir, "Unused.css"), []byte("/* unused */"), 0644)

	// Setup config using Inter font
	origConfig := internal.AppConfig
	internal.AppConfig = internal.Config{
		Theme: internal.Theme{
			FontFamily: "Inter", // custom font
		},
		Services: []internal.Section{},
	}
	defer func() { internal.AppConfig = origConfig }()

	req := httptest.NewRequest(http.MethodDelete, "/api/assets/clean-unused", nil)
	recorder := httptest.NewRecorder()

	internal.HandleCleanUnusedAssets(recorder, req)

	var result struct {
		FontsRemoved int      `json:"fonts_removed"`
		FilesRemoved []string `json:"files_removed"`
	}
	json.Unmarshal(recorder.Body.Bytes(), &result)

	// Should only remove Unused.css, keep Inter files
	if result.FontsRemoved != 1 {
		t.Errorf("Expected 1 font removed, got %d", result.FontsRemoved)
	}

	// Inter files should still exist
	if _, err := os.Stat(filepath.Join(fontsDir, "Inter.css")); os.IsNotExist(err) {
		t.Error("Inter.css should NOT have been deleted")
	}
	if _, err := os.Stat(filepath.Join(fontsDir, "Inter_regular.woff2")); os.IsNotExist(err) {
		t.Error("Inter_regular.woff2 should NOT have been deleted")
	}
}

func TestHandleCleanUnusedAssets_CleansUnusedIcons(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Create icons directory with icons
	iconsDir := filepath.Join(tmpDir, "icons")
	os.MkdirAll(iconsDir, 0755)
	os.WriteFile(filepath.Join(iconsDir, "used.svg"), []byte("<svg/>"), 0644)
	os.WriteFile(filepath.Join(iconsDir, "unused.svg"), []byte("<svg/>"), 0644)
	os.WriteFile(filepath.Join(iconsDir, "also-unused.png"), []byte("png"), 0644)

	// Setup config that uses one icon
	origConfig := internal.AppConfig
	internal.AppConfig = internal.Config{
		Theme: internal.Theme{
			FontFamily: "system",
		},
		Services: []internal.Section{
			{
				Name: "Test",
				Items: []internal.Item{
					{Name: "Used", Icon: "/icons/used.svg"},
				},
			},
		},
	}
	defer func() { internal.AppConfig = origConfig }()

	req := httptest.NewRequest(http.MethodDelete, "/api/assets/clean-unused", nil)
	recorder := httptest.NewRecorder()

	internal.HandleCleanUnusedAssets(recorder, req)

	var result struct {
		IconsRemoved int      `json:"icons_removed"`
		FilesRemoved []string `json:"files_removed"`
	}
	json.Unmarshal(recorder.Body.Bytes(), &result)

	if result.IconsRemoved != 2 {
		t.Errorf("Expected 2 icons removed, got %d", result.IconsRemoved)
	}

	// Used icon should still exist
	if _, err := os.Stat(filepath.Join(iconsDir, "used.svg")); os.IsNotExist(err) {
		t.Error("used.svg should NOT have been deleted")
	}

	// Unused icons should be deleted
	if _, err := os.Stat(filepath.Join(iconsDir, "unused.svg")); !os.IsNotExist(err) {
		t.Error("unused.svg should have been deleted")
	}
}

func TestHandleCleanUnusedAssets_KeepsIconFromIconName(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Create icons directory
	iconsDir := filepath.Join(tmpDir, "icons")
	os.MkdirAll(iconsDir, 0755)
	os.WriteFile(filepath.Join(iconsDir, "proxmox.svg"), []byte("<svg/>"), 0644)
	os.WriteFile(filepath.Join(iconsDir, "unused.svg"), []byte("<svg/>"), 0644)

	// Setup config that uses icon_name
	origConfig := internal.AppConfig
	internal.AppConfig = internal.Config{
		Theme: internal.Theme{
			FontFamily: "system",
		},
		Services: []internal.Section{
			{
				Name: "Test",
				Items: []internal.Item{
					{Name: "Proxmox", IconName: "proxmox"}, // Uses icon_name, not icon
				},
			},
		},
	}
	defer func() { internal.AppConfig = origConfig }()

	req := httptest.NewRequest(http.MethodDelete, "/api/assets/clean-unused", nil)
	recorder := httptest.NewRecorder()

	internal.HandleCleanUnusedAssets(recorder, req)

	var result struct {
		IconsRemoved int `json:"icons_removed"`
	}
	json.Unmarshal(recorder.Body.Bytes(), &result)

	if result.IconsRemoved != 1 {
		t.Errorf("Expected 1 icon removed, got %d", result.IconsRemoved)
	}

	// proxmox.svg should be kept (referenced by icon_name)
	if _, err := os.Stat(filepath.Join(iconsDir, "proxmox.svg")); os.IsNotExist(err) {
		t.Error("proxmox.svg should NOT have been deleted (referenced by icon_name)")
	}
}

// --- Font Helper Tests ---

func TestIsPredefinedFont_AllPredefined(t *testing.T) {
	predefined := []string{
		"system", "Arial", "Helvetica", "Georgia",
		"Times New Roman", "Courier New", "Verdana",
		"Trebuchet MS", "Impact",
	}

	for _, font := range predefined {
		if !internal.IsPredefinedFont(font) {
			t.Errorf("Expected '%s' to be predefined", font)
		}
	}
}

func TestIsPredefinedFont_CaseSensitive(t *testing.T) {
	// These should NOT be predefined (case sensitive)
	notPredefined := []string{"SYSTEM", "arial", "ARIAL", "System"}

	for _, font := range notPredefined {
		if internal.IsPredefinedFont(font) {
			t.Errorf("Expected '%s' to NOT be predefined (case sensitive)", font)
		}
	}
}

func TestNormalizeFont_EdgeCases(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{" ", "-"},
		{"  ", "--"},
		{"Font  Name", "Font--Name"},
		{" Leading", "-Leading"},
		{"Trailing ", "Trailing-"},
	}

	for _, tc := range tests {
		result := internal.NormalizeFont(tc.input)
		if result != tc.expected {
			t.Errorf("NormalizeFont(%q) = %q, expected %q", tc.input, result, tc.expected)
		}
	}
}

func TestGetUsedIcons_EmptyConfig(t *testing.T) {
	origConfig := internal.AppConfig
	internal.AppConfig = internal.Config{
		Services: []internal.Section{},
	}
	defer func() { internal.AppConfig = origConfig }()

	used := internal.GetUsedIcons()
	if len(used) != 0 {
		t.Errorf("Expected empty map for empty config, got %d entries", len(used))
	}
}

func TestGetUsedIcons_MultipleSections(t *testing.T) {
	origConfig := internal.AppConfig
	internal.AppConfig = internal.Config{
		Services: []internal.Section{
			{
				Name: "Section1",
				Items: []internal.Item{
					{Name: "S1", Icon: "/icons/s1.svg"},
				},
			},
			{
				Name: "Section2",
				Items: []internal.Item{
					{Name: "S2", Icon: "/icons/s2.svg"},
					{Name: "S3", IconName: "s3"},
				},
			},
		},
	}
	defer func() { internal.AppConfig = origConfig }()

	used := internal.GetUsedIcons()

	expected := []string{"/icons/s1.svg", "/icons/s2.svg", "/icons/s3.svg"}
	for _, path := range expected {
		if !used[path] {
			t.Errorf("Expected %s to be in used icons", path)
		}
	}
}
