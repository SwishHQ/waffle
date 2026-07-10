package layout

import (
	"encoding/json"
	"strconv"
	"strings"
	"unicode"

	"github.com/swish/feast/internal/contract"
	"github.com/swish/feast/internal/flexbox"
	"github.com/swish/feast/internal/fontstore"
	"github.com/swish/feast/internal/pdf"
	"github.com/swish/feast/internal/pdf/afm"
	"github.com/swish/feast/internal/stylesheet"
	"github.com/swish/feast/internal/tree"
)

// defaultFontSize is react-pdf's default font size in points.
const defaultFontSize = 18

// textResolve holds a Text node's flattened content and resolved typography.
type textResolve struct {
	content    string
	base       string            // standard-14 base font; "" when a custom font is embedded
	embedded   *pdf.EmbeddedFont // registered custom font to embed; nil for standard fonts
	size       float64
	ascent     float64
	lineHeight float64
	color      string
	measurer   fontstore.Font // measures StringWidth; a *afm.Metrics or a *fontstore.Face
	lines      []string       // wrapped lines (set by the most recent measure)
	orphans    int
	widows     int
	template   string // string render-prop template, substituted per page (page numbers)
	callbackID string // function render-prop callback id ($cb), evaluated per page
	transform  string // textTransform, re-applied to per-page substituted content

	letterSpacing float64 // extra advance per character (points), added to every glyph
	wordSpacing   float64 // extra advance per space character (points)
	maxLines      int     // cap on wrapped lines (0 = unlimited)
	ellipsis      bool    // textOverflow:ellipsis — trailing … on a truncated last line
}

// stringWidth measures a string's advance, including letterSpacing (per character)
// and wordSpacing (per space), matching the PDF Tc/Tw operators used at paint time.
func (t *textResolve) stringWidth(s string) float64 {
	w := t.measurer.StringWidth(s, t.size)
	if t.letterSpacing != 0 {
		w += t.letterSpacing * float64(len([]rune(s)))
	}
	if t.wordSpacing != 0 {
		w += t.wordSpacing * float64(strings.Count(s, " "))
	}
	return w
}

// measure is the flexbox measure function for a Text leaf. It greedily wraps the
// content to the available width and reports the widest line and the total
// height (lines × line height). The wrapped lines are stored for the renderer.
func (t *textResolve) measure(availW, availH float64) flexbox.Size {
	if t.measurer == nil {
		return flexbox.Size{W: 0, H: t.lineHeight}
	}
	t.lines = t.wrap(availW)
	if len(t.lines) == 0 {
		return flexbox.Size{W: 0, H: t.lineHeight}
	}
	maxW := 0.0
	for _, ln := range t.lines {
		if w := t.stringWidth(ln); w > maxW {
			maxW = w
		}
	}
	return flexbox.Size{W: maxW, H: float64(len(t.lines)) * t.lineHeight}
}

// wrap greedily breaks the content into lines no wider than maxW. Whitespace is
// collapsed; a single word wider than maxW is left to overflow (character-level
// breaking and explicit newlines are refinements for the text engine).
func (t *textResolve) wrap(maxW float64) []string {
	words := strings.Fields(t.content)
	if len(words) == 0 {
		return nil
	}
	if maxW <= 0 {
		return t.truncate([]string{strings.Join(words, " ")}, maxW)
	}
	var lines []string
	cur := words[0]
	for _, w := range words[1:] {
		trial := cur + " " + w
		if t.stringWidth(trial) <= maxW {
			cur = trial
		} else {
			lines = append(lines, cur)
			cur = w
		}
	}
	return t.truncate(append(lines, cur), maxW)
}

// truncate caps the wrapped lines at maxLines. When textOverflow:ellipsis is set
// and content was dropped, the last kept line gets a trailing … trimmed to fit.
func (t *textResolve) truncate(lines []string, maxW float64) []string {
	if t.maxLines <= 0 || len(lines) <= t.maxLines {
		return lines
	}
	lines = lines[:t.maxLines]
	if t.ellipsis {
		i := len(lines) - 1
		lines[i] = t.ellipsize(lines[i], maxW)
	}
	return lines
}

