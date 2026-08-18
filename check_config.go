package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"motd/config"
	"motd/display"
	"motd/media"
	"motd/system"
)

func flagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	return fs
}

func checkConfig(configPath string) ([]configIssue, config.Config, error) {
	cfg, err := config.Load(configPath, false, nil)
	if err != nil {
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
		enabledCount := 0
		for _, svc := range services {
			if svc.Enabled {
				enabledCount++
			}
		}
		if enabledCount > media.MaxMediaServicesPerType() {
			issues = append(issues, configIssue{Level: "error", Message: fmt.Sprintf("%s has %d enabled services; maximum is %d", kind, enabledCount, media.MaxMediaServicesPerType())})
		}

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

	if statusCfg := cfg.System.ContainerStatus; statusCfg != nil {
		if err := system.ValidateContainerStatusConfig(statusCfg); err != nil {
			issues = append(issues, configIssue{Level: "error", Message: err.Error()})
		} else {
			path := strings.TrimSpace(statusCfg.SocketPath)
			if path == "" {
				path = system.DefaultContainerStatusSocket
			}
			usable, err := system.ContainerStatusSocketIsUsable(path)
			if err != nil {
				issues = append(issues, configIssue{Level: "error", Message: err.Error()})
			} else if !usable {
				issues = append(issues, configIssue{Level: "warning", Message: fmt.Sprintf("container status socket %s does not exist", path)})
			} else if _, ok := system.GetContainerStatus(system.ConfigAccessorFrom(cfg), false); !ok {
				issues = append(issues, configIssue{Level: "error", Message: "container status socket exists but did not return a valid current status"})
			}
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
