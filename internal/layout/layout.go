// Package layout is the layout pipeline: it resolves styles and page sizes,
// drives the flexbox engine, and produces a tree of positioned boxes ready for
// painting. This is waffle's analogue of react-pdf's layout steps
// (resolveStyles → resolveInheritance → resolvePageSizes → resolveDimensions),
// for the View-only subset; text measurement and pagination land in later phases.
package layout

import (
	"github.com/SwishHQ/waffle/internal/contract"
	"github.com/SwishHQ/waffle/internal/flexbox"
	"github.com/SwishHQ/waffle/internal/fontstore"
	"github.com/SwishHQ/waffle/internal/pdf"
	"github.com/SwishHQ/waffle/internal/stylesheet"
	"github.com/SwishHQ/waffle/internal/tree"
)

// Rect is an absolute box in page coordinates (points), border-box.
type Rect struct {
	X, Y, W, H float64
}

// Box is a positioned node: its resolved style, its absolute frame, and its
// children.
type Box struct {
	Node     *tree.Node
	Style    map[string]any
	Frame    Rect
	Text     *TextInfo  // non-nil for Text boxes with renderable content
	Image    *ImageInfo // non-nil for Image boxes with a decoded source
	SVG      *tree.Node // non-nil for Svg boxes (the subtree, drawn by the renderer)
	Canvas   []any      // non-nil for Canvas boxes: the recorded painter ops
	Children []*Box
}

// ImageInfo is a decoded image ready to draw.
type ImageInfo struct {
	Spec      *pdf.ImageSpec
	ObjectFit string
}

// TextInfo is the resolved text content and typography for a Text box, ready to
// paint. Exactly one font source is set: BaseFont (a standard-14 font name) for
// standard fonts, or EmbeddedFont for a registered custom font.
type TextInfo struct {
	Content       string
	Lines         []string // wrapped lines
	BaseFont      string
	Size          float64
	Ascent        float64 // points from the box top to the baseline
	LineHeight    float64
	Color         string
	Orphans       int
	Widows        int
	Template      string            // string render-prop template (page numbers); "" if none
	CallbackID    string            // function render-prop callback id ($cb); "" if none
	Transform     string            // textTransform, re-applied to per-page substituted content
	LetterSpacing float64           // extra advance per character (points)
	WordSpacing   float64           // extra advance per space character (points)
	TextIndent    float64           // first-line indent (points)
	EmbeddedFont  *pdf.EmbeddedFont // registered custom font to embed; nil for standard fonts

	// RunLines holds per-line styled fragments for Text with inline runs (nested
	// styled <Text>/<Link>/<Tspan>). When non-empty the renderer paints these
	// instead of the flat Lines; Lines still carries the plain text per line for
	// pagination height math.
	RunLines [][]RunFragment
}

// RunFragment is one styled piece of text positioned on a line, at X points from
// the line's start. Exactly one of BaseFont / EmbeddedFont is set.
type RunFragment struct {
	Text          string
	X             float64
	BaseFont      string
	EmbeddedFont  *pdf.EmbeddedFont
	Size          float64
	Color         string
	LetterSpacing float64
	WordSpacing   float64
	Underline     bool
	Strike        bool
}

// Page is one laid-out page.
type Page struct {
	Width, Height float64
	Root          *Box
}

// Result is the laid-out document.
type Result struct {
	Pages    []*Page
	Warnings []string
}

// Options tune unit resolution.
type Options struct {
	DPI     float64 // default 72
	RemBase float64 // default 18
	// Eval evaluates function render-props (render={fn}) per page. Nil for the
	// static path (such render-props then resolve to empty).
	Eval Evaluator
	// Cache memoizes decoded images and parsed fonts across renders of the same
	// document (a Template owns one and passes it here). Nil disables caching —
	// every render decodes and parses its assets from scratch.
	Cache *Cache
}

func (o Options) dpi() float64 {
	if o.DPI == 0 {
		return 72
	}
	return o.DPI
}

func (o Options) remBase() float64 {
	if o.RemBase == 0 {
		return 18
	}
	return o.RemBase
}

// layoutNode pairs a tree node and its resolved style with the flexbox node used
// to compute its geometry.
type layoutNode struct {
	tnode  *tree.Node
	style  map[string]any
	flex   *flexbox.Node
	text   *textResolve
	image  *imageResolve
	svg    *tree.Node
	canvas []any
	kids   []*layoutNode
}

// Layout lays out every Page in the document.
func Layout(t *tree.Tree, opts Options) (*Result, error) {
	base := stylesheet.Context{DPI: opts.dpi(), RemBase: opts.remBase()}
	res := &Result{Warnings: append([]string(nil), t.Warnings...)}
	store, warns := opts.Cache.fontStore(t.Fonts)
	res.Warnings = append(res.Warnings, warns...)

	for _, pageNode := range t.Root.Children {
		if pageNode.Type != contract.TypePage {
			continue
		}
		w, h, orientation := resolvePageSize(pageNode, base)
		ctx := base
		ctx.PageW, ctx.PageH = w, h
		media := stylesheet.MediaContext{Width: w, Height: h, Orientation: orientation}

		ln := buildLayoutNode(pageNode, nil, media, ctx, opts.Eval, store, opts.Cache)
		flexbox.Calculate(ln.flex, w, h)

		page := &Page{Width: w, Height: h, Root: toBox(ln, 0, 0)}
		if opts.Eval != nil {
			resolveCanvasPaint(page.Root, opts.Eval)
		}
		contentTop := lp(ln.style, "paddingTop", ctx) + lp(ln.style, "borderTopWidth", ctx)
		availH := h - contentTop - lp(ln.style, "paddingBottom", ctx) - lp(ln.style, "borderBottomWidth", ctx)
		res.Pages = append(res.Pages, paginate(page, contentTop, availH, pageWraps(pageNode))...)
	}
	applyPageNumbers(res.Pages, opts.Eval)
	return res, nil
}

