package layout

import "github.com/SwishHQ/waffle/internal/tree"

// paginate splits a laid-out page into output pages, filling each page from the
// flow and splitting a straddling child at a line (Text) or child (View)
// boundary. Unbreakable content (a leaf, or an element with wrap={false} that a
// fresh page can hold) is moved whole; anything taller than a whole page is
// split, or placed whole when it cannot be split. A View that splits repeats its
// fixed children atop each continuation (see splitViewBox). Each output page
// keeps the page's background and padding. When the page's own wrap is false the
// page is never split.
func paginate(p *Page, contentTop, availH float64, wrap bool) []*Page {
	root := p.Root

	// Separate fixed elements (repeated on every page) from the normal flow.
	var fixed, flow []*Box
	for _, ch := range root.Children {
		if ch.Node != nil && ch.Node.Fixed {
			fixed = append(fixed, ch)
		} else {
			flow = append(flow, ch)
		}
	}

	var out []*Page
	if !wrap || availH <= 0 || len(flow) == 0 {
		rc := *root
		rc.Children = flow
		out = []*Page{{Width: p.Width, Height: p.Height, Root: &rc}}
	} else {
		out = paginateFlow(p, root, flow, contentTop, availH)
	}

	// Append fixed elements to every page. They are deep-cloned per page so that
	// per-page render-prop substitution (page numbers) stays independent.
	if len(fixed) > 0 {
		for _, pg := range out {
			kids := make([]*Box, 0, len(pg.Root.Children)+len(fixed))
			kids = append(kids, pg.Root.Children...)
			for _, fx := range fixed {
				kids = append(kids, cloneBox(fx))
			}
			pg.Root.Children = kids
		}
	}
	return out
}

// cloneBox deep-copies a box subtree, duplicating Text so per-page mutation
// (render-prop substitution) does not alias across pages.
func cloneBox(b *Box) *Box {
	c := *b
	if b.Text != nil {
		tc := *b.Text
		tc.Lines = append([]string(nil), b.Text.Lines...)
		c.Text = &tc
	}
	c.Children = nil
	for _, ch := range b.Children {
		c.Children = append(c.Children, cloneBox(ch))
	}
	return &c
}

func paginateFlow(p *Page, root *Box, flow []*Box, contentTop, availH float64) []*Page {
	var out []*Page
	remaining := flow
	top := contentTop // the flow y drawn at the top of the current page's content

	for len(remaining) > 0 {
		fit, rest, pageTop := fillPage(remaining, top, availH)
		for _, ch := range fit {
			shiftBox(ch, contentTop-pageTop)
		}
		rc := *root
		rc.Children = fit
		out = append(out, &Page{Width: p.Width, Height: p.Height, Root: &rc})

		if len(rest) == 0 {
			break
		}
		top = rest[0].Frame.Y
		remaining = rest
	}
	return out
}

// fillPage picks what goes on a page whose content starts at flow y top and is
// availH tall, returning the flow y it drew at that top. It tries each of these
// in turn until one places something:
//
//  1. keep wrap={false} and fixed boxes whole;
//  2. as 1, without the header copies that open the page: a header and the row
//     under it cannot share a page, so the row goes alone and the header returns
//     on the next page;
//  3. split boxes kept whole: one starts at the top and still runs past it;
//  4. as 3, without the header copies;
//  5. let a View's own header end the page with no row under it.
//
// If none does, it places the first item whole and lets it overflow.
func fillPage(remaining []*Box, top, availH float64) (fit, rest []*Box, pageTop float64) {
	type try struct {
		boxes []*Box
		top   float64
		m     splitMode
	}
	whole := splitMode{pageH: availH, keepWhole: true}
	split := splitMode{pageH: availH}
	bare, bareTop := remaining, top
	tries := []try{{remaining, top, whole}}
	if d := dropCopies(remaining[0]); d != nil {
		bare = append([]*Box{d}, remaining[1:]...)
		bareTop = d.Frame.Y
		tries = append(tries, try{bare, bareTop, whole}, try{remaining, top, split}, try{bare, bareTop, split})
	} else {
		tries = append(tries, try{remaining, top, split})
	}
	tries = append(tries, try{bare, bareTop, splitMode{pageH: availH, loneHeader: true}})
	for _, t := range tries {
		if fit, rest, _ = splitFlow(t.boxes, t.top+availH, t.m); len(fit) > 0 {
			return fit, rest, t.top
		}
	}
	return bare[:1], bare[1:], bareTop
}

// dropCopies returns b without the header copies that open it (or open its
// first child, recursively), keeping them to repeat on its next continuation.
// It is nil when b opens with none. b itself is left untouched.
func dropCopies(b *Box) *Box {
	k := 0
	for k < len(b.Children) && b.Children[k].copied {
		k++
	}
	if k == len(b.Children) {
		return nil
	}
	first := dropCopies(b.Children[k])
	if k == 0 && first == nil {
		return nil
	}
	d := *b
	d.Children = append([]*Box(nil), b.Children[k:]...)
	if first != nil {
		d.Children[0] = first
	}
	d.pending = append(append([]*Box(nil), b.pending...), b.Children[:k]...)
	d.Frame.Y = d.Children[0].Frame.Y
	d.Frame.H = b.Frame.Y + b.Frame.H - d.Frame.Y
	return &d
}

