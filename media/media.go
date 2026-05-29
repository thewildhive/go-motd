package media

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"motd/config"
	"motd/display"
)

type plexSessionsResponse struct {
	Size   int `xml:"size,attr"`
	Videos []struct {
		TranscodeSession struct {
			VideoDecision string `xml:"videoDecision,attr"`
		} `xml:"TranscodeSession"`
		Session struct {
			Bandwidth int `xml:"bandwidth,attr"`
		} `xml:"Session"`
	} `xml:"Video"`
}

type jellyfinSession struct {
	NowPlayingItem  json.RawMessage `json:"NowPlayingItem"`
	TranscodingInfo *struct {
		Bitrate int64 `json:"Bitrate"`
	} `json:"TranscodingInfo,omitempty"`
	PlayState struct {
		PlayMethod string `json:"PlayMethod"`
	} `json:"PlayState"`
}

type arrWantedMissingResponse struct {
	TotalRecords int               `json:"totalRecords"`
	Records      []json.RawMessage `json:"records"`
}

// wantedRecord is a minimal view of a single record from wanted/missing
// to check availability without decoding the full movie object.
type wantedRecord struct {
	Status      string `json:"status"`
	IsAvailable bool   `json:"isAvailable"`
}

type seerrRequestCountResponse struct {
	Pending int `json:"pending"`
}

type mediaStatus struct {
	order int
	line  string
}

func HasMediaServices(cfg config.Config) bool {
	for _, plex := range cfg.Services.Plex {
		if isPlexReady(plex) {
			return true
		}
	}

	for _, jellyfin := range cfg.Services.Jellyfin {
		if isJellyfinReady(jellyfin) {
			return true
		}
	}

	for _, sonarr := range cfg.Services.Sonarr {
		if isAPIServiceReady(sonarr) {
			return true
		}
	}

	for _, radarr := range cfg.Services.Radarr {
		if isAPIServiceReady(radarr) {
			return true
		}
	}

	for _, seerr := range cfg.Services.Seerr {
		if isAPIServiceReady(seerr) {
			return true
		}
	}

	return false
}

func ShowMediaServices(cfg config.Config, client *http.Client, debug bool) {
	results := collectMediaStatuses(cfg, client, debug)
	if len(results) == 0 {
		return
	}

	display.PrintSection("Media Services")
	for _, result := range results {
		fmt.Print(result.line)
	}
}

