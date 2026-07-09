package feast

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/swish/feast/internal/contract"
	"github.com/swish/feast/internal/layout"
	"github.com/swish/feast/internal/tree"
)

// A complete react-pdf-style document authored in JSX: primitive imports, a
// function component, props, .map over data, styles, and a fixed page-number
// footer. It is compiled and rendered to a PDF entirely in-process (esbuild +
// goja) — this is the project's premise: write JSX, get a Go-rendered PDF, no
// Node.js in the loop.
const reactDoc = `
import { Document, Page, View, Text } from '@feast/react';

const Badge = ({ label, color }) => (
  <Text style={{ fontSize: 11, color, border: '1pt solid ' + color, padding: 5 }}>{label}</Text>
);

export default function Report({ title, features }) {
  return (
    <Document title={title} author="feast">
      <Page size="A6" style={{ padding: 24, backgroundColor: '#f8f9fa', gap: 12 }}>
        <Text style={{ fontSize: 22, color: '#1d3557' }}>{title}</Text>
        <Text style={{ fontSize: 12, color: '#457b9d' }}>
          Authored in React, rendered by the feast Go engine via goja.
        </Text>
        <View style={{ flexDirection: 'row', gap: 8 }}>
          {features.map((f, i) => <Badge key={i} label={f[0]} color={f[1]} />)}
        </View>
        <Text fixed render="page {pageNumber} of {totalPages}"
          style={{ position: 'absolute', bottom: 10, left: 24, fontSize: 9, color: '#888' }} />
      </Page>
    </Document>
  );
}
`

