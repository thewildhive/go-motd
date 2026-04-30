# AGENT.md - Development Guidelines for MOTD

This document provides guidelines for AI agents and developers when adding new features to the MOTD (Message of the Day) Go implementation.

## Project Overview

**Purpose**: Display system information, service status, and media service statistics on login or command execution.

**Language**: Go 1.25+
**Architecture**: Single-package CLI split by responsibility and platform
**Performance Goal**: Maintain fast startup time (10-50ms) and low memory usage (5-10MB)

## Core Principles

### 1. Maintain Performance

- **Fast Startup**: Every millisecond matters. Avoid slow initialization.
- **Concurrent Operations**: Use goroutines for I/O-bound operations (API calls, file reads)
- **Connection Pooling**: Reuse HTTP connections via the global `httpClient`
- **Fail Fast**: Don't wait for timeouts; use aggressive timeout values

### 2. Graceful Degradation

- **Optional Features**: Missing tools, services, or config files should not cause errors
- **Silent Failures**: If a service is unavailable, skip it silently (unless debug mode)
- **Command Checks**: Always verify commands exist with `hasCommand()` before using
- **API Failures**: Handle HTTP errors gracefully; show nothing rather than error messages

### 3. Code Organization

- **Function Naming**: Use `show*()` for display functions (e.g., `showDisk()`, `showDocker()`)
- **File Placement**: Put config, UI, media, updater, utility, and platform-specific system code in their focused files
- **Consistent Patterns**: Follow existing patterns for new features
- **Type Safety**: Use structs for API responses; avoid `interface{}` when possible

### 4. User Experience

- **Consistent Formatting**: Use `dotLabel()` for all metric labels
- **Color Coding**:
  - `GREEN`: Good/no issues
  - `YELLOW`: Warning/moderate activity
  - `RED`: Critical/high activity
  - `BLUE`: Informational
  - `CYAN`: Section headers
- **Concise Output**: One line per metric; avoid verbose output

## Adding New Features

### Adding a New System Metric

System metrics should degrade gracefully on every supported platform. Put Windows implementations in `system_windows.go`, Unix implementations in `system_unix.go`, and shared parsers in `system_parse.go`.

```go
func showNewMetric() {
    // 1. Check if the command/tool exists
    if !hasCommand("newtool") {
        return
    }
    
    // 2. Execute command and capture output
    output, err := exec.Command("newtool", "--some-flag").Output()
    if err != nil {
        debugLog("newtool failed: %v", err)
        return
    }
    
    // 3. Parse output
    value := strings.TrimSpace(string(output))
    
    // 4. Format and display with dotLabel
    dotLabel("New Metric")
    fmt.Printf("%s%s%s\n", BLUE, value, RESET)
}
```

**Where to call it**: Add to the appropriate section in `main()`:
- System Information: `showOS()`, `showUptime()`, etc.
- Services & Resources: `showUser()`, `showProcesses()`, etc.
- Media Services: add a renderer and wire it through `collectMediaStatuses()` in `media.go`.

### Adding a New Media Service Integration

```go
func showNewService() {
    // 1. Check if configured (URL and API key/token exist)
    if config.NewServiceURL == "" || config.NewServiceAPIKey == "" {
        return
    }
    
    // 2. Define response struct (type-safe parsing)
    type Response struct {
        Count int    `json:"count"`
        Status string `json:"status"`
    }
    
    // 3. Make HTTP request
    url := fmt.Sprintf("%s/api/endpoint?apikey=%s", 
        config.NewServiceURL, config.NewServiceAPIKey)
    resp, err := httpClient.Get(url)
    if err != nil {
        debugLog("NewService API failed: %v", err)
        return
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != http.StatusOK {
        debugLog("NewService returned %d", resp.StatusCode)
        return
    }
    
    // 4. Parse JSON response
    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return
    }
    
    var result Response
    if err := json.Unmarshal(body, &result); err != nil {
        debugLog("NewService JSON parse failed: %v", err)
        return
    }
    
    // 5. Display with appropriate color
    dotLabel("NewService")
    if result.Count == 0 {
        fmt.Printf("%sNo items%s\n", GREEN, RESET)
    } else {
        fmt.Printf("%s%d item%s%s\n", YELLOW, result.Count, 
            plural(result.Count), RESET)
    }
}
```

**Configuration steps**:

1. Add fields to `Config` in `config.go`
2. Add loading/default behavior if needed
3. Add to `hasMediaServices()` check, requiring `enabled`, URL, and credentials
4. Add JSON config field documentation and tests

### Adding Command-Line Flags

```go
// In main(), after existing flag definitions:
newFlag := flag.Bool("n", false, "Enable new feature")

// After flag.Parse():
if *newFlag {
    // Handle new flag behavior
}
```

### Adding JSON Config Fields

```go
// In Config struct:
type Config struct {
    Services struct {
        NewService []ServiceConfig `json:"newservice"`
    } `json:"services"`
}

// In config.json.sample:
{
  "services": {
    "newservice": [
      {
        "name": "Main",
        "url": "http://newservice:1234",
        "api_key": "your-api-key",
        "enabled": true
      }
    ]
  }
}
```

