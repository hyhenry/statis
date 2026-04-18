package internal

import (
	"crypto/tls"
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
	"sync"
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

// TrueNAS SCALE widget

type truenasSystem struct {
	Hostname      string  `json:"hostname"`
	Version       string  `json:"version"`
	UptimeSeconds float64 `json:"uptime_seconds"`
	MemoryBytes   uint64  `json:"memory_bytes"`
	CPUModel      string  `json:"cpu_model"`
	CPUCores      int     `json:"cpu_cores"`
}

type truenasPool struct {
	Name         string  `json:"name"`
	Status       string  `json:"status"`
	StatusDetail string  `json:"status_detail,omitempty"`
	Healthy      bool    `json:"healthy"`
	SizeBytes    uint64  `json:"size_bytes"`
	AllocBytes   uint64  `json:"allocated_bytes"`
	FreeBytes    uint64  `json:"free_bytes"`
	UsedPercent  float64 `json:"used_percent"`

	// Aggregate per-vdev error counts from the pool's topology (read + write
	// + checksum errors across all data vdevs). Non-zero means ZFS has seen
	// problems even if the pool is still ONLINE.
	ReadErrors     uint64 `json:"read_errors"`
	WriteErrors    uint64 `json:"write_errors"`
	ChecksumErrors uint64 `json:"checksum_errors"`

	// Last scrub/resilver summary.
	ScanFunction string `json:"scan_function,omitempty"`
	ScanState    string `json:"scan_state,omitempty"`
	ScanErrors   uint64 `json:"scan_errors"`
	ScanEndTime  int64  `json:"scan_end_time,omitempty"` // unix seconds, 0 if unknown
	ScanPercent  float64 `json:"scan_percent,omitempty"`
}

type truenasDisk struct {
	Name        string `json:"name"`
	Model       string `json:"model"`
	Serial      string `json:"serial"`
	SizeBytes   uint64 `json:"size_bytes"`
	Type        string `json:"type"`
	Temperature int    `json:"temperature,omitempty"`
}

// truenasBackup is a unified view of a cloud sync or rsync task with its
// most recent job run. TrueNAS exposes these as separate endpoints but the
// UI treats them the same — a scheduled transfer with a last-run state.
type truenasBackup struct {
	ID             int     `json:"id"`
	Kind           string  `json:"kind"` // "cloudsync" | "rsync"
	Description    string  `json:"description"`
	Direction      string  `json:"direction,omitempty"` // PUSH / PULL
	Enabled        bool    `json:"enabled"`
	State          string  `json:"state"` // SUCCESS / FAILED / RUNNING / WAITING / NEVER
	LastRunUnix    int64   `json:"last_run_unix,omitempty"`
	ProgressPct    float64 `json:"progress_percent,omitempty"`
	Error          string  `json:"error,omitempty"`
}

// truenasClient is shared for the handler; self-signed certs are common on
// homelab TrueNAS boxes, so verification is off.
var truenasClient = &http.Client{
	Timeout: 10 * time.Second,
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	},
}

func fetchTrueNASJSON(url, apiKey string, target interface{}) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "application/json")
	resp, err := truenasClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return fmt.Errorf("upstream status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return json.NewDecoder(resp.Body).Decode(target)
}

// HandleTrueNASSCALEWidget proxies TrueNAS SCALE REST API (v2.0) calls,
// fetching only the sections the client opted in to. Any single failing
// section is returned as a "<section>_error" field rather than failing the
// whole response, so a borked disk endpoint doesn't hide pool status.
func HandleTrueNASSCALEWidget(w http.ResponseWriter, r *http.Request) {
	baseURL := strings.TrimRight(r.URL.Query().Get("url"), "/")
	apiKey := r.URL.Query().Get("api_key")
	if baseURL == "" || apiKey == "" {
		http.Error(w, "Missing url or api_key parameter", http.StatusBadRequest)
		return
	}

	showSystem := r.URL.Query().Get("show_system") != "false"
	showPools := r.URL.Query().Get("show_pools") != "false"
	showDisks := r.URL.Query().Get("show_disks") != "false"
	showBackups := r.URL.Query().Get("show_backups") != "false"

	response := map[string]interface{}{}
	var mu sync.Mutex
	setField := func(key string, value interface{}) {
		mu.Lock()
		response[key] = value
		mu.Unlock()
	}

	var wg sync.WaitGroup

	if showSystem {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var raw map[string]interface{}
			if err := fetchTrueNASJSON(baseURL+"/api/v2.0/system/info", apiKey, &raw); err != nil {
				setField("system_error", err.Error())
				return
			}
			setField("system", parseTrueNASSystem(raw))
		}()
	}

	if showPools {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var raw []map[string]interface{}
			if err := fetchTrueNASJSON(baseURL+"/api/v2.0/pool", apiKey, &raw); err != nil {
				setField("pools_error", err.Error())
				return
			}
			setField("pools", parseTrueNASPools(raw))
		}()
	}

	if showDisks {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var raw []map[string]interface{}
			if err := fetchTrueNASJSON(baseURL+"/api/v2.0/disk", apiKey, &raw); err != nil {
				setField("disks_error", err.Error())
				return
			}
			setField("disks", parseTrueNASDisks(raw))
		}()
	}

	if showBackups {
		// Fetch both cloud sync and rsync task lists in parallel. Either may
		// fail independently (e.g., if the user doesn't have that feature
		// configured) — we merge whatever came back rather than giving up.
		wg.Add(1)
		go func() {
			defer wg.Done()
			var cloudsync, rsync []map[string]interface{}
			var cloudErr, rsyncErr error
			var innerWG sync.WaitGroup
			innerWG.Add(2)
			go func() {
				defer innerWG.Done()
				cloudErr = fetchTrueNASJSON(baseURL+"/api/v2.0/cloudsync", apiKey, &cloudsync)
			}()
			go func() {
				defer innerWG.Done()
				rsyncErr = fetchTrueNASJSON(baseURL+"/api/v2.0/rsynctask", apiKey, &rsync)
			}()
			innerWG.Wait()

			if cloudErr != nil && rsyncErr != nil {
				setField("backups_error", fmt.Sprintf("cloudsync: %v; rsync: %v", cloudErr, rsyncErr))
				return
			}
			backups := parseTrueNASBackups(cloudsync, rsync)
			setField("backups", backups)
		}()
	}

	wg.Wait()
	WriteJSON(w, response)
}

