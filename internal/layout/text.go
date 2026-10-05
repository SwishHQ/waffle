package layout

import (
	"encoding/json"
	"strconv"
	"strings"
	"unicode"

	"github.com/SwishHQ/waffle/internal/contract"
	"github.com/SwishHQ/waffle/internal/flexbox"
	"github.com/SwishHQ/waffle/internal/fontstore"
	"github.com/SwishHQ/waffle/internal/pdf"
	"github.com/SwishHQ/waffle/internal/pdf/afm"
	"github.com/SwishHQ/waffle/internal/stylesheet"
	"github.com/SwishHQ/waffle/internal/tree"
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
	textIndent    float64 // first-line indent (points)

	runs     []textRun       // inline styled runs; empty for the single-style path
	runLines [][]RunFragment // wrapped run fragments per line (set by measure)

	memo []measureMemo // recent measurements by available width
}

// measureMemo is one measurement at an available width. Wrapping depends only on
// availW (content and typography are fixed once resolved), and layout measures
// the same leaf at the same width repeatedly (an ancestor's measurement, then its
// own arrangement), so a hit restores the lines that measurement produced.
type measureMemo struct {
	availW   float64
	size     flexbox.Size
	lines    []string
	runLines [][]RunFragment
}

// measure is the flexbox measure function for a Text leaf, memoised by availW.
func (t *textResolve) measure(availW, availH float64) flexbox.Size {
	for i := range t.memo {
		if m := &t.memo[i]; m.availW == availW {
			t.lines, t.runLines = m.lines, m.runLines
			return m.size
		}
	}
	sz := t.measureAt(availW, availH)
	if len(t.memo) >= 4 {
		copy(t.memo, t.memo[1:])
		t.memo = t.memo[:3]
	}
	t.memo = append(t.memo, measureMemo{availW: availW, size: sz, lines: t.lines, runLines: t.runLines})
	return sz
}

