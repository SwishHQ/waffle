package pdf

import "fmt"

// FormField is an interactive AcroForm field rendered as a widget annotation on a
// page. Only text fields are supported for now.
type FormField struct {
	Name      string  // fully-qualified field name (/T)
	Value     string  // current value (/V)
	FontSize  float64 // 0 means auto-size (/DA "0 Tf")
	MultiLine bool    // text field flag: multi-line
	Password  bool    // text field flag: password (value masked)

	X0, Y0, X1, Y1 float64 // widget rectangle in page (default user) space
}

// AddFormField records an interactive form field widget for this page.
func (c *Content) AddFormField(f FormField) { c.fields = append(c.fields, f) }

// FormFields returns the form fields recorded for this page.
func (c *Content) FormFields() []FormField { return c.fields }

// textFieldFlags builds the /Ff bit field for a text field.
func (f FormField) textFieldFlags() int {
	const (
		flagMultiline = 1 << 12 // bit 13
		flagPassword  = 1 << 13 // bit 14
	)
	ff := 0
	if f.MultiLine {
		ff |= flagMultiline
	}
	if f.Password {
		ff |= flagPassword
	}
	return ff
}

// widgetAnnot builds a text-field widget annotation dictionary. The widget also
// serves as the terminal field (its /T makes it a field), so it is referenced
// both from the page /Annots and the AcroForm /Fields.
func (f FormField) widgetAnnot() Dict {
	da := fmt.Sprintf("/Helv %s Tf 0 g", num(f.FontSize)) // 0 size => auto
	d := Dict{
		Name("Type"):    Name("Annot"),
		Name("Subtype"): Name("Widget"),
		Name("FT"):      Name("Tx"),
		Name("T"):       TextString(f.Name),
		Name("V"):       TextString(f.Value),
		Name("DA"):      LiteralString(da),
		Name("F"):       Integer(4), // Print
		Name("Rect"):    Array{Real(f.X0), Real(f.Y0), Real(f.X1), Real(f.Y1)},
		Name("MK"):      Dict{Name("BC"): Array{Integer(0)}}, // 1px black border
		Name("BS"):      Dict{Name("W"): Integer(1), Name("S"): Name("S")},
	}
	if ff := f.textFieldFlags(); ff != 0 {
		d[Name("Ff")] = Integer(ff)
	}
	return d
}

// acroForm builds the catalog /AcroForm dictionary from the collected field
// references. NeedAppearances asks the viewer to generate field appearances so
// values display without feast building appearance streams.
func (d *Document) acroForm() (Reference, bool) {
	if len(d.formFieldRefs) == 0 {
		return Reference{}, false
	}
	fields := make(Array, len(d.formFieldRefs))
	for i, r := range d.formFieldRefs {
		fields[i] = r
	}
	helv := d.fontRef("Helvetica")
	af := Dict{
		Name("Fields"):          fields,
		Name("NeedAppearances"): Boolean(true),
		Name("DA"):              LiteralString("/Helv 0 Tf 0 g"),
		Name("DR"): Dict{
			Name("Font"): Dict{Name("Helv"): helv},
		},
	}
	return d.w.Add(af), true
}
