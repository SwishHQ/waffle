// Command hello renders a minimal single-page PDF using the Phase 1 writer core.
// It writes hello.pdf in the current directory.
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/swish/feast/internal/pdf"
)

// A4 in points (72 dpi).
const (
	a4Width  = 595.28
	a4Height = 841.89
)

func main() {
	c := pdf.NewContent()
	c.BeginText().
		SetFont("Helvetica", 24).
		TextPosition(72, a4Height-72).
		ShowText("Hello, World!").
		EndText()

	doc := pdf.New(pdf.Options{
		Title:        "Hello",
		Creator:      "feast",
		Producer:     "feast",
		PDFVersion:   "1.4",
		CreationDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	})
	doc.AddPage(a4Width, a4Height, c)

	f, err := os.Create("hello.pdf")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer f.Close()
	if _, err := doc.WriteTo(f); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("wrote hello.pdf")
}