func parseTrueNASSystem(raw map[string]interface{}) truenasSystem {
	sys := truenasSystem{
		Hostname:      getString(raw, "hostname"),
		Version:       getString(raw, "version"),
		UptimeSeconds: getFloat(raw, "uptime_seconds"),
		MemoryBytes:   getUint64(raw, "physmem"),
		CPUModel:      getString(raw, "model"),
		CPUCores:      int(getFloat(raw, "cores")),
	}
	return sys
}

func parseTrueNASPools(raw []map[string]interface{}) []truenasPool {
	pools := make([]truenasPool, 0, len(raw))
	for _, p := range raw {
		size := getUint64Nested(p, "topology", "data") // fallback below
		if size == 0 {
			size = getUint64(p, "size")
		}
		alloc := getUint64(p, "allocated")
		if alloc == 0 {
			alloc = getUint64(p, "used")
		}
		free := getUint64(p, "free")
		if size == 0 && alloc > 0 && free > 0 {
			size = alloc + free
		}
		var usedPct float64
		if size > 0 {
			usedPct = float64(alloc) / float64(size) * 100
		}
		status := getString(p, "status")
		read, write, cksum := sumVdevErrors(p["topology"])
		scanFn, scanState, scanErrors, scanEnd, scanPct := parseScan(p["scan"])
		pools = append(pools, truenasPool{
			Name:           getString(p, "name"),
			Status:         status,
			StatusDetail:   getString(p, "status_detail"),
			Healthy:        getBool(p, "healthy") || strings.EqualFold(status, "ONLINE"),
			SizeBytes:      size,
			AllocBytes:     alloc,
			FreeBytes:      free,
			UsedPercent:    usedPct,
			ReadErrors:     read,
			WriteErrors:    write,
			ChecksumErrors: cksum,
			ScanFunction:   scanFn,
			ScanState:      scanState,
			ScanErrors:     scanErrors,
			ScanEndTime:    scanEnd,
			ScanPercent:    scanPct,
		})
	}
	return pools
}

// sumVdevErrors walks a pool's "topology" object and sums read_errors,
// write_errors, and checksum_errors across every vdev in every group
// (data, log, cache, spare). ZFS reports these at each vdev level, so a
// healthy-looking ONLINE pool can still have accumulated errors worth
// surfacing.
func sumVdevErrors(topology interface{}) (read, write, cksum uint64) {
	topo, ok := topology.(map[string]interface{})
	if !ok {
		return
	}
	for _, groupVal := range topo {
		group, ok := groupVal.([]interface{})
		if !ok {
			continue
		}
		for _, vdevVal := range group {
			r, w, c := walkVdevErrors(vdevVal)
			read += r
			write += w
			cksum += c
		}
	}
	return
}

func walkVdevErrors(vdev interface{}) (read, write, cksum uint64) {
	v, ok := vdev.(map[string]interface{})
	if !ok {
		return
	}
	if stats, ok := v["stats"].(map[string]interface{}); ok {
		read += getUint64(stats, "read_errors")
		write += getUint64(stats, "write_errors")
		cksum += getUint64(stats, "checksum_errors")
	}
	if children, ok := v["children"].([]interface{}); ok {
		for _, child := range children {
			r, w, c := walkVdevErrors(child)
			read += r
			write += w
			cksum += c
		}
	}
	return
}

