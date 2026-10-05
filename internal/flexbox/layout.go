package flexbox

// Calculate lays out the tree rooted at root within the given border-box size
// (typically the page dimensions). It sets Layout on root and every descendant.
func Calculate(root *Node, width, height float64) {
	root.Layout = Layout{Left: 0, Top: 0, Width: width, Height: height}
	arrange(root)
}

// measure returns a node's border-box size given the available space. Explicit
// width/height win; otherwise leaves use their measure function and containers
// use their intrinsic (content) size. A height is always measured at the width
// the node is laid out at (see heightAt), so a Text never measures one line
// shorter than paint draws it.
func measure(n *Node, availW, availH float64) Size {
	s := &n.Style
	w, wOK := s.Width.resolve(availW)
	h, hOK := s.Height.resolve(availH)
	if len(n.Children) == 0 && n.Measure != nil {
		// A leaf is always measured, even at a fixed size: measuring is also what
		// lays a Text out into the lines paint draws. It measures its content box,
		// at its own width when that is definite. The measure function reports
		// content size; the border box adds the node's own padding and border.
		at := availW
		if wOK {
			at = w
		}
		mw, mh := contentSize(s, at, availH)
		m := n.Measure(mw, mh)
		if !wOK {
			w = m.W + s.horizEdges()
		}
		if !hOK {
			h = m.H + s.vertEdges()
		}
	} else {
		if !wOK {
			if s.Wrap != WrapNoWrap && !s.isRow() && hOK {
				w = crossContentSize(n, h-s.vertEdges(), availW, availH) + s.horizEdges()
			} else {
				w = intrinsicWidth(n, availW, availH)
			}
		}
		if !hOK {
			// Its own width when definite, else the width available to it, which an
			// auto-width child stretches to.
			at := availW
			if wOK {
				at = w
			}
			h = heightAt(n, at, availH)
		}
	}
	w, h = applyAspect(s, w, h)
	return Size{s.clampWidth(w, availW), s.clampHeight(h, availH)}
}

// heightAt returns n's border-box height when it is laid out at border-box width
// w, the width paint wraps its text at. An explicit height wins; a leaf measures
// its content box (a padded, indented Text wraps at its content width); a
// container measures its children inside its own content box.
func heightAt(n *Node, w, availH float64) float64 {
	s := &n.Style
	leaf := len(n.Children) == 0 && n.Measure != nil
	var h float64
	if leaf {
		// Measured even under an explicit height: measuring is what lays the text
		// out into lines at w, the width it is painted at.
		mw, mh := contentSize(s, w, availH)
		h = n.Measure(mw, mh).H + s.vertEdges()
	}
	if v, ok := s.Height.resolve(availH); ok {
		return s.clampHeight(v, availH)
	}
	if !leaf {
		h = intrinsicHeight(n, w, availH)
	}
	if s.AspectRatio > 0 && !s.Width.IsAuto() {
		h = w / s.AspectRatio
	}
	return s.clampHeight(h, availH)
}

// contentSize is the content box inside a w×h border box, never negative.
func contentSize(s *Style, w, h float64) (float64, float64) {
	return max(w-s.horizEdges(), 0), max(h-s.vertEdges(), 0)
}

func intrinsicWidth(n *Node, availW, availH float64) float64 {
	s := &n.Style
	edges := s.horizEdges()
	if len(n.Children) == 0 {
		return edges
	}
	if s.isRow() {
		total, cnt := 0.0, 0
		for _, c := range n.Children {
			if c.Style.isAbsolute() {
				continue
			}
			cs := measure(c, availW, availH)
			if cnt > 0 {
				total += s.Gap
			}
			total += cs.W + c.Style.MarginLeft + c.Style.MarginRight
			cnt++
		}
		return total + edges
	}
	max := 0.0
	for _, c := range n.Children {
		if c.Style.isAbsolute() {
			continue
		}
		cs := measure(c, availW, availH)
		if ow := cs.W + c.Style.MarginLeft + c.Style.MarginRight; ow > max {
			max = ow
		}
	}
	return max + edges
}

// intrinsicHeight is a container's content-driven border-box height at border-box
// width availW. Its children lay out inside its content box, so they are measured
// there; a row also flexes its line(s) first, so each child is measured at the
// width it is painted at, not at the row's full width.
func intrinsicHeight(n *Node, availW, availH float64) float64 {
	s := &n.Style
	edges := s.vertEdges()
	if len(n.Children) == 0 {
		return edges
	}
	cw, _ := contentSize(s, availW, availH)
	if s.isRow() {
		return crossContentSize(n, cw, availW, availH) + edges
	}
	total, cnt := 0.0, 0
	for _, c := range n.Children {
		if c.Style.isAbsolute() {
			continue
		}
		cs := measure(c, cw, availH)
		if cnt > 0 {
			total += s.Gap
		}
		total += cs.H + c.Style.MarginTop + c.Style.MarginBottom
		cnt++
	}
	return total + edges
}

