//go:build windows

package system

import (
	"testing"
	"time"
)

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

func TestParseWMICDateTime_Truncated(t *testing.T) {
	if _, ok := parseWMICDateTime("2026"); ok {
		t.Fatal("expected truncated input to fail")
	}
}

func TestParseWMICDateTime_Empty(t *testing.T) {
	if _, ok := parseWMICDateTime(""); ok {
		t.Fatal("expected empty input to fail")
	}
}

func TestParseWMICDateTime_NonNumericMonth(t *testing.T) {
	if _, ok := parseWMICDateTime("2026ab30123456"); ok {
		t.Fatal("expected non-numeric month to fail")
	}
}

func TestParseWMICDateTime_OutOfRangeMonth(t *testing.T) {
	if _, ok := parseWMICDateTime("20261330123456"); ok {
		t.Fatal("expected month 13 to fail")
	}
}

func TestParseWMICDateTime_OutOfRangeDay(t *testing.T) {
	if _, ok := parseWMICDateTime("20260432123456"); ok {
		t.Fatal("expected day 32 to fail")
	}
}

func TestParseWMICDateTime_OutOfRangeHour(t *testing.T) {
	if _, ok := parseWMICDateTime("20260430243456"); ok {
		t.Fatal("expected hour 24 to fail")
	}
}

func TestParseWMICDateTime_OutOfRangeMinute(t *testing.T) {
	if _, ok := parseWMICDateTime("20260430126056"); ok {
		t.Fatal("expected minute 60 to fail")
	}
}

func TestParseWMICDateTime_OutOfRangeSecond(t *testing.T) {
	if _, ok := parseWMICDateTime("20260430123460"); ok {
		t.Fatal("expected second 60 to fail")
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
