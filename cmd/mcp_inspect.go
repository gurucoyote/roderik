package cmd

import (
	"fmt"

	"github.com/go-rod/rod"
)

var getElementHTML = func(el *rod.Element) (string, error) {
	return el.HTML()
}

func mcpHTML() (string, error) {
	if CurrentElement == nil {
		return "", fmt.Errorf("no current element – call load_url/search first")
	}
	html, err := getElementHTML(CurrentElement)
	if err != nil {
		return "", fmt.Errorf("failed to get current element HTML: %w", err)
	}
	return html, nil
}
