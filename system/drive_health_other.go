//go:build !linux

package system

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

func GetDriveHealth(debug bool) DriveHealthReport {
	return DriveHealthReport{Status: "unavailable"}
}

func DriveHealthText(report DriveHealthReport) string {
	return report.Status
}
