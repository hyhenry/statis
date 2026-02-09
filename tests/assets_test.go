package tests

import (
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
