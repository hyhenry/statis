package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
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

// --- Widget Handler Tests ---

func TestHandleUptimeKumaProxy_MissingURL(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/widget/uptime-kuma?slug=test", nil)
	recorder := httptest.NewRecorder()

	internal.HandleUptimeKumaProxy(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", recorder.Code)
	}

	if recorder.Body.String() != "Missing url or slug parameter\n" {
		t.Errorf("Unexpected error message: %s", recorder.Body.String())
	}
}

func TestHandleUptimeKumaProxy_MissingSlug(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/widget/uptime-kuma?url=http://localhost:3001", nil)
	recorder := httptest.NewRecorder()

	internal.HandleUptimeKumaProxy(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", recorder.Code)
	}
}

func TestHandleUptimeKumaProxy_MissingBothParams(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/widget/uptime-kuma", nil)
	recorder := httptest.NewRecorder()

	internal.HandleUptimeKumaProxy(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", recorder.Code)
	}
}

func TestHandleUptimeKumaProxy_InvalidURL(t *testing.T) {
	// Use an invalid URL that will fail to connect
	req := httptest.NewRequest(http.MethodGet, "/api/widget/uptime-kuma?url=http://invalid-host-that-does-not-exist.local:9999&slug=test", nil)
	recorder := httptest.NewRecorder()

	internal.HandleUptimeKumaProxy(recorder, req)

	if recorder.Code != http.StatusBadGateway {
		t.Errorf("Expected status 502, got %d", recorder.Code)
	}
}

func TestHandleUptimeKumaProxy_WithMockServer(t *testing.T) {
	// Create mock Uptime Kuma server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/status-page/test-slug":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"publicGroupList": []map[string]interface{}{
					{"id": 1, "name": "Services"},
				},
			})
		case "/api/status-page/heartbeat/test-slug":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"heartbeatList": map[string]interface{}{
					"1": []map[string]interface{}{{"status": 1}},
				},
				"uptimeList": map[string]interface{}{
					"1": 99.9,
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer mockServer.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/widget/uptime-kuma?url="+mockServer.URL+"&slug=test-slug", nil)
	recorder := httptest.NewRecorder()

	internal.HandleUptimeKumaProxy(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", recorder.Code)
	}

	// Parse response
	var result map[string]interface{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Check merged data
	if result["publicGroupList"] == nil {
		t.Error("Expected publicGroupList in response")
	}
	if result["heartbeatList"] == nil {
		t.Error("Expected heartbeatList in response")
	}
	if result["uptimeList"] == nil {
		t.Error("Expected uptimeList in response")
	}
}

func TestHandleRSSWidget_MissingURL(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/widget/rss", nil)
	recorder := httptest.NewRecorder()

	internal.HandleRSSWidget(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", recorder.Code)
	}

	if recorder.Body.String() != "Missing url parameter\n" {
		t.Errorf("Unexpected error message: %s", recorder.Body.String())
	}
}

func TestHandleRSSWidget_InvalidURL(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/widget/rss?url=http://invalid-host-that-does-not-exist.local", nil)
	recorder := httptest.NewRecorder()

	internal.HandleRSSWidget(recorder, req)

	if recorder.Code != http.StatusBadGateway {
		t.Errorf("Expected status 502, got %d", recorder.Code)
	}
}

func TestHandleRSSWidget_WithMockServer(t *testing.T) {
	// Create mock RSS server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Test Feed</title>
    <item>
      <title>Test Article</title>
      <link>https://example.com/article</link>
      <description>Test description</description>
      <pubDate>Mon, 03 Feb 2025 12:00:00 GMT</pubDate>
    </item>
  </channel>
</rss>`))
	}))
	defer mockServer.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/widget/rss?url="+mockServer.URL, nil)
	recorder := httptest.NewRecorder()

	internal.HandleRSSWidget(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", recorder.Code)
	}

	// Parse response
	var result struct {
		Items []internal.RSSResponseItem `json:"items"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if len(result.Items) != 1 {
		t.Fatalf("Expected 1 item, got %d", len(result.Items))
	}

	if result.Items[0].Title != "Test Article" {
		t.Errorf("Expected title 'Test Article', got '%s'", result.Items[0].Title)
	}
}

func TestHandleRSSWidget_Non200Response(t *testing.T) {
	// Create mock server that returns 404
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer mockServer.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/widget/rss?url="+mockServer.URL, nil)
	recorder := httptest.NewRecorder()

	internal.HandleRSSWidget(recorder, req)

	if recorder.Code != http.StatusBadGateway {
		t.Errorf("Expected status 502, got %d", recorder.Code)
	}
}

func TestHandleRSSWidget_InvalidFeed(t *testing.T) {
	// Create mock server that returns invalid XML
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(`<invalid>not a feed</invalid>`))
	}))
	defer mockServer.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/widget/rss?url="+mockServer.URL, nil)
	recorder := httptest.NewRecorder()

	internal.HandleRSSWidget(recorder, req)

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", recorder.Code)
	}
}

