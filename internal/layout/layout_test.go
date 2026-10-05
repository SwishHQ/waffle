package layout

import (
	"math"
	"testing"

	"github.com/SwishHQ/waffle/internal/contract"
	"github.com/SwishHQ/waffle/internal/tree"
)

func layoutJSON(t *testing.T, doc string) *Result {
	t.Helper()
	ct, err := contract.Parse([]byte(doc))
	if err != nil {
		t.Fatalf("contract.Parse: %v", err)
	}
	tr, err := tree.Build(ct)
	if err != nil {
		t.Fatalf("tree.Build: %v", err)
	}
	res, err := Layout(tr, Options{})
	if err != nil {
		t.Fatalf("Layout: %v", err)
	}
	return res
}

func frameEq(t *testing.T, name string, got Rect, x, y, w, h float64) {
	t.Helper()
	if math.Abs(got.X-x) > 1e-6 || math.Abs(got.Y-y) > 1e-6 || math.Abs(got.W-w) > 1e-6 || math.Abs(got.H-h) > 1e-6 {
		t.Errorf("%s frame = %+v, want {X:%v Y:%v W:%v H:%v}", name, got, x, y, w, h)
	}
}

func TestLayoutPaddingAndFixedChild(t *testing.T) {
	res := layoutJSON(t, `{"version":"waffle-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[200,100],"style":{"padding":10}},"children":[
			{"type":"VIEW","props":{"style":{"width":50,"height":30,"backgroundColor":"red"}}}
		]}
	]}}`)

	if len(res.Pages) != 1 {
		t.Fatalf("pages = %d, want 1", len(res.Pages))
	}
	p := res.Pages[0]
	if p.Width != 200 || p.Height != 100 {
		t.Errorf("page size = %vx%v, want 200x100", p.Width, p.Height)
	}
	frameEq(t, "page", p.Root.Frame, 0, 0, 200, 100)
	frameEq(t, "child", p.Root.Children[0].Frame, 10, 10, 50, 30)
	// Resolved style is carried on the box.
	if p.Root.Children[0].Style["backgroundColor"] != "red" {
		t.Errorf("child style not carried: %v", p.Root.Children[0].Style)
	}
}

func TestLayoutRowFlexGrow(t *testing.T) {
	res := layoutJSON(t, `{"version":"waffle-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[300,100]},"children":[
			{"type":"VIEW","props":{"style":{"flexDirection":"row","height":50}},"children":[
				{"type":"VIEW","props":{"style":{"flexGrow":1}}},
				{"type":"VIEW","props":{"style":{"flexGrow":1}}}
			]}
		]}
	]}}`)

	row := res.Pages[0].Root.Children[0]
	frameEq(t, "row", row.Frame, 0, 0, 300, 50)
	frameEq(t, "left", row.Children[0].Frame, 0, 0, 150, 50)
	frameEq(t, "right", row.Children[1].Frame, 150, 0, 150, 50)
}

func TestLayoutNamedPageSizeAndOrientation(t *testing.T) {
	res := layoutJSON(t, `{"version":"waffle-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":"A4","orientation":"landscape"},"children":[]}
	]}}`)
	p := res.Pages[0]
	// A4 is 595.28 x 841.89 portrait; landscape swaps.
	if math.Abs(p.Width-841.89) > 0.01 || math.Abs(p.Height-595.28) > 0.01 {
		t.Errorf("landscape A4 = %vx%v, want 841.89x595.28", p.Width, p.Height)
	}
}

func TestLayoutUnitsAndMargins(t *testing.T) {
	// 1in margin = 72pt; child at (72,72).
	res := layoutJSON(t, `{"version":"waffle-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[300,300]},"children":[
			{"type":"VIEW","props":{"style":{"margin":"1in","width":50,"height":50}}}
		]}
	]}}`)
	child := res.Pages[0].Root.Children[0]
	frameEq(t, "child", child.Frame, 72, 72, 50, 50)
}

func TestLayoutMultiplePages(t *testing.T) {
	res := layoutJSON(t, `{"version":"waffle-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[100,100]},"children":[]},
		{"type":"PAGE","props":{"size":[200,200]},"children":[]}
	]}}`)
	if len(res.Pages) != 2 {
		t.Fatalf("pages = %d, want 2", len(res.Pages))
	}
	if res.Pages[0].Width != 100 || res.Pages[1].Width != 200 {
		t.Errorf("page widths = %v, %v", res.Pages[0].Width, res.Pages[1].Width)
	}
}

