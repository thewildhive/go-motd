package media

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"

	"motd/config"
	"motd/display"
	"motd/util"
)

const (
	maxMediaResponseSize     = 10 << 20
	maxMediaServicesPerType  = 32
	maxConcurrentMediaChecks = 8
)

type Service interface {
	Name() string
	Render(client *http.Client, debug bool) (text string, color string, ok bool)
}

type plexService struct {
	cfg config.ServiceConfig
}

func (s plexService) Name() string { return serviceLabel("Plex", s.cfg.Name) }

type jellyfinService struct {
	cfg config.ServiceConfig
}

func (s jellyfinService) Name() string { return serviceLabel("Jellyfin", s.cfg.Name) }

type sonarrService struct {
	cfg config.ServiceConfig
}

func (s sonarrService) Name() string { return serviceLabel("Sonarr", s.cfg.Name) }

type radarrService struct {
	cfg config.ServiceConfig
}

func (s radarrService) Name() string { return serviceLabel("Radarr", s.cfg.Name) }

type seerrService struct {
	cfg config.ServiceConfig
}

func (s seerrService) Name() string { return serviceLabel("Seerr", s.cfg.Name) }

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

type wantedRecord struct {
	IsAvailable bool `json:"isAvailable"`
}

type seerrRequestCountResponse struct {
	Pending int `json:"pending"`
}

type MediaStatus struct {
	Order int
	Name  string
	Text  string
	Color string
	Error string
}

func AllServices(cfg config.Config, selected map[string]bool) []Service {
	return allServices(cfg, selected, false)
}

func allServices(cfg config.Config, selected map[string]bool, debug bool) []Service {
	out := make([]Service, 0,
		cappedServiceCount(len(cfg.Services.Plex))+cappedServiceCount(len(cfg.Services.Jellyfin))+
			cappedServiceCount(len(cfg.Services.Sonarr))+cappedServiceCount(len(cfg.Services.Radarr))+
			cappedServiceCount(len(cfg.Services.Seerr)))

	for i := range cfg.Services.Plex {
		if !serviceSelected(selected, "plex") || i >= MaxMediaServicesPerType() {
			break
		}
		svc := cfg.Services.Plex[i]
		if reason := serviceSkipReason(svc, true); reason != "" {
			logSkippedService(debug, "Plex", svc, reason)
			continue
		}
		out = append(out, plexService{cfg: svc})
	}
	for i := range cfg.Services.Jellyfin {
		if !serviceSelected(selected, "jellyfin") || i >= MaxMediaServicesPerType() {
			break
		}
		svc := cfg.Services.Jellyfin[i]
		if reason := serviceSkipReason(svc, true); reason != "" {
			logSkippedService(debug, "Jellyfin", svc, reason)
			continue
		}
		out = append(out, jellyfinService{cfg: svc})
	}
	for i := range cfg.Services.Sonarr {
		if !serviceSelected(selected, "sonarr") || i >= MaxMediaServicesPerType() {
			break
		}
		svc := cfg.Services.Sonarr[i]
		if reason := serviceSkipReason(svc, false); reason != "" {
			logSkippedService(debug, "Sonarr", svc, reason)
			continue
		}
		out = append(out, sonarrService{cfg: svc})
	}
	for i := range cfg.Services.Radarr {
		if !serviceSelected(selected, "radarr") || i >= MaxMediaServicesPerType() {
			break
		}
		svc := cfg.Services.Radarr[i]
		if reason := serviceSkipReason(svc, false); reason != "" {
			logSkippedService(debug, "Radarr", svc, reason)
			continue
		}
		out = append(out, radarrService{cfg: svc})
	}
	for i := range cfg.Services.Seerr {
		if !serviceSelected(selected, "seerr") || i >= MaxMediaServicesPerType() {
			break
		}
		svc := cfg.Services.Seerr[i]
		if reason := serviceSkipReason(svc, false); reason != "" {
			logSkippedService(debug, "Seerr", svc, reason)
			continue
		}
		out = append(out, seerrService{cfg: svc})
	}
	return out
}

func MaxMediaServicesPerType() int {
	return maxMediaServicesPerType
}

func MaxConcurrentMediaChecks() int {
	return maxConcurrentMediaChecks
}

func cappedServiceCount(count int) int {
	if count > MaxMediaServicesPerType() {
		return MaxMediaServicesPerType()
	}
	return count
}

func serviceSelected(selected map[string]bool, name string) bool {
	return len(selected) == 0 || selected[name]
}

func HasMediaServices(cfg config.Config, selected map[string]bool) bool {
	return len(AllServices(cfg, selected)) > 0
}

func ShowMediaServices(cfg config.Config, selected map[string]bool, client *http.Client, debug bool) {
	services := allServices(cfg, selected, debug)
	if len(services) == 0 {
		return
	}

	display.PrintSection("Media Services")
	for _, result := range collectMediaStatuses(services, client, debug) {
		fmt.Print(formatMediaLine(result.Name, result.Text, result.Color))
	}
}