// textRun is one styled inline piece of a Text's content.
type textRun struct {
	text  string
	style runStyle
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
func (t *textResolve) measureAt(availW, availH float64) flexbox.Size {
	if len(t.runs) > 0 {
		return t.measureRuns(availW)
	}
	if t.measurer == nil {
		return flexbox.Size{W: 0, H: t.lineHeight}
	}
	t.lines = t.wrap(availW)
	if len(t.lines) == 0 {
		return flexbox.Size{W: 0, H: t.lineHeight}
	}
	maxW := 0.0
	for i, ln := range t.lines {
		w := t.stringWidth(ln)
		if i == 0 {
			w += t.textIndent // the first line is pushed right by the indent
		}
		if w > maxW {
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
	// The first line's usable width is reduced by any textIndent.
	avail := func(lineIdx int) float64 {
		if lineIdx == 0 {
			return maxW - t.textIndent
		}
		return maxW
	}
	var lines []string
	cur := words[0]
	for _, w := range words[1:] {
		trial := cur + " " + w
		if t.stringWidth(trial) <= avail(len(lines)) {
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

// stringWidth measures a word in this run's font, including its letter spacing.
func (rs *runStyle) stringWidth(s string) float64 {
	if rs.measurer == nil {
		return 0
	}
	w := rs.measurer.StringWidth(s, rs.size)
	if rs.letterSpacing != 0 {
		w += rs.letterSpacing * float64(len([]rune(s)))
	}
	return w
}

// spaceWidth is the advance of an inter-word space in this run (a space glyph
// plus its wordSpacing and letterSpacing).
func (rs *runStyle) spaceWidth() float64 {
	if rs.measurer == nil {
		return 0
	}
	return rs.measurer.StringWidth(" ", rs.size) + rs.wordSpacing + rs.letterSpacing
}

// hasInlineRuns reports whether a Text node contains nested styled inline
// elements (another Text, a Link, or a Tspan), which require run layout.
func hasInlineRuns(node *tree.Node) bool {
	for _, ch := range node.Children {
		switch ch.Type {
		case contract.TypeText, contract.TypeLink, contract.TypeTspan:
			return true
		}
		if hasInlineRuns(ch) {
			return true
		}
	}
	return false
}

// buildRuns flattens a Text's inline tree into styled runs in document order.
// Each nested Text/Link/Tspan resolves its own style (inheriting from its
// parent); TEXT_INSTANCE leaves become runs carrying the surrounding style.
func buildRuns(node *tree.Node, parentStyle map[string]any, media stylesheet.MediaContext, ctx stylesheet.Context, store *fontstore.Store) []textRun {
	var runs []textRun
	var walk func(n *tree.Node, inherited map[string]any)
	walk = func(n *tree.Node, inherited map[string]any) {
		for _, ch := range n.Children {
			switch ch.Type {
			case contract.TypeTextInstance:
				txt := applyTextTransform(ch.Value, str(inherited["textTransform"]))
				if txt == "" {
					continue
				}
				runs = append(runs, textRun{text: txt, style: resolveRunStyle(inherited, ctx, store)})
			case contract.TypeText, contract.TypeLink, contract.TypeTspan:
				own := stylesheet.Resolve(ch.Style, media)
				eff := stylesheet.Inherit(inherited, own, true)
				walk(ch, eff)
			default:
				walk(ch, inherited)
			}
		}
	}
	walk(node, parentStyle)
	return runs
}

// wordPiece is one whitespace-delimited word tagged with its run and whether a
// space separated it from the previous word (in source order).
type wordPiece struct {
	style       *runStyle
	text        string
	spaceBefore bool
}

// runPieces tokenizes the ordered runs into word pieces, collapsing runs of
// whitespace to single inter-word gaps while preserving gaps across run
// boundaries.
func runPieces(runs []textRun) []wordPiece {
	var pieces []wordPiece
	pendingSpace := false
	started := false
	for i := range runs {
		rs := &runs[i].style
		s := runs[i].text
		field := strings.Builder{}
		flush := func() {
			if field.Len() == 0 {
				return
			}
			pieces = append(pieces, wordPiece{style: rs, text: field.String(), spaceBefore: pendingSpace && started})
			field.Reset()
			pendingSpace = false
			started = true
		}
		for _, r := range s {
			if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
				flush()
				pendingSpace = true
			} else {
				field.WriteRune(r)
			}
		}
		flush()
	}
	return pieces
}

// measureRuns wraps the run pieces into lines of positioned fragments and reports
// the widest line and total height. Line height and ascent are uniform across the
// paragraph (the max over all runs), so pagination can split by line count.
func (t *textResolve) measureRuns(availW float64) flexbox.Size {
	pieces := runPieces(t.runs)
	// Uniform paragraph metrics from the widest/tallest run.
	lineH, ascent := 0.0, 0.0
	for i := range t.runs {
		if h := t.runs[i].style.lineHeight; h > lineH {
			lineH = h
		}
		if a := t.runs[i].style.ascent; a > ascent {
			ascent = a
		}
	}
	if lineH == 0 {
		lineH = t.lineHeight
	}
	t.lineHeight = lineH
	t.ascent = ascent

	var lines [][]RunFragment
	var lineText []string
	var cur []RunFragment
	var curText strings.Builder
	x := 0.0
	maxW := 0.0
	flushLine := func() {
		if len(cur) == 0 {
			return
		}
		w := x
		if len(lines) == 0 {
			w += t.textIndent // the first line is pushed right by the indent
		}
		lines = append(lines, cur)
		lineText = append(lineText, curText.String())
		if w > maxW {
			maxW = w
		}
		cur = nil
		curText.Reset()
		x = 0
	}
	for _, p := range pieces {
		gap := 0.0
		if len(cur) > 0 && p.spaceBefore {
			gap = p.style.spaceWidth()
		}
		w := p.style.stringWidth(p.text)
		lineAvail := availW
		if len(lines) == 0 {
			lineAvail -= t.textIndent // first line: less room for the indent
		}
		if len(cur) > 0 && availW > 0 && x+gap+w > lineAvail {
			flushLine()
			gap = 0 // no leading space at line start
		}
		if gap > 0 {
			x += gap
			curText.WriteByte(' ')
		}
		cur = append(cur, RunFragment{
			Text:          p.text,
			X:             x,
			SpaceBefore:   gap > 0, // a widened-for-justify inter-word gap precedes this fragment
			BaseFont:      p.style.base,
			EmbeddedFont:  p.style.embedded,
			Size:          p.style.size,
			Color:         p.style.color,
			LetterSpacing: p.style.letterSpacing,
			WordSpacing:   p.style.wordSpacing,
			Underline:     p.style.underline,
			Strike:        p.style.strike,
		})
		curText.WriteString(p.text)
		x += w
	}
	flushLine()

	t.runLines = lines
	t.lines = lineText
	if len(lines) == 0 {
		return flexbox.Size{W: 0, H: lineH}
	}
	return flexbox.Size{W: maxW, H: float64(len(lines)) * lineH}
}

func resolveText(node *tree.Node, style map[string]any, media stylesheet.MediaContext, ctx stylesheet.Context, eval Evaluator, store *fontstore.Store) *textResolve {
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
	rs := resolveRunStyle(style, ctx, store)
	tr := &textResolve{
		content:       content,
		size:          rs.size,
		color:         rs.color,
		lineHeight:    rs.lineHeight,
		ascent:        rs.ascent,
		measurer:      rs.measurer,
		base:          rs.base,
		embedded:      rs.embedded,
		orphans:       propInt(node.Props, "orphans", 2),
		widows:        propInt(node.Props, "widows", 2),
		template:      template,
		callbackID:    callbackID,
		transform:     transform,
		letterSpacing: rs.letterSpacing,
		wordSpacing:   rs.wordSpacing,
		maxLines:      propInt(style, "maxLines", 0),
		ellipsis:      str(style["textOverflow"]) == "ellipsis",
		textIndent:    spacingOf(style, "textIndent", ctx),
	}

	// Inline runs: a Text containing nested styled elements (<Text>/<Link>/<Tspan>)
	// lays out as styled runs sharing lines, instead of one flattened block.
	if hasInlineRuns(node) {
		tr.runs = buildRuns(node, style, media, ctx, store)
	}
	return tr
}

// runStyle is the resolved typography for a single-style Text or one inline run.
type runStyle struct {
	measurer      fontstore.Font
	base          string
	embedded      *pdf.EmbeddedFont
	size          float64
	ascent        float64
	lineHeight    float64
	color         string
	letterSpacing float64
	wordSpacing   float64
	underline     bool
	strike        bool
}

// resolveRunStyle resolves a style map to concrete typography, preferring a
// registered custom font over the standard 14.
func resolveRunStyle(style map[string]any, ctx stylesheet.Context, store *fontstore.Store) runStyle {
	size := fontSizeOf(style, ctx)
	rs := runStyle{
		size:          size,
		color:         str(style["color"]),
		lineHeight:    lineHeightOf(style, size, ctx),
		letterSpacing: spacingOf(style, "letterSpacing", ctx),
		wordSpacing:   spacingOf(style, "wordSpacing", ctx),
	}
	rs.underline, rs.strike = decorationFlags(style)

	weight := 400
	if w, ok := stylesheet.ParseFontWeight(style["fontWeight"]); ok {
		weight = w
	}
	fstyle := fontstore.ParseStyle(str(style["fontStyle"]))
	family := fontFamilyOf(style)

	if store != nil {
		if face, ok := store.ResolveFace(family, weight, fstyle); ok {
			rs.measurer = face
			rs.embedded = face.EmbeddedFont()
			rs.ascent = face.Descriptor().Ascent / 1000 * size
			return rs
		}
	}
	if base, ok := fontstore.StandardBaseFont(family, weight, fstyle); ok {
		rs.base = base
		if m, err := afm.Load(base); err == nil {
			rs.measurer = m
			rs.ascent = m.Ascender / 1000 * size
		}
	}
	return rs
}

// decorationFlags reports underline / line-through from textDecoration (or the
// textDecorationLine longhand), which may name either or both.
func decorationFlags(style map[string]any) (underline, strike bool) {
	deco := str(style["textDecoration"])
	if deco == "" {
		deco = str(style["textDecorationLine"])
	}
	return strings.Contains(deco, "underline"), strings.Contains(deco, "line-through")
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
