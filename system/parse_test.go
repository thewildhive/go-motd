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
