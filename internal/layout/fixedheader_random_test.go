package layout

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/SwishHQ/waffle/internal/contract"
	"github.com/SwishHQ/waffle/internal/tree"
)

// tableGen builds random documents of tables: some with a fixed header (with
// margins, under a top border and padding), an absolute badge after it, rows
// that are leaves or wrap={false} rows of cells (some forcing a break or asking
// for presence ahead), and tables three levels deep. Every box is small enough
// that a header and the row under it always share a page, so no page has to
// leave its header copies off.
type tableGen struct {
	r    *rand.Rand
	n    int
	rows []string          // content ids in document order
	hdr  map[string]string // table id → header id
}

func (g *tableGen) id(prefix string) string {
	g.n++
	return fmt.Sprintf("%s%d", prefix, g.n)
}

func (g *tableGen) between(lo, hi int) int { return lo + g.r.Intn(hi-lo+1) }

// maybe is between(lo, hi) half the time and 0 otherwise.
func (g *tableGen) maybe(lo, hi int) int { return g.between(lo, hi) * g.r.Intn(2) }

func (g *tableGen) cells(h int) string {
	return fmt.Sprintf(`{"type":"VIEW","props":{"style":{"width":40,"height":%d}}},{"type":"VIEW","props":{"style":{"width":40,"height":%d}}}`, h, h)
}

func (g *tableGen) row() string {
	id := g.id("r")
	g.rows = append(g.rows, id)
	props := fmt.Sprintf(`"id":%q`, id)
	switch g.r.Intn(15) {
	case 0:
		props += `,"break":true`
	case 1:
		props += fmt.Sprintf(`,"minPresenceAhead":%d`, g.between(20, 60))
	}
	if g.r.Intn(2) == 0 {
		return fmt.Sprintf(`{"type":"VIEW","props":{%s,"style":{"height":%d,"marginTop":%d}}}`, props, g.between(8, 30), g.maybe(1, 4))
	}
	return fmt.Sprintf(`{"type":"VIEW","props":{%s,"wrap":false,"style":{"flexDirection":"row","marginTop":%d}},"children":[%s]}`, props, g.maybe(1, 4), g.cells(g.between(8, 30)))
}

func (g *tableGen) table(depth int) string {
	tid := g.id("t")
	var kids []string
	if g.r.Intn(10) < 7 {
		hid := g.id("h")
		g.hdr[tid] = hid
		margins := fmt.Sprintf(`"marginTop":%d,"marginBottom":%d`, g.maybe(1, 4), g.maybe(1, 4))
		if g.r.Intn(2) == 0 {
			kids = append(kids, fmt.Sprintf(`{"type":"VIEW","props":{"id":%q,"fixed":true,"style":{"height":%d,%s}}}`, hid, g.between(8, 20), margins))
		} else {
			kids = append(kids, fmt.Sprintf(`{"type":"VIEW","props":{"id":%q,"fixed":true,"style":{"flexDirection":"row",%s}},"children":[%s]}`, hid, margins, g.cells(g.between(8, 20))))
		}
		if g.r.Intn(5) == 0 {
			kids = append(kids, fmt.Sprintf(`{"type":"VIEW","props":{"id":%q,"style":{"position":"absolute","top":0,"right":0,"width":5,"height":5}}}`, g.id("a")))
		}
	}
	for i, n := 0, g.between(1, 12); i < n; i++ {
		if depth < 2 && g.r.Intn(6) == 0 {
			kids = append(kids, g.table(depth+1))
		} else {
			kids = append(kids, g.row())
		}
	}
	return fmt.Sprintf(`{"type":"VIEW","props":{"id":%q,"style":{"paddingTop":%d,"paddingBottom":%d,"borderTopWidth":%d}},"children":[%s]}`,
		tid, g.maybe(1, 6), g.maybe(1, 6), g.maybe(1, 2), strings.Join(kids, ","))
}

// layoutDoc is layoutJSON without a *testing.T, so it can run off the test
// goroutine under a timeout.
func layoutDoc(doc string) (*Result, error) {
	ct, err := contract.Parse([]byte(doc))
	if err != nil {
		return nil, err
	}
	tr, err := tree.Build(ct)
	if err != nil {
		return nil, err
	}
	return Layout(tr, Options{})
}

// contentIDs lists the row ids under b in document order.
func contentIDs(b *Box, out []string) []string {
	if b.Node != nil && strings.HasPrefix(b.Node.ID, "r") {
		out = append(out, b.Node.ID)
	}
	for _, c := range b.Children {
		out = contentIDs(c, out)
	}
	return out
}

// rowBottom is the lowest edge of any row or header under b. A table's own
// bottom padding is left out: a View whose rows all fit is placed whole even
// when its padding crosses the page bottom, as it was before headers repeated.
func rowBottom(b *Box) float64 {
	m := 0.0
	if b.Node != nil && (strings.HasPrefix(b.Node.ID, "r") || strings.HasPrefix(b.Node.ID, "h")) {
		m = maxBottom(b)
	}
	for _, c := range b.Children {
		m = max(m, rowBottom(c))
	}
	return m
}

