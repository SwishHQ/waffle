package fontstore

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"testing"

	"golang.org/x/image/font/gofont/goregular"
)

// encodeWOFF wraps an SFNT font into a WOFF 1.0 container, zlib-compressing each
// table when that is smaller (else storing it). Used to exercise decodeWOFF.
func encodeWOFF(t *testing.T, sfnt []byte) []byte {
	t.Helper()
	be := binary.BigEndian
	flavor := be.Uint32(sfnt[0:])
	numTables := int(be.Uint16(sfnt[4:]))

	type out struct {
		tag, checksum, origLen uint32
		data                   []byte
	}
	outs := make([]out, numTables)
	for i := 0; i < numTables; i++ {
		p := 12 + i*16
		tag := be.Uint32(sfnt[p:])
		checksum := be.Uint32(sfnt[p+4:])
		offset := be.Uint32(sfnt[p+8:])
		length := be.Uint32(sfnt[p+12:])
		raw := sfnt[offset : offset+length]
		var zb bytes.Buffer
		zw := zlib.NewWriter(&zb)
		zw.Write(raw)
		zw.Close()
		data := zb.Bytes()
		if len(data) >= len(raw) {
			data = raw // store uncompressed
		}
		outs[i] = out{tag, checksum, length, data}
	}

	dir := new(bytes.Buffer)
	body := new(bytes.Buffer)
	offset := 44 + numTables*20
	for _, o := range outs {
		writeU32(dir, o.tag)
		writeU32(dir, uint32(offset))
		writeU32(dir, uint32(len(o.data)))
		writeU32(dir, o.origLen)
		writeU32(dir, o.checksum)
		body.Write(o.data)
		for len(o.data)%4 != 0 {
			body.WriteByte(0)
			o.data = append(o.data, 0)
		}
		offset += align4(len(o.data))
	}

	total := 44 + dir.Len() + body.Len()
	h := new(bytes.Buffer)
	h.WriteString("wOFF")
	writeU32(h, flavor)
	writeU32(h, uint32(total))
	writeU16(h, uint16(numTables))
	writeU16(h, 0)                 // reserved
	writeU32(h, uint32(len(sfnt))) // totalSfntSize
	writeU16(h, 1)                 // majorVersion
	writeU16(h, 0)                 // minorVersion
	writeU32(h, 0)                 // metaOffset
	writeU32(h, 0)                 // metaLength
	writeU32(h, 0)                 // metaOrigLength
	writeU32(h, 0)                 // privOffset
	writeU32(h, 0)                 // privLength
	h.Write(dir.Bytes())
	h.Write(body.Bytes())
	return h.Bytes()
}

func TestDecodeWOFFRoundTrip(t *testing.T) {
	woff := encodeWOFF(t, goregular.TTF)
	if string(woff[:4]) != "wOFF" {
		t.Fatal("encoder did not produce a WOFF signature")
	}
	sfnt, err := decodeWOFF(woff)
	if err != nil {
		t.Fatalf("decodeWOFF: %v", err)
	}
	if string(sfnt[:4]) == "wOFF" {
		t.Fatal("decoded output is still WOFF")
	}
	// The decoded SFNT must register and measure like the original.
	s := New()
	if err := s.Register("Web", FaceSpec{Data: woff, Weight: 400}); err != nil {
		t.Fatalf("register WOFF: %v", err)
	}
	face, ok := s.ResolveFace("Web", 400, StyleNormal)
	if !ok {
		t.Fatal("WOFF face should resolve")
	}
	if w := face.StringWidth("Hello", 12); w <= 0 {
		t.Errorf("WOFF face should measure text, got width %v", w)
	}
}

func TestSFNTBytesPassthrough(t *testing.T) {
	out, err := sfntBytes(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	if &out[0] != &goregular.TTF[0] {
		t.Error("non-WOFF data should pass through unchanged (same backing array)")
	}
}
