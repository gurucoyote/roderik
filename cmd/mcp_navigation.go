package cmd

import (
	"fmt"
	"strings"

	"github.com/go-rod/rod"
)

var summarizeElementFunc = summarizeElement

var currentElementSelector = func(selector string) (*rod.Element, error) {
	if CurrentElement == nil {
		return nil, fmt.Errorf("no current element to scope selector %q", selector)
	}
	return CurrentElement.Element(selector)
}

var pageSelector = func(selector string) (*rod.Element, error) {
	if Page == nil {
		return nil, fmt.Errorf("no page loaded to resolve selector %q", selector)
	}
	return Page.Element(selector)
}

var childSelector = func(el *rod.Element) (*rod.Element, error) {
	if el == nil {
		return nil, fmt.Errorf("no current element to resolve child")
	}
	return el.Element(":first-child")
}

var parentSelector = func(el *rod.Element) (*rod.Element, error) {
	if el == nil {
		return nil, fmt.Errorf("no current element to resolve parent")
	}
	return el.Parent()
}

func mcpSearch(selector string) (string, error) {
	if Page == nil {
		return "", fmt.Errorf("no page loaded – call load_url first")
	}
	elements, err := queryElementsFunc(Page, selector)
	if err != nil {
		return "", fmt.Errorf("search failed: %w", err)
	}
	if len(elements) == 0 {
		elementList = nil
		CurrentElement = nil
		return fmt.Sprintf("no elements found for selector %s", selector), nil
	}

	elementList = elements
	currentIndex = 0
	CurrentElement = elementList[currentIndex]

	msg := formatElementListResponse(
		fmt.Sprintf("found %d elements for selector %q.", len(elementList), selector),
		elementList,
		currentIndex,
	)
	return msg, nil
}

func mcpNext(indexOpt *int) (string, error) {
	if len(elementList) == 0 {
		return "", fmt.Errorf("no search results – run search first")
	}
	if indexOpt != nil {
		return selectElementAt(*indexOpt)
	}
	if currentIndex >= len(elementList)-1 {
		return "", fmt.Errorf("already at the last element (index %d)", currentIndex)
	}
	currentIndex++
	CurrentElement = elementList[currentIndex]
	return formatCurrentFocus(len(elementList)), nil
}

func mcpHead(level string) (string, error) {
	if Page == nil {
		return "", fmt.Errorf("no page loaded – call load_url first")
	}
	selector := "h1, h2, h3, h4, h5, h6"
	level = strings.TrimSpace(level)
	if level != "" {
		selector = fmt.Sprintf("h%s", level)
	}

	elements, err := queryElementsFunc(Page, selector)
	if err != nil {
		return "", fmt.Errorf("head failed: %w", err)
	}
	if len(elements) == 0 {
		return "", fmt.Errorf("no headings found for selector %s", selector)
	}
	elementList = elements
	currentIndex = 0
	CurrentElement = elementList[0]
	msg := formatElementListResponse(
		fmt.Sprintf("found %d headings for selector %q.", len(elementList), selector),
		elementList,
		currentIndex,
	)
	return msg, nil
}

func mcpElem(selector string) (string, error) {
	if strings.TrimSpace(selector) == "" {
		return "", fmt.Errorf("selector cannot be empty")
	}

	if Page == nil {
		return "", fmt.Errorf("no page loaded – call load_url first")
	}

	matches, err := queryElementsFunc(Page, selector)
	if err != nil {
		return "", fmt.Errorf("elem search failed: %w", err)
	}
	if len(matches) == 0 {
		elementList = nil
		return fmt.Sprintf("no elements matched selector %q", selector), nil
	}

	target := matches[0]
	if el, err := currentElementSelector(selector); err == nil && el != nil {
		target = el
	}
	if target == nil {
		el, err := pageSelector(selector)
		if err != nil {
			return "", fmt.Errorf("elem failed to locate %q: %w", selector, err)
		}
		if el == nil {
			return "", fmt.Errorf("elem failed to locate %q: no element returned", selector)
		}
		target = el
	}

	index := indexOfElement(matches, target)
	if index == -1 {
		matches = append([]*rod.Element{target}, matches...)
		index = 0
	}
	CurrentElement = target
	elementList = matches
	currentIndex = index

	msg := formatElementListResponse(
		fmt.Sprintf("matched %d elements for selector %q.", len(elementList), selector),
		elementList,
		currentIndex,
	)
	return msg, nil
}