func CollectMediaStatuses(cfg config.Config, selected map[string]bool, client *http.Client, debug bool) []MediaStatus {
	return collectMediaStatuses(allServices(cfg, selected, debug), client, debug)
}

func collectMediaStatuses(services []Service, client *http.Client, debug bool) []MediaStatus {
	var wg sync.WaitGroup
	results := make(chan MediaStatus, len(services))
	limit := MaxConcurrentMediaChecks()
	if limit > len(services) {
		limit = len(services)
	}
	semaphore := make(chan struct{}, limit)
	order := 0

	start := func(svc Service) {
		currentOrder := order
		order++
		wg.Add(1)
		go func() {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			text, color, ok := svc.Render(client, debug)
			if ok {
				results <- MediaStatus{Order: currentOrder, Name: svc.Name(), Text: text, Color: color}
			} else {
				results <- MediaStatus{Order: currentOrder, Name: svc.Name(), Text: "unavailable", Color: display.Yellow, Error: "unavailable"}
			}
		}()
	}

	for _, svc := range services {
		start(svc)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	collected := make([]MediaStatus, 0, order)
	for result := range results {
		collected = append(collected, result)
	}

	sort.Slice(collected, func(i, j int) bool {
		return collected[i].Order < collected[j].Order
	})

	return collected
}

// IsPlaintextToRemote returns true when rawURL uses http:// with a
// non-loopback host. Callers should warn users about credential exposure.
func IsPlaintextToRemote(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	if u.Scheme != "http" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return false
	}
	return true
}

