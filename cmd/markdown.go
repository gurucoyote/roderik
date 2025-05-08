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

	// recursive renderer
	var render func(nodeID proto.AccessibilityAXNodeID, depth int)
	render = func(nodeID proto.AccessibilityAXNodeID, depth int) {
		node, ok := idMap[nodeID]
		if !ok || node.Ignored {
			return
		}

		// grab the node’s accessible name (if any)
		name := ""
		if node.Name != nil {
			name = node.Name.Value.String()
		}

		role := node.Role.Value.String()

		// helper to fetch an attribute (href/src) for links/images
		fetchAttr := func(attr string) string {
			var val string
			if node.BackendDOMNodeID != proto.DOMBackendNodeID(0) {
				resolver := proto.DOMResolveNode{
					BackendNodeID: proto.DOMBackendNodeID(node.BackendDOMNodeID),
				}
				if res, err := resolver.Call(page); err == nil {
					if el, err2 := page.ElementFromObject(res.Object); err2 == nil {
						if a, err := el.Attribute(attr); err == nil {
							val = *a
						}
					}
				}
			}
			return val
		}

		indent := strings.Repeat("  ", depth)

		switch role {
		case "heading":
			sb.WriteString("# " + name + "\n\n")

		case "paragraph":
			if name != "" {
				sb.WriteString(name + "\n\n")
			}
			return // do NOT recurse into static-text children

		case "list":
			// just recurse; listitems will render themselves
			for _, cid := range node.ChildIDs {
				render(cid, depth)
			}
			return

		case "listitem":
			sb.WriteString(indent + "- " + name + "\n")
			// handle any nested lists/items
			for _, cid := range node.ChildIDs {
				render(cid, depth+1)
			}
			return

		case "link":
			href := fetchAttr("href")
			if href == "" {
				// fallback to text only if no href
				sb.WriteString("[" + name + "]" + "\n\n")
			} else {
				sb.WriteString(fmt.Sprintf("[%s](%s)\n\n", name, href))
			}
			return

		case "image", "img":
			src := fetchAttr("src")
			// alt text = name
			sb.WriteString(fmt.Sprintf("![%s](%s)\n\n", name, src))
			return

		case "blockquote":
			for _, line := range strings.Split(name, "\n") {
				sb.WriteString("> " + line + "\n")
			}
			sb.WriteString("\n")
			return

		case "pre":
			// code block
			content := name
			sb.WriteString("```\n" + content + "\n```\n\n")
			return

		default:
			// any other container or region: just recurse
			for _, cid := range node.ChildIDs {
				render(cid, depth)
			}
			return
		}
	}

	// emit document starting at root
	render(rootID, 0)
	return sb.String()
}