func mcpPrev(indexOpt *int) (string, error) {
	if len(elementList) == 0 {
		return "", fmt.Errorf("no search results – run search first")
	}
	if indexOpt != nil {
		return selectElementAt(*indexOpt)
	}
	if currentIndex == 0 {
		return "", fmt.Errorf("already at the first element (index 0)")
	}
	currentIndex--
	CurrentElement = elementList[currentIndex]
	return formatCurrentFocus(len(elementList)), nil
}

func mcpChild() (string, error) {
	if CurrentElement == nil {
		return "", fmt.Errorf("no current element – call load_url/search first")
	}
	child, err := childSelector(CurrentElement)
	if err != nil {
		return "", fmt.Errorf("child navigation failed: %w", err)
	}
	if child == nil {
		return "", fmt.Errorf("child navigation failed: selector returned nil")
	}
	CurrentElement = child
	summary := summarizeElementFunc(CurrentElement)
	return fmt.Sprintf("focused child element: %s", summary), nil
}

func mcpParent() (string, error) {
	if CurrentElement == nil {
		return "", fmt.Errorf("no current element – call load_url/search first")
	}
	parent, err := parentSelector(CurrentElement)
	if err != nil {
		return "", fmt.Errorf("parent navigation failed: %w", err)
	}
	if parent == nil {
		return "", fmt.Errorf("parent navigation failed: selector returned nil")
	}
	CurrentElement = parent
	summary := summarizeElementFunc(CurrentElement)
	return fmt.Sprintf("focused parent element: %s", summary), nil
}

func formatCurrentFocus(total int) string {
	return fmt.Sprintf("focused index %d of %d: %s", currentIndex, total, summarizeElementFunc(CurrentElement))
}

func selectElementAt(idx int) (string, error) {
	if idx < 0 || idx >= len(elementList) {
		return "", fmt.Errorf("index %d out of range (0-%d)", idx, len(elementList)-1)
	}
	currentIndex = idx
	CurrentElement = elementList[currentIndex]
	return formatCurrentFocus(len(elementList)), nil
}

func indexOfElement(list []*rod.Element, target *rod.Element) int {
	for i, el := range list {
		if el == target {
			return i
		}
	}
	return -1
}

func formatElementListResponse(header string, elements []*rod.Element, focus int) string {
	var b strings.Builder
	b.WriteString(header)
	b.WriteString("\n")
	if len(elements) == 0 {
		b.WriteString("no elements available\n")
		return strings.TrimSuffix(b.String(), "\n")
	}
	if focus < 0 || focus >= len(elements) {
		focus = 0
	}
	summary := summarizeElementFunc(elements[focus])
	fmt.Fprintf(&b, "focused index %d of %d: %s\n", focus, len(elements), summary)
	for i, el := range elements {
		marker := " "
		if i == focus {
			marker = "*"
		}
		fmt.Fprintf(&b, "%d%s %s\n", i, marker, summarizeElementFunc(el))
	}
	return strings.TrimSuffix(b.String(), "\n")
}

func summarizeElement(el *rod.Element) string {
	if el == nil {
		return "(no element)"
	}

	var parts []string

	if tag, err := safeEvalAttr(el, "return (this.tagName || '').toLowerCase();"); err == nil && tag != "" {
		parts = append(parts, tag)
	}

	if idPtr, err := el.Attribute("id"); err == nil && idPtr != nil && *idPtr != "" {
		parts = append(parts, fmt.Sprintf("#%s", *idPtr))
	}

	if classPtr, err := el.Attribute("class"); err == nil && classPtr != nil && *classPtr != "" {
		className := strings.Join(strings.Fields(*classPtr), ".")
		if className != "" {
			parts = append(parts, "."+className)
		}
	}

	text, err := el.Text()
	if err == nil && text != "" {
		text = normalizeWhitespace(text)
		if len(text) > 60 {
			text = text[:57] + "..."
		}
		parts = append(parts, fmt.Sprintf("text=%q", text))
	}

	if len(parts) == 0 {
		return "(element)"
	}
	return strings.Join(parts, " ")
}

func safeEvalAttr(el *rod.Element, body string) (string, error) {
	val, err := el.Eval(fmt.Sprintf("() => { %s }", body))
	if err != nil {
		return "", err
	}
	return fmt.Sprint(val.Value), nil
}

func normalizeWhitespace(s string) string {
	fields := strings.Fields(s)
	return strings.Join(fields, " ")
}
