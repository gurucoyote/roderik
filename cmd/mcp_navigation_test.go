package cmd

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/go-rod/rod"
)

func resetNavGlobals() {
	pageEventMu.Lock()
	pageEventPage = nil
	pageEventMu.Unlock()

	elementList = nil
	currentIndex = 0
	CurrentElement = nil
	Page = &rod.Page{}
}

func TestMCPNavSearchPopulatesList(t *testing.T) {
	resetNavGlobals()

	fakeElements := []*rod.Element{&rod.Element{}, &rod.Element{}}

	var calledSelector string
	swapQueryElements(t, func(_ *rod.Page, selector string) ([]*rod.Element, error) {
		calledSelector = selector
		return fakeElements, nil
	})
	swapSummarizeElement(t, func(el *rod.Element) string {
		return fmt.Sprintf("summary-%p", el)
	})

	msg, err := mcpSearch("a.nav")
	if err != nil {
		t.Fatalf("mcpSearch returned error: %v", err)
	}
	if calledSelector != "a.nav" {
		t.Fatalf("expected selector 'a.nav', got %q", calledSelector)
	}
	if len(elementList) != 2 {
		t.Fatalf("expected elementList len 2, got %d", len(elementList))
	}
	if elementList[0] != CurrentElement {
		t.Fatalf("CurrentElement not set to first element")
	}
	if currentIndex != 0 {
		t.Fatalf("expected currentIndex 0, got %d", currentIndex)
	}
	if !strings.Contains(msg, `found 2 elements for selector "a.nav".`) {
		t.Fatalf("header missing in message: %q", msg)
	}
	if !strings.Contains(msg, fmt.Sprintf("focused index 0 of 2: summary-%p", fakeElements[0])) {
		t.Fatalf("focus line missing: %q", msg)
	}
	if !strings.Contains(msg, fmt.Sprintf("0* summary-%p", fakeElements[0])) {
		t.Fatalf("focused entry missing: %q", msg)
	}
	if !strings.Contains(msg, fmt.Sprintf("1  summary-%p", fakeElements[1])) {
		t.Fatalf("second entry missing: %q", msg)
	}
}

func TestMCPNavSearchHandlesNoMatches(t *testing.T) {
	resetNavGlobals()

	swapQueryElements(t, func(*rod.Page, string) ([]*rod.Element, error) {
		return nil, nil
	})

	msg, err := mcpSearch("main")
	if err != nil {
		t.Fatalf("mcpSearch returned error: %v", err)
	}
	if elementList != nil {
		t.Fatalf("elementList should remain nil, got %#v", elementList)
	}
	if msg != "no elements found for selector main" {
		t.Fatalf("unexpected message: %q", msg)
	}
}

func TestMCPNavSearchForwardsErrors(t *testing.T) {
	resetNavGlobals()

	swapQueryElements(t, func(*rod.Page, string) ([]*rod.Element, error) {
		return nil, errors.New("boom")
	})

	if _, err := mcpSearch(".item"); err == nil {
		t.Fatalf("expected error when queryElements fails")
	}
}

func TestMCPNavNextAdvancesAndStopsAtEnd(t *testing.T) {
	resetNavGlobals()

	fakeElements := []*rod.Element{&rod.Element{}, &rod.Element{}}
	elementList = fakeElements
	CurrentElement = fakeElements[0]
	currentIndex = 0

	swapSummarizeElement(t, func(el *rod.Element) string {
		return fmt.Sprintf("el-%p", el)
	})

	msg, err := mcpNext(nil)
	if err != nil {
		t.Fatalf("mcpNext returned error: %v", err)
	}
	if currentIndex != 1 || CurrentElement != fakeElements[1] {
		t.Fatalf("did not advance to second element")
	}
	want := fmt.Sprintf("focused index 1 of 2: el-%p", fakeElements[1])
	if msg != want {
		t.Fatalf("unexpected message: want %q got %q", want, msg)
	}

	msg, err = mcpNext(nil)
	if err == nil {
		t.Fatalf("expected error at end of list")
	}
	if msg != "" {
		t.Fatalf("expected empty message on error, got %q", msg)
	}
	if currentIndex != 1 {
		t.Fatalf("currentIndex modified despite error")
	}
}

