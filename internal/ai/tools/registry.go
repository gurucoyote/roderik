package tools

// Definition describes a tool that can be exposed to AI integrations and MCP clients.
type Definition struct {
	Name        string
	Description string
	Parameters  []Parameter
	FocusAware  bool
}

type ParameterType string

const (
	ParamString  ParameterType = "string"
	ParamNumber  ParameterType = "number"
	ParamBoolean ParameterType = "boolean"
)

type Parameter struct {
	Name        string
	Type        ParameterType
	Description string
	Required    bool
	Enum        []string
}

func (pt ParameterType) JSONType() string {
	switch pt {
	case ParamString:
		return "string"
	case ParamNumber:
		return "number"
	case ParamBoolean:
		return "boolean"
	default:
		return "string"
	}
}

var definitions = []Definition{
	{
		Name:        "load_url",
		Description: "Load a webpage at the given URL and set it as the current page for subsequent tools.",
		Parameters: []Parameter{
			{
				Name:        "url",
				Type:        ParamString,
				Description: "the URL of the webpage to load",
				Required:    true,
			},
		},
	},
	{
		Name: "get_html",
		Description: "Get the raw HTML of the current element (or an optional URL). " +
			"Beware: this returns the full source and can be very large. " +
			"In most cases, use \"to_markdown\" for a more concise, token-efficient output.",
		FocusAware: true,
		Parameters: []Parameter{
			{
				Name:        "url",
				Type:        ParamString,
				Description: "optional URL to load first; overrides the current element",
				Required:    false,
			},
		},
	},
	{
		Name:        "text",
		Description: "Print the text of the current element, optionally truncating to a specified length.",
		FocusAware:  true,
		Parameters: []Parameter{
			{
				Name:        "length",
				Type:        ParamNumber,
				Description: "optional maximum number of characters to return",
				Required:    false,
			},
		},
	},
	{
		Name:        "capture_screenshot",
		Description: "Capture a screenshot of the current page or an optional URL.",
		Parameters: []Parameter{
			{Name: "url", Type: ParamString, Description: "optional URL to load before capturing the screenshot"},
			{Name: "selector", Type: ParamString, Description: "optional CSS selector to capture a specific element"},
			{Name: "full_page", Type: ParamBoolean, Description: "capture the entire page by resizing the viewport"},
			{Name: "scroll", Type: ParamBoolean, Description: "scroll and stitch the entire page without resizing the viewport"},
			{Name: "format", Type: ParamString, Description: "image format: png or jpeg (default png)"},
			{Name: "quality", Type: ParamNumber, Description: "JPEG quality (0-100)"},
			{Name: "return", Type: ParamString, Description: "delivery mode: binary (inline) or file (writes to disk)"},
			{Name: "output", Type: ParamString, Description: "optional path to save the capture on disk when return=file"},
		},
	},
	{
		Name:        "capture_pdf",
		Description: "Render the current page or an optional URL to PDF.",
		Parameters: []Parameter{
			{Name: "url", Type: ParamString, Description: "optional URL to load before generating the PDF"},
			{Name: "landscape", Type: ParamBoolean, Description: "render pages in landscape orientation"},
			{Name: "header_footer", Type: ParamBoolean, Description: "display header and footer templates"},
			{Name: "background", Type: ParamBoolean, Description: "print background graphics"},
			{Name: "scale", Type: ParamNumber, Description: "scale factor for rendering (default 1.0)"},
			{Name: "paper_width", Type: ParamNumber, Description: "paper width in inches"},
			{Name: "paper_height", Type: ParamNumber, Description: "paper height in inches"},
			{Name: "margin_top", Type: ParamNumber, Description: "top margin in inches"},
			{Name: "margin_bottom", Type: ParamNumber, Description: "bottom margin in inches"},
			{Name: "margin_left", Type: ParamNumber, Description: "left margin in inches"},
			{Name: "margin_right", Type: ParamNumber, Description: "right margin in inches"},
			{Name: "page_ranges", Type: ParamString, Description: "page ranges to print, e.g. '1-5,8'"},
			{Name: "header_template", Type: ParamString, Description: "HTML template for the header"},
			{Name: "footer_template", Type: ParamString, Description: "HTML template for the footer"},
			{Name: "prefer_css_page_size", Type: ParamBoolean, Description: "prefer CSS-defined page size"},
			{Name: "tagged", Type: ParamBoolean, Description: "generate tagged (accessible) PDF"},
			{Name: "outline", Type: ParamBoolean, Description: "embed document outline in the PDF"},
			{Name: "return", Type: ParamString, Description: "delivery mode: binary (embedded) or file (writes to disk)"},
			{Name: "output", Type: ParamString, Description: "optional path to save the PDF on disk when return=file"},
		},
	},
	{
		Name:        "box",
		Description: "Get the bounding box of the current element.",
	},
	{
		Name:        "computedstyles",
		Description: "Output the computed styles of the current element in JSON format.",
	},
	{
		Name:        "describe",
		Description: "Describe the current element as formatted JSON.",
	},
	{
		Name:        "xpath",
		Description: "Get the optimized XPath of the current element.",
	},
	{
		Name:        "shutdown",
		Description: "Shut down the MCP server.",
	},
	{
		Name:        "duck",
		Description: "Search DuckDuckGo and return top N results.",
		Parameters: []Parameter{
			{Name: "query", Type: ParamString, Description: "the search terms", Required: true},
			{Name: "num", Type: ParamNumber, Description: "how many results to return (default 20)"},
		},
	},
	{
		Name:        "network_list",
		Description: "List captured network activity entries with optional filters.",
		Parameters: []Parameter{
			{Name: "mime", Type: ParamString, Description: "optional comma-separated MIME substrings to match"},
			{Name: "suffix", Type: ParamString, Description: "optional comma-separated URL suffixes (e.g. .mp4)"},
			{Name: "status", Type: ParamString, Description: "optional comma-separated HTTP status codes"},
			{Name: "contains", Type: ParamString, Description: "optional comma-separated substrings to match in the URL"},
			{Name: "method", Type: ParamString, Description: "optional comma-separated HTTP methods"},
			{Name: "domain", Type: ParamString, Description: "optional comma-separated domain substrings"},
			{Name: "type", Type: ParamString, Description: "optional comma-separated resource types (Document, Image, Media, etc.)"},
			{Name: "limit", Type: ParamNumber, Description: "maximum number of entries to return (default 20, capped at 1000)"},
			{Name: "offset", Type: ParamNumber, Description: "number of matching entries to skip before returning results"},
			{Name: "tail", Type: ParamBoolean, Description: "when true (default) return the newest matching entries"},
		},
	},
	{
		Name:        "network_save",
		Description: "Retrieve or persist the response body for a captured network request.",
		Parameters: []Parameter{
			{Name: "request_id", Type: ParamString, Description: "request identifier returned by network_list", Required: true},
			{Name: "return", Type: ParamString, Description: "delivery mode: binary (default) or file", Enum: []string{"binary", "file"}},
			{Name: "save_dir", Type: ParamString, Description: "optional directory to write the file when return=file"},
			{Name: "filename", Type: ParamString, Description: "optional filename override when saving to disk"},
		},
	},
	{
		Name:        "network_set_logging",
		Description: "Enable, disable, or query network activity logging without restarting Roderik.",
		Parameters: []Parameter{
			{Name: "enabled", Type: ParamBoolean, Description: "optional flag; when provided sets logging state to the given value"},
		},
	},
	{
		Name: "to_markdown",
		Description: "Convert the current page/element (or an optional URL) into a structured Markdown document. " +
			"This produces a well-formatted, token-efficient summary. " +
			"Use this instead of \"get_html\" unless you specifically need raw HTML.",
		FocusAware: true,
	},
	{
		Name:        "search",
		Description: "Search for elements matching a CSS selector, focus the first match, and return a numbered list for subsequent navigation commands.",
		Parameters: []Parameter{
			{Name: "selector", Type: ParamString, Description: "CSS selector to query", Required: true},
		},
	},
	{
		Name:        "head",
		Description: "List page headings (optionally by level), focus the first match, and return a numbered index.",
		Parameters: []Parameter{
			{Name: "level", Type: ParamString, Description: "Heading level number (1-6)"},
		},
	},
	{
		Name:        "next",
		Description: "Advance to the next element in the active search/head list or jump to a specific index.",
		Parameters: []Parameter{
			{Name: "index", Type: ParamNumber, Description: "optional index to jump to"},
		},
	},
	{
		Name:        "prev",
		Description: "Move to the previous element in the active search/head list or jump to a specific index.",
		Parameters: []Parameter{
			{Name: "index", Type: ParamNumber, Description: "optional index to jump to"},
		},
	},
	{
		Name:        "elem",
		Description: "Match elements by selector (scoped to the current element, falling back to the page), focus the best match, and return a numbered list.",
		Parameters: []Parameter{
			{Name: "selector", Type: ParamString, Description: "CSS selector to resolve", Required: true},
		},
	},
	{
		Name:        "child",
		Description: "Focus the first child element of the current selection.",
	},
	{
		Name:        "parent",
		Description: "Focus the parent element of the current selection.",
	},
	{
		Name:        "html",
		Description: "Return the outer HTML of the current element that prior navigation selected.",
		FocusAware:  true,
	},
	{
		Name:        "click",
		Description: "Click the currently focused element; falls back to href navigation or synthetic click on failure.",
	},
	{
		Name:        "type",
		Description: "Type text into the currently focused element; trims optional quotes and falls back to JavaScript value injection.",
		Parameters: []Parameter{
			{Name: "text", Type: ParamString, Description: "Text to type", Required: true},
		},
	},
	{
		Name:        "run_js",
		Description: "Execute JavaScript on the current page and return the result as JSON.",
		FocusAware:  true,
		Parameters: []Parameter{
			{Name: "script", Type: ParamString, Description: "JavaScript code to execute in the page context", Required: true},
			{Name: "showErrors", Type: ParamBoolean, Description: "if true, return any evaluation errors in the tool result text"},
		},
	},
}

// List returns all registered tool definitions.
func List() []Definition {
	out := make([]Definition, len(definitions))
	copy(out, definitions)
	return out
}

// Lookup finds a tool definition by name.
func Lookup(name string) (Definition, bool) {
	for _, def := range definitions {
		if def.Name == name {
			return def, true
		}
	}
	return Definition{}, false
}
