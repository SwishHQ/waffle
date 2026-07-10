// Command text renders a small typographic document to text.pdf, exercising the
// standard-14 font path (Helvetica/Times/Courier, weights and italics) end to end.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/swish/waffle"
)

const doc = `{
  "version": "waffle-tree/v1",
  "document": {
    "props": { "title": "waffle text demo", "author": "waffle" },
    "children": [
      { "type": "PAGE",
        "props": { "size": "A6", "style": { "padding": 28, "backgroundColor": "#f8f9fa", "gap": 14 } },
        "children": [
          { "type": "TEXT",
            "props": { "style": { "fontFamily": "Helvetica", "fontWeight": "bold", "fontSize": 34, "color": "#1d3557" } },
            "children": [ { "type": "TEXT_INSTANCE", "value": "waffle" } ] },

          { "type": "TEXT",
            "props": { "style": { "fontFamily": "Times-Roman", "fontStyle": "italic", "fontSize": 13, "color": "#457b9d" } },
            "children": [ { "type": "TEXT_INSTANCE", "value": "React-authored PDFs, rendered in Go." } ] },

          { "type": "VIEW",
            "props": { "style": { "flexDirection": "row", "gap": 8 } },
            "children": [
              { "type": "TEXT",
                "props": { "style": { "fontFamily": "Helvetica", "fontSize": 11, "color": "#e63946", "border": "1pt solid #e63946", "padding": 5 } },
                "children": [ { "type": "TEXT_INSTANCE", "value": "flexbox" } ] },
              { "type": "TEXT",
                "props": { "style": { "fontFamily": "Helvetica", "fontSize": 11, "color": "#2a9d8f", "border": "1pt solid #2a9d8f", "padding": 5 } },
                "children": [ { "type": "TEXT_INSTANCE", "value": "standard-14 fonts" } ] }
            ]
          },

          { "type": "TEXT",
            "props": { "style": { "fontFamily": "Courier", "fontSize": 11, "color": "#333333", "marginTop": 6 } },
            "children": [ { "type": "TEXT_INSTANCE", "value": "contract -> tree -> layout -> PDF" } ] },

          { "type": "TEXT",
            "props": { "style": { "fontFamily": "Times-Roman", "fontSize": 11, "color": "#1d3557", "lineHeight": 1.45, "marginTop": 4 } },
            "children": [ { "type": "TEXT_INSTANCE", "value": "This paragraph is long enough to wrap across several lines within the page, demonstrating greedy word wrapping using real advance widths from the standard fonts." } ] }
        ]
      }
    ]
  }
}`

func main() {
	f, err := os.Create("text.pdf")
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
	fmt.Printf("wrote text.pdf (%d page)\n", info.PageCount)
}
