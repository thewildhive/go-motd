package system

import (
	"testing"
	"time"
)

func TestPickLatestVnstatMonth(t *testing.T) {
	now := time.Date(2026, time.April, 15, 10, 0, 0, 0, time.UTC)
	entries := []vnstatMonthlyEntry{
		{Date: struct {
			Year  int `json:"year"`
			Month int `json:"month"`
		}{Year: 2026, Month: 3}},
		{Date: struct {
			Year  int `json:"year"`
			Month int `json:"month"`
		}{Year: 2026, Month: 4}},
	}

	picked, ok := pickLatestVnstatMonth(entries, now)
	if !ok {
		t.Fatal("expected month selection")
	}
	if picked.Date.Month != 4 {
		t.Fatalf("expected current month, got %d", picked.Date.Month)
	}
}

func TestParseVnstatMonthlyUsage(t *testing.T) {
	now := time.Date(2026, time.April, 10, 12, 0, 0, 0, time.UTC)
	payload := []byte(`{
		"interfaces":[
			{"id":"eth0","traffic":{"month":[{"rx":1073741824,"tx":2147483648,"date":{"year":2026,"month":4}}]}}
		]
	}`)

	rxGB, txGB, rxEst, txEst, err := parseVnstatMonthlyUsage(payload, "eth0", now)
	if err != nil {
		t.Fatalf("parseVnstatMonthlyUsage failed: %v", err)
	}

	if rxGB != 1.0 || txGB != 2.0 {
		t.Fatalf("unexpected usage values rx=%.2f tx=%.2f", rxGB, txGB)
	}

	expectedRxEst := 1.0 * (30.0 / 10.0)
	expectedTxEst := 2.0 * (30.0 / 10.0)
	if rxEst != expectedRxEst || txEst != expectedTxEst {
		t.Fatalf("unexpected estimates rx=%.2f tx=%.2f", rxEst, txEst)
	}
}

func TestParseVnstatMonthlyUsage_InterfaceSelection(t *testing.T) {
	now := time.Date(2026, time.April, 10, 12, 0, 0, 0, time.UTC)
	payload := []byte(`{
		"interfaces":[
			{"id":"eth0","traffic":{"month":[{"rx":1073741824,"tx":1073741824,"date":{"year":2026,"month":4}}]}},
			{"id":"wlan0","traffic":{"month":[{"rx":2147483648,"tx":2147483648,"date":{"year":2026,"month":4}}]}}
		]
	}`)

	rxGB, txGB, _, _, err := parseVnstatMonthlyUsage(payload, "wlan0", now)
	if err != nil {
		t.Fatalf("parseVnstatMonthlyUsage failed: %v", err)
	}
	if rxGB != 2.0 || txGB != 2.0 {
		t.Fatalf("expected preferred wlan0 values, got rx=%.2f tx=%.2f", rxGB, txGB)
	}

	_, _, _, _, err = parseVnstatMonthlyUsage([]byte(`{"interfaces":[{"id":"eth0","traffic":{"month":[]}}]}`), "", now)
	if err == nil {
		t.Fatal("expected error when no monthly data is available")
	}
}

func TestCountUniqueWhoUsers(t *testing.T) {
	output := []byte("alice pts/0 2026-04-30 10:00\nbob pts/1 2026-04-30 10:01\nalice pts/2 2026-04-30 10:02\n")
	if got := countUniqueWhoUsers(output); got != 2 {
		t.Fatalf("expected 2 unique users, got %d", got)
	}
}

func TestCountNonEmptyLines(t *testing.T) {
	if got := countNonEmptyLines([]byte("one\n\n two \n")); got != 2 {
		t.Fatalf("expected 2 non-empty lines, got %d", got)
	}
}

func TestCountWindowsTasklistProcesses(t *testing.T) {
	output := []byte("System Idle Process              0 Services                   0          8 K\nSystem                           4 Services                   0        156 K\n\n")
	if got := countWindowsTasklistProcesses(output); got != 2 {
		t.Fatalf("expected 2 Windows tasklist rows, got %d", got)
	}
}

func TestParseWMICValueAndUint(t *testing.T) {
	output := []byte("Caption=Microsoft Windows 11 Pro\nTotalVisibleMemorySize=33554432\n")
	caption, ok := parseWMICValue(output, "Caption")
	if !ok || caption != "Microsoft Windows 11 Pro" {
		t.Fatalf("unexpected caption: %q ok=%v", caption, ok)
	}
	mem, ok := parseWMICUint(output, "TotalVisibleMemorySize")
	if !ok || mem != 33554432 {
		t.Fatalf("unexpected memory value: %d ok=%v", mem, ok)
	}
}

func TestParseWindowsOSPowerShell(t *testing.T) {
	info, ok := parseWindowsOSPowerShell([]byte("Microsoft Windows 11 IoT Enterprise LTSC|26100|26100.3915\n"))
	if !ok {
		t.Fatal("expected Windows PowerShell OS output to parse")
	}
	if info.Version != "11" || info.Edition != "IoT Enterprise LTSC" || info.Build != "26100.3915" {
		t.Fatalf("unexpected Windows OS info: %+v", info)
	}
}

func TestParseWindowsOSWMIC(t *testing.T) {
	info, ok := parseWindowsOSWMIC([]byte("Caption=Microsoft Windows 10 Enterprise\nBuildNumber=19045\n"))
	if !ok {
		t.Fatal("expected Windows WMIC OS output to parse")
	}
	if info.Version != "10" || info.Edition != "Enterprise" || info.Build != "19045" {
		t.Fatalf("unexpected Windows OS info: %+v", info)
	}
}

