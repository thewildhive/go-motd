# Progress

## Issues completed in this batch

### Group: update + util (worktree)

- **#12 (critical)**: Cross-device os.Rename failure during self-update
  - Fixed `replaceBinary()` in update/update.go: when os.Rename fails with a
    cross-device link error (EXDEV), falls back to read+write copy + remove
  - Added `copyFileContents()` helper

- **#13 (high)**: copyFile does not sync or check Close errors
  - Fixed `CopyFile()` in util/util.go: calls Sync() after io.Copy, checks
    Close() error, cleans up destination file on failure
  - Added tests: TestCopyFile, TestCopyFileMissingSource,
    TestCopyFileSyncFailureDoesNotLeavePartial

- **#14 (high)**: fmt.Scanln blocks indefinitely in non-interactive mode
  - Fixed `HandleSelfUpdate()` in update/update.go: checks os.Stdin.Stat()
    for character device mode; if non-interactive (pipe/redirect), skips
    prompt and tells user to use --force
  - Added `isInteractive()` helper