func TestMCPNavHeadUsesLevelSelector(t *testing.T) {
	resetNavGlobals()

	var selectors []string
	fakeElements := []*rod.Element{&rod.Element{}}

	swapQueryElements(t, func(_ *rod.Page, selector string) ([]*rod.Element, error) {
		selectors = append(selectors, selector)
		return fakeElements, nil
	})
	swapSummarizeElement(t, func(*rod.Element) string { return "heading" })

	msg, err := mcpHead("2")
	if err != nil {
		t.Fatalf("mcpHead error: %v", err)
	}
	if len(selectors) != 1 || selectors[0] != "h2" {
		t.Fatalf("expected selector h2, got %#v", selectors)
	}
	if CurrentElement != fakeElements[0] {
		t.Fatalf("CurrentElement not set to first heading")
	}
	if !strings.Contains(msg, `found 1 headings for selector "h2".`) {
		t.Fatalf("unexpected message: %q", msg)
	}
	if !strings.Contains(msg, "0* heading") {
		t.Fatalf("expected numbered heading list, got %q", msg)
	}
}

func TestMCPNavElemPrefersCurrentElement(t *testing.T) {
	resetNavGlobals()

	current := &rod.Element{}
	CurrentElement = current
	Page = &rod.Page{}

	matches := []*rod.Element{current, &rod.Element{}}

	swapQueryElements(t, func(*rod.Page, string) ([]*rod.Element, error) {
		return matches, nil
	})

	var currentCalls, pageCalls int
	swapElementBySelector(t, func(selector string) (*rod.Element, error) {
		currentCalls++
		if selector != ".target" {
			t.Fatalf("unexpected selector: %s", selector)
		}
		return current, nil
	}, func(selector string) (*rod.Element, error) {
		pageCalls++
		return nil, errors.New("should not call page elem in this test")
	})

	swapSummarizeElement(t, func(*rod.Element) string { return "current" })

	msg, err := mcpElem(".target")
	if err != nil {
		t.Fatalf("mcpElem returned error: %v", err)
	}
	if currentCalls != 1 {
		t.Fatalf("expected current element lookup to be used once, got %d", currentCalls)
	}
	if pageCalls != 0 {
		t.Fatalf("page lookup should not be called when current element succeeds")
	}
	if CurrentElement != current {
		t.Fatalf("CurrentElement should remain unchanged")
	}
	if !strings.Contains(msg, `matched 2 elements for selector ".target".`) {
		t.Fatalf("unexpected header: %q", msg)
	}
	if !strings.Contains(msg, "0* current") {
		t.Fatalf("expected focused entry in list: %q", msg)
	}
}

func TestMCPNavElemFallsBackToPage(t *testing.T) {
	resetNavGlobals()
	Page = &rod.Page{}

	target := &rod.Element{}
	matches := []*rod.Element{target, &rod.Element{}}

	swapQueryElements(t, func(*rod.Page, string) ([]*rod.Element, error) {
		return matches, nil
	})

	swapElementBySelector(t, func(string) (*rod.Element, error) {
		return nil, errors.New("no child")
	}, func(selector string) (*rod.Element, error) {
		if selector != "main" {
			t.Fatalf("expected selector main, got %s", selector)
		}
		return target, nil
	})
	swapSummarizeElement(t, func(*rod.Element) string { return "page" })

	msg, err := mcpElem("main")
	if err != nil {
		t.Fatalf("mcpElem returned error: %v", err)
	}
	if CurrentElement != target {
		t.Fatalf("CurrentElement not updated to page result")
	}
	if !strings.Contains(msg, `matched 2 elements for selector "main".`) {
		t.Fatalf("unexpected header %q", msg)
	}
	if !strings.Contains(msg, "0* page") {
		t.Fatalf("expected focus entry in list %q", msg)
	}
}

