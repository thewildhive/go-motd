//go:build linux

package system

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"syscall"
	"time"
)

const (
	driveHealthDir      = "/var/lib/motd-drive-health"
	driveHealthFile     = "status.json"
	driveHealthMaxBytes = 64 * 1024
)

type DriveHealthRow struct {
	Label  string `json:"label"`
	State  string `json:"state"`
	Reason string `json:"reason"`
}

type DriveHealthReport struct {
	Status     string           `json:"status"`
	ObservedAt string           `json:"observed_at,omitempty"`
	Drives     []DriveHealthRow `json:"drives,omitempty"`
}

func readDriveHealth(now time.Time, debug bool) DriveHealthReport {
	report := DriveHealthReport{Status: "unavailable"}
	dirFD, err := syscall.Open(driveHealthDir, syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_CLOEXEC|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return driveHealthUnavailable(report, debug, "open state directory: %v", err)
	}
	defer syscall.Close(dirFD)
	var dirStat syscall.Stat_t
	if err := syscall.Fstat(dirFD, &dirStat); err != nil || dirStat.Uid != 0 || dirStat.Mode&0022 != 0 {
		return driveHealthUnavailable(report, debug, "state directory is not trusted")
	}
	fd, err := syscall.Openat(dirFD, driveHealthFile, syscall.O_RDONLY|syscall.O_CLOEXEC|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return driveHealthUnavailable(report, debug, "open snapshot: %v", err)
	}
	file := os.NewFile(uintptr(fd), driveHealthFile)
	defer file.Close()
	var stat syscall.Stat_t
	if err := syscall.Fstat(fd, &stat); err != nil || stat.Mode&syscall.S_IFMT != syscall.S_IFREG || stat.Uid != 0 || stat.Mode&0022 != 0 || stat.Size < 0 || stat.Size > driveHealthMaxBytes {
		return driveHealthUnavailable(report, debug, "snapshot metadata is unsafe")
	}
	data, err := io.ReadAll(io.LimitReader(file, driveHealthMaxBytes+1))
	if err != nil || len(data) > driveHealthMaxBytes {
		return driveHealthUnavailable(report, debug, "snapshot read failed or exceeded size limit")
	}
	var snapshot struct {
		SchemaVersion int              `json:"schema_version"`
		ObservedAt    string           `json:"observed_at"`
		Drives        []DriveHealthRow `json:"drives"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&snapshot); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return driveHealthUnavailable(report, debug, "snapshot JSON is malformed or unsupported")
	}
	observedAt, err := time.Parse(time.RFC3339Nano, snapshot.ObservedAt)
	if snapshot.SchemaVersion != 1 || err != nil || !strings.HasSuffix(snapshot.ObservedAt, "Z") || observedAt.After(now.Add(5*time.Minute)) || len(snapshot.Drives) == 0 {
		return driveHealthUnavailable(report, debug, "snapshot schema or timestamp is invalid")
	}
	seen := make(map[string]struct{}, len(snapshot.Drives))
	for _, row := range snapshot.Drives {
		if !validDriveLabel(row.Label) {
			return driveHealthUnavailable(report, debug, "snapshot contains an invalid label")
		}
		if _, exists := seen[row.Label]; exists {
			return driveHealthUnavailable(report, debug, "snapshot contains duplicate labels")
		}
		seen[row.Label] = struct{}{}
		if !validDriveHealthPair(row.State, row.Reason) {
			return driveHealthUnavailable(report, debug, "snapshot contains an invalid state/reason pair")
		}
	}
	sort.Slice(snapshot.Drives, func(i, j int) bool { return snapshot.Drives[i].Label < snapshot.Drives[j].Label })
	report.ObservedAt = observedAt.UTC().Format(time.RFC3339Nano)
	if now.Sub(observedAt) > 36*time.Hour {
		report.Status = "stale"
		if debug {
			fmt.Fprintln(os.Stderr, "[DEBUG] drive health stale: snapshot is older than 36 hours")
		}
		return report
	}
	report.Status = "fresh"
	report.Drives = snapshot.Drives
	return report
}

func driveHealthUnavailable(report DriveHealthReport, debug bool, format string, args ...interface{}) DriveHealthReport {
	if debug {
		fmt.Fprintf(os.Stderr, "[DEBUG] drive health unavailable: "+format+"\n", args...)
	}
	return report
}

func validDriveLabel(label string) bool {
	if len(label) < 1 || len(label) > 32 || label[0] == ' ' || label[len(label)-1] == ' ' {
		return false
	}
	for i := 0; i < len(label); i++ {
		if label[i] < 0x20 || label[i] > 0x7e {
			return false
		}
	}
	return true
}

func validDriveHealthPair(state, reason string) bool {
	switch state {
	case "healthy":
		return reason == "passed"
	case "failed":
		return reason == "overall_failed" || reason == "nvme_critical"
	case "warning":
		return reason == "ata_prefail_now" || reason == "nvme_advisory"
	case "unknown":
		switch reason {
		case "missing", "identity_mismatch", "access_denied", "sleeping", "unsupported", "timeout", "invalid_response", "probe_error":
			return true
		}
	}
	return false
}

func GetDriveHealth(debug bool) DriveHealthReport {
	return readDriveHealth(time.Now().UTC(), debug)
}

func DriveHealthText(report DriveHealthReport) string {
	if report.Status == "stale" || report.Status == "unavailable" {
		return report.Status
	}
	degraded := make([]string, 0, len(report.Drives))
	for _, drive := range report.Drives {
		if drive.State != "healthy" {
			degraded = append(degraded, drive.Label+" ("+drive.State+")")
		}
	}
	if len(degraded) == 0 {
		return "healthy"
	}
	return "degraded: " + strings.Join(degraded, ", ")
}
