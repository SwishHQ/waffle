package pdf

import "github.com/swish/waffle/internal/pdf/afm"

// MeasureText returns the advance width in points of text set in a standard font
// at the given size, under WinAnsiEncoding. It errors if baseFont is not one of
// the 14 standard fonts.
func MeasureText(baseFont string, size float64, text string) (float64, error) {
	m, err := afm.Load(baseFont)
	if err != nil {
		return 0, err
	}
	return m.StringWidth(text, size), nil
}

// StandardFonts returns the 14 standard PDF font names.
func StandardFonts() []string { return afm.StandardFonts }

// IsStandardFont reports whether name is one of the 14 standard fonts.
func IsStandardFont(name string) bool { return afm.IsStandard(name) }
