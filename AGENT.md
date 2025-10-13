# AGENT.md - Development Guidelines for MOTD

This document provides guidelines for AI agents and developers when adding new features to the MOTD (Message of the Day) Go implementation.

## Project Overview

**Purpose**: Display system information, service status, and media service statistics on login or command execution.

**Language**: Go 1.20+  
**Architecture**: Single-file monolithic design for simplicity  
**Performance Goal**: Maintain fast startup time (10-50ms) and low memory usage (5-10MB)

## Core Principles

### 1. Maintain Performance
- **Fast Startup**: Every millisecond matters. Avoid slow initialization.
- **Concurrent Operations**: Use goroutines for I/O-bound operations (API calls, file reads)
- **Connection Pooling**: Reuse HTTP connections via the global `httpClient`
- **Fail Fast**: Don't wait for timeouts; use aggressive timeout values

### 2. Graceful Degradation
- **Optional Features**: Missing tools/services should not cause errors
- **Silent Failures**: If a service is unavailable, skip it silently (unless debug mode)
- **Command Checks**: Always verify commands exist with `hasCommand()` before using
- **API Failures**: Handle HTTP errors gracefully; show nothing rather than error messages

### 3. Code Organization
- **Function Naming**: Use `show*()` for display functions (e.g., `showPlex()`, `showDocker()`)
- **Helper Functions**: Keep utilities at the bottom of the file
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
- Media Services: `showPlex()`, `showJellyfin()`, etc.

### Adding a New Media Service Integration

```go
func showNewService() {
    // 1. Check if configured (API key/token exists)
    if config.NewServiceAPIKey == "" {
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
1. Add fields to `Config` struct
2. Add to `loadConfig()` function
3. Add to `hasMediaServices()` check
4. Add environment variable to documentation

### Adding Command-Line Flags

```go
// In main(), after existing flag definitions:
newFlag := flag.Bool("n", false, "Enable new feature")

// After flag.Parse():
if *newFlag {
    // Handle new flag behavior
}
```

### Adding Environment Variables

```go
// In Config struct:
type Config struct {
    // ... existing fields ...
    NewServiceURL    string
    NewServiceAPIKey string
}

// In loadConfig():
config = Config{
    // ... existing fields ...
    NewServiceURL:    getEnv("NEW_SERVICE_URL", "http://localhost:8080"),
    NewServiceAPIKey: getEnv("NEW_SERVICE_API_KEY", ""),
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
# Test with no configuration (should degrade gracefully)
./motd

# Test with debug mode
./motd -d

# Test help and version
./motd -h
./motd -v

# Test with partial configuration
PLEX_TOKEN=test ./motd
```

### Performance Testing
```bash
# Benchmark startup time
time ./motd > /dev/null

# Should complete in < 100ms typically
# If > 500ms, investigate slow operations
```

### Build Testing
```bash
# Ensure code compiles
go build -o motd main.go

# Check for common issues
go vet main.go
gofmt -d main.go

# Test optimized build
go build -ldflags="-s -w" -o motd main.go
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

### Why Single File?
- **Simplicity**: Easy to understand and deploy
- **Performance**: No import overhead between packages
- **Portability**: Single binary, single source file
- **Maintenance**: All code in one place for small utility

**When to split**: If file exceeds 2000 lines or has clearly separable concerns (e.g., move all media service functions to `media.go`).

### Why Global httpClient?
- **Connection Pooling**: Reuses TCP connections across API calls
- **Performance**: Avoids connection setup overhead
- **Consistency**: Single timeout configuration

### Why No Concurrency (Yet)?
- **Simplicity**: Sequential execution is easier to debug
- **Current Performance**: Already 5-10x faster than Bash
- **Future Enhancement**: Can easily add goroutines when needed

**Adding concurrency** (future):
```go
// Use sync.WaitGroup for concurrent API calls
var wg sync.WaitGroup
results := make(chan string, 5)

wg.Add(1)
go func() {
    defer wg.Done()
    // API call here
    results <- "result"
}()

wg.Wait()
close(results)
```

## Documentation Requirements

When adding features, update:

1. **This file (AGENT.md)**: Add patterns for new feature types
2. **README.md**: Document user-facing changes
3. **Usage function**: Update help text if adding flags
4. **Environment variables list**: Update if adding config options
5. **REFACTORING_SUMMARY.md**: Note significant architectural changes

## Configuration Management

### Adding New Environment Variables
1. Add to `Config` struct
2. Add to `loadConfig()` with default value
3. Document in `usage()` function
4. Add to README.md environment variables section
5. Consider adding to `.env` file example

### Configuration Priority
1. Environment variables (highest)
2. .env file
3. Default values (lowest)

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
./motd -d
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
**Solution**: Use `time ./motd -d` and check debug logs for slow operations

**Problem**: HTTP timeout  
**Solution**: Increase `CURL_TIMEOUT` constant or check service availability

**Problem**: Parsing errors  
**Solution**: Print raw response with `debugLog("raw: %s", string(body))`

## Version Management

### Updating Version Number
```go
const VERSION = "2.1.0" // Update here
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

### Required (Build)
- Go 1.20+ 
- Standard build tools (make, etc.)

## Security Considerations

### API Keys and Tokens
- ✅ Load from environment variables or .env file
- ✅ Never hardcode in source
- ✅ Don't log in non-debug mode
- ❌ Never commit .env files with real credentials

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
- [ ] Concurrent API calls with goroutines
- [ ] Response caching with TTL
- [ ] Configuration file support (YAML/TOML)

### Medium Priority
- [ ] Metrics export (Prometheus format)
- [ ] JSON output mode for scripting
- [ ] Plugin system for custom checks
- [ ] Threshold-based color coding

### Low Priority
- [ ] Web dashboard (HTTP server)
- [ ] Historical metrics storage
- [ ] Email/Slack notifications
- [ ] Custom themes/color schemes

## Getting Help

### For AI Agents
- Read this file completely before making changes
- Follow existing patterns in the codebase
- Test changes with `make build-optimized && ./motd`
- Check performance with `time ./motd`

### For Developers
- Review `README.md` for project overview
- Review `REFACTORING_SUMMARY.md` for technical details
- Use `./motd -d` for debugging
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
- [ ] No performance regression (test with `time ./motd`)
- [ ] No new external dependencies (if possible)
- [ ] Tested on target platform (Linux/macOS)

---

**Remember**: MOTD is a login utility. It should be fast, reliable, and never annoying. When in doubt, fail silently and maintain performance.
