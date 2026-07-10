// Command appointment renders the offer/appointment-letter example to a PDF. It
// exercises waffle end-to-end on a full document: custom embedded fonts, a
// full-page letterhead background, inline bold/italic runs, numbered clauses, an
// embedded signature image, and a compensation table.
//
//	go run ./examples/appointment          # writes examples/appointment/appointment.pdf
//	go test ./examples/appointment         # generates + checks the PDF
//
// The JSX references its fonts/images by paths relative to this directory, so
// the program changes into the package directory before rendering.
package main

import (
	"context"
	_ "embed"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"github.com/SwishHQ/waffle"
)

//go:embed appointment.jsx
var appointmentJSX []byte

// packageDir is the directory of this source file, used so the JSX's relative
// asset paths (assets/…) resolve regardless of the caller's working directory.
func packageDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Dir(file)
}

// sampleProps fills the letter's placeholder tokens with representative data.
// Any subset can be overridden; unset fields fall back to the JSX's <Token>s.
func sampleProps() map[string]any {
	return map[string]any{
		"date":        "15-05-2026",
		"candidate":   "Ravi Kumar",
		"father":      "Suresh Kumar",
		"addr1":       "No. 12, 4th Cross, MG Road",
		"addr2":       "Indiranagar, Bengaluru",
		"pincode":     "560038",
		"name":        "Ravi",
		"designation": "Operations Associate",
		"joining":     "01-06-2026",
		"location":    "Bengaluru",
		"salary":      "3,60,000",
		"location2":   "Bengaluru",
		"employee":    "Ravi Kumar",
		"acceptDate":  "01-06-2026",
		// Annexure A compensation breakdown (₹). Any key may be omitted, in which
		// case the table shows that cell's <Monthly …>/<Annual …> placeholder.
		"comp": map[string]any{
			"ctc":      map[string]string{"m": "30,000", "a": "3,60,000"},
			"basic":    map[string]string{"m": "15,000", "a": "1,80,000"},
			"hra":      map[string]string{"m": "6,000", "a": "72,000"},
			"special":  map[string]string{"m": "7,200", "a": "86,400"},
			"gross":    map[string]string{"m": "28,200", "a": "3,38,400"},
			"pfEr":     map[string]string{"m": "1,800", "a": "21,600"},
			"pfEp":     map[string]string{"m": "1,800", "a": "21,600"},
			"pt":       map[string]string{"m": "200", "a": "2,400"},
			"esic":     map[string]string{"m": "0", "a": "0"},
			"totalDed": map[string]string{"m": "2,000", "a": "24,000"},
			"net":      map[string]string{"m": "26,200", "a": "3,14,400"},
		},
	}
}

func main() {
	if err := os.Chdir(packageDir()); err != nil {
		log.Fatalf("chdir: %v", err)
	}
	out, err := os.Create("appointment.pdf")
	if err != nil {
		log.Fatal(err)
	}
	defer out.Close()

	info, err := waffle.RenderReact(context.Background(), appointmentJSX, sampleProps(), out)
	if err != nil {
		log.Fatalf("render: %v", err)
	}
	fmt.Printf("wrote %s (%d page(s))\n", filepath.Join(packageDir(), "appointment.pdf"), info.PageCount)
	for _, w := range info.Warnings {
		fmt.Println("warning:", w)
	}
}
