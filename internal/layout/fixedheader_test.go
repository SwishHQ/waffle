package layout

import (
	"fmt"
	"math"
	"strings"
	"testing"
)

// rowsJSON is n rows with ids r1..rN: wrap={false} rows of two 15pt cells, as a
// table body is written.
func rowsJSON(n int) string {
	rows := make([]string, n)
	for i := range rows {
		rows[i] = fmt.Sprintf(`{"type":"VIEW","props":{"id":"r%d","wrap":false,"style":{"flexDirection":"row"}},"children":[
			{"type":"VIEW","props":{"style":{"width":50,"height":15}}},
			{"type":"VIEW","props":{"style":{"width":50,"height":15}}}
		]}`, i+1)
	}
	return strings.Join(rows, ",")
}

// headerJSON is a 20pt fixed header row whose cell prints its page number.
const headerJSON = `{"type":"VIEW","props":{"id":"hdr","fixed":true,"style":{"flexDirection":"row","height":20}},"children":[
	{"type":"TEXT","props":{"render":"p{pageNumber}","style":{"fontSize":10}}}
]}`

// byID finds the box whose node carries id in b's subtree.
func byID(b *Box, id string) *Box {
	if b.Node != nil && b.Node.ID == id {
		return b
	}
	for _, c := range b.Children {
		if f := byID(c, id); f != nil {
			return f
		}
	}
	return nil
}

