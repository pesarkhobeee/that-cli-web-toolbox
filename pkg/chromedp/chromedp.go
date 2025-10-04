package chromedphelper

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

// Browser wraps a Chromedp context and target.
type Browser struct {
	Ctx       context.Context
	Cancel    context.CancelFunc
	TargetURL string
}

// InitializeChromedp creates a new browser session with timeout.
// If remoteDebuggingPort is provided, connects to existing Chrome instance.
func InitializeChromedp(target string, timeout int, remoteDebuggingPort string) (*Browser, error) {
	slog.Debug("Initializing Chrome browser", "target", target, "timeout", timeout, "remotePort", remoteDebuggingPort)

	var allocCtx context.Context
	var cancelAlloc context.CancelFunc

	if remoteDebuggingPort != "" {
		// Connect to existing Chrome instance
		remoteURL := remoteDebuggingPort
		if !strings.HasPrefix(remoteURL, "http://") && !strings.HasPrefix(remoteURL, "https://") {
			remoteURL = "http://" + remoteURL
		}

		// Validate format: should contain host:port
		if !strings.Contains(remoteDebuggingPort, ":") {
			return nil, fmt.Errorf("invalid remote debugging port format: %s (expected format: localhost:9222)", remoteDebuggingPort)
		}

		// Additional validation for common mistakes
		parts := strings.Split(remoteDebuggingPort, ":")
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return nil, fmt.Errorf("invalid remote debugging port format: %s (expected format: host:port)", remoteDebuggingPort)
		}

		// Test connection before proceeding
		testURL := remoteURL + "/json/version"
		slog.Debug("Testing connection to remote Chrome instance", "testURL", testURL)

		client := &http.Client{Timeout: 3 * time.Second}
		resp, err := client.Get(testURL)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to remote debugging port %s: %w (ensure Chrome is running with --remote-debugging-port=%s)", remoteDebuggingPort, err, strings.Split(remoteDebuggingPort, ":")[1])
		}
		if err := resp.Body.Close(); err != nil {
			slog.Warn("failed to close response body", "error", err)
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("remote debugging endpoint returned status %d at %s (ensure Chrome is running with remote debugging enabled)", resp.StatusCode, testURL)
		}

		slog.Debug("Successfully connected to remote Chrome instance", "url", remoteURL)
		// Create allocator context for remote debugging
		allocCtx, cancelAlloc = chromedp.NewRemoteAllocator(context.Background(), remoteURL)

		// Create a new task context from the allocator context (not a timeout context)
		taskCtx, cancelTask := chromedp.NewContext(allocCtx)

		// Apply timeout to the task context
		ctx, cancelCtx := context.WithTimeout(taskCtx, time.Duration(timeout)*time.Second)

		slog.Debug("Remote Chrome context created successfully")

		return &Browser{
			Ctx:       ctx,
			Cancel:    func() { cancelCtx(); cancelTask(); cancelAlloc() },
			TargetURL: target,
		}, nil
	} else {
		// Create new headless Chrome instance
		slog.Debug("Creating new headless Chrome instance")
		allocCtx, cancelAlloc = chromedp.NewContext(context.Background())

		ctx, cancelCtx := context.WithTimeout(allocCtx, time.Duration(timeout)*time.Second)

		slog.Debug("Chrome context created successfully")

		return &Browser{
			Ctx:       ctx,
			Cancel:    func() { cancelCtx(); cancelAlloc() },
			TargetURL: target,
		}, nil
	}
}

// CaptureConsoleLogs starts listening for console logs, exceptions, and dialogs.
func (b *Browser) CaptureConsoleLogs() error {
	slog.Debug("Setting up console log listeners")

	chromedp.ListenTarget(b.Ctx, func(ev interface{}) {
		switch ev := ev.(type) {
		case *runtime.EventConsoleAPICalled:
			for _, arg := range ev.Args {
				slog.Info("Console message captured",
					"type", ev.Type,
					"value", arg.Value)
			}
		case *runtime.EventExceptionThrown:
			slog.Error("JavaScript exception captured",
				"text", ev.ExceptionDetails.Text)
			if ev.ExceptionDetails.StackTrace != nil {
				for _, frame := range ev.ExceptionDetails.StackTrace.CallFrames {
					slog.Debug("Stack trace frame",
						"function", frame.FunctionName,
						"url", frame.URL,
						"line", frame.LineNumber,
						"column", frame.ColumnNumber)
				}
			}
		case *page.EventJavascriptDialogOpening:
			slog.Debug("JavaScript dialog detected, handling automatically")
			go func() {
				if err := chromedp.Run(b.Ctx, page.HandleJavaScriptDialog(true)); err != nil {
					slog.Error("Failed to handle JavaScript dialog", "error", err)
				}
			}()
		}
	})

	slog.Debug("Navigating to target URL for console log capture", "url", b.TargetURL)

	if err := chromedp.Run(b.Ctx,
		chromedp.Navigate(b.TargetURL),
	); err != nil {
		slog.Error("Failed to navigate to target URL", "url", b.TargetURL, "error", err)
		return err
	}

	slog.Debug("Console log capture completed")
	return nil
}

// GetBodyText extracts all visible text from the <body>.
func (b *Browser) GetBodyText() (string, error) {
	return b.GetTextBySelector("body")
}

// GetTextBySelector extracts text from elements matching the given CSS selector.
func (b *Browser) GetTextBySelector(selector string) (string, error) {
	slog.Debug("Extracting text by CSS selector", "selector", selector, "url", b.TargetURL)

	var texts []string
	err := chromedp.Run(b.Ctx,
		chromedp.Navigate(b.TargetURL),
		chromedp.Evaluate(`
			Array.from(document.querySelectorAll('`+selector+`')).map(el => el.textContent.trim()).filter(text => text.length > 0)
		`, &texts),
	)
	if err != nil {
		slog.Error("Failed to extract text by selector", "selector", selector, "error", err)
		return "", err
	}

	result := ""
	for i, text := range texts {
		if i > 0 {
			result += "\n"
		}
		result += text
	}

	slog.Debug("Successfully extracted text", "selector", selector, "elementsFound", len(texts), "totalTextLength", len(result))
	return result, nil
}

// TakeScreenshot captures a screenshot of the current page.
func (b *Browser) TakeScreenshot() ([]byte, error) {
	slog.Debug("Taking screenshot", "url", b.TargetURL)

	var buf []byte
	err := chromedp.Run(b.Ctx,
		chromedp.Navigate(b.TargetURL),
		chromedp.FullScreenshot(&buf, 90),
	)
	if err != nil {
		slog.Error("Failed to capture screenshot", "url", b.TargetURL, "error", err)
		return nil, err
	}

	slog.Debug("Screenshot captured successfully", "size", len(buf))
	return buf, nil
}

// PrintToPDF generates a PDF of the current page.
func (b *Browser) PrintToPDF() ([]byte, error) {
	slog.Debug("Generating PDF", "url", b.TargetURL)

	var pdfBuf []byte
	err := chromedp.Run(b.Ctx,
		chromedp.Navigate(b.TargetURL),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			pdfBuf, _, err = page.PrintToPDF().WithPrintBackground(true).Do(ctx)
			return err
		}),
	)
	if err != nil {
		slog.Error("Failed to generate PDF", "url", b.TargetURL, "error", err)
		return nil, err
	}

	slog.Debug("PDF generated successfully", "size", len(pdfBuf))
	return pdfBuf, nil
}
