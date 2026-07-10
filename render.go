package feast

import (
	"context"
	"encoding/json"
	"io"

	"github.com/swish/feast/internal/contract"
	"github.com/swish/feast/internal/layout"
	"github.com/swish/feast/internal/pdf"
	"github.com/swish/feast/internal/render"
	"github.com/swish/feast/internal/tree"
)

// RenderInfo reports the outcome of a render.
type RenderInfo struct {
	PageCount int
	Warnings  []string
}

// RenderTree renders a feast-tree/v1 document (as produced by @feast/react) to a
// PDF written to w. It runs the full pipeline: parse the contract, build and
// validate the element tree, lay it out, and paint it.
//
// This is the static-tree entry point (execution mode 2). It does not evaluate
// function-valued props (render callbacks, canvas paint, hyphenation); documents
// that require them need a VM execution mode, added in a later phase.
func RenderTree(ctx context.Context, treeJSON []byte, w io.Writer) (*RenderInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ct, err := contract.Parse(treeJSON)
	if err != nil {
		return nil, err
	}
	return renderContract(ctx, ct, nil, w)
}

// renderContract runs the layout+paint pipeline on a parsed contract tree. eval
// evaluates function render-props per page (nil for the static path).
func renderContract(ctx context.Context, ct *contract.Tree, eval layout.Evaluator, w io.Writer) (*RenderInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	tr, err := tree.Build(ct)
	if err != nil {
		return nil, err
	}
	res, err := layout.Layout(tr, layout.Options{Eval: eval})
	if err != nil {
		return nil, err
	}
	if err := render.Render(res, w, render.Options{Doc: docOptions(tr.Root)}); err != nil {
		return nil, err
	}
	return &RenderInfo{PageCount: len(res.Pages), Warnings: res.Warnings}, nil
}

// docOptions maps a Document node's props to PDF document metadata.
func docOptions(root *tree.Node) pdf.Options {
	get := func(k string) string {
		s, _ := root.Props[k].(string)
		return s
	}
	o := pdf.Options{
		Title:      get("title"),
		Author:     get("author"),
		Subject:    get("subject"),
		Keywords:   get("keywords"),
		Creator:    get("creator"),
		Producer:   get("producer"),
		PDFVersion: get("pdfVersion"),
		Language:   get("language"),
		// react-pdf's camelCase viewer prefs map to PDF names by capitalizing:
		// "useOutlines" → "UseOutlines", "twoColumnLeft" → "TwoColumnLeft".
		PageMode:   ucfirst(get("pageMode")),
		PageLayout: ucfirst(get("pageLayout")),
		// Encryption (standard security handler) when a password is present.
		UserPassword:  get("userPassword"),
		OwnerPassword: get("ownerPassword"),
	}
	if p, ok := propInt32(root.Props, "permissions"); ok {
		o.Permissions = p
	}
	// encryptionMethod selects the cipher: "aes256" (AES-256), "aes" (AES-128),
	// or the default RC4-128.
	switch get("encryptionMethod") {
	case "aes256", "AES256":
		o.EncryptAES256 = true
	case "aes", "AES":
		o.EncryptAES = true
	}
	if o.Creator == "" {
		o.Creator = "feast"
	}
	if o.Producer == "" {
		o.Producer = "feast"
	}
	return o
}

// propInt32 reads a numeric prop as int32 (props carry json.Number).
func propInt32(p map[string]any, key string) (int32, bool) {
	switch v := p[key].(type) {
	case json.Number:
		if n, err := v.Int64(); err == nil {
			return int32(n), true
		}
	case float64:
		return int32(v), true
	case int:
		return int32(v), true
	}
	return 0, false
}

func ucfirst(s string) string {
	if s == "" {
		return ""
	}
	b := []byte(s)
	if b[0] >= 'a' && b[0] <= 'z' {
		b[0] -= 'a' - 'A'
	}
	return string(b)
}