// item holds a child's computed main/cross metrics during arrangement.
type item struct {
	node         *Node
	base         float64 // flex-basis main size
	main         float64 // resolved main size
	crossBase    float64
	mainMargin0  float64 // main-axis leading margin
	mainMargin1  float64 // main-axis trailing margin
	crossMargin0 float64
	crossMargin1 float64
}

// flowItems measures n's in-flow (non-absolute) children and captures their
// main/cross metrics. Children are measured against (refW, refH) so that
// width/height percentages resolve against the correct axis regardless of
// flex-direction; mainAvail is the reference for flex-basis.
func flowItems(n *Node, refW, refH, mainAvail float64) []item {
	s := &n.Style
	row := s.isRow()
	items := make([]item, 0, len(n.Children))
	for _, c := range n.Children {
		if c.Style.isAbsolute() {
			continue
		}
		cs := measure(c, refW, refH)
		cst := &c.Style
		it := item{node: c}
		if row {
			it.base = cs.W
			it.crossBase = cs.H
			it.mainMargin0, it.mainMargin1 = cst.MarginLeft, cst.MarginRight
			it.crossMargin0, it.crossMargin1 = cst.MarginTop, cst.MarginBottom
		} else {
			it.base = cs.H
			it.crossBase = cs.W
			it.mainMargin0, it.mainMargin1 = cst.MarginTop, cst.MarginBottom
			it.crossMargin0, it.crossMargin1 = cst.MarginLeft, cst.MarginRight
		}
		if bv, ok := cst.Basis.resolve(mainAvail); ok {
			it.base = bv
		}
		it.main = it.base
		items = append(items, it)
	}
	return items
}

// flexLine is one wrap line: the half-open item range items[start:end] plus
// the line's cross-axis extent.
type flexLine struct {
	start, end int
	cross      float64
}

// breakLines splits items into wrap lines. NoWrap yields a single line; the
// wrap modes start a new line whenever the next item's outer base size (plus
// the main-axis gap) would overflow mainAvail.
func breakLines(s *Style, items []item, mainAvail float64) []flexLine {
	if len(items) == 0 {
		return nil
	}
	if s.Wrap == WrapNoWrap {
		return []flexLine{{start: 0, end: len(items)}}
	}
	var lines []flexLine
	start, used := 0, 0.0
	for i := range items {
		outer := items[i].base + items[i].mainMargin0 + items[i].mainMargin1
		if i > start && used+s.Gap+outer > mainAvail+1e-9 {
			lines = append(lines, flexLine{start: start, end: i})
			start, used = i, outer
			continue
		}
		if i > start {
			used += s.Gap
		}
		used += outer
	}
	return append(lines, flexLine{start: start, end: len(items)})
}

// lineCross returns a line's natural cross size: the largest outer cross size
// (measured cross size plus cross margins) among its items.
func lineCross(items []item, l flexLine) float64 {
	max := 0.0
	for i := l.start; i < l.end; i++ {
		if oc := items[i].crossBase + items[i].crossMargin0 + items[i].crossMargin1; oc > max {
			max = oc
		}
	}
	return max
}

// crossContentSize returns a container's cross-axis content size for a main-axis
// content size of mainAvail: the summed cross sizes of its lines (one, unless it
// wraps) plus the gap between lines, each line resolved as arrange will lay it
// out. availW/availH is the space the container itself is being measured within.
func crossContentSize(n *Node, mainAvail, availW, availH float64) float64 {
	s := &n.Style
	refW, refH := availW, mainAvail
	if s.isRow() {
		refW, refH = mainAvail, availH
	}
	items := flowItems(n, refW, refH, mainAvail)
	total := 0.0
	for li, l := range breakLines(s, items, mainAvail) {
		if li > 0 {
			total += s.Gap
		}
		resolveLine(s, items[l.start:l.end], mainAvail, refH)
		total += lineCross(items, l)
	}
	return total
}

