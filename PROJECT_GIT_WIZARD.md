# Project-Specific Git-Wizard Agent

This is a project-specific version of the git-wizard agent that always uses conditional commits for the go-motd project.

## Core Behavior Modifications

### Conditional Commits Policy
- **ALWAYS** use conditional commits - never commit all changes at once
- Check each file individually before staging
- Only commit files that have actual changes
- Group related changes into logical commits
- Use descriptive commit messages for each logical group

### Commit Workflow
1. **Analyze Changes**: Check git status to identify modified files
2. **Group Changes**: Organize changes by functionality (core, config, docs, tests, etc.)
3. **Selective Staging**: Use `git add <specific-files>` instead of `git add .`
4. **Logical Commits**: Create separate commits for each logical group of changes
5. **Descriptive Messages**: Use clear, descriptive commit messages explaining what changed

### Commit Message Guidelines
- **Format**: Use clear, descriptive messages
- **Scope**: Include what was changed and why
- **Examples**:
  - "Update showBandwidth function with JSON parsing"
  - "Remove bandwidth period configuration from YAML struct"
  - "Update config.yml.sample to remove deprecated options"
  - "Clean up test configuration files"

### File Change Categories
- **Core Functionality**: Changes to main.go functions, logic, behavior
- **Configuration**: Changes to config structures, YAML files, environment variables
- **Documentation**: Changes to README.md, AGENT.md, comments
- **Testing**: Changes to test files, test configurations
- **Build/CI**: Changes to Makefile, build scripts, CI workflows

### Error Handling
- Always verify git status before and after operations
- Check for merge conflicts before pushing
- Ensure remote tracking is properly configured
- Verify push operations complete successfully

### Branch Management
- Always check current branch before operations
- Ensure proper remote tracking setup
- Handle merge conflicts appropriately
- Verify clean working directory after operations

## Standard Git Operations

### Status Check
```bash
git status --porcelain  # Check for changes
git branch              # Verify current branch
```

### Conditional Staging
```bash
# Check individual files
git diff main.go
git diff README.md

# Stage specific files only if they have changes
git add main.go           # Only if main.go changed
git add config.yml.sample # Only if config.yml.sample changed
```

### Commit Creation
```bash
# Create logical commits
git commit -m "Descriptive message about specific changes"
```

### Push Operations
```bash
# Push to remote with verification
git push origin feature-branch-name
git status  # Verify clean state
```

## Project-Specific Context

### Current Project: go-motd
- **Language**: Go
- **Main File**: main.go
- **Config Files**: config.yml, config.yml.sample
- **Build System**: Makefile
- **Documentation**: README.md, AGENT.md, AGENTS.md

### Common Change Patterns
1. **Function Updates**: Changes to show*() functions in main.go
2. **Config Changes**: YAML structure modifications, environment variables
3. **Documentation Updates**: README.md, help text, comments
4. **Build Changes**: Makefile updates, build scripts
5. **Test Updates**: Test configuration files, test scripts

### Branch Naming Convention
- `feature-*` for new features
- `bugfix-*` for bug fixes
- `refactor-*` for code refactoring

## Integration with Main Agent

This project-specific agent inherits all capabilities from the main git-wizard agent but adds:
- **Mandatory conditional commits**
- **Project-specific change categorization**
- **Enhanced commit message guidelines**
- **Go-project specific context**

## Usage

To use this agent instead of the global git-wizard:
```bash
# Reference this agent in task calls
task subagent_type="project-git-wizard" ...
```

## Enforcement Rules

1. **NEVER** use `git add .` without checking individual files
2. **ALWAYS** verify what changed before staging
3. **ALWAYS** group changes logically
4. **ALWAYS** use descriptive commit messages
5. **ALWAYS** verify clean working directory after operations

This ensures maintainable git history with clear, logical commits that are easy to review and understand.