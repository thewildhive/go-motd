# Installation Guide

This guide covers multiple ways to install and configure the MOTD (Message of the Day) tool on your system.

## Quick Install

### Option 1: Download Pre-built Binary (Recommended)

1. **Go to the [Releases page](https://github.com/thewildhive/go-motd/releases)**
2. **Download the appropriate binary for your system:**
   - Linux: `motd-VERSION-linux-amd64.tar.gz` (Intel/AMD) or `motd-VERSION-linux-arm64.tar.gz` (ARM)
   - macOS: `motd-VERSION-darwin-amd64.tar.gz` (Intel) or `motd-VERSION-darwin-arm64.tar.gz` (Apple Silicon)
   - Windows: `motd-VERSION-windows-amd64.zip`

3. **Extract and install:**
   ```bash
   # Linux/macOS
   tar -xzf motd-*-linux-amd64.tar.gz
   sudo mv motd-linux-amd64 /usr/local/bin/motd
   sudo chmod +x /usr/local/bin/motd
   
   # Windows (PowerShell)
   Expand-Archive motd-*-windows-amd64.zip
   Move-Item motd-windows-amd64.exe C:\ProgramData\chocolatey\bin\motd.exe
   ```

### Option 2: Install from Source

```bash
# Clone the repository
git clone https://github.com/thewildhive/go-motd.git
cd go-motd

# Build and install
make install
```

## System Integration

### Add to Shell Profile

To run MOTD automatically when you log in, add it to your shell profile:

#### Bash

```bash
echo 'motd' >> ~/.bashrc
```

#### Zsh

```bash
echo 'motd' >> ~/.zshrc
```

#### Fish

```bash
echo 'motd' >> ~/.config/fish/config.fish
```

### System-wide MOTD (Linux)

For system-wide MOTD that shows for all users:

```bash
# Create system MOTD directory
sudo mkdir -p /etc/update-motd.d

# Create MOTD script
sudo tee /etc/update-motd.d/99-motd > /dev/null << 'EOF'
#!/bin/bash
/usr/local/bin/motd
EOF

# Make it executable
sudo chmod +x /etc/update-motd.d/99-motd

# Update MOTD
sudo update-motd
```

## Configuration

### Option 1: YAML Configuration (Recommended)

Create a configuration file at `~/.config/motd/config.yml`:

```bash
mkdir -p ~/.config/motd
nano ~/.config/motd/config.yml
```

Example configuration:

```yaml
services:
  plex:
    - name: "Main"
      url: "http://plex:32400"
      token: "your-plex-token"
      enabled: true
  jellyfin:
    - name: "Main"
      url: "http://jellyfin:8096"
      token: "your-jellyfin-token"
      enabled: true
  sonarr:
    - name: "Main"
      url: "http://sonarr:8989"
      api_key: "your-sonarr-api-key"
      enabled: true
  radarr:
    - name: "Main"
      url: "http://radarr:7878"
      api_key: "your-radarr-api-key"
      enabled: true
  organizr:
    - name: "Main"
      url: "http://organizr:80"
      api_key: "your-organizr-api-key"
      enabled: true
system:
  compose_dir: "/opt/apps/compose"
  tank_mount: "/mnt/tank"
  network:
    interface: "enp7s0"
```

## Optional Dependencies

For enhanced functionality, install these optional tools:

### Enhanced Display

```bash
# Ubuntu/Debian
sudo apt install figlet lolcat

# macOS
brew install figlet lolcat

# Arch Linux
sudo pacman -S figlet lolcat
```

### System Monitoring

```bash
# Ubuntu/Debian
sudo apt install lm-sensors vnstat docker.io

# macOS
brew install docker

# Arch Linux
sudo pacman -S lm_sensors vnstat docker
```

### Initialize Sensors (Linux)

```bash
sudo sensors-detect
sudo systemctl enable lm-sensors
```

## Verification

Test your installation:

```bash
# Show version
motd -v

# Show help
motd -h

# Test configuration
motd -d  # Debug mode
```

## Troubleshooting

### Permission Denied

```bash
sudo chmod +x /usr/local/bin/motd
```

### Command Not Found

```bash
# Check if binary is in PATH
which motd

# Add to PATH if needed
echo 'export PATH=$PATH:/usr/local/bin' >> ~/.bashrc
```

### Configuration Issues

```bash
# Test configuration
motd -d

# Check config file location
ls -la ~/.config/motd/config.yml
ls -la /opt/motd/config.yml
```

### Service Connection Issues

1. Verify service URLs are accessible
2. Check API tokens/keys are correct
3. Ensure no firewall is blocking connections
4. Use debug mode: `motd -d`

### Performance Issues

- MOTD typically runs in 10-50ms
- If slow, check network connectivity to services
- Disable unused services in configuration

## Uninstallation

### Remove Binary

```bash
sudo rm /usr/local/bin/motd
```

### Remove Configuration

```bash
rm -rf ~/.config/motd
sudo rm -rf /opt/motd
```

### Remove from Shell Profile

```bash
# Edit your shell profile and remove the 'motd' line
nano ~/.bashrc  # or ~/.zshrc, etc.
```

### Remove System MOTD (Linux)

```bash
sudo rm /etc/update-motd.d/99-motd
sudo update-motd
```

## Getting Help

- **GitHub Issues**: [Report bugs or request features](https://github.com/thewildhive/go-motd/issues)
- **Documentation**: [Full README](https://github.com/thewildhive/go-motd/blob/main/README.md)
- **Releases**: [Download latest versions](https://github.com/thewildhive/go-motd/releases)

## Advanced Usage

### Custom Build Options

```bash
# Build with custom version
go build -ldflags="-X main.VERSION=1.2.3-custom" -o motd main.go

# Build for specific platform
GOOS=linux GOARCH=arm64 go build -o motd-arm64 main.go
```

### Docker Integration

If running in Docker, you can mount the configuration:

```bash
docker run -v ~/.config/motd:/root/.config/motd:ro your-image motd
```

### Cron Integration

Run MOTD periodically:

```bash
# Edit crontab
crontab -e

# Add line to run every hour
0 * * * * /usr/local/bin/motd > /dev/null 2>&1
```

### Option 2: Install from Source

```bash
# Clone the repository
git clone https://github.com/thewildhive/go-motd.git
cd go-motd

# Build and install
make install
```

## System Integration

### Add to Shell Profile

To run MOTD automatically when you log in, add it to your shell profile:

#### Bash
```bash
echo 'motd' >> ~/.bashrc
```

#### Zsh
```bash
echo 'motd' >> ~/.zshrc
```

#### Fish
```bash
echo 'motd' >> ~/.config/fish/config.fish
```

### System-wide MOTD (Linux)

For system-wide MOTD that shows for all users:

```bash
# Create system MOTD directory
sudo mkdir -p /etc/update-motd.d

# Create MOTD script
sudo tee /etc/update-motd.d/99-motd > /dev/null << 'EOF'
#!/bin/bash
/usr/local/bin/motd
EOF

# Make it executable
sudo chmod +x /etc/update-motd.d/99-motd

# Update MOTD
sudo update-motd
```

## Configuration

### Option 1: YAML Configuration (Recommended)

Create a configuration file at `~/.config/motd/config.yml`:

```bash
mkdir -p ~/.config/motd
nano ~/.config/motd/config.yml
```

Example configuration:
```yaml
services:
  plex:
    - name: "Main"
      url: "http://plex:32400"
      token: "your-plex-token"
      enabled: true
  jellyfin:
    - name: "Main"
      url: "http://jellyfin:8096"
      token: "your-jellyfin-token"
      enabled: true
  sonarr:
    - name: "Main"
      url: "http://sonarr:8989"
      api_key: "your-sonarr-api-key"
      enabled: true
  radarr:
    - name: "Main"
      url: "http://radarr:7878"
      api_key: "your-radarr-api-key"
      enabled: true
  organizr:
    - name: "Main"
      url: "http://organizr:80"
      api_key: "your-organizr-api-key"
      enabled: true
system:
  compose_dir: "/opt/apps/compose"
  tank_mount: "/mnt/tank"
  network:
    interface: "enp7s0"
```



## Optional Dependencies

For enhanced functionality, install these optional tools:

### Enhanced Display
```bash
# Ubuntu/Debian
sudo apt install figlet lolcat

# macOS
brew install figlet lolcat

# Arch Linux
sudo pacman -S figlet lolcat
```

### System Monitoring
```bash
# Ubuntu/Debian
sudo apt install lm-sensors vnstat docker.io

# macOS
brew install docker

# Arch Linux
sudo pacman -S lm_sensors vnstat docker
```

### Initialize Sensors (Linux)
```bash
sudo sensors-detect
sudo systemctl enable lm-sensors
```

## Verification

Test your installation:

```bash
# Show version
motd -v

# Show help
motd -h

# Test configuration
motd -d  # Debug mode
```

## Troubleshooting

### Permission Denied
```bash
sudo chmod +x /usr/local/bin/motd
```

### Command Not Found
```bash
# Check if binary is in PATH
which motd

# Add to PATH if needed
echo 'export PATH=$PATH:/usr/local/bin' >> ~/.bashrc
```

### Configuration Issues
```bash
# Test configuration
motd -d

# Check config file location
ls -la ~/.config/motd/config.yml
ls -la /opt/motd/config.yml
```

### Service Connection Issues
1. Verify service URLs are accessible
2. Check API tokens/keys are correct
3. Ensure no firewall is blocking connections
4. Use debug mode: `motd -d`

### Performance Issues
- MOTD typically runs in 10-50ms
- If slow, check network connectivity to services
- Disable unused services in configuration

## Uninstallation

### Remove Binary
```bash
sudo rm /usr/local/bin/motd
```

### Remove Configuration
```bash
rm -rf ~/.config/motd
sudo rm -rf /opt/motd
```

### Remove from Shell Profile
```bash
# Edit your shell profile and remove the 'motd' line
nano ~/.bashrc  # or ~/.zshrc, etc.
```

### Remove System MOTD (Linux)
```bash
sudo rm /etc/update-motd.d/99-motd
sudo update-motd
```

## Getting Help

- **GitHub Issues**: [Report bugs or request features](https://github.com/thewildhive/go-motd/issues)
- **Documentation**: [Full README](https://github.com/thewildhive/go-motd/blob/main/README.md)
- **Releases**: [Download latest versions](https://github.com/thewildhive/go-motd/releases)

## Advanced Usage

### Custom Build Options
```bash
# Build with custom version
go build -ldflags="-X main.VERSION=1.2.3-custom" -o motd main.go

# Build for specific platform
GOOS=linux GOARCH=arm64 go build -o motd-arm64 main.go
```

### Docker Integration
If running in Docker, you can mount the configuration:

```bash
docker run -v ~/.config/motd:/root/.config/motd:ro your-image motd
```

### Cron Integration
Run MOTD periodically:

```bash
# Edit crontab
crontab -e

# Add line to run every hour
0 * * * * /usr/local/bin/motd > /dev/null 2>&1
```