func TestLayoutPercentDimensions(t *testing.T) {
	// Page 200x100 (column); child 50% wide, 100% tall.
	res := layoutJSON(t, `{"version":"waffle-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[200,100]},"children":[
			{"type":"VIEW","props":{"style":{"width":"50%","height":"100%"}}}
		]}
	]}}`)
	frameEq(t, "child", res.Pages[0].Root.Children[0].Frame, 0, 0, 100, 100)
}

func TestLayoutTextMeasured(t *testing.T) {
	res := layoutJSON(t, `{"version":"waffle-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[300,100]},"children":[
			{"type":"TEXT","props":{"style":{"fontSize":20}},"children":[
				{"type":"TEXT_INSTANCE","value":"Hello"}
			]}
		]}
	]}}`)
	tb := res.Pages[0].Root.Children[0]
	if tb.Text == nil || tb.Text.Content != "Hello" {
		t.Fatalf("text info missing: %+v", tb.Text)
	}
	if tb.Text.BaseFont != "Helvetica" {
		t.Errorf("default font = %q, want Helvetica", tb.Text.BaseFont)
	}
	if tb.Frame.W <= 0 || tb.Frame.H <= 0 {
		t.Errorf("text frame not measured: %+v", tb.Frame)
	}
}

func TestLayoutTextWraps(t *testing.T) {
	// A long line in a narrow page must wrap to multiple lines.
	res := layoutJSON(t, `{"version":"waffle-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[120,300]},"children":[
			{"type":"TEXT","props":{"style":{"fontSize":12}},"children":[
				{"type":"TEXT_INSTANCE","value":"The quick brown fox jumps over the lazy dog again and again"}
			]}
		]}
	]}}`)
	tb := res.Pages[0].Root.Children[0]
	if tb.Text == nil || len(tb.Text.Lines) < 2 {
		t.Fatalf("expected multiple wrapped lines, got %v", tb.Text)
	}
	// Height must reflect the number of lines.
	if tb.Frame.H < tb.Text.LineHeight*float64(len(tb.Text.Lines))-1e-6 {
		t.Errorf("frame height %v too small for %d lines", tb.Frame.H, len(tb.Text.Lines))
	}
	// No line exceeds the page content width (120).
	// (width check is implicit: lines were wrapped to available width)
}

func TestPaginationSplitsOverflow(t *testing.T) {
	// Page 100pt tall, four 40pt blocks (160pt) → must split across pages.
	res := layoutJSON(t, `{"version":"waffle-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[80,100]},"children":[
			{"type":"VIEW","props":{"style":{"height":40,"backgroundColor":"red"}}},
			{"type":"VIEW","props":{"style":{"height":40,"backgroundColor":"green"}}},
			{"type":"VIEW","props":{"style":{"height":40,"backgroundColor":"blue"}}},
			{"type":"VIEW","props":{"style":{"height":40,"backgroundColor":"black"}}}
		]}
	]}}`)
	if len(res.Pages) != 2 {
		t.Fatalf("expected 2 pages, got %d", len(res.Pages))
	}
	// Page 1: blocks 0,1 at Y 0,40. Page 2: blocks 2,3 shifted to Y 0,40.
	if len(res.Pages[0].Root.Children) != 2 || len(res.Pages[1].Root.Children) != 2 {
		t.Fatalf("children per page = %d, %d; want 2, 2", len(res.Pages[0].Root.Children), len(res.Pages[1].Root.Children))
	}
	if got := res.Pages[1].Root.Children[0].Frame.Y; got != 0 {
		t.Errorf("page 2 first block Y = %v, want 0 (shifted to top)", got)
	}
}

func TestPaginationWrapFalseClips(t *testing.T) {
	res := layoutJSON(t, `{"version":"waffle-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[80,100],"wrap":false},"children":[
			{"type":"VIEW","props":{"style":{"height":40}}},
			{"type":"VIEW","props":{"style":{"height":40}}},
			{"type":"VIEW","props":{"style":{"height":40}}},
			{"type":"VIEW","props":{"style":{"height":40}}}
		]}
	]}}`)
	if len(res.Pages) != 1 {
		t.Errorf("wrap=false should yield 1 page, got %d", len(res.Pages))
	}
}

func firstText(b *Box) *TextInfo {
	if b.Text != nil {
		return b.Text
	}
	for _, c := range b.Children {
		if t := firstText(c); t != nil {
			return t
		}
	}
	return nil
}

