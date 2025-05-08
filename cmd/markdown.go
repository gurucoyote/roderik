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

	// flat render based on quax CLI output
	for _, node := range tree.Nodes {
		if node.Ignored {
			continue
		}
		role := node.Role.Value.String()
		switch role {
		case "LineBreak":
		case "generic":
		case "InlineTextBox":
		case "paragraph":
			sb.WriteString("\n" + node.Name.Value.String() + "\n")
		case "separator":
			sb.WriteString("---\n")
		case "listitem":
			sb.WriteString("- \n")
		case "link":
			sb.WriteString(role + "(" + fmt.Sprint(node.BackendDOMNodeID) + ") ")
		case "button", "textbox":
			sb.WriteString(role + "(" + fmt.Sprint(node.BackendDOMNodeID) + ") " + node.Name.Value.String() + "\n")
		case "LabelText":
			sb.WriteString("Label: ")
		case "StaticText":
			sb.WriteString(node.Name.Value.String() + "\n")
		default:
			sb.WriteString(role + ": ")
		}
	}
	return sb.String()
}