// ellipsize appends … to a line, dropping trailing runes until it fits maxW.
func (t *textResolve) ellipsize(s string, maxW float64) string {
	const e = "…"
	if maxW <= 0 {
		return s + e
	}
	r := []rune(strings.TrimRight(s, " "))
	for len(r) > 0 {
		if cand := string(r) + e; t.stringWidth(cand) <= maxW {
			return cand
		}
		r = r[:len(r)-1]
	}
	return e
}

func resolveText(node *tree.Node, style map[string]any, ctx stylesheet.Context, eval Evaluator, store *fontstore.Store) *textResolve {
	content := collectText(node)

	// A render prop provides per-page content, substituted after pagination once
	// page numbers are known. A string is a template ("{pageNumber} / {totalPages}");
	// a function is a $cb callback evaluated on the VM. Seed initial content (with
	// page 1 of 1) so the box measures representatively.
	template, _ := node.Props["render"].(string)
	var callbackID string
	if node.Render != nil {
		callbackID = node.Render.ID
	}
	if content == "" && template != "" {
		content = renderTemplate(template, 1, 1)
	}
	if content == "" && callbackID != "" && eval != nil {
		if s, err := eval.EvalText(callbackID, PageContext{PageNumber: 1, TotalPages: 1, SubPageNumber: 1, SubPageTotalPages: 1}); err == nil {
			content = s
		}
	}
	if content == "" {
		return nil
	}
	transform := str(style["textTransform"])
	content = applyTextTransform(content, transform)
	size := fontSizeOf(style, ctx)
	tr := &textResolve{
		content:       content,
		size:          size,
		color:         str(style["color"]),
		lineHeight:    lineHeightOf(style, size, ctx),
		orphans:       propInt(node.Props, "orphans", 2),
		widows:        propInt(node.Props, "widows", 2),
		template:      template,
		callbackID:    callbackID,
		transform:     transform,
		letterSpacing: spacingOf(style, "letterSpacing", ctx),
		wordSpacing:   spacingOf(style, "wordSpacing", ctx),
		maxLines:      propInt(style, "maxLines", 0),
		ellipsis:      str(style["textOverflow"]) == "ellipsis",
	}

	weight := 400
	if w, ok := stylesheet.ParseFontWeight(style["fontWeight"]); ok {
		weight = w
	}
	fstyle := fontstore.ParseStyle(str(style["fontStyle"]))
	family := fontFamilyOf(style)

	// A registered custom font (Font.register) wins over the standard fonts.
	if store != nil {
		if face, ok := store.ResolveFace(family, weight, fstyle); ok {
			tr.measurer = face
			tr.embedded = face.EmbeddedFont()
			tr.ascent = face.Descriptor().Ascent / 1000 * size
			return tr
		}
	}
	if base, ok := fontstore.StandardBaseFont(family, weight, fstyle); ok {
		tr.base = base
		if m, err := afm.Load(base); err == nil {
			tr.measurer = m
			tr.ascent = m.Ascender / 1000 * size
		}
	}
	return tr
}