// parseScan extracts fields from pool.scan. TrueNAS wraps timestamps as
// {"$date": <epoch-ms>}, so we unwrap that shape for end_time.
func parseScan(scan interface{}) (function, state string, errors uint64, endUnix int64, percent float64) {
	s, ok := scan.(map[string]interface{})
	if !ok {
		return
	}
	function = getString(s, "function")
	state = getString(s, "state")
	errors = getUint64(s, "errors")
	percent = getFloat(s, "percentage")
	if end, ok := s["end_time"].(map[string]interface{}); ok {
		if ms, ok := end["$date"].(float64); ok {
			endUnix = int64(ms / 1000)
		}
	} else if ts := getString(s, "end_time"); ts != "" {
		if t, err := time.Parse(time.RFC3339, ts); err == nil {
			endUnix = t.Unix()
		}
	}
	return
}

func parseTrueNASDisks(raw []map[string]interface{}) []truenasDisk {
	disks := make([]truenasDisk, 0, len(raw))
	for _, d := range raw {
		disks = append(disks, truenasDisk{
			Name:        getString(d, "name"),
			Model:       getString(d, "model"),
			Serial:      getString(d, "serial"),
			SizeBytes:   getUint64(d, "size"),
			Type:        getString(d, "type"),
			Temperature: int(getFloat(d, "temperature")),
		})
	}
	return disks
}

func parseTrueNASBackups(cloudsync, rsync []map[string]interface{}) []truenasBackup {
	backups := make([]truenasBackup, 0, len(cloudsync)+len(rsync))
	for _, task := range cloudsync {
		desc := getString(task, "description")
		if desc == "" {
			// Cloud sync tasks fall back to "<path> → <remote>".
			desc = strings.TrimSpace(getString(task, "path"))
		}
		backups = append(backups, parseBackupTask(task, "cloudsync", desc))
	}
	for _, task := range rsync {
		// rsync tasks use "desc" field (not "description") in some versions;
		// also fall back to path.
		desc := getString(task, "description")
		if desc == "" {
			desc = getString(task, "desc")
		}
		if desc == "" {
			path := getString(task, "path")
			remote := getString(task, "remotepath")
			desc = strings.TrimSpace(path + " → " + remote)
			if desc == "→" {
				desc = "rsync task"
			}
		}
		backups = append(backups, parseBackupTask(task, "rsync", desc))
	}
	return backups
}

func parseBackupTask(task map[string]interface{}, kind, desc string) truenasBackup {
	b := truenasBackup{
		ID:          int(getFloat(task, "id")),
		Kind:        kind,
		Description: desc,
		Direction:   getString(task, "direction"),
		Enabled:     getBool(task, "enabled"),
		State:       "NEVER",
	}

	job, ok := task["job"].(map[string]interface{})
	if !ok || job == nil {
		return b
	}

	b.State = strings.ToUpper(getString(job, "state"))
	if b.State == "" {
		b.State = "NEVER"
	}
	b.Error = getString(job, "error")

	if prog, ok := job["progress"].(map[string]interface{}); ok {
		b.ProgressPct = getFloat(prog, "percent")
	}

	// TrueNAS hands time_finished / time_started in the "$date": epoch-ms
	// wrapper. We prefer finished for completed jobs, fall back to started
	// for running ones so the UI can still show "started X ago".
	if finished := unwrapDateField(job["time_finished"]); finished != 0 {
		b.LastRunUnix = finished
	} else if started := unwrapDateField(job["time_started"]); started != 0 {
		b.LastRunUnix = started
	}
	return b
}

func unwrapDateField(v interface{}) int64 {
	switch x := v.(type) {
	case map[string]interface{}:
		if ms, ok := x["$date"].(float64); ok {
			return int64(ms / 1000)
		}
	case string:
		if t, err := time.Parse(time.RFC3339, x); err == nil {
			return t.Unix()
		}
	case float64:
		return int64(x / 1000)
	}
	return 0
}

// Small typed accessors for the loosely-typed TrueNAS JSON payloads.

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getFloat(m map[string]interface{}, key string) float64 {
	switch v := m[key].(type) {
	case float64:
		return v
	case string:
		f, _ := strconv.ParseFloat(v, 64)
		return f
	}
	return 0
}

func getUint64(m map[string]interface{}, key string) uint64 {
	switch v := m[key].(type) {
	case float64:
		if v < 0 {
			return 0
		}
		return uint64(v)
	case string:
		n, _ := strconv.ParseUint(v, 10, 64)
		return n
	}
	return 0
}

func getUint64Nested(m map[string]interface{}, keys ...string) uint64 {
	cur := m
	for i, k := range keys {
		v, ok := cur[k]
		if !ok {
			return 0
		}
		if i == len(keys)-1 {
			if n, ok := v.(float64); ok {
				return uint64(n)
			}
			return 0
		}
		cur, ok = v.(map[string]interface{})
		if !ok {
			return 0
		}
	}
	return 0
}

func getBool(m map[string]interface{}, key string) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return false
}