// splitMode is one attempt at filling a page.
type splitMode struct {
	pageH      float64 // a whole page's flow height
	keepWhole  bool    // move wrap={false} and nested fixed boxes whole when a fresh page holds them
	loneHeader bool    // let a View's own header end a page with no row under it
}

// splitFlow partitions a sequence of sibling boxes at the vertical boundary,
// splitting the first straddling child if it can be split. A forced break or
// minPresenceAhead ends the page only after in-flow content other than
// headers: a break on a View's first row is ignored, as on its first child.
// With keepWhole, a
// box kept whole (wrap={false}, or a nested fixed header) no taller than pageH
// moves whole to the next page instead of splitting. grow is how much taller the
// rest runs than the boxes it continues (repeated headers), already applied to
// the siblings below the split child.
func splitFlow(boxes []*Box, boundary float64, m splitMode) (fit, rest []*Box, grow float64) {
	placed := false // fit holds in-flow content besides headers
	for idx, ch := range boxes {
		if placed && ch.Node != nil {
			// A forced break ends the current page before this element.
			if ch.Node.Break {
				rest = append(rest, boxes[idx:]...)
				return fit, rest, 0
			}
			// minPresenceAhead: if the element fits but leaves less than N points
			// of room after it, break before it so it stays with following content.
			if mpa := ch.Node.MinPresenceAhead; mpa > 0 {
				bottom := ch.Frame.Y + ch.Frame.H
				if bottom <= boundary+1e-6 && bottom+mpa > boundary+1e-6 {
					rest = append(rest, boxes[idx:]...)
					return fit, rest, 0
				}
			}
		}
		if ch.Frame.Y+ch.Frame.H <= boundary+1e-6 {
			fit = append(fit, ch)
			placed = placed || (!header(ch) && !absolute(ch))
			continue
		}
		// This child does not fully fit. It moves whole if it starts past the
		// boundary, or if it is kept whole and a fresh page can hold it; one taller
		// than a page still splits, since nowhere could keep it whole.
		if ch.Frame.Y >= boundary-1e-6 || (m.keepWhole && keptWhole(ch) && ch.Frame.H <= m.pageH+1e-6) {
			rest = append(rest, boxes[idx:]...)
			return fit, rest, 0
		}
		cf, cr, g := splitBox(ch, boundary, m)
		if cf != nil {
			fit = append(fit, cf)
		}
		if cr != nil {
			rest = append(rest, cr)
		}
		if g > 0 {
			// The continuation repeats a header, so it runs taller than the child it
			// continues: push down what sat below that child.
			bottom := ch.Frame.Y + ch.Frame.H
			for _, s := range boxes[idx+1:] {
				if s.Frame.Y >= bottom-1e-6 {
					shiftBox(s, g)
				}
			}
		}
		rest = append(rest, boxes[idx+1:]...)
		return fit, rest, g
	}
	return fit, rest, 0
}

// splitBox splits a box that straddles the boundary into a part that stays on
// the page and a part that continues on the next. It returns (box, nil) if it
// wholly fits, (nil, box) if it cannot be split, or both parts on a real split;
// grow is how much taller the continuation is than the part of box it carries.
func splitBox(b *Box, boundary float64, m splitMode) (fit, rest *Box, grow float64) {
	switch {
	case b.Text != nil && len(b.Text.Lines) > 1:
		fit, rest = splitTextBox(b, boundary)
		return fit, rest, 0
	case len(b.Children) > 0:
		return splitViewBox(b, boundary, m)
	default:
		return nil, b, 0 // unbreakable leaf
	}
}

// keptWhole reports whether a box moves to the next page whole rather than
// split: it opted out with wrap={false}, or it is a fixed box inside a View (a
// header that repeats). Page-level fixed boxes never reach pagination's flow.
func keptWhole(b *Box) bool {
	return b.Node != nil && (b.Node.Fixed || (b.Node.Wrap != nil && !*b.Node.Wrap))
}

func splitTextBox(b *Box, boundary float64) (fit, rest *Box) {
	t := b.Text
	nFit := 0
	for i := range t.Lines {
		if b.Frame.Y+float64(i+1)*t.LineHeight <= boundary+1e-6 {
			nFit = i + 1
		} else {
			break
		}
	}
	total := len(t.Lines)
	if nFit == 0 {
		return nil, b
	}
	if nFit >= total {
		return b, nil
	}

	// orphans/widows: keep at least `orphans` lines on this page and carry at
	// least `widows` to the next; if that can't be satisfied, move the whole Text.
	orphans, widows := t.Orphans, t.Widows
	if orphans < 1 {
		orphans = 1
	}
	if widows < 1 {
		widows = 1
	}
	if total-nFit < widows {
		nFit = total - widows
	}
	if nFit < orphans {
		return nil, b
	}

	fitBox, fitText := *b, *t
	fitText.Lines = t.Lines[:nFit]
	fitBox.Text = &fitText
	fitBox.Frame.H = float64(nFit) * t.LineHeight

	restBox, restText := *b, *t
	restText.Lines = t.Lines[nFit:]
	restBox.Text = &restText
	restBox.Frame.Y = b.Frame.Y + float64(nFit)*t.LineHeight
	restBox.Frame.H = float64(len(t.Lines)-nFit) * t.LineHeight

	// Keep inline run fragments aligned with the split line ranges.
	if len(t.RunLines) == len(t.Lines) {
		fitText.RunLines = t.RunLines[:nFit]
		restText.RunLines = t.RunLines[nFit:]
	}
	return &fitBox, &restBox
}