// kidIDs lists the ids of b's children, in order.
func kidIDs(b *Box) []string {
	var ids []string
	for _, c := range b.Children {
		ids = append(ids, c.Node.ID)
	}
	return ids
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

func TestPaginationTableHeaderRepeats(t *testing.T) {
	// 80pt of content per page (10pt top and bottom padding). A 20pt header and
	// ten 15pt rows: page 1 holds the header and r1..r4, page 2 a repeated header
	// and r5..r8, page 3 the header again and r9, r10.
	res := layoutJSON(t, `{"version":"waffle-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[200,100],"style":{"paddingTop":10,"paddingBottom":10}},"children":[
			{"type":"VIEW","props":{"id":"table"},"children":[`+headerJSON+`,`+rowsJSON(10)+`]}
		]}
	]}}`)
	if len(res.Pages) != 3 {
		t.Fatalf("pages = %d, want 3", len(res.Pages))
	}
	want := [][]string{
		{"hdr", "r1", "r2", "r3", "r4"},
		{"hdr", "r5", "r6", "r7", "r8"},
		{"hdr", "r9", "r10"},
	}
	var headers []*Box
	for i, pg := range res.Pages {
		table := byID(pg.Root, "table")
		if table == nil {
			t.Fatalf("page %d has no table", i+1)
		}
		if got := kidIDs(table); strings.Join(got, ",") != strings.Join(want[i], ",") {
			t.Errorf("page %d table rows = %v, want %v", i+1, got, want[i])
			continue
		}
		hdr := table.Children[0]
		headers = append(headers, hdr)
		// The header sits at the top of the content area, the rows right under it.
		frameEq(t, fmt.Sprintf("page %d header", i+1), hdr.Frame, 0, 10, 200, 20)
		for j, row := range table.Children[1:] {
			if wantY := 30 + 15*float64(j); !near(row.Frame.Y, wantY) {
				t.Errorf("page %d %s Y = %v, want %v", i+1, row.Node.ID, row.Frame.Y, wantY)
			}
		}
		if !near(table.Frame.Y, 10) {
			t.Errorf("page %d table Y = %v, want 10", i+1, table.Frame.Y)
		}
		if b := maxBottom(pg.Root.Children[0]); b > 90+1e-6 {
			t.Errorf("page %d draws to y=%v, past the 90pt content bottom", i+1, b)
		}
		// Each page's copy carries its own page number.
		if txt := firstText(hdr); txt == nil || txt.Content != fmt.Sprintf("p%d", i+1) {
			t.Errorf("page %d header text = %+v, want p%d", i+1, txt, i+1)
		}
	}
	if len(headers) == 3 && (headers[0] == headers[1] || headers[1] == headers[2]) {
		t.Error("continuation headers must be clones, not the page 1 box")
	}
}

func TestPaginationNestedTableHeaderRepeats(t *testing.T) {
	// A section with a fixed 10pt title holds a table (20pt header, six 15pt rows)
	// and a 15pt note after it. Page 1 ends after r4; page 2 repeats the section
	// title, then the table header, then r5, r6, and the note below them.
	res := layoutJSON(t, `{"version":"waffle-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[200,100]},"children":[
			{"type":"VIEW","props":{"id":"section"},"children":[
				{"type":"VIEW","props":{"id":"title","fixed":true,"style":{"height":10}}},
				{"type":"VIEW","props":{"id":"table"},"children":[`+headerJSON+`,`+rowsJSON(6)+`]},
				{"type":"VIEW","props":{"id":"after","style":{"height":15}}}
			]}
		]}
	]}}`)
	if len(res.Pages) != 2 {
		t.Fatalf("pages = %d, want 2", len(res.Pages))
	}
	if got := kidIDs(byID(res.Pages[0].Root, "table")); strings.Join(got, ",") != "hdr,r1,r2,r3,r4" {
		t.Errorf("page 1 table rows = %v, want hdr,r1..r4", got)
	}
	sec := byID(res.Pages[1].Root, "section")
	if got := kidIDs(sec); strings.Join(got, ",") != "title,table,after" {
		t.Fatalf("page 2 section = %v, want title,table,after", got)
	}
	table := sec.Children[1]
	if got := kidIDs(table); strings.Join(got, ",") != "hdr,r5,r6" {
		t.Fatalf("page 2 table = %v, want hdr,r5,r6", got)
	}
	frameEq(t, "page 2 section title", sec.Children[0].Frame, 0, 0, 200, 10)
	frameEq(t, "page 2 table header", table.Children[0].Frame, 0, 10, 200, 20)
	frameEq(t, "page 2 r5", table.Children[1].Frame, 0, 30, 200, 15)
	frameEq(t, "page 2 r6", table.Children[2].Frame, 0, 45, 200, 15)
	frameEq(t, "page 2 table", table.Frame, 0, 10, 200, 50)
	// The note after the table moves down with the repeated headers.
	frameEq(t, "page 2 note", sec.Children[2].Frame, 0, 60, 200, 15)
	frameEq(t, "page 2 section", sec.Frame, 0, 0, 200, 75)
}

func TestPaginationTableHeaderMovesWithItsRows(t *testing.T) {
	// A table whose header cannot start a page with a row under it moves whole to
	// the next page, rather than leaving the header alone at the page bottom.
	for _, tc := range []struct {
		name   string
		spacer int
	}{
		{"header taller than the space left", 85}, // header 85..105 crosses 100
		{"header fits but no row does", 70},       // header 70..90, r1 90..105
	} {
		t.Run(tc.name, func(t *testing.T) {
			res := layoutJSON(t, `{"version":"waffle-tree/v1","document":{"children":[
				{"type":"PAGE","props":{"size":[200,100]},"children":[
					{"type":"VIEW","props":{"id":"spacer","style":{"height":`+fmt.Sprint(tc.spacer)+`}}},
					{"type":"VIEW","props":{"id":"table"},"children":[`+headerJSON+`,`+rowsJSON(3)+`]}
				]}
			]}}`)
			if len(res.Pages) != 2 {
				t.Fatalf("pages = %d, want 2", len(res.Pages))
			}
			if got := kidIDs(res.Pages[0].Root); strings.Join(got, ",") != "spacer" {
				t.Errorf("page 1 = %v, want only the spacer", got)
			}
			table := byID(res.Pages[1].Root, "table")
			if got := kidIDs(table); strings.Join(got, ",") != "hdr,r1,r2,r3" {
				t.Errorf("page 2 table = %v, want hdr,r1,r2,r3 (one header)", got)
			}
			frameEq(t, "page 2 table", table.Frame, 0, 0, 200, 65)
		})
	}
}

func TestPaginationHeaderAndRowTallerThanPage(t *testing.T) {
	// A 30pt header over an 80pt leaf row cannot share any 100pt page. Page 1
	// ends on the header (its first and only place to start); page 2 holds big
	// and s1 without a copy, since a copy would push big off the page; the header
	// returns on page 3. Content is never lost and pagination ends.
	pages, res := tablePages(t, `{"type":"PAGE","props":{"size":[200,100]},"children":[
		{"type":"VIEW","props":{"id":"table"},"children":[
			{"type":"VIEW","props":{"id":"hdr","fixed":true,"style":{"height":30}}},
			{"type":"VIEW","props":{"id":"big","style":{"height":80}}},`+leaves("s", 3, 15)+`
		]}
	]}`)
	wantPages(t, pages, "hdr", "big,s1", "hdr,s2,s3")
	if len(res.Pages) == 3 {
		frameEq(t, "page 2 big", byID(res.Pages[1].Root, "big").Frame, 0, 0, 200, 80)
		frameEq(t, "page 3 header", byID(res.Pages[2].Root, "hdr").Frame, 0, 0, 200, 30)
		frameEq(t, "page 3 s2", byID(res.Pages[2].Root, "s2").Frame, 0, 30, 200, 15)
	}
	for i, pg := range res.Pages {
		if b := maxBottom(pg.Root.Children[0]); b > 100+1e-6 {
			t.Errorf("page %d draws to y=%v, past the page", i+1, b)
		}
	}
}

func TestPaginationFixedInRowIsNotAHeader(t *testing.T) {
	// A row lays its children side by side, so a fixed child there has nothing
	// under it to head: the column beside it continues without a copy of it.
	res := layoutJSON(t, `{"version":"waffle-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[200,100]},"children":[
			{"type":"VIEW","props":{"id":"row","style":{"flexDirection":"row"}},"children":[
				{"type":"VIEW","props":{"id":"side","fixed":true,"style":{"width":20,"height":20}}},
				{"type":"VIEW","props":{"id":"col"},"children":[`+rowsJSON(10)+`]}
			]}
		]}
	]}}`)
	if len(res.Pages) != 2 {
		t.Fatalf("pages = %d, want 2", len(res.Pages))
	}
	if got := kidIDs(byID(res.Pages[0].Root, "row")); strings.Join(got, ",") != "side,col" {
		t.Errorf("page 1 row = %v, want side,col", got)
	}
	if got := kidIDs(byID(res.Pages[1].Root, "row")); strings.Join(got, ",") != "col" {
		t.Errorf("page 2 row = %v, want col alone (no repeated copy)", got)
	}
	col := byID(res.Pages[1].Root, "col")
	if got := kidIDs(col); strings.Join(got, ",") != "r7,r8,r9,r10" {
		t.Errorf("page 2 col = %v, want r7..r10", got)
	}
	frameEq(t, "page 2 col", col.Frame, 20, 0, 100, 60)
	frameEq(t, "page 2 r7", col.Children[0].Frame, 20, 0, 100, 15)
}

func TestPaginationHeaderLeavesCaptionBehind(t *testing.T) {
	// Content above the header stays on page 1; the header, with no row under it
	// there, opens page 2 instead.
	res := layoutJSON(t, `{"version":"waffle-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[200,100]},"children":[
			{"type":"VIEW","props":{"id":"spacer","style":{"height":60}}},
			{"type":"VIEW","props":{"id":"table"},"children":[
				{"type":"VIEW","props":{"id":"caption","style":{"height":10}}},`+headerJSON+`,`+rowsJSON(3)+`
			]}
		]}
	]}}`)
	if len(res.Pages) != 2 {
		t.Fatalf("pages = %d, want 2", len(res.Pages))
	}
	if got := kidIDs(byID(res.Pages[0].Root, "table")); strings.Join(got, ",") != "caption" {
		t.Errorf("page 1 table = %v, want caption", got)
	}
	table := byID(res.Pages[1].Root, "table")
	if got := kidIDs(table); strings.Join(got, ",") != "hdr,r1,r2,r3" {
		t.Errorf("page 2 table = %v, want hdr,r1,r2,r3", got)
	}
	frameEq(t, "page 2 header", table.Children[0].Frame, 0, 0, 200, 20)
}

// leaves is n leaf rows of height h with ids prefix1..prefixN.
func leaves(prefix string, n int, h float64) string {
	rows := make([]string, n)
	for i := range rows {
		rows[i] = fmt.Sprintf(`{"type":"VIEW","props":{"id":"%s%d","style":{"height":%v}}}`, prefix, i+1, h)
	}
	return strings.Join(rows, ",")
}

// tablePages lays out doc and returns, per page, the child ids of the box
// with id "table" ("" when the page has none).
func tablePages(t *testing.T, doc string) ([]string, *Result) {
	t.Helper()
	res := layoutJSON(t, `{"version":"waffle-tree/v1","document":{"children":[`+doc+`]}}`)
	var pages []string
	for _, pg := range res.Pages {
		if tb := byID(pg.Root, "table"); tb != nil {
			pages = append(pages, strings.Join(kidIDs(tb), ","))
		} else {
			pages = append(pages, "")
		}
	}
	return pages, res
}

func wantPages(t *testing.T, got []string, want ...string) {
	t.Helper()
	if strings.Join(got, " | ") != strings.Join(want, " | ") {
		t.Errorf("table per page:\n got %q\nwant %q", got, want)
	}
}

func TestPaginationUnbreakableRowUnderHeaderStaysWhole(t *testing.T) {
	// big (wrap={false}, 86pt) fits a page alone but not under the 20pt header.
	// Keeping it whole wins: page 2 shows it without the header, which returns on
	// page 3. Splitting it under the header would break the wrap={false} promise.
	pages, res := tablePages(t, `{"type":"PAGE","props":{"size":[200,100]},"children":[
		{"type":"VIEW","props":{"id":"table"},"children":[`+headerJSON+`,
			{"type":"VIEW","props":{"id":"r1","style":{"height":15}}},
			{"type":"VIEW","props":{"id":"big","wrap":false},"children":[`+leaves("b", 4, 21.5)+`]},
			{"type":"VIEW","props":{"id":"r2","style":{"height":15}}},
			{"type":"VIEW","props":{"id":"r3","style":{"height":15}}}
		]}
	]}`)
	wantPages(t, pages, "hdr,r1", "big", "hdr,r2,r3")
	if len(res.Pages) == 3 {
		big := byID(res.Pages[1].Root, "big")
		frameEq(t, "page 2 big", big.Frame, 0, 0, 200, 86)
		if n := len(big.Children); n != 4 {
			t.Errorf("big should arrive whole with 4 children, got %d", n)
		}
		frameEq(t, "page 3 header", byID(res.Pages[2].Root, "hdr").Frame, 0, 0, 200, 20)
	}
}

func TestPaginationTallRowMidTableSkipsHeaderOnce(t *testing.T) {
	// r4 (90pt) cannot share a page with the 20pt header: page 2 holds r4 alone,
	// and the header comes back on page 3 rather than being lost.
	pages, _ := tablePages(t, `{"type":"PAGE","props":{"size":[200,100]},"children":[
		{"type":"VIEW","props":{"id":"table"},"children":[`+headerJSON+`,`+leaves("r", 3, 15)+`,
			{"type":"VIEW","props":{"id":"r4","style":{"height":90}}},
			{"type":"VIEW","props":{"id":"r5","style":{"height":15}}},
			{"type":"VIEW","props":{"id":"r6","style":{"height":15}}}
		]}
	]}`)
	wantPages(t, pages, "hdr,r1,r2,r3", "r4", "hdr,r5,r6")
}

func TestPaginationBreakUnderRepeatedHeader(t *testing.T) {
	// r2 breaks onto page 2. The repeated header above it there is not content
	// placed before it, so the break does not fire again and empty the page.
	pages, _ := tablePages(t, `{"type":"PAGE","props":{"size":[200,100]},"children":[
		{"type":"VIEW","props":{"id":"table"},"children":[`+headerJSON+`,
			{"type":"VIEW","props":{"id":"r1","style":{"height":15}}},
			{"type":"VIEW","props":{"id":"r2","break":true,"style":{"height":15}}},`+leaves("x", 8, 15)+`
		]}
	]}`)
	wantPages(t, pages, "hdr,r1", "hdr,r2,x1,x2,x3,x4", "hdr,x5,x6,x7,x8")
}

func TestPaginationMinPresenceAheadUnderRepeatedHeader(t *testing.T) {
	// g needs 70pt after it, so it starts page 2. Under the repeated header it
	// stays put: breaking before it again would leave a page holding the header.
	pages, _ := tablePages(t, `{"type":"PAGE","props":{"size":[200,100]},"children":[
		{"type":"VIEW","props":{"id":"table"},"children":[`+headerJSON+`,
			{"type":"VIEW","props":{"id":"r1","style":{"height":30}}},
			{"type":"VIEW","props":{"id":"r2","style":{"height":30}}},
			{"type":"VIEW","props":{"id":"g","minPresenceAhead":70,"style":{"height":15}}},`+leaves("x", 6, 15)+`
		]}
	]}`)
	wantPages(t, pages, "hdr,r1,r2", "hdr,g,x1,x2,x3,x4", "hdr,x5,x6")
}

func TestPaginationHeaderWithAbsoluteSiblingMovesWithRows(t *testing.T) {
	// An absolute badge after the header is not a row: the header still may not
	// end page 1, so the table moves whole and its header prints once on page 2.
	pages, _ := tablePages(t, `{"type":"PAGE","props":{"size":[200,100]},"children":[
		{"type":"VIEW","props":{"id":"spacer","style":{"height":70}}},
		{"type":"VIEW","props":{"id":"table"},"children":[`+headerJSON+`,
			{"type":"VIEW","props":{"id":"badge","style":{"position":"absolute","top":0,"right":0,"width":10,"height":5}}},`+leaves("r", 2, 15)+`
		]}
	]}`)
	wantPages(t, pages, "", "hdr,badge,r1,r2")
}

func TestPaginationRepeatedHeaderKeepsItsInset(t *testing.T) {
	// The table's 2pt top border, 10pt top padding and the header's 5pt margins
	// place the header 17pt below the table top and its first row 25pt below the
	// header. Each continuation reproduces both offsets.
	pages, res := tablePages(t, `{"type":"PAGE","props":{"size":[200,100]},"children":[
		{"type":"VIEW","props":{"id":"table","style":{"paddingTop":10,"borderTopWidth":2}},"children":[
			{"type":"VIEW","props":{"id":"hdr","fixed":true,"style":{"height":20,"marginTop":5,"marginBottom":5}}},`+leaves("x", 8, 15)+`
		]}
	]}`)
	wantPages(t, pages, "hdr,x1,x2,x3", "hdr,x4,x5,x6", "hdr,x7,x8")
	for i, pg := range res.Pages {
		tb := byID(pg.Root, "table")
		if tb == nil || len(tb.Children) < 2 {
			continue
		}
		hdr, row := tb.Children[0], tb.Children[1]
		if !near(hdr.Frame.Y-tb.Frame.Y, 17) || !near(row.Frame.Y-hdr.Frame.Y, 25) {
			t.Errorf("page %d: header %vpt below the table top and row %vpt below the header, want 17 and 25",
				i+1, hdr.Frame.Y-tb.Frame.Y, row.Frame.Y-hdr.Frame.Y)
		}
	}
}

func TestPaginationConsecutiveHeadersRepeatInOrder(t *testing.T) {
	pages, res := tablePages(t, `{"type":"PAGE","props":{"size":[200,100]},"children":[
		{"type":"VIEW","props":{"id":"table"},"children":[
			{"type":"VIEW","props":{"id":"h1","fixed":true,"style":{"height":10}}},
			{"type":"VIEW","props":{"id":"h2","fixed":true,"style":{"height":12}}},`+leaves("x", 10, 15)+`
		]}
	]}`)
	wantPages(t, pages, "h1,h2,x1,x2,x3,x4,x5", "h1,h2,x6,x7,x8,x9,x10")
	if len(res.Pages) == 2 {
		tb := byID(res.Pages[1].Root, "table")
		frameEq(t, "page 2 h1", tb.Children[0].Frame, 0, 0, 200, 10)
		frameEq(t, "page 2 h2", tb.Children[1].Frame, 0, 10, 200, 12)
		frameEq(t, "page 2 x6", tb.Children[2].Frame, 0, 22, 200, 15)
	}
}

func TestPaginationTextRowSplitsUnderHeader(t *testing.T) {
	// A long Text row splits by lines; its continuation sits under the header.
	pages, res := tablePages(t, `{"type":"PAGE","props":{"size":[200,100]},"children":[
		{"type":"VIEW","props":{"id":"table"},"children":[`+headerJSON+`,
			{"type":"VIEW","props":{"id":"x1","style":{"height":15}}},
			{"type":"TEXT","props":{"id":"txt","style":{"width":60,"fontSize":10,"lineHeight":1}},"children":[
				{"type":"TEXT_INSTANCE","value":"one two three four five six seven eight nine ten eleven twelve thirteen fourteen fifteen sixteen seventeen eighteen nineteen twenty"}
			]}
		]}
	]}`)
	if len(pages) < 2 || pages[0] != "hdr,x1,txt" || pages[1] != "hdr,txt" {
		t.Fatalf("table per page = %q, want hdr,x1,txt then hdr,txt", pages)
	}
	total := 0
	for _, pg := range res.Pages {
		total += len(byID(pg.Root, "txt").Text.Lines)
	}
	first := byID(res.Pages[0].Root, "txt")
	if lines := len(first.Text.Lines) + len(byID(res.Pages[1].Root, "txt").Text.Lines); len(res.Pages) == 2 && lines != total {
		t.Errorf("lines split %d across pages, want %d", lines, total)
	}
	frameEq(t, "page 2 txt top", Rect{Y: byID(res.Pages[1].Root, "txt").Frame.Y}, 0, 20, 0, 0)
}

func TestPaginationPageFixedAndViewHeaderTogether(t *testing.T) {
	// A page-level fixed footer and a table header both appear on every page.
	_, res := tablePages(t, `{"type":"PAGE","props":{"size":[200,100],"style":{"paddingBottom":10}},"children":[
		{"type":"VIEW","props":{"id":"table"},"children":[`+headerJSON+`,`+leaves("x", 10, 15)+`]},
		{"type":"VIEW","props":{"id":"footer","fixed":true,"style":{"position":"absolute","bottom":0,"left":0,"width":200,"height":10}}}
	]}`)
	if len(res.Pages) != 3 {
		t.Fatalf("pages = %d, want 3", len(res.Pages))
	}
	for i, pg := range res.Pages {
		// bottom:0 resolves against the content box, above the 10pt padding.
		if f := byID(pg.Root, "footer"); f == nil || !near(f.Frame.Y, 80) {
			t.Errorf("page %d footer = %+v, want at y=80", i+1, f)
		}
		if tb := byID(pg.Root, "table"); tb == nil || tb.Children[0].Node.ID != "hdr" || !near(tb.Children[0].Frame.Y, 0) {
			t.Errorf("page %d should open with the table header at y=0", i+1)
		}
	}
}