func TestRenderReactToPDF(t *testing.T) {
	tmpl, err := LoadTemplate([]byte(reactDoc), TemplateOptions{Filename: "report.jsx"})
	if err != nil {
		t.Fatalf("LoadTemplate: %v", err)
	}

	props := map[string]any{
		"title": "feast + React (goja)",
		"features": [][]string{
			{"flexbox", "#e63946"},
			{"text", "#2a9d8f"},
			{"pagination", "#457b9d"},
		},
	}

	var buf bytes.Buffer
	info, err := tmpl.Render(context.Background(), props, &buf)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if info.PageCount != 1 {
		t.Errorf("PageCount = %d, want 1", info.PageCount)
	}
	if !bytes.HasPrefix(buf.Bytes(), []byte("%PDF-")) {
		t.Fatalf("output is not a PDF (no %%PDF- header)")
	}
	if !bytes.Contains(buf.Bytes(), []byte("%%EOF")) {
		t.Errorf("PDF missing %%EOF trailer")
	}

	// Rendering again with different props must reuse the compiled template.
	var buf2 bytes.Buffer
	if _, err := tmpl.Render(context.Background(), map[string]any{"title": "second", "features": [][]string{}}, &buf2); err != nil {
		t.Fatalf("second Render: %v", err)
	}
	if buf2.Len() == 0 || !bytes.HasPrefix(buf2.Bytes(), []byte("%PDF-")) {
		t.Errorf("second render did not produce a PDF")
	}

	// Strict-validate the goja-rendered PDF (reuses findPDFCPU from feast_test.go).
	if bin := findPDFCPU(); bin != "" {
		p := filepath.Join(t.TempDir(), "react.pdf")
		if err := os.WriteFile(p, buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		if out, err := exec.Command(bin, "validate", "-m", "strict", p).CombinedOutput(); err != nil {
			t.Fatalf("pdfcpu validate failed: %v\n%s", err, out)
		}
	} else {
		t.Log("pdfcpu not found; skipping validation")
	}
}

// A document that uses hooks and context end to end, rendered to a real PDF —
// proves the hooks dispatcher + context stack work through the whole goja
// pipeline, not just at the tree level.
const hooksDoc = `
import { Document, Page, View, Text } from '@feast/react';
import React, { useState, useMemo } from 'react';

const Theme = React.createContext({ fg: '#000', bg: '#fff' });

function Heading({ children }) {
  const theme = React.useContext(Theme);
  const [prefix] = useState('§ ');
  return <Text style={{ fontSize: 20, color: theme.fg }}>{prefix + children}</Text>;
}

export default function App({ title, count }) {
  const theme = useMemo(() => ({ fg: '#1d3557', bg: '#f1faee' }), []);
  const items = useMemo(() => Array.from({ length: count }, (_, i) => 'item ' + (i + 1)), [count]);
  return (
    <Theme.Provider value={theme}>
      <Document title={title}>
        <Page size="A5" style={{ padding: 30, backgroundColor: theme.bg }}>
          <Heading>{title}</Heading>
          <View style={{ marginTop: 12, gap: 4 }}>
            {items.map((it, i) => <Text key={i} style={{ fontSize: 11 }}>{it}</Text>)}
          </View>
        </Page>
      </Document>
    </Theme.Provider>
  );
}
`

func TestRenderReactHooksToPDF(t *testing.T) {
	tmpl, err := LoadTemplate([]byte(hooksDoc), TemplateOptions{Filename: "hooks.jsx"})
	if err != nil {
		t.Fatalf("LoadTemplate: %v", err)
	}
	var buf bytes.Buffer
	info, err := tmpl.Render(context.Background(), map[string]any{"title": "Hooks Work", "count": 5}, &buf)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if info.PageCount != 1 {
		t.Errorf("PageCount = %d, want 1", info.PageCount)
	}
	if !bytes.HasPrefix(buf.Bytes(), []byte("%PDF-")) {
		t.Fatalf("output is not a PDF")
	}

	// Verify hook/context-derived content reached the tree.
	tree, err := tmpl.Tree(map[string]any{"title": "Hooks Work", "count": 5})
	if err != nil {
		t.Fatalf("Tree: %v", err)
	}
	if !bytes.Contains(tree, []byte(`"title":"Hooks Work"`)) {
		t.Errorf("title missing from tree: %s", tree)
	}
	if !bytes.Contains(tree, []byte("§ Hooks Work")) {
		t.Errorf("useContext/useState-derived heading missing from tree: %s", tree)
	}
	if bytes.Count(tree, []byte(`"fontSize":11`)) != 5 {
		t.Errorf("expected 5 useMemo-generated items, tree: %s", tree)
	}

	if bin := findPDFCPU(); bin != "" {
		p := filepath.Join(t.TempDir(), "hooks.pdf")
		if err := os.WriteFile(p, buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		if out, err := exec.Command(bin, "validate", "-m", "strict", p).CombinedOutput(); err != nil {
			t.Fatalf("pdfcpu validate failed: %v\n%s", err, out)
		}
	}
}

// A multi-page document whose fixed footer uses a function render-prop. Each
// page's footer must show that page's number — the callback bridge end to end
// (real goja VM + layout + pagination).
const callbackFooterDoc = `
import { Document, Page, View, Text } from '@feast/react';
export default function App() {
  const blocks = [1, 2, 3, 4, 5, 6];
  return (
    <Document>
      <Page size="A7" style={{ padding: 10 }}>
        {blocks.map((n, i) => (
          <View key={i} style={{ height: 80, backgroundColor: '#eeeeee', marginBottom: 6 }} />
        ))}
        <Text
          fixed
          render={({ pageNumber, totalPages }) => 'Page ' + pageNumber + ' / ' + totalPages}
          style={{ position: 'absolute', bottom: 6, left: 10, fontSize: 8 }}
        />
      </Page>
    </Document>
  );
}
`

func TestCallbackFooterRealEvaluatorPerPage(t *testing.T) {
	tmpl, err := LoadTemplate([]byte(callbackFooterDoc), TemplateOptions{Filename: "footer.jsx"})
	if err != nil {
		t.Fatalf("LoadTemplate: %v", err)
	}
	inst, err := tmpl.prog.Instantiate(nil)
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	ct, err := contract.Parse(inst.Tree())
	if err != nil {
		t.Fatalf("contract.Parse: %v", err)
	}
	tr, err := tree.Build(ct)
	if err != nil {
		t.Fatalf("tree.Build: %v", err)
	}
	res, err := layout.Layout(tr, layout.Options{Eval: instanceEvaluator{inst}})
	if err != nil {
		t.Fatalf("layout.Layout: %v", err)
	}
	if len(res.Pages) < 2 {
		t.Fatalf("pages = %d, want >= 2", len(res.Pages))
	}
	total := len(res.Pages)
	for i, pg := range res.Pages {
		ft := findFooter(pg.Root)
		if ft == nil {
			t.Fatalf("page %d: no callback footer", i+1)
		}
		want := "Page " + itoa(i+1) + " / " + itoa(total)
		if ft.Content != want {
			t.Errorf("page %d footer = %q, want %q", i+1, ft.Content, want)
		}
	}
}

func TestRenderReactCallbackFooterPDF(t *testing.T) {
	tmpl, err := LoadTemplate([]byte(callbackFooterDoc), TemplateOptions{Filename: "footer.jsx"})
	if err != nil {
		t.Fatalf("LoadTemplate: %v", err)
	}
	var buf bytes.Buffer
	info, err := tmpl.Render(context.Background(), nil, &buf)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if info.PageCount < 2 {
		t.Fatalf("PageCount = %d, want >= 2", info.PageCount)
	}
	if !bytes.HasPrefix(buf.Bytes(), []byte("%PDF-")) {
		t.Fatal("output is not a PDF")
	}
	if bin := findPDFCPU(); bin != "" {
		p := filepath.Join(t.TempDir(), "footer.pdf")
		if err := os.WriteFile(p, buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		if out, err := exec.Command(bin, "validate", "-m", "strict", p).CombinedOutput(); err != nil {
			t.Fatalf("pdfcpu validate failed: %v\n%s", err, out)
		}
	}
}

func findFooter(b *layout.Box) *layout.TextInfo {
	if b.Text != nil && b.Text.CallbackID != "" {
		return b.Text
	}
	for _, c := range b.Children {
		if ft := findFooter(c); ft != nil {
			return ft
		}
	}
	return nil
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var d []byte
	for n > 0 {
		d = append([]byte{byte('0' + n%10)}, d...)
		n /= 10
	}
	return string(d)
}

// A Canvas whose paint prop is a function, drawn via the react-pdf painter API —
// evaluated on the live VM after layout, then replayed into the PDF.
const canvasDoc = `
import { Document, Page, Canvas } from '@feast/react';
export default function App() {
  return (
    <Document>
      <Page size="A7" style={{ padding: 10 }}>
        <Canvas style={{ width: 150, height: 100 }}
          paint={(p, w, h) =>
            p.save()
             .rect(0, 0, w, h).fillColor('#457b9d').fill()
             .circle(w / 2, h / 2, 30).fillColor('#e63946').fill()
             .restore()} />
      </Page>
    </Document>
  );
}
`

func TestCanvasPaintRealEvaluator(t *testing.T) {
	tmpl, err := LoadTemplate([]byte(canvasDoc), TemplateOptions{Filename: "canvas.jsx"})
	if err != nil {
		t.Fatalf("LoadTemplate: %v", err)
	}
	inst, err := tmpl.prog.Instantiate(nil)
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	ct, err := contract.Parse(inst.Tree())
	if err != nil {
		t.Fatalf("contract.Parse: %v", err)
	}
	tr, err := tree.Build(ct)
	if err != nil {
		t.Fatalf("tree.Build: %v", err)
	}
	res, err := layout.Layout(tr, layout.Options{Eval: instanceEvaluator{inst}})
	if err != nil {
		t.Fatalf("layout.Layout: %v", err)
	}
	ops := findCanvasOps(res.Pages[0].Root)
	if len(ops) == 0 {
		t.Fatal("canvas box has no ops from the real paint callback")
	}
	// The paint drew a rect and a circle; both must be recorded.
	var haveRect, haveCircle bool
	for _, o := range ops {
		if m, ok := o.(map[string]any); ok {
			switch m["op"] {
			case "rect":
				haveRect = true
			case "circle":
				haveCircle = true
			}
		}
	}
	if !haveRect || !haveCircle {
		t.Errorf("ops missing rect/circle: rect=%v circle=%v (%d ops)", haveRect, haveCircle, len(ops))
	}
}

func TestRenderReactCanvasPaintPDF(t *testing.T) {
	tmpl, err := LoadTemplate([]byte(canvasDoc), TemplateOptions{Filename: "canvas.jsx"})
	if err != nil {
		t.Fatalf("LoadTemplate: %v", err)
	}
	var buf bytes.Buffer
	info, err := tmpl.Render(context.Background(), nil, &buf)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if info.PageCount != 1 {
		t.Errorf("PageCount = %d, want 1", info.PageCount)
	}
	if !bytes.HasPrefix(buf.Bytes(), []byte("%PDF-")) {
		t.Fatal("output is not a PDF")
	}
	if bin := findPDFCPU(); bin != "" {
		p := filepath.Join(t.TempDir(), "canvas.pdf")
		if err := os.WriteFile(p, buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		if out, err := exec.Command(bin, "validate", "-m", "strict", p).CombinedOutput(); err != nil {
			t.Fatalf("pdfcpu validate failed: %v\n%s", err, out)
		}
	}
}

func findCanvasOps(b *layout.Box) []any {
	if len(b.Canvas) > 0 {
		return b.Canvas
	}
	for _, c := range b.Children {
		if ops := findCanvasOps(c); ops != nil {
			return ops
		}
	}
	return nil
}

// A block-level <Link> must produce a clickable URI annotation in the PDF.
const linkDoc = `
import { Document, Page, View, Link, Text } from '@feast/react';
export default function App() {
  return (
    <Document>
      <Page size="A7" style={{ padding: 20 }}>
        <View>
          <Link src="https://feast.example/docs" style={{ width: 160, height: 24 }}>
            <Text style={{ fontSize: 12, color: '#1d3557' }}>Read the docs</Text>
          </Link>
        </View>
      </Page>
    </Document>
  );
}
`

func TestRenderReactLinkAnnotation(t *testing.T) {
	tmpl, err := LoadTemplate([]byte(linkDoc), TemplateOptions{Filename: "link.jsx"})
	if err != nil {
		t.Fatalf("LoadTemplate: %v", err)
	}
	var buf bytes.Buffer
	if _, err := tmpl.Render(context.Background(), nil, &buf); err != nil {
		t.Fatalf("Render: %v", err)
	}
	for _, want := range []string{"/Subtype /Link", "https://feast.example/docs", "/S /URI"} {
		if !bytes.Contains(buf.Bytes(), []byte(want)) {
			t.Errorf("PDF missing %q", want)
		}
	}
	if bin := findPDFCPU(); bin != "" {
		p := filepath.Join(t.TempDir(), "link.pdf")
		if err := os.WriteFile(p, buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		if out, err := exec.Command(bin, "validate", "-m", "strict", p).CombinedOutput(); err != nil {
			t.Fatalf("pdfcpu validate failed: %v\n%s", err, out)
		}
	}
}

// Rounded corners end to end: a filled+bordered rounded card and a circular box
// (borderRadius 50%) must produce a pdfcpu-strict-valid PDF (well-formed paths).
const roundedDoc = `
import { Document, Page, View } from '@feast/react';
export default function App() {
  return (
    <Document>
      <Page size="A7" style={{ padding: 16 }}>
        <View style={{ width: 120, height: 60, backgroundColor: '#457b9d', borderRadius: 12, border: '2pt solid #1d3557' }} />
        <View style={{ width: 40, height: 40, marginTop: 12, backgroundColor: '#e63946', borderRadius: '50%' }} />
      </Page>
    </Document>
  );
}
`

func TestRenderReactRoundedPDF(t *testing.T) {
	tmpl, err := LoadTemplate([]byte(roundedDoc), TemplateOptions{Filename: "rounded.jsx"})
	if err != nil {
		t.Fatalf("LoadTemplate: %v", err)
	}
	var buf bytes.Buffer
	if _, err := tmpl.Render(context.Background(), nil, &buf); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !bytes.HasPrefix(buf.Bytes(), []byte("%PDF-")) {
		t.Fatal("output is not a PDF")
	}
	if bin := findPDFCPU(); bin != "" {
		p := filepath.Join(t.TempDir(), "rounded.pdf")
		if err := os.WriteFile(p, buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		if out, err := exec.Command(bin, "validate", "-m", "strict", p).CombinedOutput(); err != nil {
			t.Fatalf("pdfcpu validate failed: %v\n%s", err, out)
		}
	}
}

// opacity end to end: a semi-transparent overlay must produce a strict-valid PDF
// (well-formed ExtGState / transparency, PDF 1.4+).
const opacityDoc = `
import { Document, Page, View, Text } from '@feast/react';
export default function App() {
  return (
    <Document>
      <Page size="A7" style={{ padding: 16 }}>
        <View style={{ opacity: 0.6 }}>
          <View style={{ width: 100, height: 40, backgroundColor: '#457b9d', opacity: 0.5 }} />
          <Text style={{ opacity: 0.3, fontSize: 32, color: '#e63946' }}>DRAFT</Text>
        </View>
      </Page>
    </Document>
  );
}
`

func TestRenderReactOpacityPDF(t *testing.T) {
	tmpl, err := LoadTemplate([]byte(opacityDoc), TemplateOptions{Filename: "opacity.jsx"})
	if err != nil {
		t.Fatalf("LoadTemplate: %v", err)
	}
	var buf bytes.Buffer
	if _, err := tmpl.Render(context.Background(), nil, &buf); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("/ExtGState")) {
		t.Error("expected an /ExtGState for the opacity")
	}
	if bin := findPDFCPU(); bin != "" {
		p := filepath.Join(t.TempDir(), "opacity.pdf")
		if err := os.WriteFile(p, buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		if out, err := exec.Command(bin, "validate", "-m", "strict", p).CombinedOutput(); err != nil {
			t.Fatalf("pdfcpu validate failed: %v\n%s", err, out)
		}
	}
}
