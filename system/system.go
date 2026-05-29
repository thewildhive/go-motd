package system

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"motd/config"
	"motd/display"
)

// Byte size constants for human-readable conversions.
const (
	KB = 1024
	MB = 1024 * KB
	GB = 1024 * MB
)

func ShowDocker(debug bool) {
	if !hasCommand("docker") {
		return
	}

	output, err := exec.Command("docker", "ps", "-q").Output()
	if err != nil {
		return
	}

	count := len(strings.Split(strings.TrimSpace(string(output)), "\n"))
	if strings.TrimSpace(string(output)) == "" {
		count = 0
	}

	display.DotLabel("Docker Containers")
	fmt.Printf("%s%d running%s\n", display.Blue, count, display.Reset)
}

func hasCommand(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// ConfigAccessor provides system-relevant config values
// without exposing the full Config struct to system functions.
type ConfigAccessor struct {
	TankMount        string
	NetworkInterface string
}

func ConfigAccessorFrom(cfg config.Config) ConfigAccessor {
	return ConfigAccessor{
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
		parts = append(parts, fmt.Sprintf("%d day%s", days, pluralSuffix(days)))
	}
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%d hour%s", hours, pluralSuffix(hours)))
	}
	if minutes > 0 || len(parts) == 0 {
		parts = append(parts, fmt.Sprintf("%d minute%s", minutes, pluralSuffix(minutes)))
	}
	return strings.Join(parts, ", ")
}

func pluralSuffix(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}

// ShowDFDisk parses df output for a given path and displays it with a label.
func ShowDFDisk(path, label string) {
	output, err := exec.Command("df", path).Output()
	if err != nil {
		return
	}

	lines := strings.Split(string(output), "\n")
	if len(lines) < 2 {
		return
	}
	fields := strings.Fields(lines[1])
	if len(fields) < 5 {
		return
	}

	usedVal, usedErr := strconv.ParseFloat(fields[2], 64)
	totalVal, totalErr := strconv.ParseFloat(fields[1], 64)
	if usedErr != nil || totalErr != nil {
		return
	}

	usedGB := usedVal / float64(MB)
	totalGB := totalVal / float64(MB)
	pct := strings.TrimSuffix(fields[4], "%")

	display.DotLabel(label)
	fmt.Printf("%s%.2f GB / %.2f GB (%s%% used)%s\n", display.Blue, usedGB, totalGB, pct, display.Reset)
}

func daysInMonth(t time.Time) int {
	return time.Date(t.Year(), t.Month()+1, 0, 0, 0, 0, 0, t.Location()).Day()
}
