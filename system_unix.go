//go:build !windows && !darwin

package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func showOS() {
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
	dotLabel("OS Release")
	fmt.Printf("%s%s%s\n", BLUE, release, RESET)
}

func showUptime() {
	output, err := exec.Command("uptime", "-p").Output()
	uptime := "unknown"
	if err == nil {
		uptime = strings.TrimPrefix(strings.TrimSpace(string(output)), "up ")
	}
	dotLabel("Uptime")
	fmt.Printf("%s%s%s\n", BLUE, uptime, RESET)
}

func showLoad() {
	output, err := exec.Command("uptime").Output()
	load := ""
	if err == nil {
		parts := strings.Split(string(output), "load average: ")
		if len(parts) > 1 {
			load = strings.TrimSpace(parts[1])
		}
	}
	dotLabel("CPU Load")
	fmt.Printf("%s%s%s\n", BLUE, load, RESET)
}

func showMemory() {
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
				totalGB := total / 1073741824.0
				usedGB := used / 1073741824.0

				dotLabel("Memory")
				fmt.Printf("%s%.2f GB / %.2f GB%s\n", BLUE, usedGB, totalGB, RESET)
			}
			break
		}
	}
}

func showBandwidth() {
	if !hasCommand("vnstat") {
		return
	}

	interfaceName := strings.TrimSpace(config.System.Network.Interface)
	if interfaceName == "" {
		interfaceName = getDefaultInterface()
	}
	if interfaceName == "" {
		interfaceName = "enp7s0"
	}

	output, err := exec.Command("vnstat", "--json", "m", "-i", interfaceName).Output()
	if err != nil {
		if strings.TrimSpace(config.System.Network.Interface) == "" {
			output, err = exec.Command("vnstat", "--json", "m").Output()
		}
	}

	if err != nil {
		debugLog("vnstat command failed: %v", err)
		return
	}

	rxGB, txGB, rxEst, txEst, err := parseVnstatMonthlyUsage(output, interfaceName, time.Now())
	if err != nil {
		debugLog("Failed to parse vnstat data for %s: %v", interfaceName, err)
		return
	}

	dotLabel("Bandwidth (rx)")
	fmt.Printf("%s%.2f GB / %.2f GB est%s\n", BLUE, rxGB, rxEst, RESET)
	dotLabel("Bandwidth (tx)")
	fmt.Printf("%s%.2f GB / %.2f GB est%s\n", BLUE, txGB, txEst, RESET)
}

func showUser() {
	output, err := exec.Command("who").Output()
	if err != nil {
		return
	}

	count := countUniqueWhoUsers(output)
	dotLabel("Logged in users")
	fmt.Printf("%s%d%s\n", BLUE, count, RESET)
}

func showProcesses() {
	output, err := exec.Command("ps", "-e", "--no-headers").Output()
	if err != nil {
		return
	}

	count := countNonEmptyLines(output)
	dotLabel("Processes")
	fmt.Printf("%s%d%s\n", BLUE, count, RESET)
}

func showDisk() {
	output, err := exec.Command("df", "/").Output()
	if err == nil {
		lines := strings.Split(string(output), "\n")
		if len(lines) >= 2 {
			fields := strings.Fields(lines[1])
			if len(fields) >= 5 {
				used := fields[2]
				total := fields[1]
				pct := strings.TrimSuffix(fields[4], "%")

				usedVal, _ := strconv.ParseFloat(used, 64)
				totalVal, _ := strconv.ParseFloat(total, 64)

				usedGB := usedVal / 1048576.0
				totalGB := totalVal / 1048576.0

				dotLabel("Disk (/)")
				fmt.Printf("%s%.2f GB / %.2f GB (%s%% used)%s\n", BLUE, usedGB, totalGB, pct, RESET)
			}
		}
	}

	if config.System.TankMount != "" {
		output, err := exec.Command("df", config.System.TankMount).Output()
		if err == nil {
			lines := strings.Split(string(output), "\n")
			if len(lines) >= 2 {
				fields := strings.Fields(lines[1])
				if len(fields) >= 5 {
					used := fields[2]
					total := fields[1]
					pct := strings.TrimSuffix(fields[4], "%")

					usedVal, _ := strconv.ParseFloat(used, 64)
					totalVal, _ := strconv.ParseFloat(total, 64)

					usedGB := usedVal / 1048576.0
					totalGB := totalVal / 1048576.0

					dotLabel(fmt.Sprintf("Disk (%s)", config.System.TankMount))
					fmt.Printf("%s%.2f GB / %.2f GB (%s%% used)%s\n", BLUE, usedGB, totalGB, pct, RESET)
				}
			}
		}
	}
}

func showTemp() {
	if !hasCommand("sensors") {
		return
	}

	output, err := exec.Command("sensors").Output()
	if err != nil {
		return
	}

	for _, line := range strings.Split(string(output), "\n") {
		if strings.Contains(line, "Package id 0:") {
			fields := strings.Fields(line)
			if len(fields) >= 4 {
				temp := fields[3]
				dotLabel("CPU Temperature")
				fmt.Printf("%s%s%s\n", RED, temp, RESET)
				break
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
