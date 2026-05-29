//go:build !windows && !darwin

package system

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"motd/display"
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
	output, err := exec.Command("uptime", "-p").Output()
	uptime := "unknown"
	if err == nil {
		uptime = strings.TrimPrefix(strings.TrimSpace(string(output)), "up ")
	}
	display.DotLabel("Uptime")
	fmt.Printf("%s%s%s\n", display.Blue, uptime, display.Reset)
}

func ShowLoad(cfg ConfigAccessor, debug bool) {
	output, err := exec.Command("uptime").Output()
	load := ""
	if err == nil {
		parts := strings.Split(string(output), "load average: ")
		if len(parts) > 1 {
			load = strings.TrimSpace(parts[1])
		}
	}
	display.DotLabel("CPU Load")
	fmt.Printf("%s%s%s\n", display.Blue, load, display.Reset)
}

func ShowMemory(cfg ConfigAccessor, debug bool) {
	output, err := exec.Command("free", "-b").Output()
	if err != nil {
		return
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "Mem:") {
			fields := strings.Fields(line)
			if len(fields) >= 3 {
				total, _ := strconv.ParseFloat(fields[1], 64)
				used, _ := strconv.ParseFloat(fields[2], 64)
				totalGB := total / float64(GB)
				usedGB := used / float64(GB)

				display.DotLabel("Memory")
				fmt.Printf("%s%.2f GB / %.2f GB%s\n", display.Blue, usedGB, totalGB, display.Reset)
			}
			break
		}
	}
}

func ShowBandwidth(cfg ConfigAccessor, debug bool) {
	if !hasCommand("vnstat") {
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
	output, err := exec.Command("ps", "-e", "--no-headers").Output()
	if err != nil {
		return
	}

	count := countNonEmptyLines(output)
	display.DotLabel("Processes")
	fmt.Printf("%s%d%s\n", display.Blue, count, display.Reset)
}

func ShowDisk(cfg ConfigAccessor, debug bool) {
	ShowDFDisk("/", "Disk (/)")
	if cfg.TankMount != "" {
		ShowDFDisk(cfg.TankMount, fmt.Sprintf("Disk (%s)", cfg.TankMount))
	}
}

var tempSensorLabels = []string{
	"Package id 0:",    // Intel
	"Tctl:",            // AMD
	"Tdie:",            // AMD (Ryzen)
	"CPU Temperature:", // ARM SoC, some AMD
	"temp1:",           // Common fallback (w1_therm, coretemp, lm-sensors)
	"CPUTIN:",          // Some Super I/O sensors
}

func ShowTemp(cfg ConfigAccessor, debug bool) {
	if !hasCommand("sensors") {
		return
	}

	output, err := exec.Command("sensors").Output()
	if err != nil {
		return
	}

	lines := strings.Split(string(output), "\n")
	for _, label := range tempSensorLabels {
		for _, line := range lines {
			if strings.Contains(line, label) {
				fields := strings.Fields(line)
				if len(fields) >= 4 {
					temp := fields[3]
					display.DotLabel("CPU Temperature")
					fmt.Printf("%s%s%s\n", display.Red, temp, display.Reset)
					return
				}
			}
		}
	}
}

func getDefaultInterface() string {
	if output, err := exec.Command("ip", "route", "show", "default").Output(); err == nil {
		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		for _, line := range lines {
			if strings.Contains(line, "default via") {
				fields := strings.Fields(line)
				for i, field := range fields {
					if field == "dev" && i+1 < len(fields) {
						return fields[i+1]
					}
				}
			}
		}
	}

	if output, err := exec.Command("route", "-n").Output(); err == nil {
		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "0.0.0.0") || strings.HasPrefix(line, "default") {
				fields := strings.Fields(line)
				if len(fields) >= 8 {
					return fields[7]
				}
			}
		}
	}

	commonInterfaces := []string{"eth0", "enp0s3", "ens33", "en0", "wlan0", "wlp2s0"}
	for _, iface := range commonInterfaces {
		if output, err := exec.Command("ip", "link", "show", iface).Output(); err == nil {
			if strings.Contains(string(output), "state UP") {
				return iface
			}
		}
	}

	if output, err := exec.Command("ip", "link", "show").Output(); err == nil {
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.Contains(line, "state UP") && !strings.Contains(line, "lo:") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					ifaceName := strings.TrimSuffix(fields[1], ":")
					if ifaceName != "lo" {
						return ifaceName
					}
				}
			}
		}
	}

	return ""
}
