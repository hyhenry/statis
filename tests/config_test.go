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
