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
	// A 30pt header over an 80pt leaf row cannot share any 100pt page. Repeating
	// the header would carry the same overflow to every page, so the header stays
	// on page 1 alone and the rows continue unrepeated: content is never lost and
	// pagination ends.
	res := layoutJSON(t, `{"version":"waffle-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[200,100]},"children":[
			{"type":"VIEW","props":{"id":"table"},"children":[
				{"type":"VIEW","props":{"id":"hdr","fixed":true,"style":{"height":30}}},
				{"type":"VIEW","props":{"id":"big","style":{"height":80}}},
				{"type":"VIEW","props":{"id":"small","style":{"height":15}}}
			]}
		]}
	]}}`)
	if len(res.Pages) != 2 {
		t.Fatalf("pages = %d, want 2", len(res.Pages))
	}
	if got := kidIDs(byID(res.Pages[0].Root, "table")); strings.Join(got, ",") != "hdr" {
		t.Errorf("page 1 table = %v, want hdr", got)
	}
	table := byID(res.Pages[1].Root, "table")
	if got := kidIDs(table); strings.Join(got, ",") != "big,small" {
		t.Errorf("page 2 table = %v, want big,small", got)
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
