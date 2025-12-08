# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build Commands

```bash
# Build the binary
go build -o that-cli-web-toolbox .

# Run directly without building
go run . --screenshot https://example.com

# Build and run with Docker
docker build -t tct .
docker run --rm -it -v $(pwd):/app/data tct --screenshot https://example.com
```

## Architecture

This is a Go CLI tool for web automation using Chrome DevTools Protocol (CDP) via chromedp.

### Two-Layer Structure

1. **main.go** - CLI entry point using Cobra
   - Parses flags into `Config` struct
   - Validates input (URL vs local file, delay/timeout, mutual exclusivity)
   - Orchestrates browser actions sequentially
   - Handles file I/O for screenshots and PDFs

2. **pkg/chromedp/chromedp.go** - Browser automation wrapper
   - `Browser` struct holds context, cancel func, target URL, delay, and optional JS code
   - `InitializeChromedp()` creates browser session (local headless or remote debugging)
   - Action methods: `TakeScreenshot()`, `PrintToPDF()`, `GetTextBySelector()`, `CaptureConsoleLogs()`
   - Each action follows pattern: Navigate → Delay → Execute JS → Perform Action

### Key Dependencies

- `chromedp/chromedp` - Chrome DevTools Protocol wrapper
- `spf13/cobra` - CLI framework
- `log/slog` - Structured logging (configurable via `--loglevel`)

### Execution Flow

All browser actions share the same pattern:
1. Navigate to target URL
2. Apply rendering delay (`--delay`)
3. Execute custom JavaScript if provided (`--js` or `--js-file`)
4. Perform the actual action (screenshot, PDF, text extraction, etc.)

### Custom JavaScript Handling

The `executeJSAction()` method in chromedp.go automatically wraps code containing `await` in an async IIFE:
```go
if strings.Contains(jsCode, "await") {
    jsCode = "(async () => { " + jsCode + " })();"
}
```

### Timeout Auto-Adjustment

If `--timeout` is insufficient for `--delay`, it auto-adjusts: `timeout = delay + 10`
