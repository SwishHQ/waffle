package pdf

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
)

// Writer manages the indirect object table and serializes a complete PDF file
// (header, body, cross-reference table, trailer). Object numbers are assigned in
// allocation order, which callers drive deterministically.
type Writer struct {
	objs []Object // objs[i] is object number i+1; nil means allocated-but-unset

	Root    Reference // catalog; required
	Info    Reference // document info dict; optional
	ID      [2][]byte // file identifier halves; optional
	Version string    // e.g. "1.4"; defaults to "1.4"
}

// NewWriter returns an empty Writer.
func NewWriter() *Writer { return &Writer{} }

// Alloc reserves an object number without assigning content.
func (w *Writer) Alloc() Reference {
	w.objs = append(w.objs, nil)
	return Reference{Num: len(w.objs), Gen: 0}
}

// Set assigns content to a previously allocated reference.
func (w *Writer) Set(ref Reference, obj Object) {
	w.objs[ref.Num-1] = obj
}

// Add appends a new object and returns its reference.
func (w *Writer) Add(obj Object) Reference {
	ref := w.Alloc()
	w.Set(ref, obj)
	return ref
}

// WriteTo serializes the document to out. The whole file is assembled in memory
// so that byte offsets for the cross-reference table are exact.
func (w *Writer) WriteTo(out io.Writer) (int64, error) {
	if !w.Root.valid() {
		return 0, fmt.Errorf("pdf: writer has no /Root catalog")
	}
	version := w.Version
	if version == "" {
		version = "1.4"
	}

	var buf bytes.Buffer
	buf.WriteString("%PDF-")
	buf.WriteString(version)
	buf.WriteByte('\n')
	// Binary marker comment: four bytes > 127 so tools treat the file as binary.
	buf.Write([]byte{'%', 0xE2, 0xE3, 0xCF, 0xD3, '\n'})

	offsets := make([]int, len(w.objs))
	for i, o := range w.objs {
		if o == nil {
			return 0, fmt.Errorf("pdf: object %d allocated but never set", i+1)
		}
		offsets[i] = buf.Len()
		fmt.Fprintf(&buf, "%d 0 obj\n", i+1)
		o.encode(&buf)
		buf.WriteString("\nendobj\n")
	}

	xrefOff := buf.Len()
	n := len(w.objs)
	fmt.Fprintf(&buf, "xref\n0 %d\n", n+1)
	buf.WriteString("0000000000 65535 f \n")
	for i := 0; i < n; i++ {
		fmt.Fprintf(&buf, "%010d 00000 n \n", offsets[i])
	}

	buf.WriteString("trailer\n")
	tr := Dict{Name("Size"): Integer(n + 1)}
	tr[Name("Root")] = w.Root
	if w.Info.valid() {
		tr[Name("Info")] = w.Info
	}
	if len(w.ID[0]) > 0 || len(w.ID[1]) > 0 {
		tr[Name("ID")] = Array{HexString(w.ID[0]), HexString(w.ID[1])}
	}
	tr.encode(&buf)
	fmt.Fprintf(&buf, "\nstartxref\n%d\n%%%%EOF\n", xrefOff)

	n, err := out.Write(buf.Bytes())
	return int64(n), err
}

// FlateStream returns a stream object whose data is zlib/FlateDecode-compressed.
// extra dictionary entries (if any) are merged in alongside /Filter.
func FlateStream(extra Dict, data []byte) *Stream {
	var zbuf bytes.Buffer
	zw := zlib.NewWriter(&zbuf)
	_, _ = zw.Write(data)
	_ = zw.Close()

	d := Dict{Name("Filter"): Name("FlateDecode")}
	for k, v := range extra {
		d[k] = v
	}
	return &Stream{Dict: d, Data: zbuf.Bytes()}
}