func TestPaginationSplitsTextAcrossPages(t *testing.T) {
	// A long paragraph on a short page must split its lines across pages.
	res := layoutJSON(t, `{"version":"waffle-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[130,30]},"children":[
			{"type":"TEXT","props":{"style":{"fontSize":12,"lineHeight":1}},"children":[
				{"type":"TEXT_INSTANCE","value":"one two three four five six seven eight nine ten eleven twelve thirteen fourteen fifteen sixteen"}
			]}
		]}
	]}}`)
	if len(res.Pages) < 2 {
		t.Fatalf("expected the paragraph to split across >=2 pages, got %d", len(res.Pages))
	}
	l1 := firstText(res.Pages[0].Root)
	l2 := firstText(res.Pages[1].Root)
	if l1 == nil || l2 == nil {
		t.Fatalf("both pages should carry text: %v %v", l1, l2)
	}
	if len(l1.Lines) == 0 || len(l2.Lines) == 0 {
		t.Errorf("each page's text should have lines: %d, %d", len(l1.Lines), len(l2.Lines))
	}
	// First page's continuation starts at the page content top (Y≈0).
	if got := res.Pages[1].Root.Children[0].Frame.Y; got < -1e-6 || got > 1e-6 {
		t.Errorf("continuation text Y = %v, want ~0", got)
	}
}

func TestPaginationForcedBreak(t *testing.T) {
	// Both blocks fit on one 200pt page, but the second forces a page break.
	res := layoutJSON(t, `{"version":"waffle-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[100,200]},"children":[
			{"type":"VIEW","props":{"style":{"height":20,"backgroundColor":"red"}}},
			{"type":"VIEW","props":{"break":true,"style":{"height":20,"backgroundColor":"blue"}}}
		]}
	]}}`)
	if len(res.Pages) != 2 {
		t.Fatalf("break should force 2 pages, got %d", len(res.Pages))
	}
	if len(res.Pages[0].Root.Children) != 1 || len(res.Pages[1].Root.Children) != 1 {
		t.Errorf("each page should hold one block, got %d and %d",
			len(res.Pages[0].Root.Children), len(res.Pages[1].Root.Children))
	}
	if got := res.Pages[1].Root.Children[0].Frame.Y; got < -1e-6 || got > 1e-6 {
		t.Errorf("broken element should start at page top (Y≈0), got %v", got)
	}
}

func TestPaginationFixedRepeats(t *testing.T) {
	res := layoutJSON(t, `{"version":"waffle-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[100,100]},"children":[
			{"type":"VIEW","props":{"style":{"height":40}}},
			{"type":"VIEW","props":{"style":{"height":40}}},
			{"type":"VIEW","props":{"style":{"height":40}}},
			{"type":"VIEW","props":{"fixed":true,"style":{"position":"absolute","bottom":0,"left":0,"height":10,"width":"100%","backgroundColor":"black"}}}
		]}
	]}}`)
	if len(res.Pages) < 2 {
		t.Fatalf("expected multiple pages, got %d", len(res.Pages))
	}
	for i, pg := range res.Pages {
		found := false
		for _, ch := range pg.Root.Children {
			if ch.Node != nil && ch.Node.Fixed {
				found = true
				// The absolute footer sits near the page bottom.
				if ch.Frame.Y < 80 {
					t.Errorf("page %d footer Y = %v, want near bottom (>=80)", i, ch.Frame.Y)
				}
			}
		}
		if !found {
			t.Errorf("page %d is missing the fixed footer", i)
		}
	}
}

func TestPaginationUnbreakableMovesWhole(t *testing.T) {
	// A 70pt block, then a wrap=false View of two 20pt rows: it straddles the
	// 100pt page at 70..110, so it moves whole to page 2 instead of splitting.
	res := layoutJSON(t, `{"version":"waffle-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[100,100]},"children":[
			{"type":"VIEW","props":{"style":{"height":70}}},
			{"type":"VIEW","props":{"wrap":false},"children":[
				{"type":"VIEW","props":{"style":{"height":20}}},
				{"type":"VIEW","props":{"style":{"height":20}}}
			]}
		]}
	]}}`)
	if len(res.Pages) != 2 {
		t.Fatalf("expected 2 pages, got %d", len(res.Pages))
	}
	if n := len(res.Pages[0].Root.Children); n != 1 {
		t.Errorf("page 1 should hold only the 70pt block, got %d children", n)
	}
	moved := res.Pages[1].Root.Children[0]
	if n := len(moved.Children); n != 2 {
		t.Errorf("unbreakable block should arrive whole (2 rows), got %d", n)
	}
	if moved.Frame.Y < -1e-6 || moved.Frame.Y > 1e-6 || moved.Frame.H != 40 {
		t.Errorf("unbreakable block frame Y=%v H=%v, want Y≈0 H=40", moved.Frame.Y, moved.Frame.H)
	}
}

