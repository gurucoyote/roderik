package cmd

import (
	"errors"
	"strings"
	"testing"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
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

func TestMCPTextReturnsCurrentElementText(t *testing.T) {
	resetNavGlobals()
	element := &rod.Element{}
	CurrentElement = element

	prevGetter := getElementText
	calls := 0
	getElementText = func(el *rod.Element) (string, error) {
		calls++
		if el != element {
			t.Fatalf("getElementText called with unexpected element: %p", el)
		}
		return "Hello World", nil
	}
	defer func() { getElementText = prevGetter }()

	text, err := mcpText(nil)
	if err != nil {
		t.Fatalf("mcpText returned error: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected getElementText to be called once, got %d", calls)
	}
	if text != "Hello World" {
		t.Fatalf("unexpected text output: %q", text)
	}
}

func TestMCPTextAppliesLengthLimit(t *testing.T) {
	resetNavGlobals()
	element := &rod.Element{}
	CurrentElement = element

	prevGetter := getElementText
	getElementText = func(*rod.Element) (string, error) {
		return "abcdefghij", nil
	}
	defer func() { getElementText = prevGetter }()

	length := 4
	text, err := mcpText(&length)
	if err != nil {
		t.Fatalf("mcpText returned error: %v", err)
	}
	if text != "abcd" {
		t.Fatalf("expected truncated text, got %q", text)
	}
}

type fakeBoxModel struct {
	rect *proto.DOMRect
}

func (f *fakeBoxModel) Box() *proto.DOMRect {
	return f.rect
}

func TestMCPBoxReturnsFormattedBox(t *testing.T) {
	resetNavGlobals()
	element := &rod.Element{}
	CurrentElement = element

	prevShape := getElementShape
	getElementShape = func(*rod.Element) (boxModel, error) {
		return &fakeBoxModel{
			rect: &proto.DOMRect{X: 1, Y: 2, Width: 3, Height: 4},
		}, nil
	}
	defer func() { getElementShape = prevShape }()

	msg, err := mcpBox()
	if err != nil {
		t.Fatalf("mcpBox returned error: %v", err)
	}
	if !strings.HasPrefix(msg, "box:") {
		t.Fatalf("expected message to start with 'box:', got %q", msg)
	}
	if !strings.Contains(msg, `"width": 3`) || !strings.Contains(msg, `"height": 4`) {
		t.Fatalf("box json missing expected fields: %q", msg)
	}
}

func TestMCPBoxShapeError(t *testing.T) {
	resetNavGlobals()
	element := &rod.Element{}
	CurrentElement = element

	prevShape := getElementShape
	getElementShape = func(*rod.Element) (boxModel, error) {
		return nil, errors.New("boom")
	}
	defer func() { getElementShape = prevShape }()

	if _, err := mcpBox(); err == nil {
		t.Fatalf("expected error when shape retrieval fails")
	}
}

func TestMCPDescribeReturnsJSON(t *testing.T) {
	resetNavGlobals()
	element := &rod.Element{}
	CurrentElement = element

	prevDescribe := describeElementFunc
	describeElementFunc = func(*rod.Element) (*proto.DOMNode, error) {
		return &proto.DOMNode{NodeName: "DIV"}, nil
	}
	defer func() { describeElementFunc = prevDescribe }()

	desc, err := mcpDescribe()
	if err != nil {
		t.Fatalf("mcpDescribe returned error: %v", err)
	}
	if !strings.Contains(desc, `"nodeName": "DIV"`) {
		t.Fatalf("expected nodeName DIV in description, got %q", desc)
	}
}

func TestMCPDescribeError(t *testing.T) {
	resetNavGlobals()
	element := &rod.Element{}
	CurrentElement = element

	prevDescribe := describeElementFunc
	describeElementFunc = func(*rod.Element) (*proto.DOMNode, error) {
		return nil, errors.New("boom")
	}
	defer func() { describeElementFunc = prevDescribe }()

	if _, err := mcpDescribe(); err == nil {
		t.Fatalf("expected error when describe fails")
	}
}

func TestMCPXPathReturnsOptimizedPath(t *testing.T) {
	resetNavGlobals()
	element := &rod.Element{}
	CurrentElement = element

	prevXPath := getElementXPath
	getElementXPath = func(*rod.Element) (string, error) {
		return "/html/body/div[1]", nil
	}
	defer func() { getElementXPath = prevXPath }()

	xpath, err := mcpXPath()
	if err != nil {
		t.Fatalf("mcpXPath returned error: %v", err)
	}
	if xpath != "/html/body/div[1]" {
		t.Fatalf("unexpected xpath: %q", xpath)
	}
}

func TestMCPXPathError(t *testing.T) {
	resetNavGlobals()
	element := &rod.Element{}
	CurrentElement = element

	prevXPath := getElementXPath
	getElementXPath = func(*rod.Element) (string, error) {
		return "", errors.New("boom")
	}
	defer func() { getElementXPath = prevXPath }()

	if _, err := mcpXPath(); err == nil {
		t.Fatalf("expected error when xpath generation fails")
	}
}

func TestMCPComputedStylesReturnsJSON(t *testing.T) {
	resetNavGlobals()
	element := &rod.Element{}
	CurrentElement = element

	prevGetter := getComputedStyles
	getComputedStyles = func(*rod.Element) (interface{}, error) {
		return map[string]string{"color": "rgb(0, 0, 0)"}, nil
	}
	defer func() { getComputedStyles = prevGetter }()

	styles, err := mcpComputedStyles()
	if err != nil {
		t.Fatalf("mcpComputedStyles returned error: %v", err)
	}
	if !strings.Contains(styles, `"color": "rgb(0, 0, 0)"`) {
		t.Fatalf("expected color property in json output, got %q", styles)
	}
}

func TestMCPComputedStylesError(t *testing.T) {
	resetNavGlobals()
	element := &rod.Element{}
	CurrentElement = element

	prevGetter := getComputedStyles
	getComputedStyles = func(*rod.Element) (interface{}, error) {
		return nil, errors.New("boom")
	}
	defer func() { getComputedStyles = prevGetter }()

	if _, err := mcpComputedStyles(); err == nil {
		t.Fatalf("expected error when computed style retrieval fails")
	}
}

func TestMCPComputedStylesNoCurrentElement(t *testing.T) {
	resetNavGlobals()
	if _, err := mcpComputedStyles(); err == nil {
		t.Fatalf("expected error when CurrentElement is nil")
	}
}