func collectMediaStatuses(cfg config.Config, client *http.Client, debug bool) []mediaStatus {
	var wg sync.WaitGroup
	resultCapacity := len(cfg.Services.Plex) + len(cfg.Services.Jellyfin) + len(cfg.Services.Sonarr) + len(cfg.Services.Radarr) + len(cfg.Services.Seerr)
	results := make(chan mediaStatus, resultCapacity)
	order := 0

	start := func(fn func() (string, bool)) {
		currentOrder := order
		order++
		wg.Add(1)
		go func() {
			defer wg.Done()
			line, ok := fn()
			if ok {
				results <- mediaStatus{order: currentOrder, line: line}
			}
		}()
	}

	for _, plex := range cfg.Services.Plex {
		if isPlexReady(plex) {
			service := plex
			start(func() (string, bool) { return renderPlexInstance(service, client, debug) })
		}
	}

	for _, jellyfin := range cfg.Services.Jellyfin {
		if isJellyfinReady(jellyfin) {
			service := jellyfin
			start(func() (string, bool) { return renderJellyfinInstance(service, client, debug) })
		}
	}

	for _, sonarr := range cfg.Services.Sonarr {
		if isAPIServiceReady(sonarr) {
			service := sonarr
			start(func() (string, bool) { return renderSonarrInstance(service, client, debug) })
		}
	}

	for _, radarr := range cfg.Services.Radarr {
		if isAPIServiceReady(radarr) {
			service := radarr
			start(func() (string, bool) { return renderRadarrInstance(service, client, debug) })
		}
	}

	for _, seerr := range cfg.Services.Seerr {
		if isAPIServiceReady(seerr) {
			service := seerr
			start(func() (string, bool) { return renderSeerrInstance(service, client, debug) })
		}
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	collected := make([]mediaStatus, 0, order)
	for result := range results {
		collected = append(collected, result)
	}

	for i := 1; i < len(collected); i++ {
		for j := i; j > 0 && collected[j-1].order > collected[j].order; j-- {
			collected[j-1], collected[j] = collected[j], collected[j-1]
		}
	}

	return collected
}

func isValidURL(rawURL string) bool {
	if rawURL == "" {
		return false
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme == "" {
		return false
	}
	return true
}

func isPlexReady(plex config.ServiceConfig) bool {
	return plex.Enabled && isValidURL(plex.URL) && plex.Token != ""
}

func isJellyfinReady(jellyfin config.ServiceConfig) bool {
	return jellyfin.Enabled && isValidURL(jellyfin.URL) && jellyfin.Token != ""
}

func isAPIServiceReady(service config.ServiceConfig) bool {
	return service.Enabled && isValidURL(service.URL) && service.APIKey != ""
}

func formatMediaLine(label, text, color string) string {
	const dotLabelWidth = 22
	dots := dotLabelWidth - len(label)
	if dots < 0 {
		dots = 0
	}
	return fmt.Sprintf("%s%s: %s%s%s\n", label, strings.Repeat(".", dots), color, text, display.Reset)
}

func serviceLabel(base, name string) string {
	if name != "" && name != "Default" {
		return fmt.Sprintf("%s (%s)", base, name)
	}
	return base
}

func serviceURL(baseURL, path string) string {
	return strings.TrimRight(baseURL, "/") + path
}

func hasNowPlayingItem(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	return strings.TrimSpace(string(raw)) != "null"
}

func decodeJSONResponse(resp *http.Response, target interface{}) error {
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(target); err != nil {
		return err
	}

	return nil
}

func parseARRMissingCount(data arrWantedMissingResponse) int {
	if data.TotalRecords > 0 {
		return data.TotalRecords
	}
	return len(data.Records)
}

// countAvailableRecords returns only records that are released or available.
// This provides client-side filtering as a fallback for Radarr versions
// that don't fully support the excludeUnavailable server-side parameter.
func countAvailableRecords(records []json.RawMessage) int {
	count := 0
	for _, raw := range records {
		var rec wantedRecord
		if err := json.Unmarshal(raw, &rec); err != nil {
			// If we can't parse it, count it to be safe
			count++
			continue
		}
		if rec.IsAvailable || rec.Status == "released" || rec.Status == "inCinemas" {
			count++
		}
	}
	return count
}

func parseJellyfinSessions(sessions []jellyfinSession) (int, int, float64, bool) {
	active := 0
	transcodes := 0
	totalBitrate := int64(0)
	hasBitrate := false

	for _, session := range sessions {
		if !hasNowPlayingItem(session.NowPlayingItem) {
			continue
		}

		active++
		if strings.EqualFold(session.PlayState.PlayMethod, "Transcode") {
			transcodes++
		}

		if session.TranscodingInfo != nil && session.TranscodingInfo.Bitrate > 0 {
			totalBitrate += session.TranscodingInfo.Bitrate
			hasBitrate = true
		}
	}

	mbps := float64(totalBitrate) / 1000000.0
	return active, transcodes, mbps, hasBitrate
}

func renderPlexInstance(plex config.ServiceConfig, client *http.Client, debug bool) (string, bool) {
	req, err := http.NewRequest("GET", serviceURL(plex.URL, "/status/sessions"), nil)
	if err != nil {
		display.DebugLog(debug, "Plex request failed for %s: %v", plex.Name, err)
		return "", false
	}
	req.Header.Set("X-Plex-Token", plex.Token)

	resp, err := client.Do(req)
	if err != nil {
		display.DebugLog(debug, "Plex request failed for %s: %v", plex.Name, err)
		return "", false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		display.DebugLog(debug, "Plex returned status %d for %s", resp.StatusCode, plex.Name)
		return "", false
	}

	var sessions plexSessionsResponse
	if err := xml.NewDecoder(resp.Body).Decode(&sessions); err != nil {
		display.DebugLog(debug, "Failed to parse Plex XML for %s: %v", plex.Name, err)
		return "", false
	}

	transcodes := 0
	bandwidth := 0
	for _, video := range sessions.Videos {
		if video.TranscodeSession.VideoDecision == "transcode" {
			transcodes++
		}
		bandwidth += video.Session.Bandwidth
	}

	label := serviceLabel("Plex", plex.Name)
	if sessions.Size == 0 {
		return formatMediaLine(label, "No active streams", display.Green), true
	}

	bwMbps := float64(bandwidth) / 1000.0
	if transcodes == 0 {
		return formatMediaLine(label, fmt.Sprintf("%d streams (%.2f Mbps)", sessions.Size, bwMbps), display.Yellow), true
	}

	return formatMediaLine(label, fmt.Sprintf("%d streams, %d transcodes (%.2f Mbps)", sessions.Size, transcodes, bwMbps), display.Red), true
}

func renderJellyfinInstance(jellyfin config.ServiceConfig, client *http.Client, debug bool) (string, bool) {
	req, err := http.NewRequest("GET", serviceURL(jellyfin.URL, "/Sessions"), nil)
	if err != nil {
		display.DebugLog(debug, "Jellyfin request build failed for %s: %v", jellyfin.Name, err)
		return "", false
	}
	req.Header.Set("X-Emby-Token", jellyfin.Token)
	req.Header.Set("Authorization", "MediaBrowser Token=\""+jellyfin.Token+"\"")

	resp, err := client.Do(req)
	if err != nil {
		display.DebugLog(debug, "Jellyfin request failed for %s: %v", jellyfin.Name, err)
		return "", false
	}
	defer resp.Body.Close()

	var sessions []jellyfinSession
	if err := decodeJSONResponse(resp, &sessions); err != nil {
		display.DebugLog(debug, "Failed to decode Jellyfin response for %s: %v", jellyfin.Name, err)
		return "", false
	}

	count, transcodes, bwMbps, hasBW := parseJellyfinSessions(sessions)
	label := serviceLabel("Jellyfin", jellyfin.Name)

	if count == 0 {
		return formatMediaLine(label, "No active streams", display.Green), true
	}

	if transcodes == 0 {
		if hasBW {
			return formatMediaLine(label, fmt.Sprintf("%d streams (%.2f Mbps)", count, bwMbps), display.Yellow), true
		}
		return formatMediaLine(label, fmt.Sprintf("%d streams", count), display.Yellow), true
	}

	if hasBW {
		return formatMediaLine(label, fmt.Sprintf("%d streams, %d transcodes (%.2f Mbps)", count, transcodes, bwMbps), display.Red), true
	}

	return formatMediaLine(label, fmt.Sprintf("%d streams, %d transcodes", count, transcodes), display.Red), true
}

func renderSonarrInstance(sonarr config.ServiceConfig, client *http.Client, debug bool) (string, bool) {
	req, err := http.NewRequest("GET", serviceURL(sonarr.URL, "/api/v3/wanted/missing"), nil)
	if err != nil {
		display.DebugLog(debug, "Sonarr request build failed for %s: %v", sonarr.Name, err)
		return "", false
	}
	req.Header.Set("X-Api-Key", sonarr.APIKey)

	resp, err := client.Do(req)
	if err != nil {
		display.DebugLog(debug, "Sonarr request failed for %s: %v", sonarr.Name, err)
		return "", false
	}
	defer resp.Body.Close()

	var result arrWantedMissingResponse
	if err := decodeJSONResponse(resp, &result); err != nil {
		display.DebugLog(debug, "Failed to decode Sonarr response for %s: %v", sonarr.Name, err)
		return "", false
	}

	count := parseARRMissingCount(result)
	label := serviceLabel("Sonarr", sonarr.Name)
	if count == 0 {
		return formatMediaLine(label, "No missing episodes", display.Green), true
	}

	return formatMediaLine(label, fmt.Sprintf("%d missing episode%s", count, pluralSuffix(count)), display.Yellow), true
}

func renderRadarrInstance(radarr config.ServiceConfig, client *http.Client, debug bool) (string, bool) {
	// excludeUnavailable filters out movies that haven't been released yet,
	// so only truly available-but-missing movies are counted.
	req, err := http.NewRequest("GET", serviceURL(radarr.URL, "/api/v3/wanted/missing?excludeUnavailable=true"), nil)
	if err != nil {
		display.DebugLog(debug, "Radarr request build failed for %s: %v", radarr.Name, err)
		return "", false
	}
	req.Header.Set("X-Api-Key", radarr.APIKey)

	resp, err := client.Do(req)
	if err != nil {
		display.DebugLog(debug, "Radarr request failed for %s: %v", radarr.Name, err)
		return "", false
	}
	defer resp.Body.Close()

	var result arrWantedMissingResponse
	if err := decodeJSONResponse(resp, &result); err != nil {
		display.DebugLog(debug, "Failed to decode Radarr response for %s: %v", radarr.Name, err)
		return "", false
	}

	count := countAvailableRecords(result.Records)
	label := serviceLabel("Radarr", radarr.Name)
	if count == 0 {
		return formatMediaLine(label, "No missing movies", display.Green), true
	}

	return formatMediaLine(label, fmt.Sprintf("%d missing movie%s", count, pluralSuffix(count)), display.Yellow), true
}

func renderSeerrInstance(seerr config.ServiceConfig, client *http.Client, debug bool) (string, bool) {
	pending, err := fetchSeerrPendingCount(client, seerr.URL, seerr.APIKey)
	if err != nil {
		display.DebugLog(debug, "Seerr request failed for %s: %v", seerr.Name, err)
		return "", false
	}

	label := serviceLabel("Seerr", seerr.Name)
	if pending == 0 {
		return formatMediaLine(label, "No pending requests", display.Green), true
	}

	return formatMediaLine(label, fmt.Sprintf("%d pending request%s", pending, pluralSuffix(pending)), display.Yellow), true
}

func fetchSeerrPendingCount(client *http.Client, baseURL, apiKey string) (int, error) {
	req, err := http.NewRequest("GET", serviceURL(baseURL, "/api/v1/request/count"), nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("X-Api-Key", apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var result seerrRequestCountResponse
	if err := decodeJSONResponse(resp, &result); err != nil {
		return 0, err
	}

	return result.Pending, nil
}

func pluralSuffix(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}
