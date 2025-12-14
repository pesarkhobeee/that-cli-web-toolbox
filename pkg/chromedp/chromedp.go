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
	Delay     int
	JSCode    string
}

// InitializeChromedp creates a new browser session with timeout.
// If remoteDebuggingPort is provided, connects to existing Chrome instance.
// jsCode is optional JavaScript code to execute once after navigation and delay.
func InitializeChromedp(target string, timeout int, delay int, remoteDebuggingPort string, jsCode string) (*Browser, error) {
	slog.Debug("Initializing Chrome browser", "target", target, "timeout", timeout, "delay", delay, "remotePort", remoteDebuggingPort, "hasJSCode", jsCode != "")

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
			Delay:     delay,
			JSCode:    jsCode,
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
			Delay:     delay,
			JSCode:    jsCode,
		}, nil
	}
}

// executeJSAction returns a chromedp action that executes the browser's JS code.
// If the code contains 'await', it wraps it in an async IIFE and waits for completion.
func (b *Browser) executeJSAction() chromedp.Action {
	if b.JSCode == "" {
		return chromedp.ActionFunc(func(ctx context.Context) error {
			return nil
		})
	}

	jsCode := b.JSCode
	hasAwait := strings.Contains(jsCode, "await")

	// If the code contains 'await', wrap it in an async IIFE
	if hasAwait {
		jsCode = "(async () => { " + jsCode + " })();"
	}

	return chromedp.ActionFunc(func(ctx context.Context) error {
		slog.Debug("Executing custom JavaScript", "codeLength", len(b.JSCode), "hasAwait", hasAwait)

		if hasAwait {
			// For async code, use runtime.Evaluate with awaitPromise to properly wait
			p := runtime.Evaluate(jsCode).WithAwaitPromise(true)
			_, exceptionDetails, err := p.Do(ctx)
			if err != nil {
				slog.Error("Failed to execute custom JavaScript", "error", err)
				return fmt.Errorf("failed to execute custom JavaScript: %w", err)
			}
			if exceptionDetails != nil {
				slog.Error("JavaScript exception during execution", "exception", exceptionDetails.Text)
				return fmt.Errorf("JavaScript exception: %s", exceptionDetails.Text)
			}
		} else {
			// For sync code, use regular evaluate
			var result interface{}
			if err := chromedp.Evaluate(jsCode, &result, chromedp.EvalAsValue).Do(ctx); err != nil {
				slog.Error("Failed to execute custom JavaScript", "error", err)
				return fmt.Errorf("failed to execute custom JavaScript: %w", err)
			}
		}

		slog.Debug("Custom JavaScript executed successfully")
		return nil
	})
}

// NavigateAndPrepare navigates to the target URL, applies delay, and executes custom JS.
// This should be called once before performing any actions on the page.
func (b *Browser) NavigateAndPrepare() error {
	slog.Debug("Navigating to target URL", "url", b.TargetURL)

	err := chromedp.Run(b.Ctx,
		chromedp.Navigate(b.TargetURL),
		chromedp.ActionFunc(func(ctx context.Context) error {
			slog.Debug("Applying rendering delay", "delay", b.Delay, "url", b.TargetURL)
			return nil
		}),
		chromedp.Sleep(time.Duration(b.Delay)*time.Second),
		b.executeJSAction(),
	)
	if err != nil {
		slog.Error("Failed to navigate and prepare page", "url", b.TargetURL, "error", err)
		return err
	}

	slog.Debug("Navigation and preparation completed successfully")
	return nil
}

// SetupConsoleLogListeners sets up listeners for console logs, exceptions, and dialogs.
// This should be called before NavigateAndPrepare if console log capture is needed.
func (b *Browser) SetupConsoleLogListeners() {
	slog.Debug("Setting up console log listeners")

	chromedp.ListenTarget(b.Ctx, func(ev interface{}) {
		switch ev := ev.(type) {
		case *runtime.EventConsoleAPICalled:
			// Combine all arguments into a single message
			var values []string
			for _, arg := range ev.Args {
				// arg.Value is JSON-encoded, trim quotes for strings
				val := string(arg.Value)
				if len(val) >= 2 && val[0] == '"' && val[len(val)-1] == '"' {
					val = val[1 : len(val)-1]
				}
				values = append(values, val)
			}
			slog.Info("Console message captured",
				"type", ev.Type,
				"value", strings.Join(values, " "))
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

	slog.Debug("Console log listeners set up successfully")
}

// CaptureConsoleLogs is deprecated - use SetupConsoleLogListeners instead.
// Kept for backwards compatibility but now just calls SetupConsoleLogListeners.
func (b *Browser) CaptureConsoleLogs() error {
	b.SetupConsoleLogListeners()
	return nil
}

// GetBodyText extracts all visible text from the <body>.
func (b *Browser) GetBodyText() (string, error) {
	return b.GetTextBySelector("body")
}

// GetTextBySelector extracts text from elements matching the given CSS selector.
// Assumes NavigateAndPrepare has already been called.
func (b *Browser) GetTextBySelector(selector string) (string, error) {
	slog.Debug("Extracting text by CSS selector", "selector", selector)

	var texts []string
	err := chromedp.Run(b.Ctx,
		chromedp.Evaluate(`
			Array.from(document.querySelectorAll('`+selector+`')).map(el => el.innerText.trim()).filter(text => text.length > 0)
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
// Assumes NavigateAndPrepare has already been called.
func (b *Browser) TakeScreenshot() ([]byte, error) {
	slog.Debug("Taking screenshot")

	var buf []byte
	err := chromedp.Run(b.Ctx,
		chromedp.FullScreenshot(&buf, 90),
	)
	if err != nil {
		slog.Error("Failed to capture screenshot", "error", err)
		return nil, err
	}

	slog.Debug("Screenshot captured successfully", "size", len(buf))
	return buf, nil
}

// PrintToPDF generates a PDF of the current page.
// Assumes NavigateAndPrepare has already been called.
func (b *Browser) PrintToPDF() ([]byte, error) {
	slog.Debug("Generating PDF")

	var pdfBuf []byte
	err := chromedp.Run(b.Ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			pdfBuf, _, err = page.PrintToPDF().WithPrintBackground(true).Do(ctx)
			return err
		}),
	)
	if err != nil {
		slog.Error("Failed to generate PDF", "error", err)
		return nil, err
	}

	slog.Debug("PDF generated successfully", "size", len(pdfBuf))
	return pdfBuf, nil
}
