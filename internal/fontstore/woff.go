package fontstore

// WOFF 1.0 decoding: a WOFF file wraps the SFNT (TrueType/OpenType) tables,
// each optionally zlib-compressed. PDF embeds raw SFNT, and the font parser
// wants SFNT too, so a registered .woff face is first rebuilt into an SFNT byte
// stream. WOFF2 (Brotli + table transforms) is not handled.

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
	"sort"
)

// sfntBytes returns SFNT bytes for a font program: WOFF input is decoded to
// SFNT, and anything else (already SFNT) is returned unchanged.
func sfntBytes(data []byte) ([]byte, error) {
	if len(data) >= 4 && string(data[:4]) == "wOFF" {
		return decodeWOFF(data)
	}
	return data, nil
}

// decodeWOFF rebuilds the SFNT font from a WOFF 1.0 container.
func decodeWOFF(data []byte) ([]byte, error) {
	const (
		woffHeader = 44
		woffEntry  = 20
	)
	if len(data) < woffHeader {
		return nil, fmt.Errorf("fontstore: WOFF too short")
	}
	be := binary.BigEndian
	flavor := be.Uint32(data[4:])
	numTables := int(be.Uint16(data[12:]))

	type table struct {
		tag      uint32
		checksum uint32
		orig     uint32
		data     []byte
	}
	tables := make([]table, 0, numTables)
	p := woffHeader
	for i := 0; i < numTables; i++ {
		if p+woffEntry > len(data) {
			return nil, fmt.Errorf("fontstore: WOFF directory truncated")
		}
		tag := be.Uint32(data[p:])
		offset := be.Uint32(data[p+4:])
		compLen := be.Uint32(data[p+8:])
		origLen := be.Uint32(data[p+12:])
		checksum := be.Uint32(data[p+16:])
		p += woffEntry

		if int(offset) > len(data) || int(offset)+int(compLen) > len(data) {
			return nil, fmt.Errorf("fontstore: WOFF table %d out of range", i)
		}
		raw := data[offset : offset+compLen]
		var td []byte
		if compLen < origLen { // zlib-compressed
			zr, err := zlib.NewReader(bytes.NewReader(raw))
			if err != nil {
				return nil, fmt.Errorf("fontstore: WOFF table %d zlib: %w", i, err)
			}
			td, err = io.ReadAll(zr)
			zr.Close()
			if err != nil {
				return nil, fmt.Errorf("fontstore: WOFF table %d inflate: %w", i, err)
			}
		} else { // stored uncompressed
			td = append([]byte(nil), raw...)
		}
		tables = append(tables, table{tag: tag, checksum: checksum, orig: origLen, data: td})
	}
	// SFNT requires the table directory sorted by tag.
	sort.Slice(tables, func(i, j int) bool { return tables[i].tag < tables[j].tag })

	var buf bytes.Buffer
	sr, es, rs := searchParams(numTables)
	writeU32(&buf, flavor)
	writeU16(&buf, uint16(numTables))
	writeU16(&buf, sr)
	writeU16(&buf, es)
	writeU16(&buf, rs)

	offset := 12 + numTables*16
	for _, tb := range tables {
		writeU32(&buf, tb.tag)
		writeU32(&buf, tb.checksum)
		writeU32(&buf, uint32(offset))
		writeU32(&buf, tb.orig)
		offset += align4(len(tb.data))
	}
	for _, tb := range tables {
		buf.Write(tb.data)
		for pad := len(tb.data); pad%4 != 0; pad++ {
			buf.WriteByte(0)
		}
	}
	return buf.Bytes(), nil
}

// searchParams computes the SFNT offset-table binary-search fields for n tables.
func searchParams(n int) (searchRange, entrySelector, rangeShift uint16) {
	pow, sel := 1, 0
	for pow*2 <= n {
		pow *= 2
		sel++
	}
	searchRange = uint16(pow * 16)
	entrySelector = uint16(sel)
	rangeShift = uint16(n*16) - searchRange
	return
}

func align4(n int) int { return (n + 3) &^ 3 }

func writeU16(b *bytes.Buffer, v uint16) {
	b.WriteByte(byte(v >> 8))
	b.WriteByte(byte(v))
}

func writeU32(b *bytes.Buffer, v uint32) {
	b.WriteByte(byte(v >> 24))
	b.WriteByte(byte(v >> 16))
	b.WriteByte(byte(v >> 8))
	b.WriteByte(byte(v))
}
