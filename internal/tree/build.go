package tree

import (
	"encoding/json"
	"fmt"

	"github.com/SwishHQ/waffle/internal/contract"
)

// Tree is a built, validated document tree plus the document-level assets the
// render pipeline needs.
type Tree struct {
	Root                *Node
	Fonts               []contract.FontRegistration
	EmojiSource         *contract.EmojiSource
	HyphenationCallback *contract.CallbackRef
	Warnings            []string
}

// Build constructs the internal tree from a parsed contract and validates its
// structure. On a structural error it returns the error (with element paths) and
// a nil tree.
func Build(ct *contract.Tree) (*Tree, error) {
	if ct == nil || ct.Document == nil {
		return nil, fmt.Errorf("tree: nil contract document")
	}

	root := &Node{Type: contract.TypeDocument, Props: ct.Document.Props}
	applyProps(root)
	for _, c := range ct.Document.Children {
		root.Children = append(root.Children, buildNode(c, root))
	}

	if err := Validate(root); err != nil {
		return nil, err
	}

	return &Tree{
		Root:                root,
		Fonts:               ct.Document.Fonts,
		EmojiSource:         ct.Document.EmojiSource,
		HyphenationCallback: ct.Document.HyphenationCallback,
		Warnings:            append([]string(nil), ct.Warnings...),
	}, nil
}

func buildNode(cn *contract.Node, parent *Node) *Node {
	n := &Node{
		Type:   cn.Type,
		Value:  cn.Value,
		Props:  cn.Props,
		Parent: parent,
	}
	applyProps(n)
	for _, c := range cn.Children {
		n.Children = append(n.Children, buildNode(c, n))
	}
	return n
}

// applyProps parses the structural props common across elements. Type-specific
// props (Page size, Image src, form attributes, …) are left in Props for the
// subsystems that consume them.
func applyProps(n *Node) {
	if n.Props == nil {
		return
	}
	n.Style = n.Props["style"]
	if b, ok := propBool(n.Props, "fixed"); ok {
		n.Fixed = b
	}
	if b, ok := propBool(n.Props, "break"); ok {
		n.Break = b
	}
	if b, ok := propBool(n.Props, "debug"); ok {
		n.Debug = b
	}
	if b, ok := propBool(n.Props, "wrap"); ok {
		n.Wrap = &b
	}
	if f, ok := propFloat(n.Props, "minPresenceAhead"); ok {
		n.MinPresenceAhead = f
	}
	if s, ok := propString(n.Props, "id"); ok {
		n.ID = s
	}
	if cb, ok := n.Props["render"].(contract.CallbackRef); ok {
		n.Render = &cb
	}
}

func propBool(p map[string]any, k string) (bool, bool) {
	b, ok := p[k].(bool)
	return b, ok
}

func propString(p map[string]any, k string) (string, bool) {
	s, ok := p[k].(string)
	return s, ok
}

func propFloat(p map[string]any, k string) (float64, bool) {
	switch v := p[k].(type) {
	case json.Number:
		f, err := v.Float64()
		return f, err == nil
	case float64:
		return v, true
	}
	return 0, false
}
