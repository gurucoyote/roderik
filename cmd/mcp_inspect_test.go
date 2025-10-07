package cmd

import (
	"errors"
	"testing"

	"github.com/go-rod/rod"
)

func TestMCPHTMLReturnsCurrentElementMarkup(t *testing.T) {
	resetNavGlobals()

	element := &rod.Element{}
	CurrentElement = element

	calls := 0
	prevGetter := getElementHTML
	getElementHTML = func(el *rod.Element) (string, error) {
		calls++
		if el != element {
			t.Fatalf("getElementHTML called with unexpected element: %p", el)
		}
		return "<p>Hello</p>", nil
	}
	defer func() { getElementHTML = prevGetter }()

	html, err := mcpHTML()
	if err != nil {
		t.Fatalf("mcpHTML returned error: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected getElementHTML to be called once, got %d", calls)
	}
	if html != "<p>Hello</p>" {
		t.Fatalf("unexpected html output: %q", html)
	}
}

func TestMCPHTMLNoCurrentElement(t *testing.T) {
	resetNavGlobals()
	if _, err := mcpHTML(); err == nil {
		t.Fatalf("expected error when CurrentElement is nil")
	}
}

func TestMCPHTMLElementError(t *testing.T) {
	resetNavGlobals()
	element := &rod.Element{}
	CurrentElement = element

	prevGetter := getElementHTML
	getElementHTML = func(*rod.Element) (string, error) {
		return "", errors.New("boom")
	}
	defer func() { getElementHTML = prevGetter }()

	if _, err := mcpHTML(); err == nil {
		t.Fatalf("expected error when html retrieval fails")
	}
}
