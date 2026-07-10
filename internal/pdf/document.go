package pdf

import (
	"crypto/md5"
	"fmt"
	"io"
	"time"
)

// Options configure document-level metadata and output determinism.
type Options struct {
	Title    string
	Author   string
	Subject  string
	Keywords string
	Creator  string
	Producer string

	PDFVersion string // "1.3".."1.7"; defaults to "1.4"
	Language   string // catalog /Lang
	PageMode   string // catalog /PageMode, e.g. "UseOutlines"
	PageLayout string // catalog /PageLayout, e.g. "TwoColumnLeft"

	CreationDate time.Time // omitted from /Info when zero
	ModDate      time.Time // omitted from /Info when zero

	// UserPassword and OwnerPassword enable standard-security-handler
	// encryption (RC4 128-bit, V=2/R=3) when either is non-empty. An empty
	// user password with a non-empty owner password yields a file that opens
	// without a password but whose permissions require the owner password to
	// change.
	UserPassword  string
	OwnerPassword string
	// Permissions is the encryption dictionary's /P flags value. Zero means
	// "everything allowed" and is replaced by -4 (0xFFFFFFFC): every flag
	// bit set except bits 1–2, which are reserved and shall be 0 per
	// PDF 32000-1 Table 22. Ignored unless encryption is enabled.
	Permissions int32
}

// Document assembles a PDF: metadata, a page tree, and shared font objects.
type Document struct {
	w           *Writer
	opts        Options
	pagesRef    Reference
	kids        []Reference
	fontObjs    map[string]Reference        // base font -> font object reference
	imageObjs   map[*ImageSpec]Reference    // image spec -> XObject reference
	embFontObjs map[*EmbeddedFont]Reference // embedded font -> font object reference
	shadingObjs map[*Shading]Reference      // gradient shading -> shading object reference
	outline     []*Outline                  // document outline (bookmarks); nil if none

	formFieldRefs []Reference // AcroForm field widgets across all pages
}

// New creates a Document with the given options.
func New(opts Options) *Document {
	w := NewWriter()
	w.Version = opts.PDFVersion
	d := &Document{
		w:           w,
		opts:        opts,
		pagesRef:    w.Alloc(),
		fontObjs:    make(map[string]Reference),
		imageObjs:   make(map[*ImageSpec]Reference),
		embFontObjs: make(map[*EmbeddedFont]Reference),
		shadingObjs: make(map[*Shading]Reference),
	}
	return d
}

// fontRef returns (allocating on first use) the font object for a base font.
func (d *Document) fontRef(baseFont string) Reference {
	if r, ok := d.fontObjs[baseFont]; ok {
		return r
	}
	fd := Dict{
		Name("Type"):     Name("Font"),
		Name("Subtype"):  Name("Type1"),
		Name("BaseFont"): Name(baseFont),
	}
	if !afmUsesBuiltinEncoding(baseFont) {
		fd[Name("Encoding")] = Name("WinAnsiEncoding")
	}
	r := d.w.Add(fd)
	d.fontObjs[baseFont] = r
	return r
}

// AddPage appends a page of the given size (in points) with the given content.
// The page's /Font resources are built from the fonts the content used.
func (d *Document) AddPage(width, height float64, content *Content) {
	contentRef := d.w.Add(FlateStream(nil, content.Bytes()))

	resources := Dict{
		Name("ProcSet"): Array{Name("PDF"), Name("Text"), Name("ImageB"), Name("ImageC"), Name("ImageI")},
	}
	fontDict := Dict{}
	for _, bf := range content.Fonts() {
		res, _ := content.FontResource(bf)
		fontDict[res] = d.fontRef(bf)
	}
	for _, ef := range content.EmbeddedFonts() {
		res, _ := content.EmbeddedFontResource(ef)
		fontDict[res] = d.embFontRef(ef)
	}
	if len(fontDict) > 0 {
		resources[Name("Font")] = fontDict
	}
	if imgs := content.Images(); len(imgs) > 0 {
		xobj := Dict{}
		for _, spec := range imgs {
			res, _ := content.ImageResource(spec)
			xobj[res] = d.imageRef(spec)
		}
		resources[Name("XObject")] = xobj
	}
	if alphas := content.Alphas(); len(alphas) > 0 {
		gs := Dict{}
		for _, a := range alphas {
			res, _ := content.AlphaResource(a)
			gs[res] = Dict{Name("ca"): Real(a), Name("CA"): Real(a)}
		}
		resources[Name("ExtGState")] = gs
	}
	if shs := content.Shadings(); len(shs) > 0 {
		sd := Dict{}
		for _, sh := range shs {
			res, _ := content.ShadingResource(sh)
			sd[res] = d.shadingRef(sh)
		}
		resources[Name("Shading")] = sd
	}

	pageRef := d.w.Alloc()
	pageDict := Dict{
		Name("Type"):      Name("Page"),
		Name("Parent"):    d.pagesRef,
		Name("MediaBox"):  Array{Integer(0), Integer(0), Real(width), Real(height)},
		Name("Resources"): resources,
		Name("Contents"):  contentRef,
	}
	links, notes, fields := content.Links(), content.Notes(), content.FormFields()
	if len(links) > 0 || len(notes) > 0 || len(fields) > 0 {
		annots := make(Array, 0, len(links)+len(notes)+len(fields))
		for _, ln := range links {
			annots = append(annots, d.w.Add(linkAnnot(ln)))
		}
		for _, nt := range notes {
			annots = append(annots, d.w.Add(noteAnnot(nt)))
		}
		// A field widget is referenced both here (for page rendering) and from the
		// AcroForm /Fields array (for form semantics).
		for _, fl := range fields {
			ref := d.w.Add(fl.widgetAnnot())
			annots = append(annots, ref)
			d.formFieldRefs = append(d.formFieldRefs, ref)
		}
		pageDict[Name("Annots")] = annots
	}
	d.w.Set(pageRef, pageDict)
	d.kids = append(d.kids, pageRef)
}

