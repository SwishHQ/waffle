// Package tree is the internal, validated element model the layout pipeline
// walks. It is built from a parsed contract.Tree: structural props (the
// pagination- and identity-relevant ones) are parsed into typed fields, while
// style values are kept raw for the stylesheet subsystem, exactly as react-pdf
// leaves styles uninterpreted until layout.
package tree

import (
	"fmt"
	"strings"

	"github.com/SwishHQ/waffle/internal/contract"
)

// Node is one element of the internal tree.
type Node struct {
	Type  string
	Value string // TEXT_INSTANCE text content

	// Parsed structural props (react-pdf pagination and identity semantics).
	Wrap             *bool // nil = unset; the type default is applied at layout
	Fixed            bool
	Break            bool
	MinPresenceAhead float64
	Debug            bool
	ID               string
	Render           *contract.CallbackRef // render prop, when it is a function

	Style any            // raw style value (object or array); parsed by stylesheet
	Props map[string]any // all props verbatim

	Parent   *Node
	Children []*Node
}

// Clone deep-copies the subtree rooted at n, rewiring Parent pointers. Props
// maps are copied shallowly (values are treated as immutable after Build).
func (n *Node) Clone() *Node { return n.cloneWithParent(nil) }

func (n *Node) cloneWithParent(parent *Node) *Node {
	c := *n
	c.Parent = parent
	if n.Wrap != nil {
		w := *n.Wrap
		c.Wrap = &w
	}
	if n.Render != nil {
		r := *n.Render
		c.Render = &r
	}
	if n.Props != nil {
		c.Props = make(map[string]any, len(n.Props))
		for k, v := range n.Props {
			c.Props[k] = v
		}
	}
	c.Children = nil
	for _, ch := range n.Children {
		c.Children = append(c.Children, ch.cloneWithParent(&c))
	}
	return &c
}

// Path returns a human-readable location for a node, e.g.
// "Document > Page[0] > View[2] > Text[1]", used in validation and render errors.
func Path(n *Node) string {
	if n == nil {
		return ""
	}
	var parts []string
	for cur := n; cur != nil; cur = cur.Parent {
		l := label(cur.Type)
		if cur.Parent != nil {
			l = fmt.Sprintf("%s[%d]", l, indexInParent(cur))
		}
		parts = append(parts, l)
	}
	for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
		parts[i], parts[j] = parts[j], parts[i]
	}
	return strings.Join(parts, " > ")
}

func indexInParent(n *Node) int {
	if n.Parent == nil {
		return 0
	}
	for i, c := range n.Parent.Children {
		if c == n {
			return i
		}
	}
	return -1
}

func parentType(n *Node) string {
	if n.Parent == nil {
		return ""
	}
	return n.Parent.Type
}

func hasAncestor(n *Node, typ string) bool {
	for cur := n.Parent; cur != nil; cur = cur.Parent {
		if cur.Type == typ {
			return true
		}
	}
	return false
}

// walk applies fn to n and its descendants in preorder.
func walk(n *Node, fn func(*Node)) {
	fn(n)
	for _, c := range n.Children {
		walk(c, fn)
	}
}

// label maps a contract node type to its react-pdf component name for messages.
func label(t string) string {
	if l, ok := typeLabels[t]; ok {
		return l
	}
	return t
}

var typeLabels = map[string]string{
	contract.TypeDocument: "Document", contract.TypePage: "Page",
	contract.TypeView: "View", contract.TypeText: "Text",
	contract.TypeTextInstance: "TextInstance", contract.TypeImage: "Image",
	contract.TypeImageBackground: "ImageBackground", contract.TypeLink: "Link",
	contract.TypeNote: "Note", contract.TypeCanvas: "Canvas",
	contract.TypeSvg: "Svg", contract.TypeG: "G", contract.TypePath: "Path",
	contract.TypeRect: "Rect", contract.TypeCircle: "Circle",
	contract.TypeEllipse: "Ellipse", contract.TypeLine: "Line",
	contract.TypePolyline: "Polyline", contract.TypePolygon: "Polygon",
	contract.TypeTspan: "Tspan", contract.TypeDefs: "Defs",
	contract.TypeClipPath: "ClipPath", contract.TypeLinearGradient: "LinearGradient",
	contract.TypeRadialGradient: "RadialGradient", contract.TypeStop: "Stop",
	contract.TypeMarker: "Marker", contract.TypeTextInput: "TextInput",
	contract.TypeCheckbox: "Checkbox", contract.TypeSelect: "Select",
	contract.TypeList: "List", contract.TypeFieldSet: "FieldSet",
}
