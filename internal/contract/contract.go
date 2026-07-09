// Package contract defines feast-tree/v1: the versioned JSON boundary between
// the React authoring layer (@feast/react) and the Go rendering engine. It
// parses the wire format into a lightly-typed node tree, leaving interpretation
// of styles, units, and props to downstream subsystems (the stylesheet and
// layout engines), exactly as react-pdf keeps its node tree uninterpreted until
// layout.
package contract

// Version is the contract version this build implements.
const Version = "feast-tree/v1"

// Node type strings mirror react-pdf's primitive constants so the reconciler is
// a near pass-through and react-pdf's component docs describe this contract.
const (
	TypeDocument        = "DOCUMENT"
	TypePage            = "PAGE"
	TypeView            = "VIEW"
	TypeText            = "TEXT"
	TypeTextInstance    = "TEXT_INSTANCE"
	TypeImage           = "IMAGE"
	TypeImageBackground = "IMAGE_BACKGROUND"
	TypeLink            = "LINK"
	TypeNote            = "NOTE"
	TypeCanvas          = "CANVAS"

	TypeSvg            = "SVG"
	TypeG              = "G"
	TypePath           = "PATH"
	TypeRect           = "RECT"
	TypeCircle         = "CIRCLE"
	TypeEllipse        = "ELLIPSE"
	TypeLine           = "LINE"
	TypePolyline       = "POLYLINE"
	TypePolygon        = "POLYGON"
	TypeTspan          = "TSPAN"
	TypeDefs           = "DEFS"
	TypeClipPath       = "CLIP_PATH"
	TypeLinearGradient = "LINEAR_GRADIENT"
	TypeRadialGradient = "RADIAL_GRADIENT"
	TypeStop           = "STOP"
	TypeMarker         = "MARKER"

	TypeTextInput = "TEXT_INPUT"
	TypeCheckbox  = "CHECKBOX"
	TypeSelect    = "SELECT"
	TypeList      = "LIST"
	TypeFieldSet  = "FIELD_SET"
)

var knownTypes = map[string]bool{
	TypeDocument: true, TypePage: true, TypeView: true, TypeText: true,
	TypeTextInstance: true, TypeImage: true, TypeImageBackground: true,
	TypeLink: true, TypeNote: true, TypeCanvas: true,
	TypeSvg: true, TypeG: true, TypePath: true, TypeRect: true, TypeCircle: true,
	TypeEllipse: true, TypeLine: true, TypePolyline: true, TypePolygon: true,
	TypeTspan: true, TypeDefs: true, TypeClipPath: true, TypeLinearGradient: true,
	TypeRadialGradient: true, TypeStop: true, TypeMarker: true,
	TypeTextInput: true, TypeCheckbox: true, TypeSelect: true, TypeList: true,
	TypeFieldSet: true,
}

// KnownType reports whether t is a recognized node type.
func KnownType(t string) bool { return knownTypes[t] }

// CallbackRef is a serialized function-valued prop, wire form {"$cb":"cb_1"}.
// Function props (render, paint, hyphenationCallback, image src callbacks) can
// only be evaluated in a VM execution mode; the layout pipeline resolves them
// through a CallbackEvaluator.
type CallbackRef struct {
	ID string
}

// InlineAsset is an inlined binary asset, wire form {"$inline":"<base64>"}.
// The CLI's --inline-assets flag converts local font/image references into this
// form so the engine needs no filesystem access to them.
type InlineAsset struct {
	Base64 string
}

// Node is one element of the tree. Prop values are decoded JSON scalars
// (json.Number, string, bool, nil), maps, and slices, with $cb/$inline
// sentinels replaced by CallbackRef/InlineAsset. TEXT_INSTANCE nodes carry Value
// and have no children.
type Node struct {
	Type     string
	Value    string
	Props    map[string]any
	Children []*Node
}

// FontFace is one registered face of a font family.
type FontFace struct {
	Src        any // string URL/path, InlineAsset, or map with fetch options
	FontWeight any // json.Number or keyword string; resolved by the font store
	FontStyle  string
}

// FontRegistration mirrors a Font.register call from @feast/react.
type FontRegistration struct {
	Family string
	Faces  []FontFace
}

// EmojiSource mirrors Font.registerEmojiSource (URL-template form).
type EmojiSource struct {
	URL    string
	Format string
}

// Document is the root of the tree: document-level props and metadata plus the
// page children.
type Document struct {
	Props               map[string]any
	Fonts               []FontRegistration
	EmojiSource         *EmojiSource
	HyphenationCallback *CallbackRef
	Children            []*Node
}

// Tree is a parsed feast-tree document.
type Tree struct {
	Version   string
	Document  *Document
	Callbacks []string // callback ids declared by the producer
	Warnings  []string // non-fatal issues (unknown types, callback mismatches)
}

// RequiresEvaluator reports whether the tree contains function-valued props that
// need a VM execution mode (embedded runtime or sidecar) to resolve. A tree with
// none can be rendered from the static-tree path alone.
func (t *Tree) RequiresEvaluator() bool {
	return len(t.referencedCallbacks()) > 0
}

// referencedCallbacks collects every CallbackRef id reachable in the tree.
func (t *Tree) referencedCallbacks() []string {
	seen := map[string]bool{}
	var out []string
	add := func(id string) {
		if id != "" && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	if t.Document != nil {
		if t.Document.HyphenationCallback != nil {
			add(t.Document.HyphenationCallback.ID)
		}
		for _, f := range t.Document.Fonts {
			for _, face := range f.Faces {
				collectCallbacks(face.Src, add)
			}
		}
		for _, n := range t.Document.Children {
			collectNodeCallbacks(n, add)
		}
	}
	return out
}

func collectNodeCallbacks(n *Node, add func(string)) {
	if n == nil {
		return
	}
	for _, v := range n.Props {
		collectCallbacks(v, add)
	}
	for _, c := range n.Children {
		collectNodeCallbacks(c, add)
	}
}

func collectCallbacks(v any, add func(string)) {
	switch t := v.(type) {
	case CallbackRef:
		add(t.ID)
	case map[string]any:
		for _, val := range t {
			collectCallbacks(val, add)
		}
	case []any:
		for _, val := range t {
			collectCallbacks(val, add)
		}
	}
}
