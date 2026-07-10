// Command multipage builds a data-driven document (24 rows) that overflows a
// small page, demonstrating automatic pagination. The tree is generated in Go —
// the same shape @waffle/react produces from a JSX .map() over data.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/SwishHQ/waffle"
)

var palette = []string{"#e63946", "#457b9d", "#2a9d8f", "#e9c46a", "#f4a261", "#8338ec"}

func main() {
	var rows []any
	for i := 1; i <= 24; i++ {
		rows = append(rows, map[string]any{
			"type": "VIEW",
			"props": map[string]any{"style": map[string]any{
				"height": 26, "marginBottom": 6, "backgroundColor": palette[i%len(palette)],
				"padding": 6, "justifyContent": "center",
			}},
			"children": []any{map[string]any{
				"type":  "TEXT",
				"props": map[string]any{"style": map[string]any{"color": "#ffffff", "fontSize": 11, "fontWeight": "bold"}},
				"children": []any{map[string]any{
					"type": "TEXT_INSTANCE", "value": fmt.Sprintf("Row %02d — waffle automatic pagination", i),
				}},
			}},
		})
	}

	doc := map[string]any{
		"version": "waffle-tree/v1",
		"document": map[string]any{
			"props": map[string]any{"title": "waffle pagination demo"},
			"children": []any{map[string]any{
				"type":     "PAGE",
				"props":    map[string]any{"size": "A7", "style": map[string]any{"padding": 16, "backgroundColor": "#f8f9fa"}},
				"children": rows,
			}},
		},
	}

	data, err := json.Marshal(doc)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	f, err := os.Create("multipage.pdf")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer f.Close()
	info, err := waffle.RenderTree(context.Background(), data, f)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("wrote multipage.pdf (%d pages)\n", info.PageCount)
}
