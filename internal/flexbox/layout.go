package flexbox

// Calculate lays out the tree rooted at root within the given border-box size
// (typically the page dimensions). It sets Layout on root and every descendant.
func Calculate(root *Node, width, height float64) {
	root.Layout = Layout{Left: 0, Top: 0, Width: width, Height: height}
	arrange(root)
}

// measure returns a node's border-box size given the available space. Explicit
// width/height win; otherwise leaves use their measure function and containers
// use their intrinsic (content) size.
func measure(n *Node, availW, availH float64) Size {
	s := &n.Style
	if len(n.Children) == 0 && n.Measure != nil {
		m := n.Measure(availW, availH)
		// The measure function reports content size; the border-box adds the
		// node's own padding and border (unless an explicit size overrides).
		w, h := m.W+s.horizEdges(), m.H+s.vertEdges()
		if v, ok := s.Width.resolve(availW); ok {
			w = v
		}
		if v, ok := s.Height.resolve(availH); ok {
			h = v
		}
		w, h = applyAspect(s, w, h)
		return Size{w, h}
	}
	w, ok := s.Width.resolve(availW)
	if !ok {
		w = intrinsicWidth(n, availW, availH)
	}
	h, ok := s.Height.resolve(availH)
	if !ok {
		h = intrinsicHeight(n, availW, availH)
	}
	w, h = applyAspect(s, w, h)
	return Size{w, h}
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

func intrinsicHeight(n *Node, availW, availH float64) float64 {
	s := &n.Style
	edges := s.vertEdges()
	if len(n.Children) == 0 {
		return edges
	}
	if !s.isRow() {
		total, cnt := 0.0, 0
		for _, c := range n.Children {
			if c.Style.isAbsolute() {
				continue
			}
			cs := measure(c, availW, availH)
			if cnt > 0 {
				total += s.Gap
			}
			total += cs.H + c.Style.MarginTop + c.Style.MarginBottom
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
		if oh := cs.H + c.Style.MarginTop + c.Style.MarginBottom; oh > max {
			max = oh
		}
	}
	return max + edges
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
	flow := n.Children[:0:0]
	for _, c := range n.Children {
		if !c.Style.isAbsolute() {
			flow = append(flow, c)
		}
	}

	items := make([]item, len(flow))
	var totalBase, totalGrow, totalShrinkScaled float64
	for i, c := range flow {
		// measure always takes (contentW, contentH) so width/height percentages
		// resolve against the correct axis regardless of flex-direction.
		cs := measure(c, contentW, contentH)
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
		items[i] = it
		totalBase += it.base + it.mainMargin0 + it.mainMargin1
		totalGrow += cst.Grow
		totalShrinkScaled += cst.Shrink * it.base
	}

	totalGap := 0.0
	if len(items) > 1 {
		totalGap = s.Gap * float64(len(items)-1)
	}
	free := mainAvail - totalBase - totalGap

	switch {
	case free > 0 && totalGrow > 0:
		for i := range items {
			items[i].main = items[i].base + items[i].node.Style.Grow/totalGrow*free
		}
		free = 0
	case free < 0 && totalShrinkScaled > 0:
		for i := range items {
			scaled := items[i].node.Style.Shrink * items[i].base
			items[i].main = items[i].base + scaled/totalShrinkScaled*free
			if items[i].main < 0 {
				items[i].main = 0
			}
		}
		free = 0
	}

	lead, gapExtra := justifyOffsets(s.Justify, free, len(items))

	cursor := lead
	for i := range items {
		it := &items[i]
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
		crossFree := crossAvail - it.crossMargin0 - it.crossMargin1
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
			it.node.Layout = Layout{Left: originX + mainStart, Top: originY + crossPos, Width: it.main, Height: crossSize}
		} else {
			it.node.Layout = Layout{Left: originX + crossPos, Top: originY + mainStart, Width: crossSize, Height: it.main}
		}

		cursor += it.mainMargin0 + it.main + it.mainMargin1
		arrange(it.node)
	}

	// Position absolutely-positioned children against this node's content box.
	for _, c := range n.Children {
		if c.Style.isAbsolute() {
			layoutAbsolute(c, originX, originY, contentW, contentH)
		}
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
