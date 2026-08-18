package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"motd/config"
	"motd/display"
	"motd/media"
	"motd/system"
)

type outputReport struct {
	Version    string            `json:"version"`
	System     systemReport      `json:"system"`
	Containers *containersReport `json:"containers,omitempty"`
	Media      []mediaJSONItem   `json:"media,omitempty"`
}

type systemReport struct {
	TankMount string `json:"tank_mount,omitempty"`
	Interface string `json:"interface,omitempty"`
}

type containersReport struct {
	ProtocolVersion int                `json:"protocol_version"`
	ObservedAt      string             `json:"observed_at"`
	Online          int                `json:"online"`
	Total           int                `json:"total"`
	Status          string             `json:"status"`
	Workloads       []workloadJSONItem `json:"workloads"`
}

type workloadJSONItem struct {
	Name   string `json:"name"`
	State  string `json:"state"`
	Health string `json:"health"`
	Online bool   `json:"online"`
}

type mediaJSONItem struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Text   string `json:"text,omitempty"`
	Error  string `json:"error,omitempty"`
}

func parseServiceFilter(raw string) (map[string]bool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	allowed := map[string]bool{
		"plex": true, "jellyfin": true, "sonarr": true, "radarr": true, "seerr": true,
	}
	selected := make(map[string]bool)
	for _, part := range strings.Split(raw, ",") {
		name := strings.ToLower(strings.TrimSpace(part))
		if name == "" {
			continue
		}
		if !allowed[name] {
			return nil, fmt.Errorf("unknown service %q", name)
		}
		selected[name] = true
	}
	if len(selected) == 0 {
		return nil, nil
	}
	return selected, nil
}

func renderJSON(cfg config.Config, serviceSet map[string]bool, client *http.Client, debug bool) {
	sysCfg := system.ConfigAccessorFrom(cfg)
	report := outputReport{
		Version: VERSION,
		System: systemReport{
			TankMount: cfg.System.TankMount,
			Interface: cfg.System.Network.Interface,
		},
	}

	if containerStatus, ok := system.GetContainerStatus(sysCfg, debug); ok {
		workloads := make([]workloadJSONItem, 0, len(containerStatus.Workloads))
		for _, workload := range containerStatus.Workloads {
			workloads = append(workloads, workloadJSONItem{Name: workload.Name, State: workload.State, Health: workload.Health, Online: workload.Online})
		}
		report.Containers = &containersReport{
			ProtocolVersion: containerStatus.ProtocolVersion,
			ObservedAt:      containerStatus.ObservedAt.UTC().Format(time.RFC3339Nano),
			Online:          containerStatus.Online,
			Total:           containerStatus.Total,
			Status:          containerStatus.Status,
			Workloads:       workloads,
		}
	}

	for _, item := range media.CollectMediaStatuses(cfg, serviceSet, client, debug) {
		status := "ok"
		if item.Error != "" {
			status = "error"
		}
		report.Media = append(report.Media, mediaJSONItem{Name: item.Name, Status: status, Text: item.Text, Error: item.Error})
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		fmt.Printf("%sError encoding JSON: %v%s\n", display.Red, err, display.Reset)
		os.Exit(1)
	}
}

type configIssue struct {
	Level   string `json:"level"`
	Message string `json:"message"`
}

func handleCheckConfig(args []string) {
	fs := flagSet("check-config")
	configPath := fs.String("config", "", "Load config from a specific JSON file")
	jsonOutput := fs.Bool("json", false, "Output diagnostics as JSON")
	noColor := fs.Bool("no-color", false, "Disable ANSI colors")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}
	if *noColor || *jsonOutput || os.Getenv("NO_COLOR") != "" {
		display.SetColorEnabled(false)
	}

	issues, _, err := checkConfig(*configPath)
	if *jsonOutput {
		out := struct {
			OK     bool          `json:"ok"`
			Issues []configIssue `json:"issues,omitempty"`
		}{OK: err == nil && !hasErrorIssue(issues), Issues: issues}
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		_ = encoder.Encode(out)
	} else {
		printConfigIssues(issues)
	}

	if err != nil || hasErrorIssue(issues) {
		os.Exit(1)
	}
}
