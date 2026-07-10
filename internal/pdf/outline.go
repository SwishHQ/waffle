package pdf

// Outline is one document-outline (bookmark) entry: a title, the destination
// page (0-based index into the pages added to the document), the destination Y
// in PDF space (points from the page bottom), and any nested child entries.
type Outline struct {
	Title    string
	Page     int
	Top      float64
	Children []*Outline
}

// SetOutline installs the document outline (bookmark tree). Passing an empty
// slice leaves the document without an /Outlines dictionary.
func (d *Document) SetOutline(roots []*Outline) { d.outline = roots }

// buildOutline serializes the outline tree and returns the /Outlines dictionary
// reference, or ok=false when there is nothing to write.
func (d *Document) buildOutline() (Reference, bool) {
	if len(d.outline) == 0 {
		return Reference{}, false
	}
	outlinesRef := d.w.Alloc()
	first, last, total := d.addOutlineItems(d.outline, outlinesRef)
	d.w.Set(outlinesRef, Dict{
		Name("Type"):  Name("Outlines"),
		Name("First"): first,
		Name("Last"):  last,
		Name("Count"): Integer(total),
	})
	return outlinesRef, true
}

// addOutlineItems writes a sibling list under parent, wiring Prev/Next/First/Last
// and per-item destinations, and returns the first and last item references plus
// the total number of descendant items (all treated as open/expanded).
func (d *Document) addOutlineItems(items []*Outline, parent Reference) (first, last Reference, total int) {
	refs := make([]Reference, len(items))
	for i := range items {
		refs[i] = d.w.Alloc()
	}
	for i, it := range items {
		dict := Dict{
			Name("Title"):  TextString(it.Title),
			Name("Parent"): parent,
		}
		if it.Page >= 0 && it.Page < len(d.kids) {
			// [page /XYZ left top zoom]; null left/zoom retain the current values.
			dict[Name("Dest")] = Array{d.kids[it.Page], Name("XYZ"), Null{}, Real(it.Top), Null{}}
		}
		if i > 0 {
			dict[Name("Prev")] = refs[i-1]
		}
		if i < len(items)-1 {
			dict[Name("Next")] = refs[i+1]
		}
		sub := 0
		if len(it.Children) > 0 {
			cf, cl, cc := d.addOutlineItems(it.Children, refs[i])
			dict[Name("First")] = cf
			dict[Name("Last")] = cl
			dict[Name("Count")] = Integer(cc) // positive: the item is open
			sub = cc
		}
		d.w.Set(refs[i], dict)
		total += 1 + sub
	}
	return refs[0], refs[len(refs)-1], total
}
