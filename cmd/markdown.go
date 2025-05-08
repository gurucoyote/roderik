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

	// helper to look up a node's parent (or nil)
	parentOf := func(n *proto.AccessibilityAXNode) *proto.AccessibilityAXNode {
		if p, ok := idMap[n.ParentID]; ok {
			return p
		}
		return nil
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

		// get parent's role if we need it
		var pRole string
		if p := parentOf(node); p != nil {
			pRole = p.Role.Value.String()
		}

		switch role {
		case "heading":
			// one blank line before, two after
			sb.WriteString("\n# " + name + "\n\n")
			continue

		case "paragraph":
			// paragraph node already has its full text as Name
			sb.WriteString(name + "\n\n")
			continue

		case "listitem":
			// start a bullet; we'll append the text or link in the child nodes
			sb.WriteString("- ")
			continue

		case "link":
			href := fetchAttr(node, "href")
			if href != "" {
				if pRole == "listitem" {
					sb.WriteString(fmt.Sprintf("[%s](%s)\n", name, href))
				} else {
					sb.WriteString(fmt.Sprintf("[%s](%s)", name, href))
				}
			} else {
				if pRole == "listitem" {
					sb.WriteString(name + "\n")
				} else {
					sb.WriteString(name)
				}
			}
			continue

		case "StaticText", "inlineTextBox":
			if pRole == "paragraph" {
				sb.WriteString(name)
			} else if pRole == "listitem" {
				sb.WriteString(name + "\n")
			}
			continue

		case "separator":
			sb.WriteString("---\n\n")
			continue

		case "image", "img":
			src := fetchAttr(node, "src")
			sb.WriteString(fmt.Sprintf("![%s](%s)\n\n", name, src))
			continue

		case "button", "textbox":
			sb.WriteString(fmt.Sprintf("**%s**\n\n", name))
			continue

		case "LineBreak":
			sb.WriteString("  \n") // Markdown hard line-break
			continue

		default:
			// skip generic, inlineTextBox, etc.
		}
	}
	return sb.String()
}
