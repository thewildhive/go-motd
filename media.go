package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"strings"
	"sync"
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

type seerrRequestCountResponse struct {
	Pending int `json:"pending"`
}

type mediaStatus struct {
	order int
	line  string
}

func hasMediaServices() bool {
	for _, plex := range config.Services.Plex {
		if isPlexReady(plex) {
			return true
		}
	}

	for _, jellyfin := range config.Services.Jellyfin {
		if isJellyfinReady(jellyfin) {
			return true
		}
	}

	for _, sonarr := range config.Services.Sonarr {
		if isAPIServiceReady(sonarr) {
			return true
		}
	}

	for _, radarr := range config.Services.Radarr {
		if isAPIServiceReady(radarr) {
			return true
		}
	}

	for _, seerr := range config.Services.Seerr {
		if isAPIServiceReady(seerr) {
			return true
		}
	}

	return false
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

func showMediaServices() {
	results := collectMediaStatuses()
	if len(results) == 0 {
		return
	}

	printSection("Media Services")
	for _, result := range results {
		fmt.Print(result.line)
	}
}

func collectMediaStatuses() []mediaStatus {
	var wg sync.WaitGroup
	resultCapacity := len(config.Services.Plex) + len(config.Services.Jellyfin) + len(config.Services.Sonarr) + len(config.Services.Radarr) + len(config.Services.Seerr)
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

	for _, plex := range config.Services.Plex {
		if isPlexReady(plex) {
			service := plex
			start(func() (string, bool) { return renderPlexInstance(service) })
		}
	}

	for _, jellyfin := range config.Services.Jellyfin {
		if isJellyfinReady(jellyfin) {
			service := jellyfin
			start(func() (string, bool) { return renderJellyfinInstance(service) })
		}
	}

	for _, sonarr := range config.Services.Sonarr {
		if isAPIServiceReady(sonarr) {
			service := sonarr
			start(func() (string, bool) { return renderSonarrInstance(service) })
		}
	}

	for _, radarr := range config.Services.Radarr {
		if isAPIServiceReady(radarr) {
			service := radarr
			start(func() (string, bool) { return renderRadarrInstance(service) })
		}
	}

	for _, seerr := range config.Services.Seerr {
		if isAPIServiceReady(seerr) {
			service := seerr
			start(func() (string, bool) { return renderSeerrInstance(service) })
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

func isPlexReady(plex ServiceConfig) bool {
	return plex.Enabled && plex.URL != "" && plex.Token != ""
}

func isJellyfinReady(jellyfin ServiceConfig) bool {
	return jellyfin.Enabled && jellyfin.URL != "" && jellyfin.Token != ""
}

func isAPIServiceReady(service ServiceConfig) bool {
	return service.Enabled && service.URL != "" && service.APIKey != ""
}

func formatMediaLine(label, text, color string) string {
	dots := DOT_LABEL_WIDTH - len(label)
	if dots < 0 {
		dots = 0
	}
	return fmt.Sprintf("%s%s: %s%s%s\n", label, strings.Repeat(".", dots), color, text, RESET)
}

func renderPlexInstance(plex ServiceConfig) (string, bool) {
	req, err := http.NewRequest("GET", serviceURL(plex.URL, "/status/sessions"), nil)
	if err != nil {
		debugLog("Plex request failed for %s: %v", plex.Name, err)
		return "", false
	}
	req.Header.Set("X-Plex-Token", plex.Token)

	resp, err := httpClient.Do(req)
	if err != nil {
		debugLog("Plex request failed for %s: %v", plex.Name, err)
		return "", false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		debugLog("Plex returned status %d for %s", resp.StatusCode, plex.Name)
		return "", false
	}

	var sessions plexSessionsResponse
	if err := xml.NewDecoder(resp.Body).Decode(&sessions); err != nil {
		debugLog("Failed to parse Plex XML for %s: %v", plex.Name, err)
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
		return formatMediaLine(label, "No active streams", GREEN), true
	}

	bwMbps := float64(bandwidth) / 1000.0
	if transcodes == 0 {
		return formatMediaLine(label, fmt.Sprintf("%d streams (%.2f Mbps)", sessions.Size, bwMbps), YELLOW), true
	}

	return formatMediaLine(label, fmt.Sprintf("%d streams, %d transcodes (%.2f Mbps)", sessions.Size, transcodes, bwMbps), RED), true
}

func renderJellyfinInstance(jellyfin ServiceConfig) (string, bool) {
	req, err := http.NewRequest("GET", serviceURL(jellyfin.URL, "/Sessions"), nil)
	if err != nil {
		debugLog("Jellyfin request build failed for %s: %v", jellyfin.Name, err)
		return "", false
	}
	req.Header.Set("X-Emby-Token", jellyfin.Token)
	req.Header.Set("Authorization", "MediaBrowser Token=\""+jellyfin.Token+"\"")

	resp, err := httpClient.Do(req)
	if err != nil {
		debugLog("Jellyfin request failed for %s: %v", jellyfin.Name, err)
		return "", false
	}
	defer resp.Body.Close()

	var sessions []jellyfinSession
	if err := decodeJSONResponse(resp, &sessions); err != nil {
		debugLog("Failed to decode Jellyfin response for %s: %v", jellyfin.Name, err)
		return "", false
	}

	count, transcodes, bwMbps, hasBW := parseJellyfinSessions(sessions)
	label := serviceLabel("Jellyfin", jellyfin.Name)

	if count == 0 {
		return formatMediaLine(label, "No active streams", GREEN), true
	}

	if transcodes == 0 {
		if hasBW {
			return formatMediaLine(label, fmt.Sprintf("%d streams (%.2f Mbps)", count, bwMbps), YELLOW), true
		}
		return formatMediaLine(label, fmt.Sprintf("%d streams", count), YELLOW), true
	}

	if hasBW {
		return formatMediaLine(label, fmt.Sprintf("%d streams, %d transcodes (%.2f Mbps)", count, transcodes, bwMbps), RED), true
	}

	return formatMediaLine(label, fmt.Sprintf("%d streams, %d transcodes", count, transcodes), RED), true
}

func renderSonarrInstance(sonarr ServiceConfig) (string, bool) {
	req, err := http.NewRequest("GET", serviceURL(sonarr.URL, "/api/v3/wanted/missing"), nil)
	if err != nil {
		debugLog("Sonarr request build failed for %s: %v", sonarr.Name, err)
		return "", false
	}
	req.Header.Set("X-Api-Key", sonarr.APIKey)

	resp, err := httpClient.Do(req)
	if err != nil {
		debugLog("Sonarr request failed for %s: %v", sonarr.Name, err)
		return "", false
	}
	defer resp.Body.Close()

	var result arrWantedMissingResponse
	if err := decodeJSONResponse(resp, &result); err != nil {
		debugLog("Failed to decode Sonarr response for %s: %v", sonarr.Name, err)
		return "", false
	}

	count := parseARRMissingCount(result)
	label := serviceLabel("Sonarr", sonarr.Name)
	if count == 0 {
		return formatMediaLine(label, "No missing episodes", GREEN), true
	}

	return formatMediaLine(label, fmt.Sprintf("%d missing episode%s", count, pluralSuffix(count)), YELLOW), true
}

func renderRadarrInstance(radarr ServiceConfig) (string, bool) {
	req, err := http.NewRequest("GET", serviceURL(radarr.URL, "/api/v3/wanted/missing"), nil)
	if err != nil {
		debugLog("Radarr request build failed for %s: %v", radarr.Name, err)
		return "", false
	}
	req.Header.Set("X-Api-Key", radarr.APIKey)

	resp, err := httpClient.Do(req)
	if err != nil {
		debugLog("Radarr request failed for %s: %v", radarr.Name, err)
		return "", false
	}
	defer resp.Body.Close()

	var result arrWantedMissingResponse
	if err := decodeJSONResponse(resp, &result); err != nil {
		debugLog("Failed to decode Radarr response for %s: %v", radarr.Name, err)
		return "", false
	}

	count := parseARRMissingCount(result)
	label := serviceLabel("Radarr", radarr.Name)
	if count == 0 {
		return formatMediaLine(label, "No missing movies", GREEN), true
	}

	return formatMediaLine(label, fmt.Sprintf("%d missing movie%s", count, pluralSuffix(count)), YELLOW), true
}

func renderSeerrInstance(seerr ServiceConfig) (string, bool) {
	pending, err := fetchSeerrPendingCount(httpClient, seerr.URL, seerr.APIKey)
	if err != nil {
		debugLog("Seerr request failed for %s: %v", seerr.Name, err)
		return "", false
	}

	label := serviceLabel("Seerr", seerr.Name)
	if pending == 0 {
		return formatMediaLine(label, "No pending requests", GREEN), true
	}

	return formatMediaLine(label, fmt.Sprintf("%d pending request%s", pending, pluralSuffix(pending)), YELLOW), true
}