func IsValidURL(rawURL string) bool {
	if rawURL == "" {
		return false
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme == "" {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	if u.Host == "" {
		return false
	}
	if u.User != nil {
		return false
	}
	return true
}

func IsAllowedServiceURL(rawURL string) bool {
	return IsValidURL(rawURL) && !IsPlaintextToRemote(rawURL)
}

func isPlexReady(plex config.ServiceConfig) bool {
	return serviceSkipReason(plex, true) == ""
}

func isJellyfinReady(jellyfin config.ServiceConfig) bool {
	return serviceSkipReason(jellyfin, true) == ""
}

func isAPIServiceReady(service config.ServiceConfig) bool {
	return serviceSkipReason(service, false) == ""
}

func serviceSkipReason(service config.ServiceConfig, wantsToken bool) string {
	if !service.Enabled {
		return "disabled"
	}
	if wantsToken && service.Token == "" {
		return "missing credential: token"
	}
	if !wantsToken && service.APIKey == "" {
		return "missing credential: api_key"
	}
	if !IsValidURL(service.URL) {
		return "invalid URL"
	}
	if IsPlaintextToRemote(service.URL) {
		return "remote HTTP blocked; use HTTPS or loopback HTTP"
	}
	return ""
}

func logSkippedService(debug bool, kind string, service config.ServiceConfig, reason string) {
	display.DebugLog(debug, "Skipping %s: %s", serviceLabel(kind, service.Name), reason)
}

func formatMediaLine(label, text, color string) string {
	dots := display.DotLabelWidth - len(label)
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

	decoder := json.NewDecoder(io.LimitReader(resp.Body, maxMediaResponseSize))
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

func countAvailableRecords(records []json.RawMessage) int {
	count := 0
	for _, raw := range records {
		var rec wantedRecord
		if err := json.Unmarshal(raw, &rec); err != nil {
			count++
			continue
		}
		if rec.IsAvailable {
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

func (s plexService) Render(client *http.Client, debug bool) (string, string, bool) {
	req, err := http.NewRequest("GET", serviceURL(s.cfg.URL, "/status/sessions"), nil)
	if err != nil {
		display.DebugLog(debug, "Plex request failed for %s: %v", s.cfg.Name, err)
		return "", "", false
	}
	req.Header.Set("X-Plex-Token", s.cfg.Token)

	resp, err := client.Do(req)
	if err != nil {
		display.DebugLog(debug, "Plex request failed for %s: %v", s.cfg.Name, err)
		return "", "", false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		display.DebugLog(debug, "Plex returned status %d for %s", resp.StatusCode, s.cfg.Name)
		return "", "", false
	}

	var sessions plexSessionsResponse
	if err := xml.NewDecoder(io.LimitReader(resp.Body, maxMediaResponseSize)).Decode(&sessions); err != nil {
		display.DebugLog(debug, "Failed to parse Plex XML for %s: %v", s.cfg.Name, err)
		return "", "", false
	}

	transcodes := 0
	bandwidth := 0
	for _, video := range sessions.Videos {
		if video.TranscodeSession.VideoDecision == "transcode" {
			transcodes++
		}
		bandwidth += video.Session.Bandwidth
	}

	if sessions.Size == 0 {
		return "No active streams", display.Green, true
	}

	bwMbps := float64(bandwidth) / 1000.0
	if transcodes == 0 {
		return fmt.Sprintf("%d streams (%.2f Mbps)", sessions.Size, bwMbps), display.Yellow, true
	}

	return fmt.Sprintf("%d streams, %d transcodes (%.2f Mbps)", sessions.Size, transcodes, bwMbps), display.Red, true
}

func (s jellyfinService) Render(client *http.Client, debug bool) (string, string, bool) {
	req, err := http.NewRequest("GET", serviceURL(s.cfg.URL, "/Sessions"), nil)
	if err != nil {
		display.DebugLog(debug, "Jellyfin request build failed for %s: %v", s.cfg.Name, err)
		return "", "", false
	}
	req.Header.Set("X-Emby-Token", s.cfg.Token)
	req.Header.Set("Authorization", "MediaBrowser Token=\""+s.cfg.Token+"\"")

	resp, err := client.Do(req)
	if err != nil {
		display.DebugLog(debug, "Jellyfin request failed for %s: %v", s.cfg.Name, err)
		return "", "", false
	}
	defer resp.Body.Close()

	var sessions []jellyfinSession
	if err := decodeJSONResponse(resp, &sessions); err != nil {
		display.DebugLog(debug, "Failed to decode Jellyfin response for %s: %v", s.cfg.Name, err)
		return "", "", false
	}

	count, transcodes, bwMbps, hasBW := parseJellyfinSessions(sessions)

	if count == 0 {
		return "No active streams", display.Green, true
	}

	if transcodes == 0 {
		if hasBW {
			return fmt.Sprintf("%d streams (%.2f Mbps)", count, bwMbps), display.Yellow, true
		}
		return fmt.Sprintf("%d streams", count), display.Yellow, true
	}

	if hasBW {
		return fmt.Sprintf("%d streams, %d transcodes (%.2f Mbps)", count, transcodes, bwMbps), display.Red, true
	}

	return fmt.Sprintf("%d streams, %d transcodes", count, transcodes), display.Red, true
}

func (s sonarrService) Render(client *http.Client, debug bool) (string, string, bool) {
	req, err := http.NewRequest("GET", serviceURL(s.cfg.URL, "/api/v3/wanted/missing"), nil)
	if err != nil {
		display.DebugLog(debug, "Sonarr request build failed for %s: %v", s.cfg.Name, err)
		return "", "", false
	}
	req.Header.Set("X-Api-Key", s.cfg.APIKey)

	resp, err := client.Do(req)
	if err != nil {
		display.DebugLog(debug, "Sonarr request failed for %s: %v", s.cfg.Name, err)
		return "", "", false
	}
	defer resp.Body.Close()

	var result arrWantedMissingResponse
	if err := decodeJSONResponse(resp, &result); err != nil {
		display.DebugLog(debug, "Failed to decode Sonarr response for %s: %v", s.cfg.Name, err)
		return "", "", false
	}

	count := parseARRMissingCount(result)
	if count == 0 {
		return "No missing episodes", display.Green, true
	}

	return fmt.Sprintf("%d missing episode%s", count, util.PluralSuffix(count)), display.Yellow, true
}

func (s radarrService) Render(client *http.Client, debug bool) (string, string, bool) {
	req, err := http.NewRequest("GET", serviceURL(s.cfg.URL, "/api/v3/wanted/missing?excludeUnavailable=true"), nil)
	if err != nil {
		display.DebugLog(debug, "Radarr request build failed for %s: %v", s.cfg.Name, err)
		return "", "", false
	}
	req.Header.Set("X-Api-Key", s.cfg.APIKey)

	resp, err := client.Do(req)
	if err != nil {
		display.DebugLog(debug, "Radarr request failed for %s: %v", s.cfg.Name, err)
		return "", "", false
	}
	defer resp.Body.Close()

	var result arrWantedMissingResponse
	if err := decodeJSONResponse(resp, &result); err != nil {
		display.DebugLog(debug, "Failed to decode Radarr response for %s: %v", s.cfg.Name, err)
		return "", "", false
	}

	count := countAvailableRecords(result.Records)
	if count == 0 {
		return "No missing movies", display.Green, true
	}

	return fmt.Sprintf("%d missing movie%s", count, util.PluralSuffix(count)), display.Yellow, true
}

func (s seerrService) Render(client *http.Client, debug bool) (string, string, bool) {
	req, err := http.NewRequest("GET", serviceURL(s.cfg.URL, "/api/v1/request/count"), nil)
	if err != nil {
		display.DebugLog(debug, "Seerr request build failed for %s: %v", s.cfg.Name, err)
		return "", "", false
	}
	req.Header.Set("X-Api-Key", s.cfg.APIKey)

	resp, err := client.Do(req)
	if err != nil {
		display.DebugLog(debug, "Seerr request failed for %s: %v", s.cfg.Name, err)
		return "", "", false
	}
	defer resp.Body.Close()

	var result seerrRequestCountResponse
	if err := decodeJSONResponse(resp, &result); err != nil {
		display.DebugLog(debug, "Failed to decode Seerr response for %s: %v", s.cfg.Name, err)
		return "", "", false
	}

	if result.Pending == 0 {
		return "No pending requests", display.Green, true
	}

	return fmt.Sprintf("%d pending request%s", result.Pending, util.PluralSuffix(result.Pending)), display.Yellow, true
}
