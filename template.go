package feast

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/swish/feast/internal/contract"
	"github.com/swish/feast/internal/jsruntime"
	"github.com/swish/feast/internal/layout"
)

// TemplateOptions configures how a React source is compiled into a Template.
type TemplateOptions struct {
	// TypeScript parses the source as TSX instead of JSX.
	TypeScript bool
	// Filename labels the source in compile/runtime error messages.
	Filename string
}

// Template is a compiled React document that renders to PDF with varying props.
//
// The source is a JSX/TSX module whose default export is either a <Document>
// element or a function of props returning one, exactly as react-pdf documents
// are authored. It is transpiled (esbuild) and executed (goja) entirely inside
// the Go process — feast needs no Node.js at render time. A Template is safe to
// Render repeatedly, including concurrently: each Render runs on its own VM.
type Template struct {
	prog *jsruntime.Program
}

// LoadTemplate compiles a React source into a reusable Template.
func LoadTemplate(source []byte, opts TemplateOptions) (*Template, error) {
	prog, err := jsruntime.Compile(source, jsruntime.Options{
		TypeScript: opts.TypeScript,
		Filename:   opts.Filename,
	})
	if err != nil {
		return nil, err
	}
	return &Template{prog: prog}, nil
}

// Tree runs the template with props and returns the feast-tree/v1 JSON, without
// producing a PDF. Props may be any JSON-encodable value (nil for none).
func (t *Template) Tree(props any) ([]byte, error) {
	propsJSON, err := marshalProps(props)
	if err != nil {
		return nil, err
	}
	return t.prog.Render(propsJSON)
}

// Render executes the template with props and writes a PDF to w. Unlike the
// static RenderTree path, it keeps the JS VM alive so function render-props
// (render={fn}) are evaluated per page.
func (t *Template) Render(ctx context.Context, props any, w io.Writer) (*RenderInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	propsJSON, err := marshalProps(props)
	if err != nil {
		return nil, err
	}
	inst, err := t.prog.Instantiate(propsJSON)
	if err != nil {
		return nil, err
	}
	ct, err := contract.Parse(inst.Tree())
	if err != nil {
		return nil, err
	}
	return renderContract(ctx, ct, instanceEvaluator{inst}, w)
}

// instanceEvaluator adapts a live jsruntime.Instance to layout.Evaluator: it runs
// a render-prop callback on the VM and extracts the text it produced.
type instanceEvaluator struct{ inst *jsruntime.Instance }

func (e instanceEvaluator) EvalText(id string, pc layout.PageContext) (string, error) {
	ctxJSON, err := json.Marshal(map[string]int{
		"pageNumber":        pc.PageNumber,
		"totalPages":        pc.TotalPages,
		"subPageNumber":     pc.SubPageNumber,
		"subPageTotalPages": pc.SubPageTotalPages,
	})
	if err != nil {
		return "", err
	}
	nodesJSON, err := e.inst.EvalCallback(id, ctxJSON)
	if err != nil {
		return "", err
	}
	return extractText(nodesJSON), nil
}

// EvalPaint runs a Canvas paint callback on the VM and unmarshals the recorded
// painter ops for the renderer to replay.
func (e instanceEvaluator) EvalPaint(id string, w, h float64) ([]any, error) {
	opsJSON, err := e.inst.EvalPaint(id, w, h)
	if err != nil {
		return nil, err
	}
	var ops []any
	if err := json.Unmarshal(opsJSON, &ops); err != nil {
		return nil, err
	}
	return ops, nil
}

// extractText concatenates the text of all TEXT_INSTANCE nodes in a feast-tree
// node array (the shape EvalCallback returns).
func extractText(nodesJSON []byte) string {
	var nodes []json.RawMessage
	if err := json.Unmarshal(nodesJSON, &nodes); err != nil {
		return ""
	}
	var b strings.Builder
	var walk func(json.RawMessage)
	walk = func(raw json.RawMessage) {
		var n struct {
			Type     string            `json:"type"`
			Value    string            `json:"value"`
			Children []json.RawMessage `json:"children"`
		}
		if err := json.Unmarshal(raw, &n); err != nil {
			return
		}
		if n.Type == "TEXT_INSTANCE" {
			b.WriteString(n.Value)
			return
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	for _, n := range nodes {
		walk(n)
	}
	return b.String()
}

// RenderReact is a one-shot convenience: compile source and render it with props
// to a PDF in a single call. Prefer LoadTemplate when rendering the same
// document repeatedly, so compilation happens once.
func RenderReact(ctx context.Context, source []byte, props any, w io.Writer) (*RenderInfo, error) {
	t, err := LoadTemplate(source, TemplateOptions{})
	if err != nil {
		return nil, err
	}
	return t.Render(ctx, props, w)
}

func marshalProps(props any) ([]byte, error) {
	if props == nil {
		return nil, nil
	}
	b, err := json.Marshal(props)
	if err != nil {
		return nil, fmt.Errorf("feast: marshal props: %w", err)
	}
	return b, nil
}
