package cmd

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-rod/rod"
	"github.com/mark3labs/mcp-go/mcp"

	"roderik/browser"
)

func setupMCPTestState(t *testing.T) {
	Browser = &rod.Browser{}
	Page = &rod.Page{}
	browserInitErr = nil
	Desktop = false
}

func TestMCPCaptureScreenshotBinary(t *testing.T) {
	setupMCPTestState(t)

	data := []byte{0x01, 0x02, 0x03}
	var capturedOpts browser.ScreenshotOptions
	orig := captureScreenshotFunc
	captureScreenshotFunc = func(p *rod.Page, opts browser.ScreenshotOptions) (*browser.Result, error) {
		capturedOpts = opts
		return &browser.Result{Data: data, MimeType: "image/jpeg"}, nil
	}
	t.Cleanup(func() { captureScreenshotFunc = orig })

	args := map[string]interface{}{
		"selector": "#main",
		"format":   "jpeg",
		"quality":  float64(75),
		"return":   "binary",
	}

	res, err := mcpHandleCaptureScreenshot(args)
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	if capturedOpts.Selector != "#main" || capturedOpts.Format != "jpeg" {
		t.Fatalf("unexpected capture opts: %#v", capturedOpts)
	}
	if capturedOpts.Quality == nil || *capturedOpts.Quality != 75 {
		t.Fatalf("expected quality 75, got %#v", capturedOpts.Quality)
	}
	if len(res.Content) != 2 {
		t.Fatalf("expected text + image content, got %d", len(res.Content))
	}
	text, ok := res.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("first content not text: %#v", res.Content[0])
	}
	if text.Text == "" {
		t.Fatalf("text content should not be empty")
	}
	img, ok := res.Content[1].(mcp.ImageContent)
	if !ok {
		t.Fatalf("second content not image: %#v", res.Content[1])
	}
	if img.MIMEType != "image/jpeg" {
		t.Fatalf("unexpected mime type %s", img.MIMEType)
	}
	if img.Data != base64.StdEncoding.EncodeToString(data) {
		t.Fatalf("unexpected image payload")
	}
}

func TestMCPCaptureScreenshotFile(t *testing.T) {
	setupMCPTestState(t)

	dir := t.TempDir()
	output := filepath.Join(dir, "shot.png")
	data := []byte{0xaa, 0xbb}
	orig := captureScreenshotFunc
	captureScreenshotFunc = func(p *rod.Page, opts browser.ScreenshotOptions) (*browser.Result, error) {
		return &browser.Result{Data: data, MimeType: "image/png"}, nil
	}
	t.Cleanup(func() { captureScreenshotFunc = orig })

	args := map[string]interface{}{
		"return": "file",
		"output": output,
	}

	res, err := mcpHandleCaptureScreenshot(args)
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if len(res.Content) != 2 {
		t.Fatalf("expected text + resource content, got %d", len(res.Content))
	}
	text, ok := res.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("first content not text: %#v", res.Content[0])
	}
	if text.Text == "" {
		t.Fatalf("text response empty")
	}
	resContent, ok := res.Content[1].(mcp.EmbeddedResource)
	if !ok {
		t.Fatalf("second content not resource: %#v", res.Content[1])
	}
	blob, ok := resContent.Resource.(mcp.BlobResourceContents)
	if !ok {
		t.Fatalf("resource is not blob: %#v", resContent.Resource)
	}
	if blob.MIMEType != "image/png" {
		t.Fatalf("unexpected mime %s", blob.MIMEType)
	}
	if blob.URI != toFileURI(output) {
		t.Fatalf("unexpected URI %s", blob.URI)
	}
	if err := compareFile(output, data); err != nil {
		t.Fatalf("file check failed: %v", err)
	}
}

func TestMCPCapturePDFFile(t *testing.T) {
	setupMCPTestState(t)

	dir := t.TempDir()
	output := filepath.Join(dir, "doc.pdf")
	data := []byte("%PDF-1.7")
	var captured browser.PDFOptions
	orig := capturePDFFunc
	capturePDFFunc = func(p *rod.Page, opts browser.PDFOptions) (*browser.Result, error) {
		captured = opts
		return &browser.Result{Data: data, MimeType: "application/pdf"}, nil
	}
	t.Cleanup(func() { capturePDFFunc = orig })

	args := map[string]interface{}{
		"return":        "file",
		"output":        output,
		"landscape":     true,
		"scale":         1.6,
		"paper_width":   7.0,
		"paper_height":  9.0,
		"margin_top":    0.2,
		"margin_bottom": 0.4,
		"margin_left":   0.6,
		"margin_right":  0.8,
	}

	res, err := mcpHandleCapturePDF(args)
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if captured.Scale == nil || *captured.Scale != 1.6 {
		t.Fatalf("expected scale 1.6, got %#v", captured.Scale)
	}
	if captured.PaperWidth == nil || *captured.PaperWidth != 7.0 {
		t.Fatalf("paper width mismatch: %#v", captured.PaperWidth)
	}
	if captured.MarginRight == nil || *captured.MarginRight != 0.8 {
		t.Fatalf("margin not applied: %#v", captured.MarginRight)
	}
	if len(res.Content) != 2 {
		t.Fatalf("expected text + resource, got %d", len(res.Content))
	}
	resContent, ok := res.Content[1].(mcp.EmbeddedResource)
	if !ok {
		t.Fatalf("second content not resource: %#v", res.Content[1])
	}
	blob, ok := resContent.Resource.(mcp.BlobResourceContents)
	if !ok {
		t.Fatalf("resource not blob: %#v", resContent.Resource)
	}
	if blob.URI != toFileURI(output) {
		t.Fatalf("unexpected URI %s", blob.URI)
	}
	if err := compareFile(output, data); err != nil {
		t.Fatalf("file check failed: %v", err)
	}
}

func compareFile(path string, want []byte) error {
	got, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if string(got) != string(want) {
		return fmt.Errorf("unexpected file contents: %v", got)
	}
	return nil
}
