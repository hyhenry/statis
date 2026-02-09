package internal

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Uptime Kuma proxy handler

func HandleUptimeKumaProxy(w http.ResponseWriter, r *http.Request) {
	// This proxies requests to Uptime Kuma's status page API
	// Fetches both monitor list and heartbeat data, then merges them

	kumaURL := r.URL.Query().Get("url")
	slug := r.URL.Query().Get("slug")

	if kumaURL == "" || slug == "" {
		http.Error(w, "Missing url or slug parameter", http.StatusBadRequest)
		return
	}

	// Fetch monitor list (names, groups, etc.)
	statusURL := kumaURL + "/api/status-page/" + slug
	statusResp, err := http.Get(statusURL)
	if err != nil {
		http.Error(w, "Failed to fetch status page", http.StatusBadGateway)
		return
	}
	defer statusResp.Body.Close()

	var statusData map[string]interface{}
	if err := json.NewDecoder(statusResp.Body).Decode(&statusData); err != nil {
		http.Error(w, "Failed to decode status page", http.StatusInternalServerError)
		return
	}

	// Fetch heartbeat data
	heartbeatURL := kumaURL + "/api/status-page/heartbeat/" + slug
	heartbeatResp, err := http.Get(heartbeatURL)
	if err != nil {
		http.Error(w, "Failed to fetch heartbeat data", http.StatusBadGateway)
		return
	}
	defer heartbeatResp.Body.Close()

	var heartbeatData map[string]interface{}
	if err := json.NewDecoder(heartbeatResp.Body).Decode(&heartbeatData); err != nil {
		http.Error(w, "Failed to decode heartbeat data", http.StatusInternalServerError)
		return
	}

	// Merge heartbeat data into status data
	statusData["heartbeatList"] = heartbeatData["heartbeatList"]
	statusData["uptimeList"] = heartbeatData["uptimeList"]

	WriteJSON(w, statusData)
}

// System stats types

type cpuTimes struct {
	Total uint64
	Idle  uint64
}

type systemStatsResponse struct {
	CPU    cpuStats    `json:"cpu"`
	Memory memoryStats `json:"memory"`
}

type cpuStats struct {
	UsagePercent float64 `json:"usage_percent"`
}

type memoryStats struct {
	TotalBytes     uint64  `json:"total_bytes"`
	AvailableBytes uint64  `json:"available_bytes"`
	UsedBytes      uint64  `json:"used_bytes"`
	UsedPercent    float64 `json:"used_percent"`
}

func HandleSystemStats(w http.ResponseWriter, r *http.Request) {
	if runtime.GOOS != "linux" {
		http.Error(w, "system stats supported on linux only", http.StatusNotImplemented)
		return
	}

	cpuUsage, err := getCPUUsagePercent()
	if err != nil {
		http.Error(w, "failed to read cpu usage", http.StatusInternalServerError)
		return
	}

	memTotal, memAvailable, err := getMemoryStats()
	if err != nil {
		http.Error(w, "failed to read memory stats", http.StatusInternalServerError)
		return
	}

	used := memTotal - memAvailable
	usedPercent := 0.0
	if memTotal > 0 {
		usedPercent = (float64(used) / float64(memTotal)) * 100
	}

	WriteJSON(w, systemStatsResponse{
		CPU:    cpuStats{UsagePercent: cpuUsage},
		Memory: memoryStats{TotalBytes: memTotal, AvailableBytes: memAvailable, UsedBytes: used, UsedPercent: usedPercent},
	})
}

func getCPUUsagePercent() (float64, error) {
	first, err := readCPUTimes()
	if err != nil {
		return 0, err
	}

	time.Sleep(200 * time.Millisecond)

	second, err := readCPUTimes()
	if err != nil {
		return 0, err
	}

	deltaTotal := second.Total - first.Total
	deltaIdle := second.Idle - first.Idle
	if deltaTotal == 0 {
		return 0, nil
	}

	usage := (float64(deltaTotal-deltaIdle) / float64(deltaTotal)) * 100
	return usage, nil
}

func readCPUTimes() (cpuTimes, error) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return cpuTimes{}, err
	}

	lines := strings.Split(string(data), "\n")
	if len(lines) == 0 {
		return cpuTimes{}, fmt.Errorf("empty /proc/stat")
	}

	fields := strings.Fields(lines[0])
	if len(fields) < 5 {
		return cpuTimes{}, fmt.Errorf("unexpected /proc/stat format")
	}

	var total uint64
	var idle uint64
	for i := 1; i < len(fields); i++ {
		value, err := strconv.ParseUint(fields[i], 10, 64)
		if err != nil {
			return cpuTimes{}, err
		}
		total += value
		if i == 4 || i == 5 { // idle + iowait
			idle += value
		}
	}

	return cpuTimes{Total: total, Idle: idle}, nil
}

func getMemoryStats() (uint64, uint64, error) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, 0, err
	}

	var totalKB uint64
	var availableKB uint64
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		switch fields[0] {
		case "MemTotal:":
			value, err := strconv.ParseUint(fields[1], 10, 64)
			if err != nil {
				return 0, 0, err
			}
			totalKB = value
		case "MemAvailable:":
			value, err := strconv.ParseUint(fields[1], 10, 64)
			if err != nil {
				return 0, 0, err
			}
			availableKB = value
		}
	}

	if totalKB == 0 {
		return 0, 0, fmt.Errorf("MemTotal not found")
	}
	if availableKB == 0 {
		return 0, 0, fmt.Errorf("MemAvailable not found")
	}

	return totalKB * 1024, availableKB * 1024, nil
}