func TestMCPNavElemErrorsWhenPageUnavailable(t *testing.T) {
	resetNavGlobals()
	Page = nil

	swapQueryElements(t, func(*rod.Page, string) ([]*rod.Element, error) {
		return nil, nil
	})

	swapElementBySelector(t, func(string) (*rod.Element, error) {
		return nil, errors.New("no current")
	}, func(string) (*rod.Element, error) {
		return nil, nil
	})

	if _, err := mcpElem("main"); err == nil {
		t.Fatalf("expected error when no page available")
	}
}

func TestMCPNavPrevMovesBackward(t *testing.T) {
	resetNavGlobals()

	fakeElements := []*rod.Element{&rod.Element{}, &rod.Element{}, &rod.Element{}}
	elementList = fakeElements
	currentIndex = 2
	CurrentElement = fakeElements[2]

	swapSummarizeElement(t, func(el *rod.Element) string {
		return fmt.Sprintf("el-%p", el)
	})

	msg, err := mcpPrev(nil)
	if err != nil {
		t.Fatalf("mcpPrev returned error: %v", err)
	}
	if currentIndex != 1 || CurrentElement != fakeElements[1] {
		t.Fatalf("did not move back to index 1")
	}
	want := fmt.Sprintf("focused index 1 of %d: el-%p", len(elementList), fakeElements[1])
	if msg != want {
		t.Fatalf("unexpected message: want %q got %q", want, msg)
	}
}

func TestMCPNavPrevAtBeginning(t *testing.T) {
	resetNavGlobals()

	fakeElements := []*rod.Element{&rod.Element{}}
	elementList = fakeElements
	currentIndex = 0
	CurrentElement = fakeElements[0]

	if _, err := mcpPrev(nil); err == nil {
		t.Fatalf("expected error when already at first element")
	}
}

func TestMCPNavNextSelectsIndex(t *testing.T) {
	resetNavGlobals()

	fakeElements := []*rod.Element{&rod.Element{}, &rod.Element{}, &rod.Element{}}
	elementList = fakeElements
	CurrentElement = fakeElements[0]
	currentIndex = 0

	swapSummarizeElement(t, func(el *rod.Element) string {
		return fmt.Sprintf("el-%p", el)
	})

	msg, err := mcpNext(pointerTo(2))
	if err != nil {
		t.Fatalf("mcpNext returned error: %v", err)
	}
	if currentIndex != 2 || CurrentElement != fakeElements[2] {
		t.Fatalf("expected jump to index 2, got index %d", currentIndex)
	}
	want := fmt.Sprintf("focused index 2 of %d: el-%p", len(elementList), fakeElements[2])
	if msg != want {
		t.Fatalf("unexpected message: want %q got %q", want, msg)
	}
}

func TestMCPNavPrevSelectsIndex(t *testing.T) {
	resetNavGlobals()

	fakeElements := []*rod.Element{&rod.Element{}, &rod.Element{}, &rod.Element{}}
	elementList = fakeElements
	CurrentElement = fakeElements[2]
	currentIndex = 2

	swapSummarizeElement(t, func(el *rod.Element) string {
		return fmt.Sprintf("el-%p", el)
	})

	msg, err := mcpPrev(pointerTo(0))
	if err != nil {
		t.Fatalf("mcpPrev returned error: %v", err)
	}
	if currentIndex != 0 || CurrentElement != fakeElements[0] {
		t.Fatalf("expected jump to index 0, got %d", currentIndex)
	}
	want := fmt.Sprintf("focused index 0 of %d: el-%p", len(elementList), fakeElements[0])
	if msg != want {
		t.Fatalf("unexpected message: want %q got %q", want, msg)
	}
}

