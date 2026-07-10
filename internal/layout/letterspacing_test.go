package layout

import (
	"testing"

	"github.com/swish/feast/internal/pdf/afm"
	"github.com/swish/feast/internal/stylesheet"
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

func TestLetterSpacingOf(t *testing.T) {
	ctx := stylesheet.Context{}
	if got := letterSpacingOf(map[string]any{"letterSpacing": 2.5}, ctx); got != 2.5 {
		t.Errorf("numeric letterSpacing = %v, want 2.5", got)
	}
	if got := letterSpacingOf(map[string]any{}, ctx); got != 0 {
		t.Errorf("absent letterSpacing = %v, want 0", got)
	}
}