// buildLayoutNode resolves a node's style (inheriting from parentEffective),
// maps it to flexbox inputs, and recurses. TEXT_INSTANCE children are skipped
// here; text measurement arrives in the text phase.
func buildLayoutNode(node *tree.Node, parentEffective map[string]any, media stylesheet.MediaContext, ctx stylesheet.Context, eval Evaluator, store *fontstore.Store, cache *Cache) *layoutNode {
	own := stylesheet.Resolve(node.Style, media)
	eff := stylesheet.Inherit(parentEffective, own, node.Type == contract.TypeText)

	fx := &flexbox.Node{Style: toFlexStyle(eff, ctx)}
	ln := &layoutNode{tnode: node, style: eff, flex: fx}

	// A Text node is a measured leaf: its content is flattened and measured as a
	// unit (rich inline runs and wrapping arrive with the text engine).
	if node.Type == contract.TypeText {
		if tr := resolveText(node, eff, media, ctx, eval, store); tr != nil {
			ln.text = tr
			fx.Measure = tr.measure
		}
		return ln
	}

	if node.Type == contract.TypeImage {
		if im := resolveImage(node, eff, cache); im != nil {
			ln.image = im
			fx.Measure = im.measure
			if fx.Style.AspectRatio == 0 && im.h > 0 {
				fx.Style.AspectRatio = im.w / im.h
			}
		}
		return ln
	}

	// An Svg node is a leaf in flexbox; its subtree is drawn by the renderer.
	if node.Type == contract.TypeSvg {
		ln.svg = node
		vbW, vbH := svgViewBoxSize(node)
		if fx.Style.AspectRatio == 0 && vbH > 0 {
			fx.Style.AspectRatio = vbW / vbH
		}
		fx.Measure = func(availW, availH float64) flexbox.Size {
			return flexbox.Size{W: vbW, H: vbH}
		}
		return ln
	}

	// A Canvas node is a leaf; its painter ops are replayed by the renderer.
	// It relies on explicit width/height (the paint has no intrinsic size).
	if node.Type == contract.TypeCanvas {
		if ops, ok := node.Props["paint"].([]any); ok {
			ln.canvas = ops
		}
		fx.Measure = func(availW, availH float64) flexbox.Size { return flexbox.Size{} }
		return ln
	}

	for _, c := range node.Children {
		if c.Type == contract.TypeTextInstance {
			continue
		}
		child := buildLayoutNode(c, eff, media, ctx, eval, store, cache)
		ln.kids = append(ln.kids, child)
		fx.Children = append(fx.Children, child.flex)
	}
	return ln
}

// toBox converts a laid-out node tree to Boxes with absolute frames. absX/absY
// are the border-box origin of the node's parent (the flexbox layout stores each
// node's position relative to that origin).
func toBox(ln *layoutNode, absX, absY float64) *Box {
	f := ln.flex.Layout
	x := absX + f.Left
	y := absY + f.Top
	box := &Box{
		Node:  ln.tnode,
		Style: ln.style,
		Frame: Rect{X: x, Y: y, W: f.Width, H: f.Height},
	}
	if t := ln.text; t != nil {
		box.Text = &TextInfo{
			Content:       t.content,
			Lines:         t.lines,
			BaseFont:      t.base,
			Size:          t.size,
			Ascent:        t.ascent,
			LineHeight:    t.lineHeight,
			Color:         t.color,
			Orphans:       t.orphans,
			Widows:        t.widows,
			Template:      t.template,
			CallbackID:    t.callbackID,
			Transform:     t.transform,
			LetterSpacing: t.letterSpacing,
			WordSpacing:   t.wordSpacing,
			TextIndent:    t.textIndent,
			EmbeddedFont:  t.embedded,
			RunLines:      t.runLines,
		}
	}
	if im := ln.image; im != nil {
		box.Image = &ImageInfo{Spec: im.spec, ObjectFit: im.fit}
	}
	if ln.svg != nil {
		box.SVG = ln.svg
	}
	if ln.canvas != nil {
		box.Canvas = ln.canvas
	}
	for _, k := range ln.kids {
		box.Children = append(box.Children, toBox(k, x, y))
	}
	return box
}

// resolveCanvasPaint evaluates a Canvas box's function paint callback now that
// its frame (width/height) is known, attaching the recorded ops. A canvas whose
// paint is a pre-recorded op list (box.Canvas already set) is left untouched.
func resolveCanvasPaint(b *Box, eval Evaluator) {
	if b.Canvas == nil && b.Node != nil && b.Node.Type == contract.TypeCanvas {
		if cb, ok := b.Node.Props["paint"].(contract.CallbackRef); ok {
			if ops, err := eval.EvalPaint(cb.ID, b.Frame.W, b.Frame.H); err == nil {
				b.Canvas = ops
			}
		}
	}
	for _, c := range b.Children {
		resolveCanvasPaint(c, eval)
	}
}