## Code Style Guidelines

### Naming Conventions
- **Functions**: `camelCase` starting with verb (`showDocker`, `hasCommand`, `loadConfig`)
- **Constants**: `UPPER_SNAKE_CASE` (`CURL_TIMEOUT`, `DOT_LABEL_WIDTH`)
- **Variables**: `camelCase` (`httpClient`, `debugMode`)
- **Structs**: `PascalCase` (`Config`, `MediaContainer`)

### Error Handling

```go
// DO: Silent failure for optional features
output, err := exec.Command("optional-tool").Output()
if err != nil {
    return // Skip silently
}

// DO: Debug logging for troubleshooting
if err != nil {
    debugLog("failed to fetch data: %v", err)
    return
}

// DON'T: Display errors to user (unless critical dependency)
if err != nil {
    fmt.Printf("ERROR: %v\n", err) // ❌ Breaks clean output
}
```

### HTTP Requests

```go
// DO: Use global httpClient with timeout
resp, err := httpClient.Get(url)

// DO: Always defer close
defer resp.Body.Close()

// DO: Check status code
if resp.StatusCode != http.StatusOK {
    return
}

// DON'T: Create new http.Client instances
client := &http.Client{} // ❌ Wastes resources
```

### Output Formatting

```go
// DO: Use dotLabel for consistency
dotLabel("Metric Name")
fmt.Printf("%s%s%s\n", BLUE, value, RESET)

// DO: Use color constants
fmt.Printf("%sSuccess%s\n", GREEN, RESET)

// DON'T: Hardcode ANSI codes
fmt.Printf("\033[0;32mSuccess\033[0m\n") // ❌ Hard to maintain

// DON'T: Use Printf when Print suffices
fmt.Printf("%s", str) // ❌ Inefficient
fmt.Print(str)        // ✓ Better
```

## Testing Guidelines

### Manual Testing

```bash
# Test with no configuration (should still show system information)
./bin/motd

# Test with debug mode
./bin/motd -d

# Test help and version
./bin/motd -h
./bin/motd -v
./bin/motd -no-config
./bin/motd -config ./config.json

# Test with sample JSON configuration
cp config.json.sample ~/.config/motd/config.json
./bin/motd -d
```

### Performance Testing

```bash
# Benchmark startup time
time ./bin/motd > /dev/null

# Should complete in < 100ms typically
# If > 500ms, investigate slow operations
```

### Build Testing

```bash
# Ensure code compiles
go build -buildvcs=false -o bin/motd .

# Check for common issues
go vet ./...
gofmt -d .

# Test optimized build
go build -buildvcs=false -ldflags="-s -w" -o bin/motd .
```

## Common Patterns

### Pluralization Helper
```go
func plural(count int) string {
    if count == 1 {
        return ""
    }
    return "s"
}

// Usage:
fmt.Printf("%d item%s\n", count, plural(count))
```

### Numeric Formatting
```go
// Convert bytes to GB
gb := float64(bytes) / 1073741824.0
fmt.Printf("%.2f GB\n", gb)

// Convert bits to Mbps
mbps := float64(bits) / 1000000.0
fmt.Printf("%.2f Mbps\n", mbps)
```

### String Parsing
```go
// Extract value from command output
output, _ := exec.Command("tool").Output()
lines := strings.Split(string(output), "\n")
for _, line := range lines {
    if strings.Contains(line, "keyword:") {
        value := strings.TrimSpace(strings.Split(line, ":")[1])
        // Use value...
    }
}
```

### API Response Parsing
```go
// JSON
type Response struct {
    Field string `json:"field"`
    Count int    `json:"count"`
}
var resp Response
json.Unmarshal(body, &resp)

// XML
type Response struct {
    XMLName xml.Name `xml:"root"`
    Field   string   `xml:"field,attr"`
}
var resp Response
xml.Unmarshal(body, &resp)
```

## Architecture Decisions

### Why One Package With Focused Files?
- **Simplicity**: The CLI remains one `package main` and one compiled binary
- **Platform Clarity**: Go build tags select `system_windows.go` or `system_unix.go`
- **Maintainability**: Config, UI, media, updater, utility, and parser code have clear homes
- **Testing**: Shared parsers stay untagged so tests run on every platform

### Why Global httpClient?
- **Connection Pooling**: Reuses TCP connections across API calls
- **Performance**: Avoids connection setup overhead
- **Consistency**: Single timeout configuration

### Media Concurrency
- Media API calls run concurrently to avoid serial timeout delays.
- Preserve stable output order by collecting results and printing after all workers finish.
- Failed optional services should be omitted from user-facing output and logged only in debug mode.

## Documentation Requirements

When adding features, update:

1. **This file (AGENT.md)**: Add patterns for new feature types
2. **README.md**: Document user-facing changes
3. **Usage function**: Update help text if adding flags
4. **config.json.sample**: Update schema examples for new config options
5. **main_test.go**: Add or update coverage for new behavior

