// Command feast renders a React document to a PDF.
//
// Usage:
//
//	feast <input> <out.pdf> [props.json]
//
// The input may be:
//
//   - a JSX/TSX React source (.jsx/.tsx/.js/.mjs) — transpiled and executed
//     in-process on the goja JS engine (no Node.js required). An optional
//     props.json is passed to the document's default-exported function.
//   - a feast-tree/v1 JSON document (.json) — the pre-serialized element tree,
//     rendered directly.
//
// Either way the Go engine lays it out and paints the PDF: author in React,
// render with Go.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/swish/feast"
)

func main() {
	if len(os.Args) < 3 || len(os.Args) > 4 {
		fmt.Fprintln(os.Stderr, "usage: feast <input.(jsx|tsx|json)> <out.pdf> [props.json]")
		os.Exit(2)
	}
	in, outPath := os.Args[1], os.Args[2]

	data, err := os.ReadFile(in)
	if err != nil {
		fatal(err)
	}
	out, err := os.Create(outPath)
	if err != nil {
		fatal(err)
	}
	defer out.Close()

	ctx := context.Background()
	var info *feast.RenderInfo

	switch ext := strings.ToLower(filepath.Ext(in)); ext {
	case ".json":
		info, err = feast.RenderTree(ctx, data, out)
	case ".jsx", ".tsx", ".js", ".mjs":
		props, perr := loadProps()
		if perr != nil {
			fatal(perr)
		}
		tmpl, terr := feast.LoadTemplate(data, feast.TemplateOptions{
			TypeScript: ext == ".tsx",
			Filename:   filepath.Base(in),
		})
		if terr != nil {
			fatal(terr)
		}
		info, err = tmpl.Render(ctx, props, out)
	default:
		fatal(fmt.Errorf("unsupported input %q: want .jsx, .tsx, .js, .mjs, or .json", in))
	}
	if err != nil {
		fatal(err)
	}

	fmt.Printf("wrote %s (%d page(s))\n", outPath, info.PageCount)
	for _, w := range info.Warnings {
		fmt.Fprintln(os.Stderr, "warning:", w)
	}
}

// loadProps reads the optional props.json argument into a generic value.
func loadProps() (any, error) {
	if len(os.Args) < 4 {
		return nil, nil
	}
	raw, err := os.ReadFile(os.Args[3])
	if err != nil {
		return nil, err
	}
	var props any
	if err := json.Unmarshal(raw, &props); err != nil {
		return nil, fmt.Errorf("props %s: %w", os.Args[3], err)
	}
	return props, nil
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "feast:", err)
	os.Exit(1)
}
