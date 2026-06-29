package main

import "testing"

func TestParseServiceFilter(t *testing.T) {
	got, err := parseServiceFilter(" Plex,radarr ")
	if err != nil {
		t.Fatalf("parseServiceFilter failed: %v", err)
	}
	if !got["plex"] || !got["radarr"] || got["sonarr"] {
		t.Fatalf("unexpected service filter: %+v", got)
	}
}

func TestParseServiceFilterRejectsUnknown(t *testing.T) {
	if _, err := parseServiceFilter("plex,bad"); err == nil {
		t.Fatal("expected unknown service error")
	}
}

func TestParseServiceFilterEmpty(t *testing.T) {
	got, err := parseServiceFilter("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil filter, got %+v", got)
	}
}
