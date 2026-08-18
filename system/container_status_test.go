package system

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"motd/config"
)

func TestFetchContainerStatusAggregatesAndSorts(t *testing.T) {
	status := fmt.Sprintf(`{"protocol_version":1,"observed_at":%q,"workloads":[{"name":"beta","state":"stopped","health":"none"},{"name":"alpha","state":"running","health":"healthy"},{"name":"gamma","state":"running","health":"starting"}]}`, time.Now().UTC().Format(time.RFC3339Nano))
	socket := startStatusTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, status)
	}))

	result, err := fetchContainerStatus(socket, 24*time.Hour)
	if err != nil {
		t.Fatalf("fetchContainerStatus failed: %v", err)
	}
	if result.Online != 1 || result.Total != 3 || result.Status != "1 of 3 online" {
		t.Fatalf("unexpected aggregate: %+v", result)
	}
	if result.Workloads[0].Name != "alpha" || !result.Workloads[0].Online || result.Workloads[1].Name != "beta" || result.Workloads[2].Name != "gamma" {
		t.Fatalf("unexpected workload ordering/status: %+v", result.Workloads)
	}
}

func TestFetchContainerStatusRejectsStaleResponse(t *testing.T) {
	socket := startStatusTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"protocol_version":1,"observed_at":"2020-01-01T00:00:00Z","workloads":[]}`)
	}))

	if _, err := fetchContainerStatus(socket, time.Second); err == nil {
		t.Fatal("expected stale response to fail")
	}
}

func TestFetchContainerStatusRejectsInvalidWorkload(t *testing.T) {
	socket := startStatusTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"protocol_version":1,"observed_at":"2026-08-18T12:34:56Z","workloads":[{"name":"alpha","state":"running","health":"broken"}]}`)
	}))

	if _, err := fetchContainerStatus(socket, 24*time.Hour); err == nil {
		t.Fatal("expected invalid workload to fail")
	}
}

func TestValidateContainerStatusConfig(t *testing.T) {
	if err := ValidateContainerStatusConfig(&config.ContainerStatusConfig{SocketPath: "relative"}); err == nil {
		t.Fatal("expected relative socket path to fail")
	}
}

func startStatusTestServer(t *testing.T, handler http.Handler) string {
	t.Helper()
	directory, err := os.MkdirTemp("/tmp", "motd-status-test-")
	if err != nil {
		t.Fatalf("create test socket directory: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(directory) })
	socket := filepath.Join(directory, "agent.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatalf("listen on test socket: %v", err)
	}
	server := &http.Server{Handler: handler}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() {
		_ = server.Close()
		_ = os.Remove(socket)
	})
	return socket
}
