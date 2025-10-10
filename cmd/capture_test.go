package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-rod/rod"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"roderik/browser"
)

func resetScreenshotFlags() {
	screenshotOutput = ""
	screenshotDir = ""
	screenshotName = ""
	screenshotFormat = "png"
	screenshotSelector = ""
	screenshotFullPage = false
	screenshotScroll = false
	screenshotQuality = 90
	screenshotCmd.Flags().VisitAll(func(f *pflag.Flag) {
		_ = f.Value.Set(f.DefValue)
		f.Changed = false
	})
}

func resetPDFFlags() {
	pdfOutput = ""
	pdfDir = ""
	pdfName = ""
	pdfLandscape = false
	pdfDisplayHeaderFooter = false
	pdfPrintBackground = false
	pdfPreferCSSPageSize = false
	pdfGenerateTagged = false
	pdfGenerateOutline = false
	pdfScale = 1.0
	pdfPageSize = "letter"
	pdfMargin = 0.5
	pdfCmd.Flags().VisitAll(func(f *pflag.Flag) {
		_ = f.Value.Set(f.DefValue)
		f.Changed = false
	})
}

func setupBrowserState() {
	Browser = &rod.Browser{}
	Page = &rod.Page{}
	browserInitErr = nil
	Desktop = false
}

func mustSetFlag(t *testing.T, cmd *cobra.Command, name, value string) {
	t.Helper()
	if err := cmd.Flags().Set(name, value); err != nil {
		t.Fatalf("failed to set flag %s: %v", name, err)
	}
}

func TestScreenshotCmdAppliesJPEGOptions(t *testing.T) {
	resetScreenshotFlags()
	setupBrowserState()

	dir := t.TempDir()
	data := []byte{0x89, 0x50, 0x4e}
	var capturedOpts browser.ScreenshotOptions
	orig := captureScreenshotFunc
	captureScreenshotFunc = func(p *rod.Page, opts browser.ScreenshotOptions) (*browser.Result, error) {
		capturedOpts = opts
		return &browser.Result{Data: data, MimeType: "image/jpeg"}, nil
	}
	t.Cleanup(func() { captureScreenshotFunc = orig })

	mustSetFlag(t, screenshotCmd, "dir", dir)
	mustSetFlag(t, screenshotCmd, "name", "home")
	mustSetFlag(t, screenshotCmd, "format", "jpeg")
	mustSetFlag(t, screenshotCmd, "quality", "80")
	mustSetFlag(t, screenshotCmd, "full-page", "true")

	if err := screenshotCmd.RunE(screenshotCmd, nil); err != nil {
		t.Fatalf("screenshot command failed: %v", err)
	}

	if !capturedOpts.FullPage {
		t.Fatalf("expected FullPage true")
	}
	if capturedOpts.Scroll {
		t.Fatalf("expected Scroll false")
	}
	if capturedOpts.Format != "jpeg" {
		t.Fatalf("expected format jpeg, got %s", capturedOpts.Format)
	}
	if capturedOpts.Quality == nil || *capturedOpts.Quality != 80 {
		t.Fatalf("quality pointer not set to 80: %#v", capturedOpts.Quality)
	}

	outputPath := filepath.Join(dir, "home.jpg")
	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("expected output file: %v", err)
	}
	if string(content) != string(data) {
		t.Fatalf("unexpected file contents: %v", content)
	}
}

func TestScreenshotCmdRejectsConflictingFlags(t *testing.T) {
	resetScreenshotFlags()
	setupBrowserState()

	orig := captureScreenshotFunc
	captureScreenshotFunc = func(p *rod.Page, opts browser.ScreenshotOptions) (*browser.Result, error) {
		t.Fatalf("capture function should not be called on conflict")
		return nil, nil
	}
	t.Cleanup(func() { captureScreenshotFunc = orig })

	mustSetFlag(t, screenshotCmd, "selector", "body")
	mustSetFlag(t, screenshotCmd, "scroll", "true")

	if err := screenshotCmd.RunE(screenshotCmd, nil); err == nil {
		t.Fatalf("expected error when selector and scroll are both set")
	}
}

func TestPDFCmdParsesSizeAndMargin(t *testing.T) {
	resetPDFFlags()
	setupBrowserState()

	dir := t.TempDir()
	data := []byte("%PDF-1.7")
	var capturedOpts browser.PDFOptions
	orig := capturePDFFunc
	capturePDFFunc = func(p *rod.Page, opts browser.PDFOptions) (*browser.Result, error) {
		capturedOpts = opts
		return &browser.Result{Data: data, MimeType: "application/pdf"}, nil
	}
	t.Cleanup(func() { capturePDFFunc = orig })

	mustSetFlag(t, pdfCmd, "dir", dir)
	mustSetFlag(t, pdfCmd, "name", "report")
	mustSetFlag(t, pdfCmd, "size", "210mmx297mm")
	mustSetFlag(t, pdfCmd, "margin", "1.25")
	mustSetFlag(t, pdfCmd, "landscape", "true")
	mustSetFlag(t, pdfCmd, "scale", "1.4")

	if err := pdfCmd.RunE(pdfCmd, nil); err != nil {
		t.Fatalf("pdf command failed: %v", err)
	}

	if capturedOpts.PaperWidth == nil || capturedOpts.PaperHeight == nil {
		t.Fatalf("expected paper dimensions to be set")
	}
	// 210mm -> 8.2677165 inches, 297mm -> 11.692913 inches
	if diff := abs(*capturedOpts.PaperWidth - 8.2677); diff > 0.001 {
		t.Fatalf("unexpected paper width: %f", *capturedOpts.PaperWidth)
	}
	if diff := abs(*capturedOpts.PaperHeight - 11.6929); diff > 0.001 {
		t.Fatalf("unexpected paper height: %f", *capturedOpts.PaperHeight)
	}
	if capturedOpts.MarginTop == nil || *capturedOpts.MarginTop != 1.25 {
		t.Fatalf("margin top not applied: %#v", capturedOpts.MarginTop)
	}
	if capturedOpts.Landscape != true {
		t.Fatalf("expected landscape true")
	}
	if capturedOpts.Scale == nil || *capturedOpts.Scale != 1.4 {
		t.Fatalf("scale not applied: %#v", capturedOpts.Scale)
	}

	outputPath := filepath.Join(dir, "report.pdf")
	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("expected output file: %v", err)
	}
	if string(content) != string(data) {
		t.Fatalf("unexpected file contents: %v", content)
	}
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