// resolveLine resolves one line's main sizes: grow or shrink by the line's free
// space, then clamp each item to its own min/max (a one-shot clamp; space freed by
// a clamped item is not redistributed). It returns the free space left for
// justify-content. In a row it then measures each item's cross size at its
// resolved width: a Text in a flexed cell wraps at the width the cell gets, so the
// line is as tall as what paint draws, not one line per cell. refH is the height
// the items were measured against.
func resolveLine(s *Style, line []item, mainAvail, refH float64) float64 {
	row := s.isRow()
	var totalBase, totalGrow, totalShrinkScaled float64
	for i := range line {
		totalBase += line[i].base + line[i].mainMargin0 + line[i].mainMargin1
		totalGrow += line[i].node.Style.Grow
		totalShrinkScaled += line[i].node.Style.Shrink * line[i].base
	}

	totalGap := 0.0
	if len(line) > 1 {
		totalGap = s.Gap * float64(len(line)-1)
	}
	free := mainAvail - totalBase - totalGap

	switch {
	case free > 0 && totalGrow > 0:
		for i := range line {
			line[i].main = line[i].base + line[i].node.Style.Grow/totalGrow*free
		}
		free = 0
	case free < 0 && totalShrinkScaled > 0:
		for i := range line {
			scaled := line[i].node.Style.Shrink * line[i].base
			line[i].main = line[i].base + scaled/totalShrinkScaled*free
			if line[i].main < 0 {
				line[i].main = 0
			}
		}
		free = 0
	}

	for i := range line {
		st := &line[i].node.Style
		if row {
			line[i].main = st.clampWidth(line[i].main, mainAvail)
			line[i].crossBase = heightAt(line[i].node, line[i].main, refH)
		} else {
			line[i].main = st.clampHeight(line[i].main, mainAvail)
		}
	}
	return free
}

// alignContentOffsets distributes positive leftover cross space among wrap
// lines per align-content: it returns the leading offset and the extra gap
// between lines, growing each line's cross size in place for AlignStretch.
// The zero value (AlignAuto) packs lines at the cross start, like flex-start.
func alignContentOffsets(a Align, free float64, lines []flexLine) (lead, gap float64) {
	if free <= 0 || len(lines) == 0 {
		return 0, 0
	}
	switch a {
	case AlignStretch:
		extra := free / float64(len(lines))
		for li := range lines {
			lines[li].cross += extra
		}
	case AlignCenter:
		return free / 2, 0
	case AlignFlexEnd:
		return free, 0
	case AlignSpaceBetween:
		if len(lines) > 1 {
			return 0, free / float64(len(lines)-1)
		}
	case AlignSpaceAround:
		return free / (2 * float64(len(lines))), free / float64(len(lines))
	}
	return 0, 0
}

// arrange positions n's children within n's content box (n.Layout must be set).
func arrange(n *Node) {
	s := &n.Style
	if len(n.Children) == 0 {
		return
	}
	row := s.isRow()
	originX := s.BorderLeft + s.PaddingLeft
	originY := s.BorderTop + s.PaddingTop
	contentW := n.Layout.Width - s.horizEdges()
	contentH := n.Layout.Height - s.vertEdges()

	mainAvail, crossAvail := contentH, contentW
	if row {
		mainAvail, crossAvail = contentW, contentH
	}

	// Absolutely-positioned children are laid out separately (out of flow).
	items := flowItems(n, contentW, contentH, mainAvail)
	lines := breakLines(s, items, mainAvail)
	// Resolve every line's main sizes first: a row's items are then measured at
	// their resolved widths, which the lines' cross sizes below depend on.
	free := make([]float64, len(lines))
	for li, l := range lines {
		free[li] = resolveLine(s, items[l.start:l.end], mainAvail, contentH)
	}

	// Resolve each line's cross extent and offset. A single non-wrapped line
	// spans the whole cross axis, reducing to the classic single-line layout;
	// wrapped lines size to their largest item, stack along the cross axis
	// separated by the gap, and align-content places any leftover cross space.
	offsets := make([]float64, len(lines))
	if s.Wrap == WrapNoWrap {
		if len(lines) == 1 {
			lines[0].cross = crossAvail
		}
	} else {
		sum := 0.0
		for li := range lines {
			lines[li].cross = lineCross(items, lines[li])
			sum += lines[li].cross
		}
		if len(lines) > 1 {
			sum += s.Gap * float64(len(lines)-1)
		}
		lead, gapExtra := alignContentOffsets(s.AlignContent, crossAvail-sum, lines)
		cursor := lead
		for k := range lines {
			li := k
			if s.Wrap == WrapReverse { // stack the lines in reverse order
				li = len(lines) - 1 - k
			}
			if k > 0 {
				cursor += s.Gap + gapExtra
			}
			offsets[li] = cursor
			cursor += lines[li].cross
		}
	}

	for li := range lines {
		l := lines[li]
		arrangeLine(n, items[l.start:l.end], l.cross, offsets[li], row, originX, originY, free[li])
	}

	// Position absolutely-positioned children against this node's content box.
	for _, c := range n.Children {
		if c.Style.isAbsolute() {
			layoutAbsolute(c, originX, originY, contentW, contentH)
		}
	}
}

