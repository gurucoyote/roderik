package cmd

import (
	"strings"
	"testing"

	"github.com/go-rod/rod/lib/proto"
	"github.com/ysmood/gson"
)

func TestConvertAXTreeToMarkdownRespectsHeadingLevel(t *testing.T) {
	makeHeading := func(id string, level int, title string) *proto.AccessibilityAXNode {
		return &proto.AccessibilityAXNode{
			NodeID: proto.AccessibilityAXNodeID(id),
			Role: &proto.AccessibilityAXValue{
				Type:  proto.AccessibilityAXValueTypeRole,
				Value: gson.New("heading"),
			},
			Name: &proto.AccessibilityAXValue{
				Type:  proto.AccessibilityAXValueTypeString,
				Value: gson.New(title),
			},
			Properties: []*proto.AccessibilityAXProperty{
				{
					Name: proto.AccessibilityAXPropertyNameLevel,
					Value: &proto.AccessibilityAXValue{
						Type:  proto.AccessibilityAXValueTypeInteger,
						Value: gson.New(level),
					},
				},
			},
		}
	}

	tree := &proto.AccessibilityQueryAXTreeResult{
		Nodes: []*proto.AccessibilityAXNode{
			makeHeading("1", 2, "Section"),
			makeHeading("2", 3, "Subsection"),
		},
	}

	md := convertAXTreeToMarkdown(tree, nil)
	if !strings.Contains(md, "\n## Section\n") {
		t.Fatalf("expected level-2 heading, got:\n%s", md)
	}
	if !strings.Contains(md, "\n### Subsection\n") {
		t.Fatalf("expected level-3 heading, got:\n%s", md)
	}
}