// linkAnnot builds a URI link annotation dictionary (no visible border).
func linkAnnot(ln LinkAnnotation) Dict {
	return Dict{
		Name("Type"):    Name("Annot"),
		Name("Subtype"): Name("Link"),
		Name("Rect"):    Array{Real(ln.X0), Real(ln.Y0), Real(ln.X1), Real(ln.Y1)},
		Name("Border"):  Array{Integer(0), Integer(0), Integer(0)},
		Name("A"): Dict{
			Name("S"):   Name("URI"),
			Name("URI"): LiteralString(ln.URI),
		},
	}
}

// noteAnnot builds a text (sticky-note) annotation, closed by default, with a
// standard note icon. Its Rect is the small icon box at the anchor point.
func noteAnnot(nt NoteAnnotation) Dict {
	const icon = 18 // conventional note-icon size in points
	return Dict{
		Name("Type"):     Name("Annot"),
		Name("Subtype"):  Name("Text"),
		Name("Rect"):     Array{Real(nt.X), Real(nt.Y - icon), Real(nt.X + icon), Real(nt.Y)},
		Name("Contents"): TextString(nt.Text),
		Name("Name"):     Name("Note"),
		Name("Open"):     Boolean(false),
	}
}

// WriteTo finalizes the page tree, catalog, info dictionary, and file
// identifier, then serializes the document to out.
func (d *Document) WriteTo(out io.Writer) (int64, error) {
	if len(d.kids) == 0 {
		return 0, fmt.Errorf("pdf: document has no pages")
	}

	kids := make(Array, len(d.kids))
	for i, k := range d.kids {
		kids[i] = k
	}
	d.w.Set(d.pagesRef, Dict{
		Name("Type"):  Name("Pages"),
		Name("Kids"):  kids,
		Name("Count"): Integer(len(d.kids)),
	})

	catalog := Dict{
		Name("Type"):  Name("Catalog"),
		Name("Pages"): d.pagesRef,
	}
	if d.opts.Language != "" {
		catalog[Name("Lang")] = TextString(d.opts.Language)
	}
	if d.opts.PageMode != "" {
		catalog[Name("PageMode")] = Name(d.opts.PageMode)
	}
	if d.opts.PageLayout != "" {
		catalog[Name("PageLayout")] = Name(d.opts.PageLayout)
	}
	if ref, ok := d.buildOutline(); ok {
		catalog[Name("Outlines")] = ref
		if d.opts.PageMode == "" {
			catalog[Name("PageMode")] = Name("UseOutlines")
		}
	}
	if ref, ok := d.acroForm(); ok {
		catalog[Name("AcroForm")] = ref
	}
	d.w.Root = d.w.Add(catalog)

	if info := d.buildInfo(); info != nil {
		d.w.Info = d.w.Add(info)
	}
	d.w.ID = d.deriveID()
	if d.opts.UserPassword != "" || d.opts.OwnerPassword != "" {
		d.setupEncryption() // needs d.w.ID; must run after deriveID
	}

	return d.w.WriteTo(out)
}

func (d *Document) buildInfo() Dict {
	o := d.opts
	info := Dict{}
	set := func(k Name, v string) {
		if v != "" {
			info[k] = TextString(v)
		}
	}
	set(Name("Title"), o.Title)
	set(Name("Author"), o.Author)
	set(Name("Subject"), o.Subject)
	set(Name("Keywords"), o.Keywords)
	set(Name("Creator"), o.Creator)
	set(Name("Producer"), o.Producer)
	if !o.CreationDate.IsZero() {
		info[Name("CreationDate")] = LiteralString(pdfDate(o.CreationDate))
	}
	if !o.ModDate.IsZero() {
		info[Name("ModDate")] = LiteralString(pdfDate(o.ModDate))
	}
	if len(info) == 0 {
		return nil
	}
	return info
}

// deriveID computes a file identifier from the metadata and object count. Given
// deterministic inputs (e.g. a fixed CreationDate) the identifier is stable,
// which keeps golden output byte-identical.
func (d *Document) deriveID() [2][]byte {
	h := md5.New()
	io.WriteString(h, d.opts.Title)
	io.WriteString(h, d.opts.Author)
	io.WriteString(h, d.opts.Producer)
	io.WriteString(h, d.opts.Creator)
	if !d.opts.CreationDate.IsZero() {
		io.WriteString(h, d.opts.CreationDate.UTC().Format(time.RFC3339Nano))
	}
	fmt.Fprintf(h, "objects=%d", len(d.w.objs))
	sum := h.Sum(nil)
	return [2][]byte{sum, sum}
}

// pdfDate formats a time as a PDF date string in UTC, e.g. D:20260101000000+00'00'.
func pdfDate(t time.Time) string {
	t = t.UTC()
	return fmt.Sprintf("D:%04d%02d%02d%02d%02d%02d+00'00'",
		t.Year(), int(t.Month()), t.Day(), t.Hour(), t.Minute(), t.Second())
}

// afmUsesBuiltinEncoding reports whether a base font uses its own built-in
// encoding rather than WinAnsiEncoding.
func afmUsesBuiltinEncoding(baseFont string) bool {
	return baseFont == "Symbol" || baseFont == "ZapfDingbats"
}
