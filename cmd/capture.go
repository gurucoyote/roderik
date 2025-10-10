package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"roderik/browser"
)

const defaultCaptureDir = "captures"

var (
	captureScreenshotFunc = browser.CaptureScreenshot
	capturePDFFunc        = browser.CapturePDF

	screenshotOutput   string
	screenshotDir      string
	screenshotName     string
	screenshotFormat   string
	screenshotSelector string
	screenshotFullPage bool
	screenshotScroll   bool
	screenshotQuality  int

	pdfOutput              string
	pdfDir                 string
	pdfName                string
	pdfLandscape           bool
	pdfDisplayHeaderFooter bool
	pdfPrintBackground     bool
	pdfPreferCSSPageSize   bool
	pdfGenerateTagged      bool
	pdfGenerateOutline     bool
	pdfScale               float64
	pdfPageSize            string
	pdfMargin              float64
)

var paperPresets = map[string]struct {
	width  float64
	height float64
}{
	"letter":    {width: 8.5, height: 11.0},
	"legal":     {width: 8.5, height: 14.0},
	"tabloid":   {width: 11.0, height: 17.0},
	"executive": {width: 7.25, height: 10.5},
	"a3":        {width: 11.69, height: 16.54},
	"a4":        {width: 8.27, height: 11.69},
	"a5":        {width: 5.83, height: 8.27},
}

var screenshotCmd = &cobra.Command{
	Use:   "screenshot [url]",
	Short: "Capture a screenshot of the current page or the provided URL",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensurePageReady(); err != nil {
			return err
		}
		if browserInitErr != nil {
			return browserInitErr
		}

		page := Page
		if len(args) == 1 {
			var err error
			page, err = LoadURL(args[0])
			if err != nil {
				return fmt.Errorf("capture screenshot: load url: %w", err)
			}
		} else if page == nil {
			return fmt.Errorf("capture screenshot: no page loaded; pass a URL or load one first")
		}

		if screenshotSelector != "" && screenshotScroll {
			return fmt.Errorf("capture screenshot: --selector cannot be combined with --scroll")
		}
		if screenshotSelector != "" && screenshotFullPage {
			return fmt.Errorf("capture screenshot: --selector cannot be combined with --full-page")
		}
		if screenshotScroll && screenshotFullPage {
			return fmt.Errorf("capture screenshot: choose either --scroll or --full-page, not both")
		}

		opts := browser.ScreenshotOptions{
			Selector: screenshotSelector,
			FullPage: screenshotFullPage,
			Scroll:   screenshotScroll,
			Format:   screenshotFormat,
		}
		if cmd.Flags().Changed("quality") || isJPEGFormat(screenshotFormat) {
			opts.Quality = &screenshotQuality
		}

		result, err := captureScreenshotFunc(page, opts)
		if err != nil {
			return err
		}

		formatExt := "png"
		if isJPEGFormat(screenshotFormat) {
			formatExt = "jpg"
		}
		outputPath, err := resolveOutputPath(screenshotOutput, screenshotDir, screenshotName, "screenshot", formatExt)
		if err != nil {
			return err
		}
		if err := os.WriteFile(outputPath, result.Data, 0644); err != nil {
			return fmt.Errorf("capture screenshot: write file: %w", err)
		}

		fmt.Fprintf(os.Stdout, "Screenshot saved to %s (%d bytes, %s)\n", outputPath, len(result.Data), result.MimeType)
		return nil
	},
}

var pdfCmd = &cobra.Command{
	Use:   "pdf [url]",
	Short: "Print the current page or the provided URL to PDF",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensurePageReady(); err != nil {
			return err
		}
		if browserInitErr != nil {
			return browserInitErr
		}

		page := Page
		if len(args) == 1 {
			var err error
			page, err = LoadURL(args[0])
			if err != nil {
				return fmt.Errorf("capture pdf: load url: %w", err)
			}
		} else if page == nil {
			return fmt.Errorf("capture pdf: no page loaded; pass a URL or load one first")
		}

		opts := browser.PDFOptions{
			Landscape:               pdfLandscape,
			DisplayHeaderFooter:     pdfDisplayHeaderFooter,
			PrintBackground:         pdfPrintBackground,
			PreferCSSPageSize:       pdfPreferCSSPageSize,
			GenerateTaggedPDF:       pdfGenerateTagged,
			GenerateDocumentOutline: pdfGenerateOutline,
		}

		scale := pdfScale
		opts.Scale = &scale

		width, height, err := resolvePaperSize(pdfPageSize)
		if err != nil {
			return fmt.Errorf("capture pdf: %w", err)
		}
		opts.PaperWidth = &width
		opts.PaperHeight = &height

		mt := pdfMargin
		mb := pdfMargin
		ml := pdfMargin
		mr := pdfMargin
		opts.MarginTop = &mt
		opts.MarginBottom = &mb
		opts.MarginLeft = &ml
		opts.MarginRight = &mr

		result, err := capturePDFFunc(page, opts)
		if err != nil {
			return err
		}

		outputPath, err := resolveOutputPath(pdfOutput, pdfDir, pdfName, "page", "pdf")
		if err != nil {
			return err
		}
		if err := os.WriteFile(outputPath, result.Data, 0644); err != nil {
			return fmt.Errorf("capture pdf: write file: %w", err)
		}

		fmt.Fprintf(os.Stdout, "PDF saved to %s (%d bytes)\n", outputPath, len(result.Data))
		return nil
	},
}

