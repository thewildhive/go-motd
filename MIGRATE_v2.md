# Migrating to MOTD 2.0

MOTD 2.0 no longer loads or migrates legacy YAML configs. If `config.yml` or `config.yaml` is present and no JSON config is found, `motd` exits with an unsupported-config error.

Create a JSON config manually, then remove or archive the old YAML file.

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
| `compose_dir` | `system.compose_dir` |
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
    "compose_dir": "/opt/apps/compose",
    "tank_mount": "/mnt/tank",
    "network": {
      "interface": "eth0"
    }
  }
}
```

Remote media service URLs should use HTTPS. Plaintext HTTP is accepted only for loopback hosts such as `localhost`, `127.0.0.1`, and `::1`.

After writing the JSON file, run:

```bash
motd check-config
motd -d
```
