package waffle

import (
	"testing"

	"github.com/SwishHQ/waffle/internal/contract"
	"github.com/SwishHQ/waffle/internal/layout"
	"github.com/SwishHQ/waffle/internal/tree"
)

// Two <Page> elements that each overflow: subPageNumber counts within its own <Page>, as in
// react-pdf, while pageNumber counts the document.
const subPageDoc = `
import { Document, Page, View, Text } from '@waffle/react';
function Section({ blocks }) {
  return (
    <Page size="A7" style={{ padding: 10 }}>
      {Array.from({ length: blocks }).map((_, i) => (
        <View key={i} style={{ height: 80, backgroundColor: '#eeeeee', marginBottom: 6 }} />
      ))}
      <Text fixed style={{ position: 'absolute', bottom: 6, left: 10, fontSize: 8 }}
        render={({ pageNumber, totalPages, subPageNumber, subPageTotalPages }) =>
          subPageNumber + '/' + subPageTotalPages + ' ' + pageNumber + '/' + totalPages} />
      <Text fixed style={{ position: 'absolute', top: 2, left: 10, fontSize: 8 }}
        render="{subPageNumber} of {subPageTotalPages}" />
    </Page>
  );
}
export default function App() {
  return (<Document><Section blocks={6} /><Section blocks={5} /></Document>);
}
`

func TestSubPageNumbersCountWithinEachPage(t *testing.T) {
	tmpl, err := LoadTemplate([]byte(subPageDoc), TemplateOptions{Filename: "subpage.jsx"})
	if err != nil {
		t.Fatalf("LoadTemplate: %v", err)
	}
	inst, err := tmpl.prog.Instantiate(nil)
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	ct, err := contract.Parse(inst.Tree())
	if err != nil {
		t.Fatalf("contract.Parse: %v", err)
	}
	tr, err := tree.Build(ct)
	if err != nil {
		t.Fatalf("tree.Build: %v", err)
	}
	res, err := layout.Layout(tr, layout.Options{Eval: instanceEvaluator{inst}})
	if err != nil {
		t.Fatalf("layout.Layout: %v", err)
	}
	// Count each section's pages from the callback text itself is circular, so derive them from
	// the page heights: section one (6 blocks) needs more pages than section two (5 blocks).
	total := len(res.Pages)
	var first int
	for i, pg := range res.Pages {
		if ft := findFooter(pg.Root); ft != nil && i > 0 && ft.Content[:2] == "1/" {
			first = i
			break
		}
	}
	if first < 2 || first >= total-1 {
		t.Fatalf("second <Page> starts at page %d of %d; want each section to overflow", first+1, total)
	}
	for i, pg := range res.Pages {
		sub, subTotal := i+1, first
		if i >= first {
			sub, subTotal = i-first+1, total-first
		}
		want := itoa(sub) + "/" + itoa(subTotal) + " " + itoa(i+1) + "/" + itoa(total)
		if ft := findFooter(pg.Root); ft == nil || ft.Content != want {
			t.Errorf("page %d callback = %v, want %q", i+1, ft, want)
		}
		wantTmpl := itoa(sub) + " of " + itoa(subTotal)
		if got := findTemplate(pg.Root); got != wantTmpl {
			t.Errorf("page %d template = %q, want %q", i+1, got, wantTmpl)
		}
	}
}

func findTemplate(b *layout.Box) string {
	if b.Text != nil && b.Text.Template != "" {
		return b.Text.Content
	}
	for _, c := range b.Children {
		if s := findTemplate(c); s != "" {
			return s
		}
	}
	return ""
}