// splitViewBox splits a View between its children. The fixed in-flow children
// a column View places on this page are its header: they stay in the flow here,
// and the continuation opens with copies of them at the same inset, pushing its
// content down to make room (as react-pdf repeats a table's header row). A
// header never ends a page with nothing under it, unless m.loneHeader allows
// the View's own header to; it moves on with the rows it heads.
func splitViewBox(b *Box, boundary float64, m splitMode) (fit, rest *Box, grow float64) {
	fitKids, restKids, grow := splitFlow(b.Children, boundary, m)
	if len(fitKids) == 0 {
		return nil, b, 0
	}
	if len(restKids) == 0 {
		return b, nil, 0
	}
	repeats := stacksVertically(b)
	if repeats && !m.loneHeader {
		// Out-of-flow siblings after the header are not rows: they move with it.
		n := len(fitKids)
		for n > 0 && (isHeader(b, fitKids, n-1) || absolute(fitKids[n-1])) {
			n--
		}
		for i := n; i < len(fitKids); i++ {
			if isHeader(b, fitKids, i) {
				restKids = append(append([]*Box(nil), fitKids[n:]...), restKids...)
				fitKids = fitKids[:n]
				break
			}
		}
	}
	if allCopies(fitKids) {
		return nil, b, 0
	}

	fitBox := *b
	fitBox.Children = fitKids
	fitBox.Frame.H = boundary - b.Frame.Y
	fitBox.pending = nil

	restTop := restKids[0].Frame.Y
	var heads []*Box
	if repeats {
		y := restTop + b.edgeTop
		add := func(h *Box, advance float64) {
			if len(heads) == 0 {
				y += h.marginTop
			}
			c := cloneBox(h)
			c.copied, c.advance, c.pending = true, advance, nil
			shiftBox(c, y-c.Frame.Y)
			heads = append(heads, c)
			y += advance
		}
		for _, h := range b.pending {
			add(h, h.advance)
		}
		for i := range fitKids {
			if isHeader(b, fitKids, i) {
				add(fitKids[i], headerAdvance(b, i))
			}
		}
		if len(heads) > 0 {
			headerH := y - restTop
			for _, k := range restKids {
				shiftBox(k, headerH)
			}
			grow += headerH
		}
	}

	restBox := *b
	restBox.Children = append(heads, restKids...)
	restBox.pending = nil
	restBox.Frame.Y = restTop
	restBox.Frame.H = (b.Frame.Y + b.Frame.H) - restTop + grow
	return &fitBox, &restBox, grow
}

// header reports whether a box is a fixed in-flow box inside a View, or a copy
// of one: the header of the rows after it.
func header(b *Box) bool {
	return b.copied || (b.Node != nil && b.Node.Fixed && !absolute(b))
}

// isHeader reports whether fitKids[i] is a header of b: an in-flow fixed child
// placed whole on this page (a split child is a new box, not b.Children[i]).
func isHeader(b *Box, fitKids []*Box, i int) bool {
	k := fitKids[i]
	return k == b.Children[i] && k.Node != nil && k.Node.Fixed && !absolute(k)
}

// allCopies reports whether boxes holds nothing but header copies, which a page
// must not hold alone: the same copies would open the next page too.
func allCopies(boxes []*Box) bool {
	for _, k := range boxes {
		if !k.copied {
			return false
		}
	}
	return true
}

// headerAdvance is how far header child i of b pushes the content after it: from
// its top to the top of the next in-flow sibling, so its margins and the View's
// gap come along.
func headerAdvance(b *Box, i int) float64 {
	h := b.Children[i]
	for _, s := range b.Children[i+1:] {
		if !absolute(s) {
			return max(s.Frame.Y-h.Frame.Y, 0)
		}
	}
	return h.Frame.H
}

// stacksVertically reports whether a View lays its children out top to bottom,
// the only direction in which a repeated header sits above its content.
func stacksVertically(b *Box) bool {
	d := str(b.Style["flexDirection"])
	return d == "" || d == "column"
}

func absolute(b *Box) bool {
	return str(b.Style["position"]) == "absolute"
}

// shiftBox translates a box and its descendants vertically.
func shiftBox(b *Box, dy float64) {
	b.Frame.Y += dy
	for _, c := range b.Children {
		shiftBox(c, dy)
	}
}

// pageWraps reports whether a Page node paginates (the default) or clips to a
// single page.
func pageWraps(n *tree.Node) bool {
	return n.Wrap == nil || *n.Wrap
}