// collectText concatenates the text of all TEXT_INSTANCE descendants in order.
func collectText(node *tree.Node) string {
	var b strings.Builder
	var walk func(*tree.Node)
	walk = func(n *tree.Node) {
		if n.Type == contract.TypeTextInstance {
			b.WriteString(n.Value)
			return
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(node)
	return b.String()
}

// fontFamilyOf returns the first font family (a string or fallback list),
// defaulting to Helvetica.
func fontFamilyOf(style map[string]any) string {
	switch v := style["fontFamily"].(type) {
	case string:
		if v != "" {
			return v
		}
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok && s != "" {
				return s
			}
		}
	}
	return "Helvetica"
}

// applyTextTransform applies a CSS textTransform to content before it is measured
// and wrapped, so line breaking uses the transformed widths.
func applyTextTransform(s, transform string) string {
	switch transform {
	case "uppercase":
		return strings.ToUpper(s)
	case "lowercase":
		return strings.ToLower(s)
	case "capitalize":
		return capitalizeWords(s)
	default:
		return s
	}
}

// capitalizeWords uppercases the first letter of each whitespace-delimited word,
// leaving the remaining characters unchanged (CSS "capitalize" semantics).
func capitalizeWords(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	atWordStart := true
	for _, r := range s {
		if atWordStart && unicode.IsLetter(r) {
			b.WriteRune(unicode.ToUpper(r))
		} else {
			b.WriteRune(r)
		}
		atWordStart = unicode.IsSpace(r)
	}
	return b.String()
}

// renderTemplate substitutes page-number placeholders in a render template.
func renderTemplate(tmpl string, pageNumber, totalPages int) string {
	return strings.NewReplacer(
		"{pageNumber}", strconv.Itoa(pageNumber),
		"{totalPages}", strconv.Itoa(totalPages),
		"{subPageNumber}", strconv.Itoa(pageNumber),
	).Replace(tmpl)
}

// applyPageNumbers resolves per-page render-props on every page once the total
// page count is known (a post-pagination pass). String templates are substituted
// locally; function render-props are evaluated on the VM via eval. Global page
// numbering.
func applyPageNumbers(pages []*Page, eval Evaluator) {
	total := len(pages)
	for i, pg := range pages {
		substituteTemplates(pg.Root, i+1, total, eval)
	}
}

func substituteTemplates(b *Box, pageNumber, totalPages int, eval Evaluator) {
	if b.Text != nil {
		switch {
		case b.Text.Template != "":
			content := applyTextTransform(renderTemplate(b.Text.Template, pageNumber, totalPages), b.Text.Transform)
			b.Text.Content = content
			b.Text.Lines = []string{content} // page-number text is a single line
		case b.Text.CallbackID != "" && eval != nil:
			pc := PageContext{PageNumber: pageNumber, TotalPages: totalPages, SubPageNumber: pageNumber, SubPageTotalPages: totalPages}
			if s, err := eval.EvalText(b.Text.CallbackID, pc); err == nil {
				content := applyTextTransform(s, b.Text.Transform)
				b.Text.Content = content
				b.Text.Lines = []string{content}
			}
		}
	}
	for _, c := range b.Children {
		substituteTemplates(c, pageNumber, totalPages, eval)
	}
}

func propInt(p map[string]any, key string, def int) int {
	if p == nil {
		return def
	}
	switch v := p[key].(type) {
	case json.Number:
		if n, err := v.Int64(); err == nil {
			return int(n)
		}
	case float64:
		return int(v)
	case int:
		return v
	}
	return def
}

func fontSizeOf(style map[string]any, ctx stylesheet.Context) float64 {
	if v, ok := style["fontSize"]; ok {
		if val, err := stylesheet.ParseValue(v); err == nil && !val.IsAuto() {
			return val.Resolve(ctx, 0)
		}
	}
	return defaultFontSize
}

// lineHeightOf resolves line height: a bare number is a multiplier of the font
// size; a value with a unit is absolute. Defaults to 1.2 × size.
func lineHeightOf(style map[string]any, size float64, ctx stylesheet.Context) float64 {
	switch t := style["lineHeight"].(type) {
	case json.Number:
		if f, err := t.Float64(); err == nil {
			return f * size
		}
	case float64:
		return t * size
	case int:
		return float64(t) * size
	case string:
		if val, err := stylesheet.ParseValue(t); err == nil && !val.IsAuto() && val.Unit != stylesheet.UnitPercent {
			return val.Resolve(ctx, 0)
		}
	}
	return size * 1.2
}

// spacingOf resolves a spacing property (letterSpacing/wordSpacing) to points. A
// bare number is absolute points; a value with a unit resolves normally.
// Defaults to 0.
func spacingOf(style map[string]any, key string, ctx stylesheet.Context) float64 {
	switch t := style[key].(type) {
	case json.Number:
		if f, err := t.Float64(); err == nil {
			return f
		}
	case float64:
		return t
	case int:
		return float64(t)
	case string:
		if val, err := stylesheet.ParseValue(t); err == nil && !val.IsAuto() && val.Unit != stylesheet.UnitPercent {
			return val.Resolve(ctx, 0)
		}
	}
	return 0
}
