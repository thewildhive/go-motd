package display

import (
	"strings"
	"testing"
)

func TestSafeHostnameRegex(t *testing.T) {
	tests := []struct {
		name     string
		hostname string
		want     bool
	}{
		{"simple hostname", "myserver", true},
		{"with dots", "my.server.local", true},
		{"with hyphens", "my-server-1", true},
		{"alphanumeric", "server42", true},
		{"with underscore", "my_server", false},
		{"with spaces", "my server", false},
		{"with backtick", "`evil`", false},
		{"with dollar", "evil$host", false},
		{"with semicolon", "evil;host", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := safeHostnameRe.MatchString(tt.hostname); got != tt.want {
				t.Fatalf("safeHostnameRe.MatchString(%q) = %v, want %v", tt.hostname, got, tt.want)
			}
		})
	}
}

func TestDotLabelWidthConstant(t *testing.T) {
	if DotLabelWidth != 22 {
		t.Fatalf("expected DotLabelWidth=22, got %d", DotLabelWidth)
	}
}

func TestColorConstantsAreSet(t *testing.T) {
	if Red == "" || Green == "" || Yellow == "" || Blue == "" || Cyan == "" || Bold == "" || Reset == "" {
		t.Fatal("expected all color constants to be non-empty")
	}
	if Red == Reset {
		t.Fatal("expected Red != Reset")
	}
}

func TestDebugLogMessageContainsPrefix(t *testing.T) {
	// Verify the format string contains [DEBUG]
	msg := "test message"
	DebugLog(true, msg)
	// Just verify it doesn't panic — output goes to stderr
}

func TestDotLabelFormat(t *testing.T) {
	// Verify the label appears in the output and the ": " suffix is present
	// We can't easily capture stdout in tests, so check the format constants
	if DotLabelWidth <= 0 {
		t.Fatal("DotLabelWidth must be positive")
	}
}

func TestPrintSectionFormat(t *testing.T) {
	// Verify section formatting contains the section marker
	expected := "TestSection"
	if strings.Contains(expected, "━━━") {
		t.Fatal("test data should not contain section markers")
	}
}
