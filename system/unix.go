//go:build !windows && !darwin

package system

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"motd/display"
	"motd/util"
)

func ShowOS(cfg ConfigAccessor, debug bool) {
	release := "Unknown"
	data, err := os.ReadFile("/etc/os-release")
	if err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "PRETTY_NAME=") {
				release = strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), "\"")
				break
			}
		}
	}
	display.DotLabel("OS Release")
	fmt.Printf("%s%s%s\n", display.Blue, release, display.Reset)
}

func ShowUptime(cfg ConfigAccessor, debug bool) {
	uptime := "unknown"
	data, err := os.ReadFile("/proc/uptime")
	if err == nil {
		fields := strings.Fields(string(data))
		if len(fields) > 0 {
			seconds, parseErr := strconv.ParseFloat(fields[0], 64)
			if parseErr == nil {
				uptime = FormatDuration(time.Duration(seconds) * time.Second)
			}
		}
	}
	display.DotLabel("Uptime")
	fmt.Printf("%s%s%s\n", display.Blue, uptime, display.Reset)
}

func ShowLoad(cfg ConfigAccessor, debug bool) {
	load := ""
	data, err := os.ReadFile("/proc/loadavg")
	if err == nil {
		fields := strings.Fields(string(data))
		if len(fields) >= 3 {
			load = fmt.Sprintf("%s, %s, %s", fields[0], fields[1], fields[2])
		}
	}
	display.DotLabel("CPU Load")
	fmt.Printf("%s%s%s\n", display.Blue, load, display.Reset)
}

func ShowMemory(cfg ConfigAccessor, debug bool) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return
	}

	var totalKB, availKB uint64
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				totalKB, _ = strconv.ParseUint(fields[1], 10, 64)
			}
		}
		if strings.HasPrefix(line, "MemAvailable:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				availKB, _ = strconv.ParseUint(fields[1], 10, 64)
			}
		}
	}

	if totalKB == 0 {
		return
	}
	usedKB := totalKB - availKB
	totalGB := float64(totalKB) / 1048576.0
	usedGB := float64(usedKB) / 1048576.0

	display.DotLabel("Memory")
	fmt.Printf("%s%.2f GB / %.2f GB%s\n", display.Blue, usedGB, totalGB, display.Reset)
}

func ShowBandwidth(cfg ConfigAccessor, debug bool) {
	if !util.HasCommand("vnstat") {
		return
	}

	interfaceName := strings.TrimSpace(cfg.NetworkInterface)
	if interfaceName == "" {
		interfaceName = getDefaultInterface()
	}
	if interfaceName == "" {
		interfaceName = "enp7s0"
	}

	output, err := exec.Command("vnstat", "--json", "m", "-i", interfaceName).Output()
	if err != nil {
		if strings.TrimSpace(cfg.NetworkInterface) == "" {
			output, err = exec.Command("vnstat", "--json", "m").Output()
		}
	}

	if err != nil {
		display.DebugLog(debug, "vnstat command failed: %v", err)
		return
	}

	rxGB, txGB, rxEst, txEst, err := parseVnstatMonthlyUsage(output, interfaceName, time.Now())
	if err != nil {
		display.DebugLog(debug, "Failed to parse vnstat data for %s: %v", interfaceName, err)
		return
	}

	display.DotLabel("Bandwidth (rx)")
	fmt.Printf("%s%.2f GB / %.2f GB est%s\n", display.Blue, rxGB, rxEst, display.Reset)
	display.DotLabel("Bandwidth (tx)")
	fmt.Printf("%s%.2f GB / %.2f GB est%s\n", display.Blue, txGB, txEst, display.Reset)
}

func ShowUser(cfg ConfigAccessor, debug bool) {
	output, err := exec.Command("who").Output()
	if err != nil {
		return
	}

	count := countUniqueWhoUsers(output)
	display.DotLabel("Logged in users")
	fmt.Printf("%s%d%s\n", display.Blue, count, display.Reset)
}

func ShowProcesses(cfg ConfigAccessor, debug bool) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return
	}

	count := 0
	for _, entry := range entries {
		if entry.IsDir() {
			if _, err := strconv.Atoi(entry.Name()); err == nil {
				count++
			}
		}
	}

	display.DotLabel("Processes")
	fmt.Printf("%s%d%s\n", display.Blue, count, display.Reset)
}

func ShowDisk(cfg ConfigAccessor, debug bool) {
	showDiskNative("/", "Disk (/)")
	if cfg.TankMount != "" {
		showDiskNative(cfg.TankMount, fmt.Sprintf("Disk (%s)", cfg.TankMount))
	}
}

func showDiskNative(path, label string) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return
	}

	totalBytes := stat.Blocks * uint64(stat.Bsize)
	freeBytes := stat.Bavail * uint64(stat.Bsize)
	usedBytes := totalBytes - freeBytes

	totalGB := float64(totalBytes) / float64(GB)
	usedGB := float64(usedBytes) / float64(GB)
	pct := 0.0
	if totalBytes > 0 {
		pct = float64(usedBytes) / float64(totalBytes) * 100
	}

	display.DotLabel(label)
	fmt.Printf("%s%.2f GB / %.2f GB (%.0f%% used)%s\n", display.Blue, usedGB, totalGB, pct, display.Reset)
}

var tempZonesChecked bool
var tempZones []string

func scanThermalZones() {
	tempZonesChecked = true
	entries, err := os.ReadDir("/sys/class/thermal")
	if err != nil {
		return
	}
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, "thermal_zone") {
			zonePath := filepath.Join("/sys/class/thermal", name, "temp")
			if _, err := os.Stat(zonePath); err == nil {
				tempZones = append(tempZones, zonePath)
			}
		}
	}
}

func ShowTemp(cfg ConfigAccessor, debug bool) {
	if !tempZonesChecked {
		scanThermalZones()
	}
	if len(tempZones) == 0 {
		return
	}

	for _, zonePath := range tempZones {
		data, err := os.ReadFile(zonePath)
		if err != nil {
			continue
		}
		millicelsius, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
		if err != nil || millicelsius <= 0 {
			continue
		}
		celsius := float64(millicelsius) / 1000.0
		if celsius < 0 || celsius > 150 {
			continue
		}
		display.DotLabel("CPU Temperature")
		fmt.Printf("%s%.0f°C%s\n", display.Red, celsius, display.Reset)
		return
	}
}

func getDefaultInterface() string {
	data, err := os.ReadFile("/proc/net/route")
	if err != nil {
		return ""
	}

	lines := strings.Split(string(data), "\n")
	// Header: Iface   Destination  Gateway ...
	// Default route has Destination=00000000
	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[1] == "00000000" {
			return fields[0]
		}
	}

	return ""
}
