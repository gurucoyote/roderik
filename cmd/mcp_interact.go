package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

var (
	clickElementFunc = func(el *rod.Element) error {
		return el.Click(proto.InputMouseButtonLeft, 1)
	}
	clickFallbackFunc = func(err error) bool {
		return navigateViaHrefFallback(err)
	}
	typeInputFunc = func(el *rod.Element, text string) error {
		return el.Timeout(2 * time.Second).Input(text)
	}
	setValueFunc = func(el *rod.Element, text string) bool {
		return setValueViaJS(el, text)
	}
)

func mcpClick() (string, error) {
	if CurrentElement == nil {
		return "", fmt.Errorf("no current element – call load_url and navigation tools first")
	}

	if err := clickElementFunc(CurrentElement); err != nil {
		if clickFallbackFunc(err) {
			return "click fallback executed", nil
		}
		return "", fmt.Errorf("click failed: %w", err)
	}

	return "clicked current element", nil
}

func mcpType(raw string) (string, error) {
	if CurrentElement == nil {
		return "", fmt.Errorf("no current element – call load_url and navigation tools first")
	}

	text := strings.TrimSpace(raw)
	if text == "" {
		return "", fmt.Errorf("type requires non-empty text")
	}
	l := len(text)
	if l >= 2 {
		first, last := text[0], text[l-1]
		if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
			text = text[1 : l-1]
			text = strings.TrimSpace(text)
		}
	}
	if text == "" {
		return "", fmt.Errorf("type requires non-empty text")
	}

	if err := typeInputFunc(CurrentElement, text); err != nil {
		if setValueFunc(CurrentElement, text) {
			return "typed text via javascript fallback", nil
		}
		return "", fmt.Errorf("type failed: %w", err)
	}

	return "typed text into current element", nil
}