// RSS feed types for parsing

type rssFeed struct {
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Items []rssItem `xml:"item"`
}

type rssItem struct {
	Title          string         `xml:"title"`
	Link           string         `xml:"link"`
	Description    string         `xml:"description"`
	PubDate        string         `xml:"pubDate"`
	Enclosure      rssEnclosure   `xml:"enclosure"`
	MediaContent   []mediaContent `xml:"http://search.yahoo.com/mrss/ content"`
	MediaThumbnail []mediaContent `xml:"http://search.yahoo.com/mrss/ thumbnail"`
}

type rssEnclosure struct {
	URL  string `xml:"url,attr"`
	Type string `xml:"type,attr"`
}

type mediaContent struct {
	URL  string `xml:"url,attr"`
	Type string `xml:"type,attr"`
}

// Atom feed types for parsing

type atomFeed struct {
	Entries []atomEntry `xml:"entry"`
}

type atomEntry struct {
	Title   string     `xml:"title"`
	Links   []atomLink `xml:"link"`
	Summary string     `xml:"summary"`
	Content string     `xml:"content"`
	Updated string     `xml:"updated"`
}

type atomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
	Type string `xml:"type,attr"`
}

type rssResponse struct {
	Items []rssResponseItem `json:"items"`
}

type RSSResponseItem struct {
	Title       string `json:"title"`
	Link        string `json:"link"`
	Description string `json:"description"`
	PubDate     string `json:"pub_date"`
	Image       string `json:"image"`
}

// Keep internal alias for compatibility
type rssResponseItem = RSSResponseItem

func HandleRSSWidget(w http.ResponseWriter, r *http.Request) {
	feedURL := r.URL.Query().Get("url")
	if feedURL == "" {
		http.Error(w, "Missing url parameter", http.StatusBadRequest)
		return
	}

	// Fetch the RSS feed
	resp, err := http.Get(feedURL)
	if err != nil {
		http.Error(w, "Failed to fetch RSS feed", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		http.Error(w, "RSS feed returned error", http.StatusBadGateway)
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "Failed to read RSS feed", http.StatusInternalServerError)
		return
	}

	items, err := ParseFeed(body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	WriteJSON(w, rssResponse{Items: items})
}

// ParseFeed parses RSS 2.0 or Atom feed data
func ParseFeed(body []byte) ([]RSSResponseItem, error) {
	// Try RSS 2.0 first
	var rss rssFeed
	if err := xml.Unmarshal(body, &rss); err == nil && len(rss.Channel.Items) > 0 {
		return ParseRSSItems(rss.Channel.Items), nil
	}

	// Try Atom
	var atom atomFeed
	if err := xml.Unmarshal(body, &atom); err == nil && len(atom.Entries) > 0 {
		return ParseAtomEntries(atom.Entries), nil
	}

	return nil, fmt.Errorf("failed to parse feed (not valid RSS or Atom)")
}

func ParseRSSItems(items []rssItem) []RSSResponseItem {
	result := make([]RSSResponseItem, 0, len(items))
	for _, item := range items {
		result = append(result, RSSResponseItem{
			Title:       item.Title,
			Link:        item.Link,
			Description: StripHTMLTags(item.Description),
			PubDate:     item.PubDate,
			Image:       extractRSSImage(item),
		})
	}
	return result
}

func ParseAtomEntries(entries []atomEntry) []RSSResponseItem {
	result := make([]RSSResponseItem, 0, len(entries))
	for _, entry := range entries {
		desc := entry.Summary
		if desc == "" {
			desc = entry.Content
		}
		imageURL := pickAtomImage(entry.Links)
		if imageURL == "" {
			imageURL = ExtractFirstImageURL(desc)
		}
		result = append(result, RSSResponseItem{
			Title:       entry.Title,
			Link:        pickAtomLink(entry.Links),
			Description: StripHTMLTags(desc),
			PubDate:     entry.Updated,
			Image:       imageURL,
		})
	}
	return result
}

// extractRSSImage extracts image URL from RSS item using multiple sources
func extractRSSImage(item rssItem) string {
	if item.Enclosure.URL != "" && strings.HasPrefix(item.Enclosure.Type, "image/") {
		return item.Enclosure.URL
	}
	if url := findFirstMediaURL(item.MediaContent); url != "" {
		return url
	}
	if url := findFirstMediaURL(item.MediaThumbnail); url != "" {
		return url
	}
	return ExtractFirstImageURL(item.Description)
}

// findFirstMediaURL returns the first non-empty URL from media content slice
func findFirstMediaURL(media []mediaContent) string {
	for _, m := range media {
		if m.URL != "" {
			return m.URL
		}
	}
	return ""
}

func pickAtomLink(links []atomLink) string {
	for _, link := range links {
		if link.Href == "" {
			continue
		}
		if link.Rel == "" || link.Rel == "alternate" {
			return link.Href
		}
	}
	if len(links) > 0 {
		return links[0].Href
	}
	return ""
}

func pickAtomImage(links []atomLink) string {
	for _, link := range links {
		if link.Href == "" {
			continue
		}
		if link.Rel == "enclosure" && strings.HasPrefix(link.Type, "image/") {
			return link.Href
		}
	}
	return ""
}

func ExtractFirstImageURL(s string) string {
	re := regexp.MustCompile(`(?i)<img[^>]+src=["']([^"']+)["']`)
	matches := re.FindStringSubmatch(s)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// StripHTMLTags removes HTML tags from a string
func StripHTMLTags(s string) string {
	re := regexp.MustCompile(`<[^>]*>`)
	return re.ReplaceAllString(s, "")
}
