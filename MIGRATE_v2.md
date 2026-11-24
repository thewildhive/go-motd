# Migrating to MOTD 2.0

This guide is for installations that still have a v1 YAML configuration. Starting
in v2.0, `motd` no longer loads or converts YAML. If no default JSON config is
found but `config.yml` or `config.yaml` exists in a default config directory,
`motd` reports an unsupported config. Create JSON manually, validate it, then
archive the old YAML. To migrate a standalone binary to APT without changing an
existing JSON config, use the [Debian package migration guide](Migrate.md) instead.

## Config Paths

Default lookup order:

1. `~/.config/motd/config.json`
2. `/opt/motd/config.json`

Use `motd -config /path/to/config.json` to load one exact JSON file. In 2.0, that file must exist, must not be a directory, and must contain valid JSON.

Use `motd -no-config` to skip config loading entirely. This also skips legacy YAML detection and shows system-only output.

## Field Mapping

Legacy service fields map directly to the JSON service objects:

| YAML field | JSON field |
|------------|------------|
| `name` | `name` |
| `url` | `url` |
| `token` | `token` for Plex/Jellyfin |
| `api_key` | `api_key` for Sonarr/Radarr/Seerr |
| `enabled` | `enabled` |

System fields:

| YAML field | JSON field |
|------------|------------|
| `tank_mount` | `system.tank_mount` |
| `interface` | `system.network.interface` |

Organizr is not supported in MOTD 2.0. Leave Organizr entries out of the JSON config.

## Example JSON

```json
{
  "services": {
    "plex": [
      {
        "name": "Main",
        "url": "https://plex.example.com:32400",
        "token": "your-plex-token",
        "enabled": true
      }
    ],
    "jellyfin": [],
    "sonarr": [
      {
        "name": "Main",
        "url": "https://sonarr.example.com:8989",
        "api_key": "your-sonarr-api-key",
        "enabled": true
      }
    ],
    "radarr": [],
    "seerr": []
  },
  "system": {
    "tank_mount": "/mnt/tank",
    "network": {
      "interface": "eth0"
    }
  }
}
```

Remote media service URLs should use HTTPS. Plaintext HTTP is accepted only for loopback hosts such as `localhost`, `127.0.0.1`, and `::1`.

On current releases, remove any `system.compose_dir` setting: it is rejected.
For optional container status, configure `system.container_status` only after
setting up `motd-status-agent`; see the [current configuration example](README.md#example-config).

After writing the JSON file, run:

```bash
motd check-config
motd -d
```