// arrangeLine lays out one line of flow items whose main sizes resolveLine has
// already resolved, leaving free main-axis space: justify-content distributes the
// items along the main axis, and each aligns within the line's cross extent, which
// starts at crossOffset into the container's cross axis.
func arrangeLine(n *Node, line []item, cross, crossOffset float64, row bool, originX, originY, free float64) {
	s := &n.Style

	lead, gapExtra := justifyOffsets(s.Justify, free, len(line))

	cursor := lead
	for i := range line {
		it := &line[i]
		if i > 0 {
			cursor += s.Gap + gapExtra
		}
		mainStart := cursor + it.mainMargin0

		align := it.node.Style.AlignSelf
		if align == AlignAuto {
			align = s.AlignItems
		}
		if align == AlignAuto {
			align = AlignStretch
		}
		crossFree := cross - it.crossMargin0 - it.crossMargin1
		var crossPos, crossSize float64
		switch align {
		case AlignStretch:
			if crossDimAuto(it.node, row) {
				crossSize = crossFree
			} else {
				crossSize = it.crossBase
			}
			crossPos = it.crossMargin0
		case AlignCenter:
			crossSize = it.crossBase
			crossPos = it.crossMargin0 + (crossFree-crossSize)/2
		case AlignFlexEnd:
			crossSize = it.crossBase
			crossPos = it.crossMargin0 + (crossFree - crossSize)
		default: // AlignFlexStart
			crossSize = it.crossBase
			crossPos = it.crossMargin0
		}
		if row {
			crossSize = it.node.Style.clampHeight(crossSize, cross)
		} else {
			crossSize = it.node.Style.clampWidth(crossSize, cross)
		}
		crossPos += crossOffset

		if row {
			it.node.Layout = Layout{Left: originX + mainStart, Top: originY + crossPos, Width: it.main, Height: crossSize}
		} else {
			it.node.Layout = Layout{Left: originX + crossPos, Top: originY + mainStart, Width: crossSize, Height: it.main}
		}

		cursor += it.mainMargin0 + it.main + it.mainMargin1
		arrange(it.node)
	}
}

// layoutAbsolute positions an out-of-flow child relative to its parent's content
// box (origin originX/originY, size contentW×contentH), using width/height and
// the top/right/bottom/left insets.
func layoutAbsolute(n *Node, originX, originY, contentW, contentH float64) {
	s := &n.Style

	left, leftOK := s.Left.resolve(contentW)
	right, rightOK := s.Right.resolve(contentW)
	w, wOK := s.Width.resolve(contentW)
	if !wOK {
		if leftOK && rightOK {
			w = contentW - left - right
		} else {
			w = measure(n, contentW, contentH).W
		}
	}
	var x float64
	switch {
	case leftOK:
		x = left
	case rightOK:
		x = contentW - right - w
	}

	top, topOK := s.Top.resolve(contentH)
	bottom, bottomOK := s.Bottom.resolve(contentH)
	h, hOK := s.Height.resolve(contentH)
	if !hOK {
		if topOK && bottomOK {
			h = contentH - top - bottom
		} else {
			h = measure(n, contentW, contentH).H
		}
	}
	var y float64
	switch {
	case topOK:
		y = top
	case bottomOK:
		y = contentH - bottom - h
	}

	n.Layout = Layout{Left: originX + x, Top: originY + y, Width: w, Height: h}
	arrange(n)
}

// applyAspect derives the auto dimension from the definite one using the aspect
// ratio (width/height), when exactly one dimension is definite.
func applyAspect(s *Style, w, h float64) (float64, float64) {
	if s.AspectRatio <= 0 {
		return w, h
	}
	wDef := !s.Width.IsAuto()
	hDef := !s.Height.IsAuto()
	switch {
	case wDef && !hDef:
		return w, w / s.AspectRatio
	case hDef && !wDef:
		return h * s.AspectRatio, h
	}
	return w, h
}

func crossDimAuto(n *Node, row bool) bool {
	if row {
		return n.Style.Height.IsAuto()
	}
	return n.Style.Width.IsAuto()
}

// justifyOffsets returns the leading offset and the extra gap between items for
// distributing positive free space per justify-content. When free space is not
// positive, items pack from the start (and may overflow).
func justifyOffsets(j Justify, free float64, n int) (lead, gap float64) {
	if free <= 0 || n == 0 {
		return 0, 0
	}
	switch j {
	case JustifyFlexEnd:
		return free, 0
	case JustifyCenter:
		return free / 2, 0
	case JustifySpaceBetween:
		if n > 1 {
			return 0, free / float64(n-1)
		}
		return 0, 0
	case JustifySpaceAround:
		return free / (2 * float64(n)), free / float64(n)
	case JustifySpaceEvenly:
		e := free / float64(n+1)
		return e, e
	default: // JustifyFlexStart
		return 0, 0
	}
}
