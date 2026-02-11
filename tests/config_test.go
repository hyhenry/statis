package tests

import (
	"os"
	"path/filepath"
	"testing"

	"statis/internal"
)

func TestParseConfig(t *testing.T) {
	data, err := os.ReadFile("testdata/valid_config.yaml")
	if err != nil {
		t.Fatalf("Failed to read test config: %v", err)
	}

	cfg, err := internal.ParseConfig(data)
	if err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}

	if cfg.Title != "Test Dashboard" {
		t.Errorf("Expected title 'Test Dashboard', got '%s'", cfg.Title)
	}

	if cfg.Subtitle != "Test subtitle" {
		t.Errorf("Expected subtitle 'Test subtitle', got '%s'", cfg.Subtitle)
	}

	if cfg.Theme.PrimaryColor != "#33C3F0" {
		t.Errorf("Expected primary color '#33C3F0', got '%s'", cfg.Theme.PrimaryColor)
	}

	if cfg.Layout.WidgetColumns != 4 {
		t.Errorf("Expected widget columns 4, got %d", cfg.Layout.WidgetColumns)
	}

	if len(cfg.Services) != 1 {
		t.Errorf("Expected 1 service section, got %d", len(cfg.Services))
	}

	if len(cfg.Widgets) != 1 {
		t.Errorf("Expected 1 widget, got %d", len(cfg.Widgets))
	}
}

func TestParseConfigDefaults(t *testing.T) {
	// Config without secondary color should default to primary
	data := []byte(`
title: "Test"
theme:
  primary_color: "#FF0000"
`)

	cfg, err := internal.ParseConfig(data)
	if err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}

	if cfg.Theme.SecondaryColor != "#FF0000" {
		t.Errorf("Expected secondary color to default to primary '#FF0000', got '%s'", cfg.Theme.SecondaryColor)
	}
}

func TestParseConfigInvalid(t *testing.T) {
	invalidYAML := []byte(`
title: "Test
  invalid: yaml
`)

	_, err := internal.ParseConfig(invalidYAML)
	if err == nil {
		t.Error("Expected error for invalid YAML, got nil")
	}
}

