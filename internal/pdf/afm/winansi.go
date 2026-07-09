package afm

// WinAnsiEncoding tables: code (0..255) -> glyph name and Unicode rune.
// This is the standard PDF WinAnsiEncoding (a superset of ISO Latin-1 matching
// Windows CP1252 in the 0x80..0x9F range).

var (
	winAnsiName [256]string
	winAnsiRune [256]rune
	runeToCode  map[rune]byte
)

func init() {
	ascii := map[byte]string{
		0x20: "space", 0x21: "exclam", 0x22: "quotedbl", 0x23: "numbersign",
		0x24: "dollar", 0x25: "percent", 0x26: "ampersand", 0x27: "quotesingle",
		0x28: "parenleft", 0x29: "parenright", 0x2A: "asterisk", 0x2B: "plus",
		0x2C: "comma", 0x2D: "hyphen", 0x2E: "period", 0x2F: "slash",
		0x30: "zero", 0x31: "one", 0x32: "two", 0x33: "three", 0x34: "four",
		0x35: "five", 0x36: "six", 0x37: "seven", 0x38: "eight", 0x39: "nine",
		0x3A: "colon", 0x3B: "semicolon", 0x3C: "less", 0x3D: "equal",
		0x3E: "greater", 0x3F: "question", 0x40: "at",
		0x5B: "bracketleft", 0x5C: "backslash", 0x5D: "bracketright",
		0x5E: "asciicircum", 0x5F: "underscore", 0x60: "grave",
		0x7B: "braceleft", 0x7C: "bar", 0x7D: "braceright", 0x7E: "asciitilde",
	}
	for c, n := range ascii {
		winAnsiName[c] = n
		winAnsiRune[c] = rune(c)
	}
	for c := byte('A'); c <= 'Z'; c++ {
		winAnsiName[c] = string(rune(c))
		winAnsiRune[c] = rune(c)
	}
	for c := byte('a'); c <= 'z'; c++ {
		winAnsiName[c] = string(rune(c))
		winAnsiRune[c] = rune(c)
	}

	type ent struct {
		c byte
		r rune
		n string
	}
	upper := []ent{
		{0x80, 0x20AC, "Euro"}, {0x82, 0x201A, "quotesinglbase"}, {0x83, 0x0192, "florin"},
		{0x84, 0x201E, "quotedblbase"}, {0x85, 0x2026, "ellipsis"}, {0x86, 0x2020, "dagger"},
		{0x87, 0x2021, "daggerdbl"}, {0x88, 0x02C6, "circumflex"}, {0x89, 0x2030, "perthousand"},
		{0x8A, 0x0160, "Scaron"}, {0x8B, 0x2039, "guilsinglleft"}, {0x8C, 0x0152, "OE"},
		{0x8E, 0x017D, "Zcaron"}, {0x91, 0x2018, "quoteleft"}, {0x92, 0x2019, "quoteright"},
		{0x93, 0x201C, "quotedblleft"}, {0x94, 0x201D, "quotedblright"}, {0x95, 0x2022, "bullet"},
		{0x96, 0x2013, "endash"}, {0x97, 0x2014, "emdash"}, {0x98, 0x02DC, "tilde"},
		{0x99, 0x2122, "trademark"}, {0x9A, 0x0161, "scaron"}, {0x9B, 0x203A, "guilsinglright"},
		{0x9C, 0x0153, "oe"}, {0x9E, 0x017E, "zcaron"}, {0x9F, 0x0178, "Ydieresis"},
		{0xA0, 0x00A0, "space"}, {0xA1, 0x00A1, "exclamdown"}, {0xA2, 0x00A2, "cent"},
		{0xA3, 0x00A3, "sterling"}, {0xA4, 0x00A4, "currency"}, {0xA5, 0x00A5, "yen"},
		{0xA6, 0x00A6, "brokenbar"}, {0xA7, 0x00A7, "section"}, {0xA8, 0x00A8, "dieresis"},
		{0xA9, 0x00A9, "copyright"}, {0xAA, 0x00AA, "ordfeminine"}, {0xAB, 0x00AB, "guillemotleft"},
		{0xAC, 0x00AC, "logicalnot"}, {0xAD, 0x00AD, "hyphen"}, {0xAE, 0x00AE, "registered"},
		{0xAF, 0x00AF, "macron"}, {0xB0, 0x00B0, "degree"}, {0xB1, 0x00B1, "plusminus"},
		{0xB2, 0x00B2, "twosuperior"}, {0xB3, 0x00B3, "threesuperior"}, {0xB4, 0x00B4, "acute"},
		{0xB5, 0x00B5, "mu"}, {0xB6, 0x00B6, "paragraph"}, {0xB7, 0x00B7, "periodcentered"},
		{0xB8, 0x00B8, "cedilla"}, {0xB9, 0x00B9, "onesuperior"}, {0xBA, 0x00BA, "ordmasculine"},
		{0xBB, 0x00BB, "guillemotright"}, {0xBC, 0x00BC, "onequarter"}, {0xBD, 0x00BD, "onehalf"},
		{0xBE, 0x00BE, "threequarters"}, {0xBF, 0x00BF, "questiondown"}, {0xC0, 0x00C0, "Agrave"},
		{0xC1, 0x00C1, "Aacute"}, {0xC2, 0x00C2, "Acircumflex"}, {0xC3, 0x00C3, "Atilde"},
		{0xC4, 0x00C4, "Adieresis"}, {0xC5, 0x00C5, "Aring"}, {0xC6, 0x00C6, "AE"}, {0xC7, 0x00C7, "Ccedilla"},
		{0xC8, 0x00C8, "Egrave"}, {0xC9, 0x00C9, "Eacute"}, {0xCA, 0x00CA, "Ecircumflex"}, {0xCB, 0x00CB, "Edieresis"},
		{0xCC, 0x00CC, "Igrave"}, {0xCD, 0x00CD, "Iacute"}, {0xCE, 0x00CE, "Icircumflex"}, {0xCF, 0x00CF, "Idieresis"},
		{0xD0, 0x00D0, "Eth"}, {0xD1, 0x00D1, "Ntilde"}, {0xD2, 0x00D2, "Ograve"}, {0xD3, 0x00D3, "Oacute"},
		{0xD4, 0x00D4, "Ocircumflex"}, {0xD5, 0x00D5, "Otilde"}, {0xD6, 0x00D6, "Odieresis"}, {0xD7, 0x00D7, "multiply"},
		{0xD8, 0x00D8, "Oslash"}, {0xD9, 0x00D9, "Ugrave"}, {0xDA, 0x00DA, "Uacute"}, {0xDB, 0x00DB, "Ucircumflex"},
		{0xDC, 0x00DC, "Udieresis"}, {0xDD, 0x00DD, "Yacute"}, {0xDE, 0x00DE, "Thorn"}, {0xDF, 0x00DF, "germandbls"},
		{0xE0, 0x00E0, "agrave"}, {0xE1, 0x00E1, "aacute"}, {0xE2, 0x00E2, "acircumflex"}, {0xE3, 0x00E3, "atilde"},
		{0xE4, 0x00E4, "adieresis"}, {0xE5, 0x00E5, "aring"}, {0xE6, 0x00E6, "ae"}, {0xE7, 0x00E7, "ccedilla"},
		{0xE8, 0x00E8, "egrave"}, {0xE9, 0x00E9, "eacute"}, {0xEA, 0x00EA, "ecircumflex"}, {0xEB, 0x00EB, "edieresis"},
		{0xEC, 0x00EC, "igrave"}, {0xED, 0x00ED, "iacute"}, {0xEE, 0x00EE, "icircumflex"}, {0xEF, 0x00EF, "idieresis"},
		{0xF0, 0x00F0, "eth"}, {0xF1, 0x00F1, "ntilde"}, {0xF2, 0x00F2, "ograve"}, {0xF3, 0x00F3, "oacute"},
		{0xF4, 0x00F4, "ocircumflex"}, {0xF5, 0x00F5, "otilde"}, {0xF6, 0x00F6, "odieresis"}, {0xF7, 0x00F7, "divide"},
		{0xF8, 0x00F8, "oslash"}, {0xF9, 0x00F9, "ugrave"}, {0xFA, 0x00FA, "uacute"}, {0xFB, 0x00FB, "ucircumflex"},
		{0xFC, 0x00FC, "udieresis"}, {0xFD, 0x00FD, "yacute"}, {0xFE, 0x00FE, "thorn"}, {0xFF, 0x00FF, "ydieresis"},
	}
	for _, e := range upper {
		winAnsiName[e.c] = e.n
		winAnsiRune[e.c] = e.r
	}

	runeToCode = make(map[rune]byte, 256)
	for c := 0; c < 256; c++ {
		if winAnsiName[c] == "" {
			continue
		}
		if _, exists := runeToCode[winAnsiRune[c]]; !exists {
			runeToCode[winAnsiRune[c]] = byte(c)
		}
	}
}

// WinAnsiRune returns the Unicode rune a WinAnsiEncoding code maps to, or 0 if
// the code is unassigned.
func WinAnsiRune(code byte) rune { return winAnsiRune[code] }

// WinAnsiEncode encodes s to WinAnsiEncoding bytes. Runes not representable in
// WinAnsi are replaced with '?' (0x3F).
func WinAnsiEncode(s string) []byte {
	out := make([]byte, 0, len(s))
	for _, r := range s {
		if c, ok := runeToCode[r]; ok {
			out = append(out, c)
		} else {
			out = append(out, '?')
		}
	}
	return out
}

// CanEncodeWinAnsi reports whether every rune in s is representable in WinAnsi.
func CanEncodeWinAnsi(s string) bool {
	for _, r := range s {
		if _, ok := runeToCode[r]; !ok {
			return false
		}
	}
	return true
}

func winAnsiGlyphName(r rune) string {
	if c, ok := runeToCode[r]; ok {
		return winAnsiName[c]
	}
	return ""
}
