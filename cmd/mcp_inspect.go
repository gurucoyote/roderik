package cmd

import (
	"fmt"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

var getElementHTML = func(el *rod.Element) (string, error) {
	return el.HTML()
}

var getElementText = func(el *rod.Element) (string, error) {
	return el.Text()
}

type boxModel interface {
	Box() *proto.DOMRect
}

var getElementShape = func(el *rod.Element) (boxModel, error) {
	shape, err := el.Shape()
	if err != nil {
		return nil, err
	}
	return shape, nil
}

var describeElementFunc = func(el *rod.Element) (*proto.DOMNode, error) {
	return el.Describe(0, true)
}

var getElementXPath = func(el *rod.Element) (string, error) {
	return el.GetXPath(true)
}

const computedStylesScript = `() => {
	const style = window.getComputedStyle(this);
	const styleObject = {};
	for (let i = 0; i < style.length; i++) {
		const prop = style[i];
		const value = style.getPropertyValue(prop);
		if (value) {
			styleObject[prop] = value;
		}
	}
	return styleObject;
}`

var getComputedStyles = func(el *rod.Element) (interface{}, error) {
	result, err := el.Eval(computedStylesScript)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, fmt.Errorf("computed styles evaluation returned nil result")
	}
	return result.Value, nil
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

func mcpText(lengthOpt *int) (string, error) {
	if CurrentElement == nil {
		return "", fmt.Errorf("no current element – call load_url/search first")
	}
	text, err := getElementText(CurrentElement)
	if err != nil {
		return "", fmt.Errorf("failed to get current element text: %w", err)
	}
	if lengthOpt != nil {
		limit := *lengthOpt
		if limit < 0 {
			limit = 0
		}
		if limit < len(text) {
			text = text[:limit]
		}
	}
	return text, nil
}

func mcpBox() (string, error) {
	if CurrentElement == nil {
		return "", fmt.Errorf("no current element – call load_url/search first")
	}
	shape, err := getElementShape(CurrentElement)
	if err != nil {
		return "", fmt.Errorf("failed to get current element box: %w", err)
	}
	if shape == nil {
		return "", fmt.Errorf("failed to get current element box: shape was nil")
	}
	box := shape.Box()
	if box == nil {
		return "", fmt.Errorf("failed to get current element box: box was nil")
	}
	return fmt.Sprintf("box: %s", PrettyFormat(box)), nil
}

func mcpDescribe() (string, error) {
	if CurrentElement == nil {
		return "", fmt.Errorf("no current element – call load_url/search first")
	}
	node, err := describeElementFunc(CurrentElement)
	if err != nil {
		return "", fmt.Errorf("failed to describe current element: %w", err)
	}
	return PrettyFormat(node), nil
}

func mcpXPath() (string, error) {
	if CurrentElement == nil {
		return "", fmt.Errorf("no current element – call load_url/search first")
	}
	xpath, err := getElementXPath(CurrentElement)
	if err != nil {
		return "", fmt.Errorf("failed to compute current element xpath: %w", err)
	}
	return xpath, nil
}

func mcpComputedStyles() (string, error) {
	if CurrentElement == nil {
		return "", fmt.Errorf("no current element – call load_url/search first")
	}
	styles, err := getComputedStyles(CurrentElement)
	if err != nil {
		return "", fmt.Errorf("failed to get computed styles: %w", err)
	}
	return PrettyFormat(styles), nil
}
