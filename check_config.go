package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"motd/config"
	"motd/display"
	"motd/media"
)

func flagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	return fs
}

func checkConfig(configPath string) ([]configIssue, config.Config, error) {
	cfg, err := config.Load(configPath, false, nil)
	if err != nil {
		if errors.Is(err, config.ErrNoJSONConfig) {
			return []configIssue{{Level: "info", Message: "No config found; system-only mode is valid."}}, config.Config{}, nil
		}
		return []configIssue{{Level: "error", Message: err.Error()}}, config.Config{}, err
	}

	issues := validateConfig(cfg)
	if len(issues) == 0 {
		issues = append(issues, configIssue{Level: "info", Message: "Config OK."})
	}
	return issues, cfg, nil
}

func validateConfig(cfg config.Config) []configIssue {
	issues := make([]configIssue, 0)
	validateServices := func(kind string, services []config.ServiceConfig, wantsToken bool) {
		for i, svc := range services {
			label := fmt.Sprintf("%s[%d]", kind, i)
			if !svc.Enabled {
				continue
			}
			if svc.URL == "" {
				issues = append(issues, configIssue{Level: "error", Message: label + " is enabled but missing url"})
			}
			if wantsToken && svc.Token == "" {
				issues = append(issues, configIssue{Level: "error", Message: label + " is enabled but missing token"})
			}
			if !wantsToken && svc.APIKey == "" {
				issues = append(issues, configIssue{Level: "error", Message: label + " is enabled but missing api_key"})
			}
			if svc.URL != "" && !media.IsValidURL(svc.URL) {
				issues = append(issues, configIssue{Level: "error", Message: label + " has an invalid url"})
			}
			if media.IsPlaintextToRemote(svc.URL) {
				issues = append(issues, configIssue{Level: "error", Message: label + " sends credentials over plaintext HTTP"})
			}
		}
	}

	validateServices("plex", cfg.Services.Plex, true)
	validateServices("jellyfin", cfg.Services.Jellyfin, true)
	validateServices("sonarr", cfg.Services.Sonarr, false)
	validateServices("radarr", cfg.Services.Radarr, false)
	validateServices("seerr", cfg.Services.Seerr, false)

	if cfg.System.ComposeDir != "" {
		if info, err := os.Stat(cfg.System.ComposeDir); err != nil || !info.IsDir() {
			issues = append(issues, configIssue{Level: "warning", Message: "compose_dir is set but is not a readable directory"})
		}
	}
	if cfg.System.TankMount != "" {
		if info, err := os.Stat(cfg.System.TankMount); err != nil || !info.IsDir() {
			issues = append(issues, configIssue{Level: "warning", Message: "tank_mount is set but is not a readable directory"})
		}
	}

	return issues
}

func hasErrorIssue(issues []configIssue) bool {
	for _, issue := range issues {
		if issue.Level == "error" {
			return true
		}
	}
	return false
}

func printConfigIssues(issues []configIssue) {
	for _, issue := range issues {
		color := display.Blue
		if issue.Level == "error" {
			color = display.Red
		} else if issue.Level == "warning" {
			color = display.Yellow
		} else if issue.Level == "info" {
			color = display.Green
		}
		fmt.Printf("%s%s:%s %s\n", color, issue.Level, display.Reset, issue.Message)
	}
}
