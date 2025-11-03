# Bandwidth Configuration Update Summary

## Changes Made

### 1. Configuration Structure Update
- Added `Bandwidth` field to `Config.System.Network` struct
- Field accepts "daily" or "monthly" values
- Defaults to "daily" when not specified

### 2. showBandwidth Function Enhancement
- **Default behavior changed**: Now shows daily data by default instead of monthly
- **Configuration support**: Respects `system.network.bandwidth` setting from YAML config
- **Enhanced JSON parsing**: Properly handles both daily and monthly data from vnstat
- **Improved labeling**: Shows context-specific labels:
  - Daily: "(today)" suffix
  - Monthly: "(this month)" suffix with estimates
- **Fallback handling**: Gracefully falls back to total data when specific period data unavailable

### 3. Legacy Configuration Support
- Added `BANDWIDTH_PERIOD` environment variable support
- Maintains backward compatibility with existing setups
- Environment variable takes precedence when YAML config not available

### 4. Migration Functionality Update
- Enhanced migration to include bandwidth setting validation
- Updated migration summary to show bandwidth configuration
- Properly handles bandwidth setting in config file generation

### 5. Documentation Updates
- Updated help text to include `BANDWIDTH_PERIOD` environment variable
- Enhanced test configuration with bandwidth setting examples
- Added inline comments explaining configuration options

## Configuration Examples

### YAML Configuration
```yaml
system:
  network:
    interface: "eth0"
    bandwidth: "daily"    # or "monthly"
```

### Environment Variable
```bash
export BANDWIDTH_PERIOD="daily"  # or "monthly"
```

## Output Examples

### Daily Output (Default)
```
Bandwidth (rx) (today): 0.85 GB
Bandwidth (tx) (today): 1.23 GB
```

### Monthly Output
```
Bandwidth (rx) (this month): 15.42 GB / 46.26 GB est
Bandwidth (tx) (this month): 22.18 GB / 66.54 GB est
```

## Backward Compatibility
- Existing configurations without bandwidth setting will default to daily
- All existing functionality preserved
- No breaking changes to existing API or configuration structure

## Testing
- Created test configuration files for daily, monthly, and default scenarios
- Verified compilation and basic functionality
- Tested migration functionality with new bandwidth setting
- Confirmed help text updates

## Files Modified
- `main.go`: Core implementation changes
- `test-config.yml`: Updated with bandwidth setting example
- Created additional test files for validation

The implementation successfully changes the default bandwidth display to show daily data while providing users the flexibility to choose between daily and monthly views through configuration.