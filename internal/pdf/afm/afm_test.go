package afm

import "testing"

func TestHelveticaKnownWidths(t *testing.T) {
	m, err := Load("Helvetica")
	if err != nil {
		t.Fatal(err)
	}
	// Known Helvetica advance widths (1000/em): H=722 e=556 l=222 l=222 o=556.
	if got := m.StringWidth1000("Hello"); got != 2278 {
		t.Fatalf("StringWidth1000(%q) = %v, want 2278", "Hello", got)
	}
	// Spot-check individual glyphs by name.
	for name, want := range map[string]float64{"space": 278, "A": 667, "W": 944, "period": 278} {
		if got, ok := m.WidthByName(name); !ok || got != want {
			t.Errorf("WidthByName(%q) = %v (ok=%v), want %v", name, got, ok, want)
		}
	}
}

func TestAllStandardFontsLoad(t *testing.T) {
	for _, name := range StandardFonts {
		m, err := Load(name)
		if err != nil {
			t.Errorf("Load(%q): %v", name, err)
			continue
		}
		if len(m.widthsByName) == 0 {
			t.Errorf("Load(%q): no glyph widths parsed", name)
		}
		if !UsesBuiltinEncoding(name) {
			if w := m.StringWidth1000("A"); w <= 0 {
				t.Errorf("Load(%q): width of 'A' = %v, want > 0", name, w)
			}
		}
	}
}

func TestWinAnsiEncoding(t *testing.T) {
	if !CanEncodeWinAnsi("Café résumé — €5") {
		t.Errorf("CanEncodeWinAnsi should handle Latin-1 + CP1252 punctuation")
	}
	if CanEncodeWinAnsi("日本語") {
		t.Errorf("CanEncodeWinAnsi should reject CJK")
	}
	// Round-trip an ASCII + Latin-1 sample through the encoder.
	enc := WinAnsiEncode("é") // U+00E9 -> 0xE9 in WinAnsi
	if len(enc) != 1 || enc[0] != 0xE9 {
		t.Errorf("WinAnsiEncode(\"é\") = % x, want e9", enc)
	}
}
