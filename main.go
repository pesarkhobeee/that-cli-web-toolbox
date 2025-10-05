package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	chromedphelper "github.com/pesarkhobeee/that-cli-web-toolbox/pkg/chromedp"
)

type Config struct {
	ConsoleLog           bool
	Screenshot           bool
	PrintToPDF           bool
	GetBody              bool
	GetTextByCssSelector string
	Timeout              int
	Delay                int
	Target               string
	LogLevel             string
	RemoteDebuggingPort  string
}

var cfg Config

var rootCmd = &cobra.Command{
	Use:   "that-cli-web-toolbox",
	Short: "A powerful CLI tool for web automation tasks including screenshots, PDFs, console logs, and text extraction",
	Long: `An easy to use Swiss army knife for web in CLI.

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

  # Use large delay (timeout will be auto-adjusted to 25 seconds)
  that-cli-web-toolbox --screenshot --delay 15 https://slow-site.com

  # Connect to existing Chrome with remote debugging
  that-cli-web-toolbox --remote-debugging-port localhost:9222 --screenshot https://example.com`,
	RunE: runThatCliWebBrowser,
	Args: cobra.ExactArgs(1),
}

func init() {
	rootCmd.Flags().BoolVarP(&cfg.ConsoleLog, "consolelog", "c", false, "Capture console logs from the page")
	rootCmd.Flags().BoolVarP(&cfg.Screenshot, "screenshot", "s", false, "Take a screenshot of the page")
	rootCmd.Flags().BoolVarP(&cfg.PrintToPDF, "printtopdf", "p", false, "Print the page to a PDF file")
	rootCmd.Flags().BoolVarP(&cfg.GetBody, "body", "b", false, "Get the body text of the page")
	rootCmd.Flags().StringVarP(&cfg.GetTextByCssSelector, "gettextbycssselector", "g", "", "Get text by CSS selector")
	rootCmd.Flags().IntVarP(&cfg.Timeout, "timeout", "t", 10, "Timeout in seconds")
	rootCmd.Flags().IntVarP(&cfg.Delay, "delay", "d", 2, "Delay in seconds to ensure rendering (timeout auto-adjusts if needed)")
	rootCmd.Flags().StringVarP(&cfg.LogLevel, "loglevel", "l", "info",
		"Set the logging level (debug, info, warn, error)")
	rootCmd.Flags().StringVarP(&cfg.RemoteDebuggingPort, "remote-debugging-port", "r", "",
		"Connect to existing Chrome instance with remote debugging (e.g., localhost:9222)")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func runThatCliWebBrowser(cmd *cobra.Command, args []string) error {
	// Initialize slog directly
	var level slog.Level
	switch strings.ToLower(cfg.LogLevel) {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: level}
	handler := slog.NewTextHandler(os.Stderr, opts)
	logger := slog.New(handler)
	slog.SetDefault(logger)

	slog.Debug("Starting that-cli-web-toolbox",
		"timeout", cfg.Timeout,
		"delay", cfg.Delay,
		"logLevel", cfg.LogLevel,
		"consoleLog", cfg.ConsoleLog,
		"screenshot", cfg.Screenshot,
		"printToPDF", cfg.PrintToPDF,
		"getBody", cfg.GetBody,
		"cssSelector", cfg.GetTextByCssSelector)

	if len(args) == 0 {
		slog.Error("No target URL or file path provided")
		return fmt.Errorf("target URL or file path is required")
	}

	input := args[0]
	slog.Debug("Processing input", "input", input)

	// Validate input
	if strings.TrimSpace(input) == "" {
		slog.Error("Empty target provided")
		return fmt.Errorf("target cannot be empty")
	}

	// Detect if input is a local file
	var target string
	if _, err := os.Stat(input); err == nil {
		abs, err := filepath.Abs(input)
		if err != nil {
			slog.Error("Failed to get absolute path", "input", input, "error", err)
			return fmt.Errorf("failed to get absolute path for %q: %w", input, err)
		}
		target = "file://" + abs
		slog.Debug("Input detected as local file", "absolutePath", abs)
	} else {
		// Basic URL validation
		if !strings.HasPrefix(input, "http://") && !strings.HasPrefix(input, "https://") && !strings.HasPrefix(input, "file://") {
			slog.Warn("Input does not appear to be a valid URL, treating as URL anyway", "input", input)
		}
		target = input
		slog.Debug("Input treated as URL", "url", target)
	}
	cfg.Target = target

	// Validate delay parameter
	if cfg.Delay < 0 {
		slog.Error("Invalid delay value", "delay", cfg.Delay)
		return fmt.Errorf("delay cannot be negative: %d", cfg.Delay)
	}
	if cfg.Delay > 60 {
		slog.Warn("Large delay value specified", "delay", cfg.Delay)
	}

	// Adjust timeout if it's insufficient for the specified delay
	if cfg.Timeout <= (cfg.Delay + 10) {
		originalTimeout := cfg.Timeout
		cfg.Timeout = cfg.Delay + 10 // Add 10 second buffer
		slog.Info("Timeout automatically adjusted to accommodate delay",
			"originalTimeout", originalTimeout,
			"delay", cfg.Delay,
			"newTimeout", cfg.Timeout)
	}

	// Validate that at least one action is specified
	if !cfg.ConsoleLog && !cfg.Screenshot && !cfg.PrintToPDF && !cfg.GetBody && cfg.GetTextByCssSelector == "" {
		slog.Error("No action specified")
		return fmt.Errorf("at least one action must be specified (--body, --screenshot, --printtopdf, --consolelog, or --gettextbycssselector)")
	}

	// Initialize browser
	if cfg.RemoteDebuggingPort != "" {
		slog.Debug("Connecting to existing browser", "target", cfg.Target, "timeout", cfg.Timeout, "delay", cfg.Delay, "remotePort", cfg.RemoteDebuggingPort)
	} else {
		slog.Debug("Initializing new browser", "target", cfg.Target, "timeout", cfg.Timeout, "delay", cfg.Delay)
	}
	browser, err := chromedphelper.InitializeChromedp(cfg.Target, cfg.Timeout, cfg.Delay, cfg.RemoteDebuggingPort)
	if err != nil {
		slog.Error("Failed to initialize browser", "error", err)
		return fmt.Errorf("failed to initialize browser: %w", err)
	}
	defer browser.Cancel()

	// Handle GetTextByCssSelector
	if cfg.GetTextByCssSelector != "" {
		slog.Debug("Getting text by CSS selector", "selector", cfg.GetTextByCssSelector)
		text, err := browser.GetTextBySelector(cfg.GetTextByCssSelector)
		if err != nil {
			slog.Error("Failed to get text by selector", "selector", cfg.GetTextByCssSelector, "error", err)
			return fmt.Errorf("failed to get text by selector: %w", err)
		}
		slog.Debug("Successfully extracted text", "selector", cfg.GetTextByCssSelector, "textLength", len(text))
		fmt.Println(text)
	}

	// Handle GetBody
	if cfg.GetBody {
		slog.Info("Getting body text")
		text, err := browser.GetBodyText()
		if err != nil {
			slog.Error("Failed to get body text", "error", err)
			return fmt.Errorf("failed to get body text: %w", err)
		}
		slog.Debug("Successfully extracted body text", "textLength", len(text))
		fmt.Println(text)
	}

	// Handle console logs
	if cfg.ConsoleLog {
		slog.Info("Starting console log capture")
		if err := browser.CaptureConsoleLogs(); err != nil {
			slog.Error("Failed to capture console logs", "error", err)
			return fmt.Errorf("failed to capture console logs: %w", err)
		}
	}

	// Handle screenshot
	if cfg.Screenshot {
		slog.Info("Taking screenshot")
		imageBuf, err := browser.TakeScreenshot()
		if err != nil {
			slog.Error("Failed to take screenshot", "error", err)
			return fmt.Errorf("failed to take screenshot: %w", err)
		}

		fileName := fmt.Sprintf("screenshot_%s.png", time.Now().Format("20060102150405"))
		slog.Debug("Saving screenshot", "fileName", fileName, "size", len(imageBuf))
		if err := os.WriteFile(fileName, imageBuf, 0o644); err != nil {
			slog.Error("Failed to save screenshot", "fileName", fileName, "error", err)
			return fmt.Errorf("failed to save screenshot %q: %w", fileName, err)
		}
		slog.Info("Screenshot saved successfully", "fileName", fileName)
		fmt.Printf("Screenshot saved as %s\n", fileName)
	}

	// Handle print to PDF
	if cfg.PrintToPDF {
		slog.Info("Printing to PDF")
		pdfBuf, err := browser.PrintToPDF()
		if err != nil {
			slog.Error("Failed to print to PDF", "error", err)
			return fmt.Errorf("failed to print to PDF: %w", err)
		}

		fileName := fmt.Sprintf("page_%s.pdf", time.Now().Format("20060102150405"))
		slog.Debug("Saving PDF", "fileName", fileName, "size", len(pdfBuf))
		if err := os.WriteFile(fileName, pdfBuf, 0o644); err != nil {
			slog.Error("Failed to save PDF", "fileName", fileName, "error", err)
			return fmt.Errorf("failed to save PDF %q: %w", fileName, err)
		}
		slog.Info("PDF saved successfully", "fileName", fileName)
		fmt.Printf("PDF saved as %s\n", fileName)
	}

	slog.Debug("Command execution completed successfully")
	return nil
}
