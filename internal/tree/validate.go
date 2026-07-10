package tree

import (
	"errors"
	"fmt"

	"github.com/swish/waffle/internal/contract"
)

// svgOnly are element types that are only meaningful inside an <Svg> subtree.
var svgOnly = map[string]bool{
	contract.TypeG: true, contract.TypePath: true, contract.TypeRect: true,
	contract.TypeCircle: true, contract.TypeEllipse: true, contract.TypeLine: true,
	contract.TypePolyline: true, contract.TypePolygon: true, contract.TypeTspan: true,
	contract.TypeDefs: true, contract.TypeClipPath: true,
	contract.TypeLinearGradient: true, contract.TypeRadialGradient: true,
	contract.TypeStop: true, contract.TypeMarker: true,
}

// textChildTypes are the element types allowed as children of <Text>.
var textChildTypes = map[string]bool{
	contract.TypeTextInstance: true,
	contract.TypeText:         true,
	contract.TypeLink:         true,
}

// textParents are the element types under which a raw text string
// (TEXT_INSTANCE) may appear.
var textParents = map[string]bool{
	contract.TypeText:  true,
	contract.TypeLink:  true,
	contract.TypeTspan: true,
	contract.TypeNote:  true,
}

// Validate checks the structural rules react-pdf enforces on the element tree.
// It returns all violations joined into one error, each carrying an element
// path; a valid tree returns nil.
func Validate(root *Node) error {
	if root == nil {
		return fmt.Errorf("tree: nil root")
	}
	if root.Type != contract.TypeDocument {
		return fmt.Errorf("tree: root must be Document, got %s", label(root.Type))
	}
	var errs []error
	walk(root, func(n *Node) {
		errs = append(errs, validateNode(n)...)
	})
	return errors.Join(errs...)
}

func validateNode(n *Node) []error {
	var errs []error

	// Allowed-children rules, keyed by this node's type.
	switch n.Type {
	case contract.TypeDocument:
		for _, c := range n.Children {
			if c.Type != contract.TypePage {
				errs = append(errs, fmt.Errorf("%s: %s is not allowed inside Document (only Page)", Path(c), label(c.Type)))
			}
		}
	case contract.TypeText:
		for _, c := range n.Children {
			if !textChildTypes[c.Type] {
				errs = append(errs, fmt.Errorf("%s: %s is not allowed inside Text (only text, Text, or Link)", Path(c), label(c.Type)))
			}
		}
	case contract.TypeNote:
		for _, c := range n.Children {
			if c.Type != contract.TypeTextInstance {
				errs = append(errs, fmt.Errorf("%s: Note may only contain text", Path(c)))
			}
		}
	case contract.TypeTspan:
		for _, c := range n.Children {
			if c.Type != contract.TypeTextInstance && c.Type != contract.TypeTspan {
				errs = append(errs, fmt.Errorf("%s: %s is not allowed inside Tspan", Path(c), label(c.Type)))
			}
		}
	}

	// Context rules, keyed by this node's own type.
	if n.Type == contract.TypePage && parentType(n) != contract.TypeDocument {
		errs = append(errs, fmt.Errorf("%s: Page must be a direct child of Document", Path(n)))
	}
	if svgOnly[n.Type] && !hasAncestor(n, contract.TypeSvg) {
		errs = append(errs, fmt.Errorf("%s: %s is only valid inside an Svg", Path(n), label(n.Type)))
	}
	if n.Type == contract.TypeTextInstance && !textParents[parentType(n)] {
		errs = append(errs, fmt.Errorf("%s: a text string must be inside Text, Link, Tspan, or Note", Path(n)))
	}

	return errs
}
