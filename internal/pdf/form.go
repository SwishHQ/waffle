package pdf

import (
	"bytes"
	"fmt"
)

// FormFieldKind selects the widget type of an AcroForm field.
type FormFieldKind int

const (
	FieldText     FormFieldKind = iota // single- or multi-line text (/Tx)
	FieldCheckbox                      // toggle button (/Btn)
	FieldChoice                        // combo box / list box (/Ch)
)

// ChoiceOption is one entry of a choice field: an export value and the display
// label shown to the user (often identical).
type ChoiceOption struct {
	Export  string
	Display string
}

// FormField is an interactive AcroForm field rendered as a widget annotation on a
// page. Text fields (/Tx) and checkboxes (/Btn) are supported.
type FormField struct {
	Kind      FormFieldKind
	Name      string  // fully-qualified field name (/T)
	Value     string  // text value (/V) for text fields
	FontSize  float64 // text fields: 0 means auto-size (/DA "0 Tf")
	MultiLine bool    // text field flag: multi-line
	Password  bool    // text field flag: password (value masked)

	Checked bool   // checkbox: initial state
	OnState string // checkbox: export value of the "on" state (default "Yes")

	Options []ChoiceOption // choice field entries
	Combo   bool           // choice field: dropdown (combo) vs. list box

	X0, Y0, X1, Y1 float64 // widget rectangle in page (default user) space
}

// AddFormField records an interactive form field widget for this page.
func (c *Content) AddFormField(f FormField) { c.fields = append(c.fields, f) }

// FormFields returns the form fields recorded for this page.
func (c *Content) FormFields() []FormField { return c.fields }

// onState returns the checkbox's on-state export name, defaulting to "Yes".
func (f FormField) onState() string {
	if f.OnState != "" {
		return f.OnState
	}
	return "Yes"
}

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

// addFieldWidget serializes a field into a widget-annotation object and returns
// its reference. Checkboxes also emit on/off appearance-stream XObjects.
func (d *Document) addFieldWidget(f FormField) Reference {
	switch f.Kind {
	case FieldCheckbox:
		return d.w.Add(d.checkboxWidget(f))
	case FieldChoice:
		return d.w.Add(f.choiceWidget())
	default:
		return d.w.Add(f.textWidget())
	}
}

// choiceWidget builds a choice-field (/Ch) widget: a dropdown when Combo is set,
// otherwise a scrollable list box. /Opt lists the options as [export display]
// pairs (or a single string when they match).
func (f FormField) choiceWidget() Dict {
	const flagCombo = 1 << 17 // bit 18
	opt := make(Array, len(f.Options))
	for i, o := range f.Options {
		if o.Export == "" || o.Export == o.Display {
			opt[i] = TextString(o.Display)
		} else {
			opt[i] = Array{TextString(o.Export), TextString(o.Display)}
		}
	}
	d := Dict{
		Name("Type"):    Name("Annot"),
		Name("Subtype"): Name("Widget"),
		Name("FT"):      Name("Ch"),
		Name("T"):       TextString(f.Name),
		Name("V"):       TextString(f.Value),
		Name("Opt"):     opt,
		Name("DA"):      LiteralString(fmt.Sprintf("/Helv %s Tf 0 g", num(f.FontSize))),
		Name("F"):       Integer(4),
		Name("Rect"):    Array{Real(f.X0), Real(f.Y0), Real(f.X1), Real(f.Y1)},
		Name("MK"):      Dict{Name("BC"): Array{Integer(0)}},
		Name("BS"):      Dict{Name("W"): Integer(1), Name("S"): Name("S")},
	}
	if f.Combo {
		d[Name("Ff")] = Integer(flagCombo)
	}
	return d
}

// textWidget builds a text-field widget dictionary. The widget also serves as the
// terminal field (its /T makes it a field), so it is referenced both from the
// page /Annots and the AcroForm /Fields.
func (f FormField) textWidget() Dict {
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

// checkboxWidget builds a checkbox widget with on/off appearance streams. The
// appearance state (/AS) and value (/V) select which stream displays.
func (d *Document) checkboxWidget(f FormField) Dict {
	w, h := f.X1-f.X0, f.Y1-f.Y0
	on := f.onState()
	onAP := d.w.Add(checkboxAP(w, h, true))
	offAP := d.w.Add(checkboxAP(w, h, false))

	as := "Off"
	value := Name("Off")
	if f.Checked {
		as, value = on, Name(on)
	}
	return Dict{
		Name("Type"):    Name("Annot"),
		Name("Subtype"): Name("Widget"),
		Name("FT"):      Name("Btn"),
		Name("T"):       TextString(f.Name),
		Name("V"):       value,
		Name("AS"):      Name(as),
		Name("F"):       Integer(4),
		Name("Rect"):    Array{Real(f.X0), Real(f.Y0), Real(f.X1), Real(f.Y1)},
		Name("MK"):      Dict{Name("BC"): Array{Integer(0)}},
		Name("BS"):      Dict{Name("W"): Integer(1), Name("S"): Name("S")},
		Name("AP"): Dict{
			Name("N"): Dict{Name(on): onAP, Name("Off"): offAP},
		},
	}
}

// checkboxAP builds a checkbox appearance form XObject: a bordered box, plus a
// checkmark stroke for the on state.
func checkboxAP(w, h float64, on bool) *Stream {
	var b bytes.Buffer
	b.WriteString("q\n0 0 0 RG\n1 w\n")
	fmt.Fprintf(&b, "0.5 0.5 %s %s re S\n", num(w-1), num(h-1))
	if on {
		fmt.Fprintf(&b, "%s w\n", num((w+h)/16))
		fmt.Fprintf(&b, "%s %s m %s %s l %s %s l S\n",
			num(0.2*w), num(0.5*h), num(0.42*w), num(0.25*h), num(0.8*w), num(0.78*h))
	}
	b.WriteString("Q\n")
	return FlateStream(Dict{
		Name("Type"):      Name("XObject"),
		Name("Subtype"):   Name("Form"),
		Name("FormType"):  Integer(1),
		Name("BBox"):      Array{Integer(0), Integer(0), Real(w), Real(h)},
		Name("Resources"): Dict{Name("ProcSet"): Array{Name("PDF")}},
	}, b.Bytes())
}

// acroForm builds the catalog /AcroForm dictionary from the collected field
// references. NeedAppearances asks the viewer to generate text-field appearances
// (checkbox appearances are supplied explicitly).
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
