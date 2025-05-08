package cmd

import (
	"fmt"
	"strings"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// convertAXTreeToMarkdown walks the accessibility tree and emits a
// standards-compliant Markdown document.
func convertAXTreeToMarkdown(tree *proto.AccessibilityQueryAXTreeResult, page *rod.Page) string {
	// build a quick lookup by AX node ID
	idMap := make(map[proto.AccessibilityAXNodeID]*proto.AccessibilityAXNode, len(tree.Nodes))
	for _, n := range tree.Nodes {
		idMap[n.NodeID] = n
	}

	// find the root of this sub-tree (ParentID == 0)
	var rootID proto.AccessibilityAXNodeID
	for _, n := range tree.Nodes {
		if n.ParentID == proto.AccessibilityAXNodeID(0) {
			rootID = n.NodeID
			break
		}
	}
	// fallback if we never saw a true ParentID==0 (e.g. subtree starts mid-tree)
	if rootID == proto.AccessibilityAXNodeID(0) && len(tree.Nodes) > 0 {
		rootID = tree.Nodes[0].NodeID
	}

	var sb strings.Builder

	// helper: given an AX node, pull href/src from its DOM element
	fetchAttr := func(node *proto.AccessibilityAXNode, attr string) string {
		if node.BackendDOMNodeID == proto.DOMBackendNodeID(0) {
			return ""
		}
		res, err := proto.DOMResolveNode{
			BackendNodeID: node.BackendDOMNodeID,
		}.Call(page)
		if err != nil {
			return ""
		}
		el, err := page.ElementFromObject(res.Object)
		if err != nil {
			return ""
		}
		a, err := el.Attribute(attr)
		if err != nil || a == nil {
			return ""
		}
		return *a
	}

	// Simple Markdown renderer:
	for _, node := range tree.Nodes {
		if node.Ignored {
			continue
		}
		role := node.Role.Value.String()
		name := ""
		if node.Name != nil {
			name = node.Name.Value.String()
		}

		switch role {
		case "heading":
			// Use a single # for all headings; you can bump level if you track depth
			sb.WriteString("# " + name + "\n\n")

		case "paragraph", "StaticText":
			// paragraphs or free text
			if name != "" {
				sb.WriteString(name + "\n\n")
			}

		case "listitem":
			sb.WriteString("- " + name + "\n")

		case "separator":
			sb.WriteString("---\n\n")

		case "link":
			// Markdown link
			href := fetchAttr(node, "href")
			if href != "" {
				sb.WriteString(fmt.Sprintf("[%s](%s)", name, href))
			} else if name != "" {
				sb.WriteString(name)
			}

		case "image", "img":
			// Markdown image
			src := fetchAttr(node, "src")
			sb.WriteString(fmt.Sprintf("![%s](%s)\n\n", name, src))

		case "button", "textbox":
			// render as a bolded button/textbox label
			sb.WriteString(fmt.Sprintf("**%s**\n\n", name))

		case "LineBreak":
			sb.WriteString("  \n") // Markdown hard line-break

		default:
			// fallback: just emit the name
			if name != "" {
				sb.WriteString(name + "\n\n")
			}
		}
	}
	return sb.String()
}
