package system

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"motd/config"
	"motd/display"
)

const (
	DefaultContainerStatusSocket = "/var/run/motd-status/agent.sock"
	DefaultContainerStatusMaxAge = 30 * time.Second
	containerStatusTimeout       = time.Second
	containerStatusResponseLimit = 1 * 1024 * 1024
	containerStatusFutureSkew    = 5 * time.Second
)

type ContainerStatus struct {
	ProtocolVersion int
	ObservedAt      time.Time
	Online          int
	Total           int
	Status          string
	Workloads       []WorkloadStatus
}

type WorkloadStatus struct {
	Name   string
	State  string
	Health string
	Online bool
}

type agentStatusResponse struct {
	ProtocolVersion int             `json:"protocol_version"`
	ObservedAt      time.Time       `json:"observed_at"`
	Workloads       []agentWorkload `json:"workloads"`
}

type agentWorkload struct {
	Name   string `json:"name"`
	State  string `json:"state"`
	Health string `json:"health"`
}

func GetContainerStatus(cfg ConfigAccessor, debug bool) (ContainerStatus, bool) {
	if cfg.ContainerStatus == nil {
		return ContainerStatus{}, false
	}
	socketPath := strings.TrimSpace(cfg.ContainerStatus.SocketPath)
	if socketPath == "" {
		socketPath = DefaultContainerStatusSocket
	}

	maxAge := DefaultContainerStatusMaxAge
	if value := strings.TrimSpace(cfg.ContainerStatus.MaxAge); value != "" {
		parsed, err := time.ParseDuration(value)
		if err != nil || parsed <= 0 {
			display.DebugLog(debug, "Invalid container status max_age %q", value)
			return ContainerStatus{}, false
		}
		maxAge = parsed
	}

	status, err := fetchContainerStatus(socketPath, maxAge)
	if err != nil {
		display.DebugLog(debug, "Container status unavailable: %v", err)
		return ContainerStatus{}, false
	}
	return status, true
}

func ShowContainers(cfg ConfigAccessor, debug bool) {
	status, ok := GetContainerStatus(cfg, debug)
	if !ok || status.Total == 0 {
		return
	}

	display.DotLabel("Containers")
	if status.Online == status.Total {
		fmt.Printf("%sAll workloads online%s\n", display.Green, display.Reset)
		return
	}
	fmt.Printf("%s%d of %d online%s\n", display.Yellow, status.Online, status.Total, display.Reset)
}

