package layout

// PageContext is the per-page context passed to a function render-prop, mirroring
// react-pdf's { pageNumber, totalPages, subPageNumber, subPageTotalPages }.
type PageContext struct {
	PageNumber        int
	TotalPages        int
	SubPageNumber     int
	SubPageTotalPages int
}

// Evaluator evaluates a function render-prop callback (identified by its $cb id)
// with a page context, returning the text it produces. It is backed by the live
// JS VM (goja). It is nil when a document has no callbacks or is rendered from a
// static tree with no VM — in that case callback render-props resolve to empty.
//
// This resolves the common react-pdf idiom, a page-number footer:
//
//	<Text fixed render={({ pageNumber, totalPages }) => `${pageNumber} / ${totalPages}`} />
//
// A callback that returns elements has their text extracted; the styling of any
// returned elements is not yet applied (the enclosing Text's style governs).
type Evaluator interface {
	EvalText(id string, ctx PageContext) (string, error)
	// EvalPaint runs a Canvas paint callback against a painter of size w×h and
	// returns the recorded ops ({op,args} maps) for the renderer to replay.
	EvalPaint(id string, w, h float64) ([]any, error)
}