func resolveOutputPath(pathOverride, dir, name, prefix, ext string) (string, error) {
	if strings.TrimSpace(pathOverride) != "" {
		path := filepath.Clean(pathOverride)
		if filepath.Ext(path) == "" {
			path = path + "." + ext
		}
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return "", fmt.Errorf("create output directory: %w", err)
		}
		return path, nil
	}

	baseDir := strings.TrimSpace(dir)
	if baseDir == "" {
		baseDir = filepath.Join(".", defaultCaptureDir)
	}
	baseDir = filepath.Clean(baseDir)
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return "", fmt.Errorf("create captures directory: %w", err)
	}

	filename := sanitizeFileName(name)
	if filename == "" {
		filename = fmt.Sprintf("%s-%s", prefix, time.Now().Format("20060102-150405"))
	}
	if !strings.HasSuffix(strings.ToLower(filename), "."+ext) {
		filename = filename + "." + ext
	}

	return filepath.Join(baseDir, filename), nil
}

func resolvePaperSize(raw string) (float64, float64, error) {
	key := strings.TrimSpace(strings.ToLower(raw))
	if key == "" {
		key = "letter"
	}
	if preset, ok := paperPresets[key]; ok {
		return preset.width, preset.height, nil
	}

	parts := splitSize(key)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("unknown page size %q; try letter, legal, tabloid, a4, a3, or format WIDTHxHEIGHT in inches", raw)
	}

	width, err := parseDimension(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("parse page width: %w", err)
	}
	height, err := parseDimension(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("parse page height: %w", err)
	}
	return width, height, nil
}

func isJPEGFormat(format string) bool {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "jpeg", "jpg":
		return true
	default:
		return false
	}
}

func toFileURI(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return "file://" + filepath.ToSlash(abs)
}

func init() {
	screenshotFormat = "png"
	screenshotQuality = 90
	pdfScale = 1.0
	pdfPageSize = "letter"
	pdfMargin = 0.5

	screenshotCmd.Flags().StringVarP(&screenshotOutput, "output", "o", "", "full path for the screenshot (overrides directory/name)")
	screenshotCmd.Flags().StringVar(&screenshotDir, "dir", "", "directory to store the screenshot (default ./captures)")
	screenshotCmd.Flags().StringVar(&screenshotName, "name", "", "file name without extension")
	screenshotCmd.Flags().StringVarP(&screenshotFormat, "format", "f", "png", "image format: png or jpeg")
	screenshotCmd.Flags().StringVar(&screenshotSelector, "selector", "", "CSS selector to capture a specific element")
	screenshotCmd.Flags().BoolVar(&screenshotFullPage, "full-page", false, "capture the entire page by resizing the viewport")
	screenshotCmd.Flags().BoolVar(&screenshotScroll, "scroll", false, "scroll and stitch to capture the entire page")
	screenshotCmd.Flags().IntVar(&screenshotQuality, "quality", 90, "image quality (for jpeg captures)")

	pdfCmd.Flags().StringVarP(&pdfOutput, "output", "o", "", "full path for the PDF (overrides directory/name)")
	pdfCmd.Flags().StringVar(&pdfDir, "dir", "", "directory to store the PDF (default ./captures)")
	pdfCmd.Flags().StringVar(&pdfName, "name", "", "file name without extension")
	pdfCmd.Flags().BoolVar(&pdfLandscape, "landscape", false, "render the PDF in landscape orientation")
	pdfCmd.Flags().BoolVar(&pdfDisplayHeaderFooter, "header-footer", false, "display header and footer templates")
	pdfCmd.Flags().BoolVar(&pdfPrintBackground, "background", false, "print background graphics")
	pdfCmd.Flags().Float64Var(&pdfScale, "scale", 1.0, "scale factor for rendering the page")
	pdfCmd.Flags().StringVar(&pdfPageSize, "size", "letter", "page size preset (letter, legal, tabloid, executive, a3, a4, a5) or custom WIDTHxHEIGHT in inches")
	pdfCmd.Flags().Float64Var(&pdfMargin, "margin", 0.5, "page margin in inches (applied on all sides)")
	pdfCmd.Flags().BoolVar(&pdfPreferCSSPageSize, "css-size", false, "prefer CSS-specified page size over paper dimensions")
	pdfCmd.Flags().BoolVar(&pdfGenerateTagged, "tagged", false, "generate tagged (accessible) PDF")
	pdfCmd.Flags().BoolVar(&pdfGenerateOutline, "outline", false, "embed document outline into the PDF")

	RootCmd.AddCommand(screenshotCmd)
	RootCmd.AddCommand(pdfCmd)
}

func sanitizeFileName(name string) string {
	clean := strings.TrimSpace(name)
	if clean == "" {
		return ""
	}
	clean = filepath.Base(clean)
	clean = strings.TrimSuffix(clean, filepath.Ext(clean))
	clean = strings.ReplaceAll(clean, " ", "_")
	clean = strings.ReplaceAll(clean, string(os.PathSeparator), "_")
	return clean
}

func splitSize(raw string) []string {
	return strings.FieldsFunc(raw, func(r rune) bool {
		return r == 'x' || r == '×' || r == ' '
	})
}

func parseDimension(raw string) (float64, error) {
	s := strings.TrimSpace(raw)
	switch {
	case strings.HasSuffix(s, "mm"):
		v, err := strconv.ParseFloat(strings.TrimSuffix(s, "mm"), 64)
		if err != nil {
			return 0, err
		}
		return v / 25.4, nil
	case strings.HasSuffix(s, "cm"):
		v, err := strconv.ParseFloat(strings.TrimSuffix(s, "cm"), 64)
		if err != nil {
			return 0, err
		}
		return v / 2.54, nil
	case strings.HasSuffix(s, "in"):
		s = strings.TrimSuffix(s, "in")
	}

	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0, err
	}
	return v, nil
}