func fetchContainerStatus(socketPath string, maxAge time.Duration) (ContainerStatus, error) {
	if !filepath.IsAbs(socketPath) {
		return ContainerStatus{}, fmt.Errorf("container status socket path must be absolute")
	}

	transport := &http.Transport{
		DisableKeepAlives: true,
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", socketPath)
		},
	}
	client := &http.Client{Transport: transport, Timeout: containerStatusTimeout, CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	defer transport.CloseIdleConnections()

	request, err := http.NewRequest(http.MethodGet, "http://motd-status-agent/v1/status", nil)
	if err != nil {
		return ContainerStatus{}, err
	}
	request.Header.Set("Accept", "application/json")
	response, err := client.Do(request)
	if err != nil {
		return ContainerStatus{}, err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, containerStatusResponseLimit+1))
	if err != nil {
		return ContainerStatus{}, err
	}
	if len(body) > containerStatusResponseLimit {
		return ContainerStatus{}, fmt.Errorf("container status response exceeds 1 MiB")
	}
	if response.StatusCode != http.StatusOK {
		return ContainerStatus{}, fmt.Errorf("agent returned HTTP %d", response.StatusCode)
	}
	if mediaType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type")); err != nil || mediaType != "application/json" {
		return ContainerStatus{}, fmt.Errorf("agent returned non-JSON content")
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return ContainerStatus{}, fmt.Errorf("decode container status: %w", err)
	}
	for _, field := range []string{"protocol_version", "observed_at", "workloads"} {
		if _, ok := raw[field]; !ok {
			return ContainerStatus{}, fmt.Errorf("container status missing %s", field)
		}
	}
	if bytes.Equal(bytes.TrimSpace(raw["observed_at"]), []byte("null")) || bytes.Equal(bytes.TrimSpace(raw["workloads"]), []byte("null")) {
		return ContainerStatus{}, fmt.Errorf("container status contains a null required field")
	}
	var decoded agentStatusResponse
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	if err := decoder.Decode(&decoded); err != nil {
		return ContainerStatus{}, fmt.Errorf("decode container status: %w", err)
	}
	var extra struct{}
	if err := decoder.Decode(&extra); err != io.EOF {
		return ContainerStatus{}, fmt.Errorf("container status contains trailing data")
	}
	if decoded.ProtocolVersion != 1 {
		return ContainerStatus{}, fmt.Errorf("unsupported container status protocol version %d", decoded.ProtocolVersion)
	}
	now := time.Now()
	if decoded.ObservedAt.IsZero() || decoded.ObservedAt.After(now.Add(containerStatusFutureSkew)) {
		return ContainerStatus{}, fmt.Errorf("container status timestamp is invalid")
	}
	if now.Sub(decoded.ObservedAt) > maxAge {
		return ContainerStatus{}, fmt.Errorf("container status is stale")
	}

	status := ContainerStatus{
		ProtocolVersion: decoded.ProtocolVersion,
		ObservedAt:      decoded.ObservedAt,
		Total:           len(decoded.Workloads),
		Workloads:       make([]WorkloadStatus, 0, len(decoded.Workloads)),
	}
	seen := make(map[string]struct{}, len(decoded.Workloads))
	for _, workload := range decoded.Workloads {
		if err := validateWorkload(workload); err != nil {
			return ContainerStatus{}, err
		}
		if _, exists := seen[workload.Name]; exists {
			return ContainerStatus{}, fmt.Errorf("duplicate workload name %q", workload.Name)
		}
		seen[workload.Name] = struct{}{}
		item := WorkloadStatus{Name: workload.Name, State: workload.State, Health: workload.Health}
		item.Online = workload.State == "running" && (workload.Health == "healthy" || workload.Health == "none")
		if item.Online {
			status.Online++
		}
		status.Workloads = append(status.Workloads, item)
	}
	sort.Slice(status.Workloads, func(i, j int) bool { return status.Workloads[i].Name < status.Workloads[j].Name })
	if status.Total == 0 {
		status.Status = "0 workloads"
	} else if status.Online == status.Total {
		status.Status = "All workloads online"
	} else {
		status.Status = fmt.Sprintf("%d of %d online", status.Online, status.Total)
	}
	return status, nil
}

func validateWorkload(workload agentWorkload) error {
	if strings.TrimSpace(workload.Name) == "" {
		return fmt.Errorf("container status contains an empty workload name")
	}
	validStates := map[string]bool{"running": true, "stopped": true, "failed": true, "unknown": true}
	validHealth := map[string]bool{"healthy": true, "unhealthy": true, "starting": true, "none": true, "unknown": true}
	if !validStates[workload.State] || !validHealth[workload.Health] {
		return fmt.Errorf("container status contains an invalid state or health for %q", workload.Name)
	}
	return nil
}

func ValidateContainerStatusConfig(cfg *config.ContainerStatusConfig) error {
	if cfg == nil {
		return nil
	}
	if strings.TrimSpace(cfg.SocketPath) != "" && !filepath.IsAbs(cfg.SocketPath) {
		return fmt.Errorf("container_status.socket_path must be absolute")
	}
	if strings.TrimSpace(cfg.MaxAge) != "" {
		maxAge, err := time.ParseDuration(cfg.MaxAge)
		if err != nil || maxAge <= 0 {
			return fmt.Errorf("container_status.max_age must be a positive duration")
		}
	}
	return nil
}

func ContainerStatusSocketIsUsable(path string) (bool, error) {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if info.Mode()&os.ModeSocket == 0 || info.Mode()&os.ModeSymlink != 0 {
		return false, fmt.Errorf("container status path is not a Unix socket")
	}
	return true, nil
}
