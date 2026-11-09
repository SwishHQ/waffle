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
	pageTopFlow := 0.0 // flow offset mapped to the top of the current page's content

	for len(remaining) > 0 {
		boundary := contentTop + pageTopFlow + availH
		var fit, rest []*Box
		for _, m := range []splitMode{
			{pageH: availH, keepWhole: true, headers: true},
			// Keeping boxes whole left this page empty: one already starts at the top
			// (or under its parents' padding there) and still runs past the page, so
			// deferring it again cannot help. Split it like any other box.
			{pageH: availH, headers: true},
			// Still empty: a repeated header and the content under it overflow a page
			// together. Stop repeating it so that content can break.
			{pageH: availH},
		} {
			if fit, rest, _ = splitFlow(remaining, boundary, m); len(fit) > 0 {
				break
			}
		}

		if len(fit) == 0 {
			// Nothing fit (a single item taller than the page): place it whole and
			// let it overflow, to guarantee progress.
			if len(rest) == 0 {
				break
			}
			fit, rest = rest[:1], rest[1:]
		}

		for _, ch := range fit {
			shiftBox(ch, -pageTopFlow)
		}
		rc := *root
		rc.Children = fit
		out = append(out, &Page{Width: p.Width, Height: p.Height, Root: &rc})

		if len(rest) == 0 {
			break
		}
		pageTopFlow = rest[0].Frame.Y - contentTop
		remaining = rest
	}
	return out
}

// splitMode is one attempt at filling a page.
type splitMode struct {
	pageH     float64 // a whole page's flow height
	keepWhole bool    // move wrap={false} and nested fixed boxes whole when a fresh page holds them
	headers   bool    // repeat a split View's fixed children atop its continuation
}

// splitFlow partitions a sequence of sibling boxes at the vertical boundary,
// splitting the first straddling child if it can be split. With keepWhole, a
// box kept whole (wrap={false}, or a nested fixed header) no taller than pageH
// moves whole to the next page instead of splitting. grow is how much taller the
// rest runs than the boxes it continues (repeated headers), already applied to
// the siblings below the split child.
func splitFlow(boxes []*Box, boundary float64, m splitMode) (fit, rest []*Box, grow float64) {
	for idx, ch := range boxes {
		if len(fit) > 0 && ch.Node != nil {
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

// splitViewBox splits a View between its children. With headers on, the fixed
// children a column View placed on this page are its header: they stay in the
// flow here, and the continuation starts with deep clones of them, pushing its
// content down to make room (as react-pdf repeats a table's header row). A
// header never ends a page with nothing under it; it moves on with its content.
func splitViewBox(b *Box, boundary float64, m splitMode) (fit, rest *Box, grow float64) {
	fitKids, restKids, grow := splitFlow(b.Children, boundary, m)
	if len(fitKids) == 0 {
		return nil, b, 0
	}
	if len(restKids) == 0 {
		return b, nil, 0
	}
	headers := m.headers && stacksVertically(b)
	if headers {
		for n := len(fitKids); n > 0 && isHeader(b, fitKids, n-1); n-- {
			restKids = append([]*Box{fitKids[n-1]}, restKids...)
			fitKids = fitKids[:n-1]
		}
		if len(fitKids) == 0 {
			return nil, b, 0
		}
	}

	fitBox := *b
	fitBox.Children = fitKids
	fitBox.Frame.H = boundary - b.Frame.Y

	restTop := restKids[0].Frame.Y
	var kids []*Box
	if headers {
		y := restTop
		for i := range fitKids {
			if isHeader(b, fitKids, i) {
				h := cloneBox(fitKids[i])
				shiftBox(h, y-h.Frame.Y)
				kids = append(kids, h)
				y += headerAdvance(b, i)
			}
		}
		if headerH := y - restTop; headerH > 0 {
			for _, k := range restKids {
				shiftBox(k, headerH)
			}
			grow += headerH
		}
	}

	restBox := *b
	restBox.Children = append(kids, restKids...)
	restBox.Frame.Y = restTop
	restBox.Frame.H = (b.Frame.Y + b.Frame.H) - restTop + grow
	return &fitBox, &restBox, grow
}

// isHeader reports whether fitKids[i] is a header of b: an in-flow fixed child
// placed whole on this page (a split child is a new box, not b.Children[i]).
func isHeader(b *Box, fitKids []*Box, i int) bool {
	k := fitKids[i]
	return k == b.Children[i] && k.Node != nil && k.Node.Fixed && !absolute(k)
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
