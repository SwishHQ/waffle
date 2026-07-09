// Package pdf is a write-only PDF 1.3–1.7 serializer: the low-level object
// model, document/xref writer, content-stream operator builder, and standard-14
// font support. It is the Go replacement for react-pdf's pdfkit fork. Higher
// layers (layout, render) build documents through this package.
package pdf

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"
)

// Object is any PDF object that can serialize itself as PDF syntax.
type Object interface {
	encode(buf *bytes.Buffer)
}

// Null is the PDF null object.
type Null struct{}

func (Null) encode(b *bytes.Buffer) { b.WriteString("null") }

// Boolean is a PDF boolean.
type Boolean bool

func (v Boolean) encode(b *bytes.Buffer) {
	if v {
		b.WriteString("true")
	} else {
		b.WriteString("false")
	}
}

// Integer is a PDF integer.
type Integer int64

func (v Integer) encode(b *bytes.Buffer) { b.WriteString(strconv.FormatInt(int64(v), 10)) }

// Real is a PDF real number.
type Real float64

func (v Real) encode(b *bytes.Buffer) { b.WriteString(formatReal(float64(v))) }

// Name is a PDF name object (written with a leading slash).
type Name string

func (v Name) encode(b *bytes.Buffer) {
	b.WriteByte('/')
	for i := 0; i < len(v); i++ {
		c := v[i]
		if c < 0x21 || c > 0x7e || isDelimiter(c) || c == '#' {
			fmt.Fprintf(b, "#%02X", c)
		} else {
			b.WriteByte(c)
		}
	}
}

// LiteralString is a PDF literal string; the bytes are written verbatim inside
// parentheses with the required escaping.
type LiteralString string

func (v LiteralString) encode(b *bytes.Buffer) {
	b.WriteByte('(')
	for i := 0; i < len(v); i++ {
		c := v[i]
		switch c {
		case '(', ')', '\\':
			b.WriteByte('\\')
			b.WriteByte(c)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		default:
			if c < 0x20 || c > 0x7e {
				fmt.Fprintf(b, "\\%03o", c)
			} else {
				b.WriteByte(c)
			}
		}
	}
	b.WriteByte(')')
}

// HexString is a PDF hexadecimal string.
type HexString []byte

func (v HexString) encode(b *bytes.Buffer) {
	const hexdig = "0123456789ABCDEF"
	b.WriteByte('<')
	for _, c := range v {
		b.WriteByte(hexdig[c>>4])
		b.WriteByte(hexdig[c&0xf])
	}
	b.WriteByte('>')
}

// TextString returns a PDF text string: a literal ASCII string when possible,
// otherwise a UTF-16BE hex string with a byte-order mark (PDF text string form).
func TextString(s string) Object {
	ascii := true
	for i := 0; i < len(s); i++ {
		if s[i] < 0x20 || s[i] > 0x7e {
			ascii = false
			break
		}
	}
	if ascii {
		return LiteralString(s)
	}
	u := utf16.Encode([]rune(s))
	buf := make([]byte, 0, 2+len(u)*2)
	buf = append(buf, 0xFE, 0xFF)
	for _, r := range u {
		buf = append(buf, byte(r>>8), byte(r))
	}
	return HexString(buf)
}

// Array is a PDF array.
type Array []Object

func (v Array) encode(b *bytes.Buffer) {
	b.WriteByte('[')
	for i, o := range v {
		if i > 0 {
			b.WriteByte(' ')
		}
		o.encode(b)
	}
	b.WriteByte(']')
}

// Dict is a PDF dictionary. Keys are serialized in sorted order so that output
// is deterministic.
type Dict map[Name]Object

func (v Dict) encode(b *bytes.Buffer) {
	keys := make([]string, 0, len(v))
	for k := range v {
		keys = append(keys, string(k))
	}
	sort.Strings(keys)
	b.WriteString("<<")
	for _, k := range keys {
		b.WriteByte(' ')
		Name(k).encode(b)
		b.WriteByte(' ')
		v[Name(k)].encode(b)
	}
	b.WriteString(" >>")
}

// Reference is an indirect reference to another object.
type Reference struct {
	Num int
	Gen int
}

func (v Reference) encode(b *bytes.Buffer) {
	fmt.Fprintf(b, "%d %d R", v.Num, v.Gen)
}

// valid reports whether the reference points at an allocated object.
func (v Reference) valid() bool { return v.Num > 0 }

// Stream is a PDF stream object: a dictionary plus raw bytes. Streams must be
// indirect objects. The /Length entry is filled in automatically at encode time.
type Stream struct {
	Dict Dict
	Data []byte
}

func (v *Stream) encode(b *bytes.Buffer) {
	d := make(Dict, len(v.Dict)+1)
	for k, val := range v.Dict {
		d[k] = val
	}
	d[Name("Length")] = Integer(len(v.Data))
	d.encode(b)
	b.WriteString("\nstream\n")
	b.Write(v.Data)
	b.WriteString("\nendstream")
}

func formatReal(f float64) string {
	if f == 0 {
		return "0"
	}
	s := strconv.FormatFloat(f, 'f', 6, 64)
	if strings.Contains(s, ".") {
		s = strings.TrimRight(s, "0")
		s = strings.TrimRight(s, ".")
	}
	if s == "" || s == "-0" {
		return "0"
	}
	return s
}

func isDelimiter(c byte) bool {
	switch c {
	case '(', ')', '<', '>', '[', ']', '{', '}', '/', '%':
		return true
	}
	return false
}
