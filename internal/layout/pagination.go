package layout

import "github.com/SwishHQ/waffle/internal/tree"

// paginate splits a laid-out page into output pages, filling each page from the
// flow and splitting a straddling child at a line (Text) or child (View)
// boundary. Unbreakable content (a leaf, or an element with wrap={false} that a
// fresh page can hold) is moved whole; anything taller than a whole page is
// split, or placed whole when it cannot be split. Each output page keeps the
// page's background and padding. When the page's own wrap is false the page is
// never split.
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
		fit, rest := splitFlow(remaining, boundary, availH, true)
		if len(fit) == 0 {
			// Keeping wrap={false} boxes whole left this page empty: one already starts
			// at the top (or under its parents' padding there) and still runs past the
			// page, so deferring it again cannot help. Split it like any other box.
			fit, rest = splitFlow(remaining, boundary, availH, false)
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

// splitFlow partitions a sequence of sibling boxes at the vertical boundary,
// splitting the first straddling child if it can be split. With keepWhole, an
// unbreakable (wrap={false}) child no taller than pageH, a whole page's flow
// height, moves whole to the next page instead of splitting.
func splitFlow(boxes []*Box, boundary, pageH float64, keepWhole bool) (fit, rest []*Box) {
	for idx, ch := range boxes {
		if len(fit) > 0 && ch.Node != nil {
			// A forced break ends the current page before this element.
			if ch.Node.Break {
				rest = append(rest, boxes[idx:]...)
				return fit, rest
			}
			// minPresenceAhead: if the element fits but leaves less than N points
			// of room after it, break before it so it stays with following content.
			if mpa := ch.Node.MinPresenceAhead; mpa > 0 {
				bottom := ch.Frame.Y + ch.Frame.H
				if bottom <= boundary+1e-6 && bottom+mpa > boundary+1e-6 {
					rest = append(rest, boxes[idx:]...)
					return fit, rest
				}
			}
		}
		if ch.Frame.Y+ch.Frame.H <= boundary+1e-6 {
			fit = append(fit, ch)
			continue
		}
		// This child does not fully fit. It moves whole if it starts past the
		// boundary, or if it is unbreakable and a fresh page can hold it; one taller
		// than a page still splits, since nowhere could keep it whole.
		if ch.Frame.Y >= boundary-1e-6 || (keepWhole && unbreakable(ch) && ch.Frame.H <= pageH+1e-6) {
			rest = append(rest, boxes[idx:]...)
			return fit, rest
		}
		cf, cr := splitBox(ch, boundary, pageH, keepWhole)
		if cf != nil {
			fit = append(fit, cf)
		}
		if cr != nil {
			rest = append(rest, cr)
		}
		rest = append(rest, boxes[idx+1:]...)
		return fit, rest
	}
	return fit, rest
}

// splitBox splits a box that straddles the boundary into a part that stays on
// the page and a part that continues on the next. It returns (box, nil) if it
// wholly fits, (nil, box) if it cannot be split, or both parts on a real split.
func splitBox(b *Box, boundary, pageH float64, keepWhole bool) (fit, rest *Box) {
	switch {
	case b.Text != nil && len(b.Text.Lines) > 1:
		return splitTextBox(b, boundary)
	case len(b.Children) > 0:
		return splitViewBox(b, boundary, pageH, keepWhole)
	default:
		return nil, b // unbreakable leaf
	}
}

// unbreakable reports whether a box opted out of page splitting with wrap={false}.
func unbreakable(b *Box) bool {
	return b.Node != nil && b.Node.Wrap != nil && !*b.Node.Wrap
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

func splitViewBox(b *Box, boundary, pageH float64, keepWhole bool) (fit, rest *Box) {
	fitKids, restKids := splitFlow(b.Children, boundary, pageH, keepWhole)
	if len(fitKids) == 0 {
		return nil, b
	}
	if len(restKids) == 0 {
		return b, nil
	}

	fitBox := *b
	fitBox.Children = fitKids
	fitBox.Frame.H = boundary - b.Frame.Y

	restBox := *b
	restBox.Children = restKids
	restTop := restKids[0].Frame.Y
	restBox.Frame.Y = restTop
	restBox.Frame.H = (b.Frame.Y + b.Frame.H) - restTop
	return &fitBox, &restBox
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
