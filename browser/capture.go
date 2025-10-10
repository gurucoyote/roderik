package browser

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// Result describes the outcome of a capture operation.
type Result struct {
	Data     []byte
	MimeType string
}

// ScreenshotOptions controls Page or Element screenshot behaviour.
type ScreenshotOptions struct {
	// Selector targets a specific element; if empty the page is captured.
	Selector string

	// FullPage extends the viewport before capture; ignored when Selector or Scroll is set.
	FullPage bool

	// Scroll stitches captures while scrolling instead of resizing the viewport.
	Scroll bool

	// Clip defines a viewport to capture; ignored for element captures.
	Clip *proto.PageViewport

	// Format is one of png, jpeg. Defaults to png.
	Format string

	// Quality configures lossy formats (0-100). Ignored for png unless Selector is set.
	Quality *int

	// FromSurface forwards to proto.PageCaptureScreenshot. Nil keeps library default (true).
	FromSurface *bool

	// CaptureBeyondViewport forwards to proto.PageCaptureScreenshot.
	CaptureBeyondViewport *bool

	// OptimizeForSpeed slightly reduces image size accuracy for faster encoding.
	OptimizeForSpeed bool
}

// PDFOptions mirrors proto.PagePrintToPDF and expresses optional float fields as pointers.
type PDFOptions struct {
	Landscape               bool
	DisplayHeaderFooter     bool
	PrintBackground         bool
	PreferCSSPageSize       bool
	GenerateTaggedPDF       bool
	GenerateDocumentOutline bool
	PageRanges              string
	HeaderTemplate          string
	FooterTemplate          string
	Scale                   *float64
	PaperWidth              *float64
	PaperHeight             *float64
	MarginTop               *float64
	MarginBottom            *float64
	MarginLeft              *float64
	MarginRight             *float64
}

// CaptureScreenshot produces a screenshot using the provided options and returns raw bytes plus mime type.
func CaptureScreenshot(page *rod.Page, opts ScreenshotOptions) (*Result, error) {
	if page == nil {
		return nil, errors.New("capture screenshot: page is nil")
	}

	format := strings.ToLower(strings.TrimSpace(opts.Format))
	if format == "" {
		format = "png"
	}

	var (
		data     []byte
		err      error
		mimeType string
	)

	var protoFormat proto.PageCaptureScreenshotFormat
	switch format {
	case "png":
		protoFormat = proto.PageCaptureScreenshotFormatPng
		mimeType = "image/png"
	case "jpeg", "jpg":
		protoFormat = proto.PageCaptureScreenshotFormatJpeg
		mimeType = "image/jpeg"
	default:
		return nil, fmt.Errorf("capture screenshot: unsupported format %q", format)
	}

	switch {
	case opts.Selector != "":
		el, err := page.Element(opts.Selector)
		if err != nil {
			return nil, fmt.Errorf("capture screenshot: locate selector %q: %w", opts.Selector, err)
		}
		quality := 100
		if opts.Quality != nil {
			quality = *opts.Quality
		}
		data, err = el.Screenshot(protoFormat, quality)
		if err != nil {
			return nil, fmt.Errorf("capture screenshot: element: %w", err)
		}
	case opts.Scroll:
		scrollOpts := &rod.ScrollScreenshotOptions{
			Format: protoFormat,
		}
		if opts.Quality != nil {
			scrollOpts.Quality = opts.Quality
		}
		data, err = page.ScrollScreenshot(scrollOpts)
		if err != nil {
			return nil, fmt.Errorf("capture screenshot: scroll: %w", err)
		}
	default:
		req := &proto.PageCaptureScreenshot{
			Format:           protoFormat,
			Clip:             opts.Clip,
			OptimizeForSpeed: opts.OptimizeForSpeed,
		}
		if opts.Quality != nil {
			req.Quality = opts.Quality
		}
		if opts.FromSurface != nil {
			req.FromSurface = *opts.FromSurface
		} else {
			req.FromSurface = true
		}
		if opts.CaptureBeyondViewport != nil {
			req.CaptureBeyondViewport = *opts.CaptureBeyondViewport
		}
		data, err = page.Screenshot(opts.FullPage, req)
		if err != nil {
			return nil, fmt.Errorf("capture screenshot: page: %w", err)
		}
	}

	return &Result{Data: data, MimeType: mimeType}, nil
}

// CapturePDF renders the current page into a PDF using supplied options.
func CapturePDF(page *rod.Page, opts PDFOptions) (*Result, error) {
	if page == nil {
		return nil, errors.New("capture pdf: page is nil")
	}

	req := &proto.PagePrintToPDF{
		Landscape:               opts.Landscape,
		DisplayHeaderFooter:     opts.DisplayHeaderFooter,
		PrintBackground:         opts.PrintBackground,
		PageRanges:              opts.PageRanges,
		HeaderTemplate:          opts.HeaderTemplate,
		FooterTemplate:          opts.FooterTemplate,
		PreferCSSPageSize:       opts.PreferCSSPageSize,
		GenerateTaggedPDF:       opts.GenerateTaggedPDF,
		GenerateDocumentOutline: opts.GenerateDocumentOutline,
	}

	if opts.Scale != nil {
		req.Scale = opts.Scale
	}
	if opts.PaperWidth != nil {
		req.PaperWidth = opts.PaperWidth
	}
	if opts.PaperHeight != nil {
		req.PaperHeight = opts.PaperHeight
	}
	if opts.MarginTop != nil {
		req.MarginTop = opts.MarginTop
	}
	if opts.MarginBottom != nil {
		req.MarginBottom = opts.MarginBottom
	}
	if opts.MarginLeft != nil {
		req.MarginLeft = opts.MarginLeft
	}
	if opts.MarginRight != nil {
		req.MarginRight = opts.MarginRight
	}

	stream, err := page.PDF(req)
	if err != nil {
		return nil, fmt.Errorf("capture pdf: %w", err)
	}
	defer stream.Close()

	data, err := io.ReadAll(stream)
	if err != nil {
		return nil, fmt.Errorf("capture pdf: read stream: %w", err)
	}

	return &Result{Data: data, MimeType: "application/pdf"}, nil
}