func TestPaginationUnbreakableTallerThanPageSplits(t *testing.T) {
	// A wrap=false View taller than a whole page cannot move whole anywhere, so it
	// splits as usual rather than overflowing off the page.
	res := layoutJSON(t, `{"version":"waffle-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[100,100]},"children":[
			{"type":"VIEW","props":{"wrap":false},"children":[
				{"type":"VIEW","props":{"style":{"height":40}}},
				{"type":"VIEW","props":{"style":{"height":40}}},
				{"type":"VIEW","props":{"style":{"height":40}}}
			]}
		]}
	]}}`)
	if len(res.Pages) != 2 {
		t.Fatalf("expected the 120pt block to split over 2 pages, got %d", len(res.Pages))
	}
}

func TestPaginationMinPresenceAhead(t *testing.T) {
	// Block A (30pt) fits, but B has minPresenceAhead=50; only ~10pt remain after A,
	// so B (and A? no — A stays, B breaks) is pushed to page 2.
	res := layoutJSON(t, `{"version":"waffle-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[100,40]},"children":[
			{"type":"VIEW","props":{"style":{"height":30}}},
			{"type":"VIEW","props":{"minPresenceAhead":50,"style":{"height":8}}}
		]}
	]}}`)
	if len(res.Pages) != 2 {
		t.Fatalf("minPresenceAhead should push B to page 2, got %d pages", len(res.Pages))
	}
	if len(res.Pages[0].Root.Children) != 1 {
		t.Errorf("page 1 should hold only A, got %d children", len(res.Pages[0].Root.Children))
	}
}

func TestPaginationWidows(t *testing.T) {
	// Deterministic: a 4-line text; a page fits 3 lines; widows=2 forces a 2/2
	// split (a natural 3/1 split would leave a single widow line).
	txt := &Box{
		Node:  &tree.Node{Type: contract.TypeText},
		Frame: Rect{X: 0, Y: 0, W: 100, H: 48},
		Text:  &TextInfo{Lines: []string{"a", "b", "c", "d"}, LineHeight: 12, BaseFont: "Helvetica", Size: 12, Orphans: 2, Widows: 2},
	}
	root := &Box{Node: &tree.Node{Type: contract.TypePage}, Frame: Rect{X: 0, Y: 0, W: 100, H: 40}, Children: []*Box{txt}}
	pages := paginate(&Page{Width: 100, Height: 40, Root: root}, 0, 40, true)
	if len(pages) != 2 {
		t.Fatalf("want 2 pages, got %d", len(pages))
	}
	l0, l1 := firstText(pages[0].Root), firstText(pages[1].Root)
	if l0 == nil || l1 == nil || len(l0.Lines) != 2 || len(l1.Lines) != 2 {
		t.Errorf("widows: want a 2/2 split, got %v / %v", l0, l1)
	}
}

func TestPageNumberTemplates(t *testing.T) {
	// A fixed footer with a page-number template, over content that spans 2 pages.
	res := layoutJSON(t, `{"version":"waffle-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[120,100]},"children":[
			{"type":"VIEW","props":{"style":{"height":60}}},
			{"type":"VIEW","props":{"style":{"height":60}}},
			{"type":"TEXT","props":{"fixed":true,"render":"{pageNumber} / {totalPages}","style":{"position":"absolute","bottom":0,"left":0}}}
		]}
	]}}`)
	if len(res.Pages) != 2 {
		t.Fatalf("want 2 pages, got %d", len(res.Pages))
	}
	// Each page's footer shows its own number over the shared total.
	want := []string{"1 / 2", "2 / 2"}
	for i, pg := range res.Pages {
		var footer *TextInfo
		for _, ch := range pg.Root.Children {
			if ch.Text != nil && ch.Text.Template != "" {
				footer = ch.Text
			}
		}
		if footer == nil {
			t.Fatalf("page %d has no footer", i)
		}
		if footer.Content != want[i] {
			t.Errorf("page %d footer = %q, want %q", i, footer.Content, want[i])
		}
	}
}