func TestInferWindowsVersionUsesBuildNumber(t *testing.T) {
	if got := inferWindowsVersion("Microsoft Windows IoT Enterprise", "26100"); got != "11" {
		t.Fatalf("expected build 26100 to infer Windows 11, got %q", got)
	}
	if got := inferWindowsVersion("Microsoft Windows Enterprise", "19045"); got != "10" {
		t.Fatalf("expected build 19045 to infer Windows 10, got %q", got)
	}
}

func TestParseWindowsEdition(t *testing.T) {
	if got := parseWindowsEdition("Microsoft Windows 11 IoT Enterprise LTSC"); got != "IoT Enterprise LTSC" {
		t.Fatalf("unexpected edition: %q", got)
	}
}

func TestParseWMICDateTime(t *testing.T) {
	parsed, ok := parseWMICDateTime("20260430123456.000000+000")
	if !ok {
		t.Fatal("expected WMIC datetime to parse")
	}
	if parsed.Year() != 2026 || parsed.Month() != time.April || parsed.Day() != 30 || parsed.Hour() != 12 {
		t.Fatalf("unexpected datetime: %s", parsed)
	}
}

func TestFormatDuration(t *testing.T) {
	formatted := FormatDuration((49 * time.Hour) + (5 * time.Minute))
	if formatted != "2 days, 1 hour, 5 minutes" {
		t.Fatalf("unexpected duration: %s", formatted)
	}
}

func TestFormatDurationNegative(t *testing.T) {
	if got := FormatDuration(-time.Minute); got != "0 minutes" {
		t.Fatalf("expected negative duration to clamp to zero, got %q", got)
	}
}

func TestFormatDuration_JustMinutes(t *testing.T) {
	if got := FormatDuration(5 * time.Minute); got != "5 minutes" {
		t.Fatalf("expected '5 minutes', got %q", got)
	}
}

func TestFormatDuration_JustHours(t *testing.T) {
	if got := FormatDuration(3 * time.Hour); got != "3 hours" {
		t.Fatalf("expected '3 hours', got %q", got)
	}
}

func TestFormatDuration_Zero(t *testing.T) {
	if got := FormatDuration(0); got != "0 minutes" {
		t.Fatalf("expected '0 minutes', got %q", got)
	}
}

func TestFormatDuration_ExactDay(t *testing.T) {
	if got := FormatDuration(48 * time.Hour); got != "2 days" {
		t.Fatalf("expected '2 days', got %q", got)
	}
}

func TestParseWindowsCPUPercent(t *testing.T) {
	percent, ok := parseWindowsCPUPercent([]byte("LoadPercentage=10\nLoadPercentage=30\n"))
	if !ok || percent != 20 {
		t.Fatalf("unexpected CPU percent: %d ok=%v", percent, ok)
	}
}

func TestParseWindowsMemoryKB(t *testing.T) {
	total, free, ok := parseWindowsMemoryKB([]byte("33554432,16777216"))
	if !ok || total != 33554432 || free != 16777216 {
		t.Fatalf("unexpected memory parse total=%d free=%d ok=%v", total, free, ok)
	}
}

func TestParseWindowsMemoryKBRejectsMalformedInput(t *testing.T) {
	if _, _, ok := parseWindowsMemoryKB([]byte("bad")); ok {
		t.Fatal("expected malformed memory output to fail")
	}
	if _, _, ok := parseWindowsMemoryKB([]byte("1,not-a-number")); ok {
		t.Fatal("expected non-numeric memory output to fail")
	}
}

func TestParseWindowsDiskCSV(t *testing.T) {
	disks := parseWindowsDiskCSV([]byte("bad\nC:,1000,250\nD:,2000,1000\nE:,not-a-number,10\n"))
	if len(disks) != 2 {
		t.Fatalf("expected 2 disks, got %d", len(disks))
	}
	if disks[0].Drive != "C:" || disks[0].TotalBytes != 1000 || (disks[0].TotalBytes-disks[0].UsedBytes) != 250 {
		t.Fatalf("unexpected first disk: %+v", disks[0])
	}
}

func TestParseWindowsDiskWMIC(t *testing.T) {
	disks := parseWindowsDiskWMIC([]byte("DeviceID=C:\nFreeSpace=250\nSize=1000\n\nDeviceID=D:\nFreeSpace=1000\nSize=2000\n"))
	if len(disks) != 2 {
		t.Fatalf("expected 2 disks, got %d", len(disks))
	}
	if disks[1].Drive != "D:" || disks[1].TotalBytes != 2000 || (disks[1].TotalBytes-disks[1].UsedBytes) != 1000 {
		t.Fatalf("unexpected second disk: %+v", disks[1])
	}
}

func TestParseWindowsTemperature(t *testing.T) {
	celsius, ok := parseWindowsTemperature([]byte("3032"))
	if !ok {
		t.Fatal("expected temperature to parse")
	}
	if celsius < 30.0 || celsius > 30.1 {
		t.Fatalf("unexpected celsius value: %.2f", celsius)
	}
}

func TestParseWindowsTemperatureRejectsMalformedInput(t *testing.T) {
	for _, input := range [][]byte{[]byte(""), []byte("not-a-number"), []byte("0")} {
		if _, ok := parseWindowsTemperature(input); ok {
			t.Fatalf("expected temperature input %q to fail", input)
		}
	}
}
