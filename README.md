# That CLI Web Toolbox

```bash
An easy to use Swiss army knife for web in CLI.

Features:
  • Take screenshots of web pages
  • Generate PDFs from web pages
  • Capture console logs and JavaScript exceptions
  • Extract text content from pages
  • Extract text using CSS selectors
  • Support for both local HTML files and remote URLs
  • Connect to existing Chrome instances with remote debugging
  • Configurable logging levels for debugging
  • Configurable delay to ensure proper page rendering (timeout auto-adjusts if needed)

Examples:
  # Take a screenshot of a website
  that-cli-web-toolbox --screenshot https://example.com

  # Extract all text from a page with debug logging
  that-cli-web-toolbox --body --loglevel debug https://example.com

  # Get text by CSS selector from a local HTML file
  that-cli-web-toolbox --gettextbycssselector "h1" ./index.html

  # Generate PDF and capture console logs
  that-cli-web-toolbox --printtopdf --consolelog https://example.com

  # Take screenshot with custom delay for slow-loading pages
  that-cli-web-toolbox --screenshot --delay 5 https://example.com

  # Connect to existing Chrome with remote debugging
  that-cli-web-toolbox --remote-debugging-port localhost:9222 --screenshot https://example.com

Usage:
  that-cli-web-toolbox [flags]

Flags:
  -b, --body                           Get the body text of the page
  -c, --consolelog                     Capture console logs from the page
  -d, --delay int                      Delay in seconds to ensure rendering (timeout auto-adjusts if needed) (default 2)
  -g, --gettextbycssselector string    Get text by CSS selector
  -h, --help                           help for that-cli-web-toolbox
  -l, --loglevel string                Set the logging level (debug, info, warn, error) (default "info")
  -p, --printtopdf                     Print the page to a PDF file
  -r, --remote-debugging-port string   Connect to existing Chrome instance with remote debugging (e.g., localhost:9222)
  -s, --screenshot                     Take a screenshot of the page
  -t, --timeout int                    Timeout in seconds (default 10)
```

## Running with help of Docker

Instead of using project's binary file you can utulize Docker:

```bash
docker build -t tct .
docker run --rm -it -v $(pwd):/app/data tct --screenshot https://example.com
```

## Connecting to Existing Chrome Instance

You can connect to an existing Chrome browser instance that has remote debugging enabled instead of creating a new headless instance.

### Starting Chrome with Remote Debugging

First, start Chrome with remote debugging enabled:

**macOS:**
```bash
/Applications/Google\ Chrome.app/Contents/MacOS/Google\ Chrome \
  --remote-debugging-port=9222 \
  --user-data-dir="$HOME/Library/Application Support/Google/Chrome/RemoteDebug"
```

**Linux:**
```bash
google-chrome \
  --remote-debugging-port=9222 \
  --user-data-dir="$HOME/.config/google-chrome/RemoteDebug"
```

**Windows:**
```bash
"C:\Program Files\Google\Chrome\Application\chrome.exe" \
  --remote-debugging-port=9222 \
  --user-data-dir="%USERPROFILE%\AppData\Local\Google\Chrome\User Data\RemoteDebug"
```

### Connecting to the Browser

Once Chrome is running with remote debugging, connect using:

```bash
# Multiple actions on existing browser
./that-cli-web-toolbox -r localhost:9222 --screenshot --printtopdf --consolelog https://example.com
```

### Benefits of Connecting to Existing Browser

- **Reuse existing sessions**: Maintain login states and cookies
- **Remote automation**: Connect to Chrome running on different machines
- **Performance**: Avoid browser startup overhead for multiple operations

### Remote Debugging Security Notes

- Remote debugging should only be enabled on trusted networks
- Consider using a separate user data directory for automation
- The remote debugging port provides full browser control to anyone with access

### Troubleshooting Remote Connections

**Connection Failed:**
```bash
# Check if Chrome is running with remote debugging
curl http://localhost:9222/json/version

# Should return JSON with browser version info
```

**Port Already in Use:**
```bash
# Find processes using the port
lsof -i :9222

# Kill existing Chrome processes if needed
pkill -f "chrome.*remote-debugging"
```

## Timeout and Delay Relationship

The tool automatically manages the relationship between `--timeout` and `--delay` to prevent conflicts:
