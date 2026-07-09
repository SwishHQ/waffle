// Package fontstore registers and resolves fonts and measures text. It parses
// embedded fonts with go-text/typesetting and falls back to the 14 standard PDF
// fonts (via the afm package) when an unregistered standard family is requested.
//
// Measurement here uses nominal glyph advances (no shaping); OpenType shaping
// with kerning and ligatures arrives with the text engine (textkit).
package fontstore

import (
	"bytes"
	"fmt"
	"strings"
	"sync"

	"github.com/go-text/typesetting/font"

	"github.com/swish/feast/internal/pdf/afm"
)

// Style is a font style.
type Style int

const (
	StyleNormal Style = iota
	StyleItalic
)

// ParseStyle maps a CSS font-style string to a Style ("oblique" folds to italic).
func ParseStyle(s string) Style {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "italic", "oblique":
		return StyleItalic
	default:
		return StyleNormal
	}
}

// Font is anything that can measure text; both a registered Face and a standard
// font's metrics satisfy it.
type Font interface {
	// StringWidth returns the advance width of text in points at the given size.
	StringWidth(text string, size float64) float64
}

// Face is one registered font face.
type Face struct {
	Family string
	Weight int
	Style  Style

	data   []byte
	parsed *font.Face
	upem   float64
}

// Data returns the original font bytes (for embedding).
func (f *Face) Data() []byte { return f.data }

// StringWidth measures text using nominal glyph advances.
func (f *Face) StringWidth(text string, size float64) float64 {
	if f.upem == 0 {
		return 0
	}
	var units float64
	for _, r := range text {
		gid, _ := f.parsed.NominalGlyph(r) // gid 0 (.notdef) for missing runes
		units += float64(f.parsed.HorizontalAdvance(gid))
	}
	return units * size / f.upem
}

// FaceSpec describes a face to register.
type FaceSpec struct {
	Data   []byte
	Weight int // 0 ⇒ 400
	Style  Style
}

// Store is a concurrency-safe font registry.
type Store struct {
	mu       sync.RWMutex
	families map[string][]*Face // lowercased family → faces
}

// New returns an empty Store.
func New() *Store {
	return &Store{families: make(map[string][]*Face)}
}

// Register parses and registers one or more faces for a family.
func (s *Store) Register(family string, specs ...FaceSpec) error {
	if family == "" {
		return fmt.Errorf("fontstore: empty family name")
	}
	faces := make([]*Face, 0, len(specs))
	for i, spec := range specs {
		parsed, err := font.ParseTTF(bytes.NewReader(spec.Data))
		if err != nil {
			return fmt.Errorf("fontstore: parsing %q face %d: %w", family, i, err)
		}
		weight := spec.Weight
		if weight == 0 {
			weight = 400
		}
		faces = append(faces, &Face{
			Family: family,
			Weight: weight,
			Style:  spec.Style,
			data:   spec.Data,
			parsed: parsed,
			upem:   float64(parsed.Upem()),
		})
	}
	key := strings.ToLower(family)
	s.mu.Lock()
	s.families[key] = append(s.families[key], faces...)
	s.mu.Unlock()
	return nil
}

// Resolve returns a Font for the requested family/weight/style. Registered
// families are checked first; otherwise a standard PDF font matching the family
// name is used. weight 0 is treated as 400.
func (s *Store) Resolve(family string, weight int, style Style) (Font, bool) {
	if weight == 0 {
		weight = 400
	}
	if f, ok := s.resolveRegistered(family, weight, style); ok {
		return f, true
	}
	if base, ok := StandardBaseFont(family, weight, style); ok {
		if m, err := afm.Load(base); err == nil {
			return m, true
		}
	}
	return nil, false
}

func (s *Store) resolveRegistered(family string, weight int, style Style) (*Face, bool) {
	s.mu.RLock()
	faces := s.families[strings.ToLower(family)]
	s.mu.RUnlock()
	if len(faces) == 0 {
		return nil, false
	}

	// Prefer faces of the requested style; fall back to all faces if none match.
	styled := faces[:0:0]
	for _, f := range faces {
		if f.Style == style {
			styled = append(styled, f)
		}
	}
	candidates := styled
	if len(candidates) == 0 {
		candidates = faces
	}
	return nearestWeight(candidates, weight), true
}

// nearestWeight picks the face closest to the desired weight, breaking ties
// toward the lighter weight. (A refinement of the exact CSS font-matching rule.)
func nearestWeight(faces []*Face, weight int) *Face {
	best := faces[0]
	bestDist := abs(best.Weight - weight)
	for _, f := range faces[1:] {
		d := abs(f.Weight - weight)
		if d < bestDist || (d == bestDist && f.Weight < best.Weight) {
			best, bestDist = f, d
		}
	}
	return best
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// StandardBaseFont maps a family name plus weight/style to one of the 14 standard
// PDF base fonts, or reports false if the family is not a standard one.
func StandardBaseFont(family string, weight int, style Style) (string, bool) {
	bold := weight >= 600
	italic := style == StyleItalic
	pick := func(base, b, i, bi string) string {
		switch {
		case bold && italic:
			return bi
		case bold:
			return b
		case italic:
			return i
		default:
			return base
		}
	}
	switch strings.ToLower(strings.TrimSpace(family)) {
	case "helvetica":
		return pick("Helvetica", "Helvetica-Bold", "Helvetica-Oblique", "Helvetica-BoldOblique"), true
	case "times-roman", "times":
		return pick("Times-Roman", "Times-Bold", "Times-Italic", "Times-BoldItalic"), true
	case "courier":
		return pick("Courier", "Courier-Bold", "Courier-Oblique", "Courier-BoldOblique"), true
	case "symbol":
		return "Symbol", true
	case "zapfdingbats":
		return "ZapfDingbats", true
	}
	return "", false
}
