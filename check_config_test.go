package main

import (
	"path/filepath"
	"strings"
	"testing"

	"motd/config"
)

func TestValidateConfigMissingEnabledServiceFields(t *testing.T) {
	cfg := config.Config{}
	cfg.Services.Plex = []config.ServiceConfig{{Enabled: true}}
	issues := validateConfig(cfg)
	if !hasErrorIssue(issues) {
		t.Fatalf("expected validation errors, got %+v", issues)
	}
}

func TestValidateConfigNoSettingsOK(t *testing.T) {
	issues := validateConfig(config.Config{})
	if hasErrorIssue(issues) || len(issues) != 0 {
		t.Fatalf("expected no issues for empty config, got %+v", issues)
	}
}

func TestValidateConfigPlainHTTPError(t *testing.T) {
	cfg := config.Config{}
	cfg.Services.Sonarr = []config.ServiceConfig{{URL: "http://sonarr:8989", APIKey: "key", Enabled: true}}
	issues := validateConfig(cfg)
	found := false
	for _, issue := range issues {
		if issue.Level == "error" && strings.Contains(issue.Message, "plaintext") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected plaintext error, got %+v", issues)
	}
}

func TestValidateConfigAllowsLoopbackHTTP(t *testing.T) {
	cfg := config.Config{}
	cfg.Services.Sonarr = []config.ServiceConfig{{URL: "http://127.0.0.1:8989", APIKey: "key", Enabled: true}}
	issues := validateConfig(cfg)
	if hasErrorIssue(issues) {
		t.Fatalf("expected loopback HTTP to be allowed, got %+v", issues)
	}
}

func TestValidateConfigTooManyEnabledServices(t *testing.T) {
	cfg := config.Config{}
	for i := 0; i < 33; i++ {
		cfg.Services.Plex = append(cfg.Services.Plex, config.ServiceConfig{URL: "https://plex.example.com", Token: "key", Enabled: true})
	}

	issues := validateConfig(cfg)
	found := false
	for _, issue := range issues {
		if issue.Level == "error" && strings.Contains(issue.Message, "maximum") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected service count error, got %+v", issues)
	}
}

func TestCheckConfigValidFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := config.Write(path, config.Config{}); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	issues, _, err := checkConfig(path)
	if err != nil || hasErrorIssue(issues) {
		t.Fatalf("expected valid config, issues=%+v err=%v", issues, err)
	}
}
