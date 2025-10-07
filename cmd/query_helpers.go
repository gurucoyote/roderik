package cmd

import (
	"github.com/go-rod/rod"
)

// queryElements wraps Rod's ElementsByJS with an inline arrow function so we
// avoid relying on the cached helper that occasionally goes missing, which
// manifests as "eval js error ... reading 'apply'".
func queryElements(page *rod.Page, selector string) ([]*rod.Element, error) {
	opts := rod.Eval(`sel => Array.from(document.querySelectorAll(sel))`, selector).ByObject()
	return page.ElementsByJS(opts)
}
