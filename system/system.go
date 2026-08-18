package system

import (
	"fmt"
	"strings"
	"time"

	"motd/config"
	"motd/util"
)

// Byte size constants for human-readable conversions.
const (
	KB = 1024
	MB = 1024 * KB
	GB = 1024 * MB
)

// ConfigAccessor provides system-relevant config values
// without exposing the full Config struct to system functions.
type ConfigAccessor struct {
	ContainerStatus  *config.ContainerStatusConfig
	TankMount        string
	NetworkInterface string
}

func ConfigAccessorFrom(cfg config.Config) ConfigAccessor {
	return ConfigAccessor{
		ContainerStatus:  cfg.System.ContainerStatus,
		TankMount:        cfg.System.TankMount,
		NetworkInterface: cfg.System.Network.Interface,
	}
}

// FormatDuration formats a time.Duration as a human-readable string.
func FormatDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}

	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	parts := make([]string, 0, 3)
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%d day%s", days, util.PluralSuffix(days)))
	}
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%d hour%s", hours, util.PluralSuffix(hours)))
	}
	if minutes > 0 || len(parts) == 0 {
		parts = append(parts, fmt.Sprintf("%d minute%s", minutes, util.PluralSuffix(minutes)))
	}
	return strings.Join(parts, ", ")
}

// GetDefaultInterface returns the name of the default network interface
// (the one with the default route). Returns "" if detection fails.
// Each platform file provides its own implementation.
func GetDefaultInterface() string {
	return getDefaultInterface()
}

func daysInMonth(t time.Time) int {
	return time.Date(t.Year(), t.Month()+1, 0, 0, 0, 0, 0, t.Location()).Day()
}