func TestHandleRSSWidget_AtomFeed(t *testing.T) {
	// Create mock Atom server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/atom+xml")
		w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Atom Feed</title>
  <entry>
    <title>Atom Entry</title>
    <link href="https://example.com/atom-entry"/>
    <summary>Atom summary</summary>
    <updated>2025-02-03T12:00:00Z</updated>
  </entry>
</feed>`))
	}))
	defer mockServer.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/widget/rss?url="+mockServer.URL, nil)
	recorder := httptest.NewRecorder()

	internal.HandleRSSWidget(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", recorder.Code)
	}

	var result struct {
		Items []internal.RSSResponseItem `json:"items"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if len(result.Items) != 1 {
		t.Fatalf("Expected 1 item, got %d", len(result.Items))
	}

	if result.Items[0].Title != "Atom Entry" {
		t.Errorf("Expected title 'Atom Entry', got '%s'", result.Items[0].Title)
	}
}

func TestHandleSystemStats_NonLinux(t *testing.T) {
	if runtime.GOOS == "linux" {
		t.Skip("Skipping non-Linux test on Linux")
	}

	req := httptest.NewRequest(http.MethodGet, "/api/widget/system-stats", nil)
	recorder := httptest.NewRecorder()

	internal.HandleSystemStats(recorder, req)

	if recorder.Code != http.StatusNotImplemented {
		t.Errorf("Expected status 501, got %d", recorder.Code)
	}
}

// --- RSS Parsing Edge Cases ---

func TestParseRSSItems_Empty(t *testing.T) {
	result := internal.ParseRSSItems(nil)
	if len(result) != 0 {
		t.Errorf("Expected empty result, got %d items", len(result))
	}
}

func TestParseAtomEntries_Empty(t *testing.T) {
	result := internal.ParseAtomEntries(nil)
	if len(result) != 0 {
		t.Errorf("Expected empty result, got %d items", len(result))
	}
}

func TestParseFeed_EmptyRSS(t *testing.T) {
	data := []byte(`<?xml version="1.0"?>
<rss version="2.0">
  <channel>
    <title>Empty Feed</title>
  </channel>
</rss>`)

	_, err := internal.ParseFeed(data)
	// Empty RSS should fail and try Atom, then fail
	if err == nil {
		t.Error("Expected error for empty feed, got nil")
	}
}

func TestParseFeed_EmptyAtom(t *testing.T) {
	data := []byte(`<?xml version="1.0"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Empty Feed</title>
</feed>`)

	_, err := internal.ParseFeed(data)
	if err == nil {
		t.Error("Expected error for empty feed, got nil")
	}
}

func TestStripHTMLTags_ComplexHTML(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`<p style="color: red">Styled text</p>`, "Styled text"},
		{`<script>alert('xss')</script>`, "alert('xss')"},
		{`<div class="container"><p>Nested <strong>bold</strong></p></div>`, "Nested bold"},
		{`Text with &amp; entity`, "Text with &amp; entity"},
		{`<img src="x" onerror="alert(1)">`, ""},
	}

	for _, tc := range tests {
		result := internal.StripHTMLTags(tc.input)
		if result != tc.expected {
			t.Errorf("StripHTMLTags(%q) = %q, expected %q", tc.input, result, tc.expected)
		}
	}
}

func TestExtractFirstImageURL_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Multiple images", `<img src="first.jpg"><img src="second.jpg">`, "first.jpg"},
		{"Image with alt", `<img alt="description" src="https://example.com/img.png">`, "https://example.com/img.png"},
		{"Self-closing tag", `<img src="https://example.com/self.jpg"/>`, "https://example.com/self.jpg"},
		{"Data URI", `<img src="data:image/png;base64,abc123">`, "data:image/png;base64,abc123"},
		{"Relative path", `<img src="/images/local.png">`, "/images/local.png"},
		{"Empty src", `<img src="">`, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := internal.ExtractFirstImageURL(tc.input)
			if result != tc.expected {
				t.Errorf("ExtractFirstImageURL(%q) = %q, expected %q", tc.input, result, tc.expected)
			}
		})
	}
}
