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

// tableGen builds random documents of tables: some with a fixed header, rows
// that are leaves or wrap={false} rows of cells, and tables nested two deep.
// Every box is small enough that a header and the row under it always share a
// page, so the header-dropping fallback never applies.
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
	if g.r.Intn(2) == 0 {
		return fmt.Sprintf(`{"type":"VIEW","props":{"id":%q,"style":{"height":%d,"marginTop":%d}}}`, id, g.between(8, 30), g.maybe(1, 4))
	}
	return fmt.Sprintf(`{"type":"VIEW","props":{"id":%q,"wrap":false,"style":{"flexDirection":"row","marginTop":%d}},"children":[%s]}`, id, g.maybe(1, 4), g.cells(g.between(8, 30)))
}

func (g *tableGen) table(depth int) string {
	tid := g.id("t")
	var kids []string
	if g.r.Intn(10) < 7 {
		hid := g.id("h")
		g.hdr[tid] = hid
		if g.r.Intn(2) == 0 {
			kids = append(kids, fmt.Sprintf(`{"type":"VIEW","props":{"id":%q,"fixed":true,"style":{"height":%d}}}`, hid, g.between(8, 20)))
		} else {
			kids = append(kids, fmt.Sprintf(`{"type":"VIEW","props":{"id":%q,"fixed":true,"style":{"flexDirection":"row"}},"children":[%s]}`, hid, g.cells(g.between(8, 20))))
		}
	}
	for i, n := 0, g.between(1, 12); i < n; i++ {
		if depth < 2 && g.r.Intn(6) == 0 {
			kids = append(kids, g.table(depth+1))
		} else {
			kids = append(kids, g.row())
		}
	}
	return fmt.Sprintf(`{"type":"VIEW","props":{"id":%q,"style":{"paddingTop":%d,"paddingBottom":%d}},"children":[%s]}`,
		tid, g.maybe(1, 6), g.maybe(1, 6), strings.Join(kids, ","))
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

func TestPaginationHeadersRandomTrees(t *testing.T) {
	for seed := int64(1); seed <= 400; seed++ {
		g := &tableGen{r: rand.New(rand.NewSource(seed)), hdr: map[string]string{}}
		pageH, pad := g.between(120, 200), g.maybe(1, 10)
		var kids []string
		if g.r.Intn(2) == 0 {
			kids = append(kids, g.row())
		}
		for i, n := 0, g.between(1, 3); i < n; i++ {
			kids = append(kids, g.table(0))
		}
		doc := fmt.Sprintf(`{"version":"waffle-tree/v1","document":{"children":[
			{"type":"PAGE","props":{"size":[200,%d],"style":{"paddingTop":%d,"paddingBottom":%d}},"children":[%s]}
		]}}`, pageH, pad, pad, strings.Join(kids, ","))

		type outcome struct {
			res *Result
			err error
		}
		done := make(chan outcome, 1)
		go func() {
			res, err := layoutDoc(doc)
			done <- outcome{res, err}
		}()
		var res *Result
		select {
		case o := <-done:
			if o.err != nil {
				t.Fatalf("seed %d: %v", seed, o.err)
			}
			res = o.res
		case <-time.After(10 * time.Second):
			t.Fatalf("seed %d: pagination did not finish", seed)
		}

		var got []string
		started := map[string]bool{}
		for pi, pg := range res.Pages {
			got = contentIDs(pg.Root, got)
			if b := rowBottom(pg.Root); b > float64(pageH-pad)+1e-6 {
				t.Errorf("seed %d page %d: a row or header reaches y=%v, past the content bottom %d", seed, pi+1, b, pageH-pad)
			}
			tableParts(pg.Root, func(part *Box) {
				id := part.Node.ID
				if len(contentIDs(part, nil)) == 0 {
					t.Errorf("seed %d page %d: %s holds no rows (orphaned header) %v", seed, pi+1, id, kidIDs(part))
				}
				for i := 1; i < len(part.Children); i++ {
					prev, cur := part.Children[i-1], part.Children[i]
					if cur.Frame.Y < prev.Frame.Y+prev.Frame.H-1e-6 {
						t.Errorf("seed %d page %d: %s overlaps %s in %s", seed, pi+1, cur.Node.ID, prev.Node.ID, id)
					}
				}
				if hid, ok := g.hdr[id]; ok {
					if part.Children[0].Node.ID != hid {
						t.Errorf("seed %d page %d: %s opens with %s, want its header %s", seed, pi+1, id, part.Children[0].Node.ID, hid)
					} else if started[id] && !near(part.Children[0].Frame.Y, part.Frame.Y) {
						t.Errorf("seed %d page %d: %s's repeated header at y=%v, want its top %v", seed, pi+1, id, part.Children[0].Frame.Y, part.Frame.Y)
					}
				}
				started[id] = true
			})
		}
		if strings.Join(got, ",") != strings.Join(g.rows, ",") {
			t.Fatalf("seed %d: rows across pages = %v, want each once in order %v", seed, got, g.rows)
		}
	}
}