## Configuration Management

### Adding New JSON Config Fields
1. Add to `Config` struct with explicit `json` tags
2. Update `config.json.sample` with the new field
3. Ensure runtime logic handles missing/disabled values safely
4. Add or update unit tests in `main_test.go`

### Configuration Lookup Priority
Missing config is valid. The app should continue with system-only output and skip media integrations.

1. `~/.config/motd/config.json` (highest)
2. `/opt/motd/config.json` (fallback)

Use `-config` to load an explicit JSON file. Use `-no-config` to skip config lookup entirely.

## Performance Optimization Tips

### Do's
- ✅ Use `strings.Builder` for concatenating multiple strings
- ✅ Reuse buffers with `sync.Pool` for high-frequency operations
- ✅ Use `bytes.Buffer` instead of string concatenation in loops
- ✅ Cache command existence checks if called multiple times
- ✅ Use `io.ReadAll` with known size limit

### Don'ts
- ❌ Make synchronous HTTP calls in loops (use goroutines)
- ❌ Parse large files line-by-line if you can use buffered reads
- ❌ Create temporary files unless absolutely necessary
- ❌ Use regex when simple string operations suffice
- ❌ Call external commands when Go stdlib can do it

## Debugging

### Enable Debug Mode
```bash
./bin/motd -d
```

### Add Debug Statements
```go
debugLog("Processing service: %s", serviceName)
debugLog("API response: %d bytes", len(body))
```

### Common Issues

**Problem**: Feature not showing up  
**Solution**: Check if guard condition (API key, command exists) is failing

**Problem**: Slow startup  
**Solution**: Use `time ./bin/motd -d` and check debug logs for slow operations

**Problem**: HTTP timeout  
**Solution**: Increase `CURL_TIMEOUT` constant or check service availability

**Problem**: Parsing errors  
**Solution**: Print raw response with `debugLog("raw: %s", string(body))`

## Version Management

### Updating Version Number
```go
var VERSION = "dev" // Release and Makefile builds override with -ldflags "-X main.VERSION=..."
```

### Semantic Versioning
- **Major** (X.0.0): Breaking changes, removed features
- **Minor** (2.X.0): New features, backwards compatible
- **Patch** (2.0.X): Bug fixes, performance improvements

## Dependencies

### Required (Runtime)
- None (pure Go stdlib)

### Optional (Runtime)
- `figlet` - ASCII art hostname
- `lolcat` - Colorful header
- `sensors` - CPU temperature
- `vnstat` - Network bandwidth statistics
- `docker` - Container count
- `free`, `df`, `uptime`, `ps` - System metrics
- Windows built-ins: PowerShell/CIM, `wmic`, `tasklist`

### Required (Build)
- Go 1.25+
- Standard build tools (make, etc.)

## Security Considerations

### API Keys and Tokens
- ✅ Store in `config.json` with restrictive file permissions
- ✅ Never hardcode in source
- ✅ Don't log in non-debug mode
- ❌ Never commit real API keys in any config file

### HTTP Requests
- ✅ Use HTTPS when available
- ✅ Set reasonable timeouts (5 seconds default)
- ✅ Validate response status codes
- ✅ Handle malformed responses gracefully

### Command Execution
- ✅ Validate command exists before execution
- ✅ Use fixed command paths when possible
- ❌ Never construct commands from user input
- ❌ Never execute shell commands with `sh -c` on user input

## Future Enhancement Ideas

### High Priority
- Response caching with TTL
- Configuration file schema validation tooling for JSON config

### Medium Priority
- Metrics export (Prometheus format)
- JSON output mode for scripting
- Plugin system for custom checks
- Threshold-based color coding

### Low Priority
- Web dashboard (HTTP server)
- Historical metrics storage
- Email/Slack notifications
- Custom themes/color schemes

## Getting Help

### For AI Agents
- Read this file completely before making changes
- Follow existing patterns in the codebase
- Test changes with `make build-optimized && ./bin/motd`
- Check performance with `time ./bin/motd`

### For Developers
- Review `README.md` for project overview
- Review focused `*.go` files and `main_test.go` for current behavior and test coverage
- Use `./bin/motd -d` for debugging
- Run `make help` for build commands

## Checklist for New Features

Before submitting changes:

- [ ] Code compiles without errors or warnings
- [ ] Feature works with and without configuration
- [ ] Graceful degradation if dependencies missing
- [ ] Consistent formatting with `dotLabel()`
- [ ] Appropriate color coding (GREEN/YELLOW/RED/BLUE)
- [ ] Debug logging for troubleshooting
- [ ] Updated documentation (README.md, usage())
- [ ] No performance regression (test with `time ./bin/motd`)
- [ ] No new external dependencies (if possible)
- [ ] Tested on target platform (Linux/macOS)

---

**Remember**: MOTD is a login utility. It should be fast, reliable, and never annoying. When in doubt, fail silently and maintain performance.
