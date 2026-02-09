package tests

import (
	"os"
	"testing"

	"statis/internal"
)

func TestParseFeedRSS(t *testing.T) {
	data, err := os.ReadFile("testdata/rss_feed.xml")
	if err != nil {
		t.Fatalf("Failed to read test RSS feed: %v", err)
	}

	items, err := internal.ParseFeed(data)
	if err != nil {
		t.Fatalf("Failed to parse RSS feed: %v", err)
	}

	if len(items) != 2 {
		t.Fatalf("Expected 2 items, got %d", len(items))
	}

	// Check first item
	if items[0].Title != "Test Article 1" {
		t.Errorf("Expected title 'Test Article 1', got '%s'", items[0].Title)
	}

	if items[0].Link != "https://example.com/article1" {
		t.Errorf("Expected link 'https://example.com/article1', got '%s'", items[0].Link)
	}

	// First item has enclosure image
	if items[0].Image != "https://example.com/image1.jpg" {
		t.Errorf("Expected image from enclosure, got '%s'", items[0].Image)
	}

	// Second item has inline image
	if items[1].Image != "https://example.com/inline.jpg" {
		t.Errorf("Expected inline image, got '%s'", items[1].Image)
	}
}

func TestParseFeedAtom(t *testing.T) {
	data, err := os.ReadFile("testdata/atom_feed.xml")
	if err != nil {
		t.Fatalf("Failed to read test Atom feed: %v", err)
	}

	items, err := internal.ParseFeed(data)
	if err != nil {
		t.Fatalf("Failed to parse Atom feed: %v", err)
	}

	if len(items) != 2 {
		t.Fatalf("Expected 2 items, got %d", len(items))
	}

	// Check first item
	if items[0].Title != "Atom Article 1" {
		t.Errorf("Expected title 'Atom Article 1', got '%s'", items[0].Title)
	}

	// First item should use alternate link
	if items[0].Link != "https://example.com/atom1" {
		t.Errorf("Expected link 'https://example.com/atom1', got '%s'", items[0].Link)
	}

	// Second item has enclosure image
	if items[1].Image != "https://example.com/atom2-image.png" {
		t.Errorf("Expected enclosure image, got '%s'", items[1].Image)
	}
}

func TestParseFeedInvalid(t *testing.T) {
	invalidData := []byte(`<invalid>not a feed</invalid>`)

	_, err := internal.ParseFeed(invalidData)
	if err == nil {
		t.Error("Expected error for invalid feed, got nil")
	}
}

func TestStripHTMLTags(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"<p>Hello</p>", "Hello"},
		{"<div><span>Nested</span></div>", "Nested"},
		{"No tags here", "No tags here"},
		{"<a href='#'>Link</a> text", "Link text"},
		{"<br><hr>", ""},
		{"", ""},
	}

	for _, tc := range tests {
		result := internal.StripHTMLTags(tc.input)
		if result != tc.expected {
			t.Errorf("StripHTMLTags(%q) = %q, expected %q", tc.input, result, tc.expected)
		}
	}
}

func TestExtractFirstImageURL(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`<img src="https://example.com/image.jpg">`, "https://example.com/image.jpg"},
		{`<img src='https://example.com/single.png'>`, "https://example.com/single.png"},
		{`Some text <img src="https://example.com/mid.gif"> more text`, "https://example.com/mid.gif"},
		{`No image here`, ""},
		{`<IMG SRC="https://example.com/upper.jpg">`, "https://example.com/upper.jpg"},
		{``, ""},
	}

	for _, tc := range tests {
		result := internal.ExtractFirstImageURL(tc.input)
		if result != tc.expected {
			t.Errorf("ExtractFirstImageURL(%q) = %q, expected %q", tc.input, result, tc.expected)
		}
	}
}