func TestGetDefaultConfig(t *testing.T) {
	cfg := internal.GetDefaultConfig()

	if cfg.Title != "Statis" {
		t.Errorf("Expected default title 'Statis', got '%s'", cfg.Title)
	}

	if cfg.Theme.FontFamily != "system" {
		t.Errorf("Expected default font 'system', got '%s'", cfg.Theme.FontFamily)
	}

	if cfg.Layout.CardsPerRow != 3 {
		t.Errorf("Expected default cards per row 3, got %d", cfg.Layout.CardsPerRow)
	}

	if len(cfg.Services) != 3 {
		t.Errorf("Expected 3 default service sections, got %d", len(cfg.Services))
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir := t.TempDir()
	testConfigPath := filepath.Join(tmpDir, "test_config.yaml")

	// Save original configPath and restore after test
	origConfigPath := internal.ConfigPath
	internal.ConfigPath = testConfigPath
	defer func() { internal.ConfigPath = origConfigPath }()

	// Set a test config
	internal.AppConfig = internal.Config{
		Title:    "Save Test",
		Subtitle: "Testing save",
		Theme: internal.Theme{
			PrimaryColor:    "#123456",
			SecondaryColor:  "#654321",
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
		Services: []internal.Section{},
		Widgets:  []internal.Widget{},
	}

	// Save config
	err := internal.SaveConfig()
	if err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(testConfigPath); os.IsNotExist(err) {
		t.Fatal("Config file was not created")
	}

	// Clear config and reload
	internal.AppConfig = internal.Config{}
	err = internal.LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if internal.AppConfig.Title != "Save Test" {
		t.Errorf("Expected title 'Save Test', got '%s'", internal.AppConfig.Title)
	}
}

// --- LoadConfig Tests ---

func TestLoadConfig_FileNotFound(t *testing.T) {
	origConfigPath := internal.ConfigPath
	internal.ConfigPath = "/nonexistent/path/config.yaml"
	defer func() { internal.ConfigPath = origConfigPath }()

	err := internal.LoadConfig()
	if err == nil {
		t.Error("Expected error for missing file, got nil")
	}
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	testConfigPath := filepath.Join(tmpDir, "invalid.yaml")

	// Write invalid YAML
	invalidYAML := []byte("title: \"unclosed string\n  bad: indentation")
	if err := os.WriteFile(testConfigPath, invalidYAML, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	origConfigPath := internal.ConfigPath
	internal.ConfigPath = testConfigPath
	defer func() { internal.ConfigPath = origConfigPath }()

	err := internal.LoadConfig()
	if err == nil {
		t.Error("Expected error for invalid YAML, got nil")
	}
}

func TestLoadConfig_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	testConfigPath := filepath.Join(tmpDir, "empty.yaml")

	// Write empty file
	if err := os.WriteFile(testConfigPath, []byte(""), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	origConfigPath := internal.ConfigPath
	internal.ConfigPath = testConfigPath
	defer func() { internal.ConfigPath = origConfigPath }()

	// Empty YAML is valid, should not error
	err := internal.LoadConfig()
	if err != nil {
		t.Errorf("Unexpected error for empty file: %v", err)
	}
}

func TestLoadConfig_ValidFile(t *testing.T) {
	tmpDir := t.TempDir()
	testConfigPath := filepath.Join(tmpDir, "valid.yaml")

	validYAML := []byte(`
title: "Test Title"
subtitle: "Test Subtitle"
theme:
  primary_color: "#FF0000"
  background_color: "#000000"
  card_color: "#111111"
  text_color: "#FFFFFF"
  font_family: "system"
layout:
  widget_columns: 4
  service_columns: 8
  cards_per_row: 3
`)
	if err := os.WriteFile(testConfigPath, validYAML, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	origConfigPath := internal.ConfigPath
	internal.ConfigPath = testConfigPath
	defer func() { internal.ConfigPath = origConfigPath }()

	err := internal.LoadConfig()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if internal.AppConfig.Title != "Test Title" {
		t.Errorf("Expected title 'Test Title', got '%s'", internal.AppConfig.Title)
	}
}

// --- ParseConfig Additional Tests ---

func TestParseConfig_AllFields(t *testing.T) {
	data := []byte(`
title: "Full Config"
subtitle: "All Fields"
theme:
  primary_color: "#FF0000"
  secondary_color: "#00FF00"
  background_color: "#0000FF"
  card_color: "#FFFFFF"
  text_color: "#000000"
  font_family: "Inter"
  favicon: "/icons/favicon.svg"
  favicon_name: "custom-icon"
layout:
  widget_columns: 3
  service_columns: 9
  cards_per_row: 4
services:
  - name: "Section1"
    items:
      - name: "Item1"
        url: "https://example.com"
        icon: "/icons/item1.svg"
        icon_name: "item1"
        icon_text: "🎉"
        description: "Description"
        target: "_blank"
widgets:
  - type: "clock"
    title: "Clock"
    config:
      format: "24h"
      timezone: "UTC"
`)

	cfg, err := internal.ParseConfig(data)
	if err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}

	// Check all theme fields
	if cfg.Theme.PrimaryColor != "#FF0000" {
		t.Errorf("Expected primary_color '#FF0000', got '%s'", cfg.Theme.PrimaryColor)
	}
	if cfg.Theme.SecondaryColor != "#00FF00" {
		t.Errorf("Expected secondary_color '#00FF00', got '%s'", cfg.Theme.SecondaryColor)
	}
	if cfg.Theme.Favicon != "/icons/favicon.svg" {
		t.Errorf("Expected favicon '/icons/favicon.svg', got '%s'", cfg.Theme.Favicon)
	}
	if cfg.Theme.FaviconName != "custom-icon" {
		t.Errorf("Expected favicon_name 'custom-icon', got '%s'", cfg.Theme.FaviconName)
	}

	// Check layout
	if cfg.Layout.WidgetColumns != 3 {
		t.Errorf("Expected widget_columns 3, got %d", cfg.Layout.WidgetColumns)
	}
	if cfg.Layout.CardsPerRow != 4 {
		t.Errorf("Expected cards_per_row 4, got %d", cfg.Layout.CardsPerRow)
	}

	// Check services
	if len(cfg.Services) != 1 {
		t.Fatalf("Expected 1 service section, got %d", len(cfg.Services))
	}
	if len(cfg.Services[0].Items) != 1 {
		t.Fatalf("Expected 1 item, got %d", len(cfg.Services[0].Items))
	}

	item := cfg.Services[0].Items[0]
	if item.IconName != "item1" {
		t.Errorf("Expected icon_name 'item1', got '%s'", item.IconName)
	}
	if item.Target != "_blank" {
		t.Errorf("Expected target '_blank', got '%s'", item.Target)
	}

	// Check widgets
	if len(cfg.Widgets) != 1 {
		t.Fatalf("Expected 1 widget, got %d", len(cfg.Widgets))
	}
	if cfg.Widgets[0].Type != "clock" {
		t.Errorf("Expected widget type 'clock', got '%s'", cfg.Widgets[0].Type)
	}
	if cfg.Widgets[0].Config["format"] != "24h" {
		t.Errorf("Expected format '24h', got '%s'", cfg.Widgets[0].Config["format"])
	}
}

func TestParseConfig_MinimalConfig(t *testing.T) {
	data := []byte(`title: "Minimal"`)

	cfg, err := internal.ParseConfig(data)
	if err != nil {
		t.Fatalf("Failed to parse minimal config: %v", err)
	}

	if cfg.Title != "Minimal" {
		t.Errorf("Expected title 'Minimal', got '%s'", cfg.Title)
	}

	// Secondary color should default to empty (primary is empty)
	if cfg.Theme.SecondaryColor != "" {
		t.Errorf("Expected empty secondary color, got '%s'", cfg.Theme.SecondaryColor)
	}
}

func TestParseConfig_WidgetConfigs(t *testing.T) {
	data := []byte(`
widgets:
  - type: "uptime-kuma"
    title: "Status"
    config:
      url: "http://uptime:3001"
      slug: "status"
      collapsed: "true"
  - type: "rss"
    title: "News"
    config:
      url: "https://example.com/feed.xml"
      items_per_page: "5"
      refresh: "300"
  - type: "iframe"
    title: "Embed"
    config:
      url: "https://example.com"
      height: "400px"
  - type: "header"
    title: "Section Title"
    config: {}
`)

	cfg, err := internal.ParseConfig(data)
	if err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}

	if len(cfg.Widgets) != 4 {
		t.Fatalf("Expected 4 widgets, got %d", len(cfg.Widgets))
	}

	// Check uptime-kuma widget
	if cfg.Widgets[0].Config["collapsed"] != "true" {
		t.Errorf("Expected collapsed 'true', got '%s'", cfg.Widgets[0].Config["collapsed"])
	}

	// Check rss widget
	if cfg.Widgets[1].Config["items_per_page"] != "5" {
		t.Errorf("Expected items_per_page '5', got '%s'", cfg.Widgets[1].Config["items_per_page"])
	}

	// Check iframe widget
	if cfg.Widgets[2].Config["height"] != "400px" {
		t.Errorf("Expected height '400px', got '%s'", cfg.Widgets[2].Config["height"])
	}
}

// --- GetDefaultConfig Additional Tests ---

func TestGetDefaultConfig_Structure(t *testing.T) {
	cfg := internal.GetDefaultConfig()

	// Check required defaults exist
	if cfg.Title == "" {
		t.Error("Default config should have a title")
	}

	if cfg.Theme.PrimaryColor == "" {
		t.Error("Default config should have a primary color")
	}

	if cfg.Theme.FontFamily == "" {
		t.Error("Default config should have a font family")
	}

	if cfg.Layout.WidgetColumns == 0 {
		t.Error("Default config should have widget columns > 0")
	}

	if cfg.Layout.ServiceColumns == 0 {
		t.Error("Default config should have service columns > 0")
	}

	if cfg.Layout.CardsPerRow == 0 {
		t.Error("Default config should have cards per row > 0")
	}

	// Check columns add up to 12
	total := cfg.Layout.WidgetColumns + cfg.Layout.ServiceColumns
	if total != 12 {
		t.Errorf("Expected widget + service columns = 12, got %d", total)
	}
}

func TestGetDefaultConfig_HasExampleServices(t *testing.T) {
	cfg := internal.GetDefaultConfig()

	if len(cfg.Services) == 0 {
		t.Error("Default config should have example services")
	}

	// Check each section has items
	for i, section := range cfg.Services {
		if section.Name == "" {
			t.Errorf("Service section %d should have a name", i)
		}
		if len(section.Items) == 0 {
			t.Errorf("Service section '%s' should have items", section.Name)
		}
	}
}

func TestGetDefaultConfig_HasExampleWidgets(t *testing.T) {
	cfg := internal.GetDefaultConfig()

	if len(cfg.Widgets) == 0 {
		t.Error("Default config should have example widgets")
	}

	for i, widget := range cfg.Widgets {
		if widget.Type == "" {
			t.Errorf("Widget %d should have a type", i)
		}
	}
}

// --- SaveConfig Additional Tests ---

func TestSaveConfig_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	nestedPath := filepath.Join(tmpDir, "nested", "dir", "config.yaml")

	origConfigPath := internal.ConfigPath
	internal.ConfigPath = nestedPath
	defer func() { internal.ConfigPath = origConfigPath }()

	internal.AppConfig = internal.Config{
		Title: "Nested Test",
		Theme: internal.Theme{FontFamily: "system"},
	}

	err := internal.SaveConfig()
	if err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(nestedPath); os.IsNotExist(err) {
		t.Error("Config file was not created in nested directory")
	}
}

func TestSaveConfig_PreservesData(t *testing.T) {
	tmpDir := t.TempDir()
	testConfigPath := filepath.Join(tmpDir, "roundtrip.yaml")

	origConfigPath := internal.ConfigPath
	internal.ConfigPath = testConfigPath
	defer func() { internal.ConfigPath = origConfigPath }()

	// Set complex config
	internal.AppConfig = internal.Config{
		Title:    "Roundtrip Test",
		Subtitle: "Testing data preservation",
		Theme: internal.Theme{
			PrimaryColor:    "#123456",
			SecondaryColor:  "#654321",
			BackgroundColor: "#AABBCC",
			CardColor:       "#DDEEFF",
			TextColor:       "#112233",
			FontFamily:      "system",
			Favicon:         "/icons/test.svg",
		},
		Layout: internal.Layout{
			WidgetColumns:  5,
			ServiceColumns: 7,
			CardsPerRow:    4,
		},
		Services: []internal.Section{
			{
				Name: "TestSection",
				Items: []internal.Item{
					{
						Name:        "TestItem",
						URL:         "https://test.com",
						Icon:        "/icons/item.svg",
						IconText:    "🧪",
						Description: "Test description",
						Target:      "_self",
					},
				},
			},
		},
		Widgets: []internal.Widget{
			{
				Type:  "clock",
				Title: "Test Clock",
				Config: map[string]string{
					"format":   "12h",
					"timezone": "America/New_York",
				},
			},
		},
	}

	// Save
	err := internal.SaveConfig()
	if err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Clear and reload
	internal.AppConfig = internal.Config{}
	err = internal.LoadConfig()
	if err != nil {
		t.Fatalf("Failed to reload config: %v", err)
	}

	// Verify all data preserved
	if internal.AppConfig.Title != "Roundtrip Test" {
		t.Errorf("Title not preserved: got '%s'", internal.AppConfig.Title)
	}
	if internal.AppConfig.Theme.PrimaryColor != "#123456" {
		t.Errorf("Primary color not preserved: got '%s'", internal.AppConfig.Theme.PrimaryColor)
	}
	if internal.AppConfig.Layout.CardsPerRow != 4 {
		t.Errorf("Cards per row not preserved: got %d", internal.AppConfig.Layout.CardsPerRow)
	}
	if len(internal.AppConfig.Services) != 1 {
		t.Error("Services not preserved")
	}
	if len(internal.AppConfig.Widgets) != 1 {
		t.Error("Widgets not preserved")
	}
	if internal.AppConfig.Widgets[0].Config["timezone"] != "America/New_York" {
		t.Errorf("Widget config not preserved: got '%s'", internal.AppConfig.Widgets[0].Config["timezone"])
	}
}
