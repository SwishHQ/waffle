// This example renders a React-authored invoice to a PDF with data supplied
// from Go — the whole point of feast: author in React, drive it with Go data,
// render in-process on goja with no Node.js.
//
//	go run ./examples/react   # writes invoice.pdf
package main

import (
	"context"
	_ "embed"
	"fmt"
	"log"
	"os"

	"github.com/swish/feast"
)

//go:embed invoice.jsx
var invoiceJSX []byte

// item is a line on the invoice. Field tags map to the props the JSX reads.
type item struct {
	Name  string  `json:"name"`
	Qty   int     `json:"qty"`
	Price float64 `json:"price"`
}

func main() {
	// Compile the React template once; render it with Go-supplied props.
	tmpl, err := feast.LoadTemplate(invoiceJSX, feast.TemplateOptions{Filename: "invoice.jsx"})
	if err != nil {
		log.Fatal(err)
	}

	props := map[string]any{
		"seller":   "Acme Studio",
		"customer": "Globex Corporation",
		"number":   "2026-0142",
		"items": []item{
			{Name: "Design system audit", Qty: 1, Price: 2400},
			{Name: "Component library", Qty: 40, Price: 150},
			{Name: "Onboarding workshop", Qty: 2, Price: 800},
		},
	}

	out, err := os.Create("invoice.pdf")
	if err != nil {
		log.Fatal(err)
	}
	defer out.Close()

	info, err := tmpl.Render(context.Background(), props, out)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("wrote invoice.pdf (%d page(s))\n", info.PageCount)
}
