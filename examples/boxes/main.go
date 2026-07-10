// Command boxes renders a small styled View tree (a waffle-tree/v1 document) to
// boxes.pdf, demonstrating the end-to-end pipeline: contract → tree → layout →
// PDF. This is the kind of tree @waffle/react produces from JSX.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/SwishHQ/waffle"
)

const doc = `{
  "version": "waffle-tree/v1",
  "document": {
    "props": { "title": "waffle boxes demo" },
    "children": [
      { "type": "PAGE",
        "props": { "size": "A6", "style": { "padding": 24, "backgroundColor": "#f1faee" } },
        "children": [
          { "type": "VIEW",
            "props": { "style": { "flexDirection": "row", "gap": 12, "height": 90 } },
            "children": [
              { "type": "VIEW", "props": { "style": { "flexGrow": 1, "backgroundColor": "#e63946", "border": "3pt solid #1d3557" } } },
              { "type": "VIEW", "props": { "style": { "flexGrow": 2, "backgroundColor": "#457b9d" } } }
            ]
          },
          { "type": "VIEW",
            "props": { "style": { "marginTop": 16, "height": 60, "backgroundColor": "#a8dadc",
              "borderTopWidth": 6, "borderTopColor": "#e63946",
              "borderBottomWidth": 2, "borderBottomColor": "#1d3557" } }
          }
        ]
      }
    ]
  }
}`

func main() {
	f, err := os.Create("boxes.pdf")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer f.Close()

	info, err := waffle.RenderTree(context.Background(), []byte(doc), f)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("wrote boxes.pdf (%d page)\n", info.PageCount)
}