func TestSelectElementAtBounds(t *testing.T) {
	resetNavGlobals()
	elementList = []*rod.Element{&rod.Element{}}
	CurrentElement = elementList[0]
	currentIndex = 0

	if _, err := mcpNext(pointerTo(2)); err == nil {
		t.Fatalf("expected error for out-of-range index")
	}
}

func TestMCPNavChildUpdatesCurrentElement(t *testing.T) {
	resetNavGlobals()

	parentEl := &rod.Element{}
	childEl := &rod.Element{}
	CurrentElement = parentEl

	swapChildParent(t,
		func(el *rod.Element) (*rod.Element, error) {
			if el != parentEl {
				t.Fatalf("childSelector called with unexpected element")
			}
			return childEl, nil
		},
		func(*rod.Element) (*rod.Element, error) {
			return nil, fmt.Errorf("parent should not be called")
		},
	)
	swapSummarizeElement(t, func(*rod.Element) string { return "child" })

	msg, err := mcpChild()
	if err != nil {
		t.Fatalf("mcpChild returned error: %v", err)
	}
	if CurrentElement != childEl {
		t.Fatalf("CurrentElement not updated to child")
	}
	if msg != "focused child element: child" {
		t.Fatalf("unexpected message %q", msg)
	}
}

func TestMCPNavParentUpdatesCurrentElement(t *testing.T) {
	resetNavGlobals()

	childEl := &rod.Element{}
	parentEl := &rod.Element{}
	CurrentElement = childEl

	swapChildParent(t,
		func(*rod.Element) (*rod.Element, error) {
			return nil, fmt.Errorf("child should not be called")
		},
		func(el *rod.Element) (*rod.Element, error) {
			if el != childEl {
				t.Fatalf("parentSelector called with unexpected element")
			}
			return parentEl, nil
		},
	)
	swapSummarizeElement(t, func(*rod.Element) string { return "parent" })

	msg, err := mcpParent()
	if err != nil {
		t.Fatalf("mcpParent returned error: %v", err)
	}
	if CurrentElement != parentEl {
		t.Fatalf("CurrentElement not updated to parent")
	}
	if msg != "focused parent element: parent" {
		t.Fatalf("unexpected message %q", msg)
	}
}

func TestMCPNavChildErrorsWhenNoCurrentElement(t *testing.T) {
	resetNavGlobals()
	if _, err := mcpChild(); err == nil {
		t.Fatalf("expected error when no current element")
	}
}

func TestMCPNavParentErrorsWhenNoCurrentElement(t *testing.T) {
	resetNavGlobals()
	if _, err := mcpParent(); err == nil {
		t.Fatalf("expected error when no current element")
	}
}

func swapQueryElements(t *testing.T, stub func(*rod.Page, string) ([]*rod.Element, error)) {
	t.Helper()
	prev := queryElementsFunc
	queryElementsFunc = stub
	t.Cleanup(func() { queryElementsFunc = prev })
}

func swapSummarizeElement(t *testing.T, stub func(*rod.Element) string) {
	t.Helper()
	prev := summarizeElementFunc
	summarizeElementFunc = stub
	t.Cleanup(func() { summarizeElementFunc = prev })
}

func swapElementBySelector(t *testing.T, current, page func(string) (*rod.Element, error)) {
	t.Helper()
	prevCurrent := currentElementSelector
	prevPage := pageSelector
	currentElementSelector = current
	pageSelector = page
	t.Cleanup(func() {
		currentElementSelector = prevCurrent
		pageSelector = prevPage
	})
}

func swapChildParent(t *testing.T, child func(*rod.Element) (*rod.Element, error), parent func(*rod.Element) (*rod.Element, error)) {
	t.Helper()
	prevChild := childSelector
	prevParent := parentSelector
	childSelector = child
	parentSelector = parent
	t.Cleanup(func() {
		childSelector = prevChild
		parentSelector = prevParent
	})
}

func pointerTo(v int) *int {
	return &v
}
