package layout

import (
	"testing"

	"github.com/swish/waffle/internal/pdf/afm"
	"github.com/swish/waffle/internal/stylesheet"
)

// letterSpacing widens measured text by spacing × character count, and the wider
// measure is what drives wrapping/alignment.
func TestStringWidthLetterSpacing(t *testing.T) {
	m, err := afm.Load("Helvetica")
	if err != nil {
		t.Fatal(err)
	}
	tr := &textResolve{content: "abcd", size: 12, measurer: m}
	plain := tr.stringWidth("abcd")
	tr.letterSpacing = 5
	spaced := tr.stringWidth("abcd")
	if got := spaced - plain; got != 20 { // 4 runes × 5pt
		t.Errorf("letterSpacing width delta = %v, want 20", got)
	}
}

func TestStringWidthWordSpacing(t *testing.T) {
	m, err := afm.Load("Helvetica")
	if err != nil {
		t.Fatal(err)
	}
	tr := &textResolve{content: "a b c", size: 12, measurer: m}
	plain := tr.stringWidth("a b c")
	tr.wordSpacing = 10
	spaced := tr.stringWidth("a b c")
	if got := spaced - plain; got != 20 { // 2 spaces × 10pt
		t.Errorf("wordSpacing width delta = %v, want 20", got)
	}
}

func TestSpacingOf(t *testing.T) {
	ctx := stylesheet.Context{}
	if got := spacingOf(map[string]any{"letterSpacing": 2.5}, "letterSpacing", ctx); got != 2.5 {
		t.Errorf("numeric letterSpacing = %v, want 2.5", got)
	}
	if got := spacingOf(map[string]any{"wordSpacing": 4}, "wordSpacing", ctx); got != 4 {
		t.Errorf("numeric wordSpacing = %v, want 4", got)
	}
	if got := spacingOf(map[string]any{}, "letterSpacing", ctx); got != 0 {
		t.Errorf("absent spacing = %v, want 0", got)
	}
}
