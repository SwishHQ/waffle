package main

import (
	"context"
	"io"
	"os"
	"testing"

	"github.com/swish/waffle"
)

// benchTemplate compiles the appointment letter and warms its asset cache
// (letterhead + signature images, four embedded TTF faces) with one render, so
// the benchmark measures the steady-state per-render cost.
func benchTemplate(b *testing.B) (*waffle.Template, map[string]any) {
	b.Helper()
	if err := os.Chdir(packageDir()); err != nil {
		b.Fatalf("chdir: %v", err)
	}
	tmpl, err := waffle.LoadTemplate(appointmentJSX, waffle.TemplateOptions{Filename: "appointment.jsx"})
	if err != nil {
		b.Fatalf("compile: %v", err)
	}
	props := sampleProps()
	if _, err := tmpl.Render(context.Background(), props, io.Discard); err != nil {
		b.Fatalf("warmup render: %v", err)
	}
	return tmpl, props
}

// BenchmarkAppointmentRender measures one full render of the 8-page letter:
// full-page letterhead on every page, four embedded fonts, inline bold/italic
// runs, numbered clauses, a signature image, and the compensation table.
func BenchmarkAppointmentRender(b *testing.B) {
	tmpl, props := benchTemplate(b)
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := tmpl.Render(ctx, props, io.Discard); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkAppointmentRenderParallel renders the letter concurrently across
// GOMAXPROCS goroutines from one shared Template.
func BenchmarkAppointmentRenderParallel(b *testing.B) {
	tmpl, props := benchTemplate(b)
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, err := tmpl.Render(ctx, props, io.Discard); err != nil {
				b.Fatal(err)
			}
		}
	})
}