// tableParts calls fn for every table part (a box with a t-id) under b.
func tableParts(b *Box, fn func(*Box)) {
	if b.Node != nil && strings.HasPrefix(b.Node.ID, "t") {
		fn(b)
	}
	for _, c := range b.Children {
		tableParts(c, fn)
	}
}

// randomDoc generates seed's document on pages minH..maxH points tall.
func randomDoc(seed int64, minH, maxH int) (g *tableGen, doc string, pageH, pad int) {
	g = &tableGen{r: rand.New(rand.NewSource(seed)), hdr: map[string]string{}}
	pageH, pad = g.between(minH, maxH), g.maybe(1, 10)
	var kids []string
	if g.r.Intn(2) == 0 {
		kids = append(kids, g.row())
	}
	for i, n := 0, g.between(1, 3); i < n; i++ {
		kids = append(kids, g.table(0))
	}
	doc = fmt.Sprintf(`{"version":"waffle-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[200,%d],"style":{"paddingTop":%d,"paddingBottom":%d}},"children":[%s]}
	]}}`, pageH, pad, pad, strings.Join(kids, ","))
	return g, doc, pageH, pad
}

// layoutWithin lays out doc, failing the test if pagination does not finish.
func layoutWithin(t *testing.T, seed int64, doc string) *Result {
	t.Helper()
	type outcome struct {
		res *Result
		err error
	}
	done := make(chan outcome, 1)
	go func() {
		res, err := layoutDoc(doc)
		done <- outcome{res, err}
	}()
	select {
	case o := <-done:
		if o.err != nil {
			t.Fatalf("seed %d: %v", seed, o.err)
		}
		return o.res
	case <-time.After(10 * time.Second):
		t.Fatalf("seed %d: pagination did not finish", seed)
	}
	return nil
}

// checkRows fails unless every row lands once, in document order, and no row
// or header reaches past the content bottom.
func checkRows(t *testing.T, seed int64, g *tableGen, res *Result, bottom float64) {
	t.Helper()
	var got []string
	for pi, pg := range res.Pages {
		got = contentIDs(pg.Root, got)
		if b := rowBottom(pg.Root); b > bottom+1e-6 {
			t.Errorf("seed %d page %d: a row or header reaches y=%v, past the content bottom %v", seed, pi+1, b, bottom)
		}
	}
	if strings.Join(got, ",") != strings.Join(g.rows, ",") {
		t.Fatalf("seed %d: rows across pages = %v, want each once in order %v", seed, got, g.rows)
	}
}

func TestPaginationHeadersRandomTrees(t *testing.T) {
	for seed := int64(1); seed <= 400; seed++ {
		g, doc, pageH, pad := randomDoc(seed, 200, 280)
		res := layoutWithin(t, seed, doc)
		checkRows(t, seed, g, res, float64(pageH-pad))

		inset := map[string]float64{} // each headed table's header offset on its first page
		for pi, pg := range res.Pages {
			tableParts(pg.Root, func(part *Box) {
				id := part.Node.ID
				if len(contentIDs(part, nil)) == 0 {
					t.Errorf("seed %d page %d: %s holds no rows (orphaned header) %v", seed, pi+1, id, kidIDs(part))
				}
				var prev *Box
				for _, cur := range part.Children {
					if absolute(cur) {
						continue
					}
					if prev != nil && cur.Frame.Y < prev.Frame.Y+prev.Frame.H-1e-6 {
						t.Errorf("seed %d page %d: %s overlaps %s in %s", seed, pi+1, cur.Node.ID, prev.Node.ID, id)
					}
					prev = cur
				}
				if hid, ok := g.hdr[id]; ok {
					h := part.Children[0]
					off, seen := inset[id]
					switch {
					case h.Node.ID != hid:
						t.Errorf("seed %d page %d: %s opens with %s, want its header %s", seed, pi+1, id, h.Node.ID, hid)
					case !seen:
						inset[id] = h.Frame.Y - part.Frame.Y
					case !near(h.Frame.Y-part.Frame.Y, off):
						t.Errorf("seed %d page %d: %s's header copy sits %vpt into the part, want %v as on its first page",
							seed, pi+1, id, h.Frame.Y-part.Frame.Y, off)
					}
				}
			})
		}
	}
}

func TestPaginationHeadersRandomTightPages(t *testing.T) {
	// On 100-160pt pages nested headers and the row under them often cannot share
	// a page, so pages leave header copies off or end on a table's own header.
	// Pagination must still finish, keep every row once and in order and on the
	// page, and never print a page of header copies alone.
	for seed := int64(1); seed <= 400; seed++ {
		g, doc, pageH, pad := randomDoc(seed, 100, 160)
		res := layoutWithin(t, seed, doc)
		checkRows(t, seed, g, res, float64(pageH-pad))
		seen := map[string]bool{}
		for pi, pg := range res.Pages {
			tableParts(pg.Root, func(part *Box) {
				id := part.Node.ID
				if seen[id] && len(contentIDs(part, nil)) == 0 {
					t.Errorf("seed %d page %d: %s's continuation holds header copies alone %v", seed, pi+1, id, kidIDs(part))
				}
				seen[id] = true
			})
		}
	}
}
