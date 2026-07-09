// Package afm parses Adobe Font Metrics for the 14 standard PDF fonts and
// exposes glyph widths and font metrics used for text measurement.
//
// The .afm files under metrics/ are the Adobe Core 14 metrics as redistributed
// by the Apache PDFBox project; Adobe's notice permits redistribution of the
// metric data. They are embedded into the binary so no external files are
// needed at runtime.
package afm

import (
	"bufio"
	"bytes"
	"embed"
	"fmt"
	"strconv"
	"strings"
	"sync"
)

//go:embed metrics/*.afm
var metricsFS embed.FS

// Metrics holds the parsed metrics for one standard font.
type Metrics struct {
	FontName    string
	CapHeight   float64
	XHeight     float64
	Ascender    float64
	Descender   float64
	ItalicAngle float64
	FontBBox    [4]float64

	widthsByName map[string]float64 // glyph name -> advance width (1000/em units)
	widthsByCode map[int]float64    // built-in encoding code -> advance width
}

// StandardFonts lists the 14 standard font BaseFont names.
var StandardFonts = []string{
	"Courier", "Courier-Bold", "Courier-Oblique", "Courier-BoldOblique",
	"Helvetica", "Helvetica-Bold", "Helvetica-Oblique", "Helvetica-BoldOblique",
	"Times-Roman", "Times-Bold", "Times-Italic", "Times-BoldItalic",
	"Symbol", "ZapfDingbats",
}

// IsStandard reports whether name is one of the 14 standard fonts.
func IsStandard(name string) bool {
	for _, f := range StandardFonts {
		if f == name {
			return true
		}
	}
	return false
}

// UsesBuiltinEncoding reports whether the font uses its own built-in encoding
// (Symbol and ZapfDingbats) rather than WinAnsiEncoding.
func UsesBuiltinEncoding(name string) bool {
	return name == "Symbol" || name == "ZapfDingbats"
}

var (
	cacheMu sync.Mutex
	cache   = map[string]*Metrics{}
)

// Load returns the metrics for a standard font, parsing and caching on first use.
func Load(baseFont string) (*Metrics, error) {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	if m, ok := cache[baseFont]; ok {
		return m, nil
	}
	data, err := metricsFS.ReadFile("metrics/" + baseFont + ".afm")
	if err != nil {
		return nil, fmt.Errorf("afm: unknown standard font %q", baseFont)
	}
	m, err := parse(data)
	if err != nil {
		return nil, fmt.Errorf("afm: parsing %q: %w", baseFont, err)
	}
	cache[baseFont] = m
	return m, nil
}

func parse(data []byte) (*Metrics, error) {
	m := &Metrics{
		widthsByName: make(map[string]float64),
		widthsByCode: make(map[int]float64),
	}
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	inChars := false
	for sc.Scan() {
		line := sc.Text()
		if inChars {
			if strings.HasPrefix(line, "EndCharMetrics") {
				inChars = false
				continue
			}
			parseCharLine(line, m)
			continue
		}
		switch {
		case strings.HasPrefix(line, "StartCharMetrics"):
			inChars = true
		case strings.HasPrefix(line, "FontName "):
			m.FontName = strings.TrimSpace(line[len("FontName "):])
		case strings.HasPrefix(line, "CapHeight "):
			m.CapHeight = firstFloat(line)
		case strings.HasPrefix(line, "XHeight "):
			m.XHeight = firstFloat(line)
		case strings.HasPrefix(line, "Ascender "):
			m.Ascender = firstFloat(line)
		case strings.HasPrefix(line, "Descender "):
			m.Descender = firstFloat(line)
		case strings.HasPrefix(line, "ItalicAngle "):
			m.ItalicAngle = firstFloat(line)
		case strings.HasPrefix(line, "FontBBox "):
			f := strings.Fields(line)
			if len(f) >= 5 {
				m.FontBBox = [4]float64{atof(f[1]), atof(f[2]), atof(f[3]), atof(f[4])}
			}
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return m, nil
}

// parseCharLine parses a line such as: "C 32 ; WX 278 ; N space ;"
func parseCharLine(line string, m *Metrics) {
	code := -1
	var wx float64
	var name string
	haveWX := false
	for _, part := range strings.Split(line, ";") {
		f := strings.Fields(part)
		if len(f) < 2 {
			continue
		}
		switch f[0] {
		case "C":
			code = atoi(f[1])
		case "WX":
			wx = atof(f[1])
			haveWX = true
		case "N":
			name = f[1]
		}
	}
	if !haveWX {
		return
	}
	if name != "" {
		m.widthsByName[name] = wx
	}
	if code >= 0 {
		m.widthsByCode[code] = wx
	}
}

// WidthByName returns the advance width (1000/em units) for a glyph name.
func (m *Metrics) WidthByName(name string) (float64, bool) {
	w, ok := m.widthsByName[name]
	return w, ok
}

// WidthByCode returns the advance width for a built-in encoding code.
func (m *Metrics) WidthByCode(code int) (float64, bool) {
	w, ok := m.widthsByCode[code]
	return w, ok
}

// RuneWidth1000 returns the advance width of r in 1000/em units under
// WinAnsiEncoding. Runes not representable in WinAnsi fall back to the space
// width (or 0 if the font has no space glyph).
func (m *Metrics) RuneWidth1000(r rune) float64 {
	if name := winAnsiGlyphName(r); name != "" {
		if w, ok := m.widthsByName[name]; ok {
			return w
		}
	}
	if w, ok := m.widthsByName["space"]; ok {
		return w
	}
	return 0
}

// StringWidth1000 returns the total advance width of s in 1000/em units under
// WinAnsiEncoding.
func (m *Metrics) StringWidth1000(s string) float64 {
	var total float64
	for _, r := range s {
		total += m.RuneWidth1000(r)
	}
	return total
}

// StringWidth returns the advance width of s in points at the given font size,
// under WinAnsiEncoding.
func (m *Metrics) StringWidth(s string, fontSize float64) float64 {
	return m.StringWidth1000(s) * fontSize / 1000
}

func firstFloat(line string) float64 {
	f := strings.Fields(line)
	if len(f) < 2 {
		return 0
	}
	return atof(f[1])
}

func atof(s string) float64 {
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

func atoi(s string) int {
	v, _ := strconv.Atoi(s)
	return v
}
