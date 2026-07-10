package pdf

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestAcroFormTextField(t *testing.T) {
	doc := New(Options{})
	c := NewContent()
	c.Save().Rect(0, 0, 10, 10).Fill().Restore()
	c.AddFormField(FormField{Name: "email", Value: "a@b.com", X0: 50, Y0: 150, X1: 250, Y1: 170})
	doc.AddPage(300, 300, c)

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	s := buf.String()
	for _, want := range []string{"/AcroForm", "/FT /Tx", "(email)", "(a@b.com)", "/Subtype /Widget", "/NeedAppearances true", "/Helv"} {
		if !strings.Contains(s, want) {
			t.Errorf("PDF missing %q", want)
		}
	}
	if bin, _ := exec.LookPath("pdfcpu"); bin != "" {
		p := filepath.Join(t.TempDir(), "form.pdf")
		os.WriteFile(p, buf.Bytes(), 0o644)
		if out, err := exec.Command(bin, "validate", "-m", "strict", p).CombinedOutput(); err != nil {
			t.Fatalf("pdfcpu validate failed: %v\n%s", err, out)
		}
	}
}

func TestTextFieldFlags(t *testing.T) {
	if got := (FormField{MultiLine: true}).textFieldFlags(); got != 1<<12 {
		t.Errorf("multiline flag = %d, want %d", got, 1<<12)
	}
	if got := (FormField{Password: true}).textFieldFlags(); got != 1<<13 {
		t.Errorf("password flag = %d, want %d", got, 1<<13)
	}
	if got := (FormField{}).textFieldFlags(); got != 0 {
		t.Errorf("no flags = %d, want 0", got)
	}
}

func TestAcroFormCheckbox(t *testing.T) {
	doc := New(Options{})
	c := NewContent()
	c.Save().Rect(0, 0, 10, 10).Fill().Restore()
	c.AddFormField(FormField{Kind: FieldCheckbox, Name: "agree", Checked: true, X0: 50, Y0: 150, X1: 70, Y1: 170})
	doc.AddPage(300, 300, c)

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	s := buf.String()
	for _, want := range []string{"/FT /Btn", "(agree)", "/AS /Yes", "/AP", "/Subtype /Form", "/AcroForm"} {
		if !strings.Contains(s, want) {
			t.Errorf("PDF missing %q", want)
		}
	}
	if bin, _ := exec.LookPath("pdfcpu"); bin != "" {
		p := filepath.Join(t.TempDir(), "cb.pdf")
		os.WriteFile(p, buf.Bytes(), 0o644)
		if out, err := exec.Command(bin, "validate", "-m", "strict", p).CombinedOutput(); err != nil {
			t.Fatalf("pdfcpu validate failed: %v\n%s", err, out)
		}
	}
}

func TestCheckboxOnStateDefault(t *testing.T) {
	if got := (FormField{}).onState(); got != "Yes" {
		t.Errorf("default on-state = %q, want Yes", got)
	}
	if got := (FormField{OnState: "On"}).onState(); got != "On" {
		t.Errorf("explicit on-state = %q, want On", got)
	}
}

func TestAcroFormChoice(t *testing.T) {
	doc := New(Options{})
	c := NewContent()
	c.Save().Rect(0, 0, 10, 10).Fill().Restore()
	c.AddFormField(FormField{
		Kind: FieldChoice, Name: "country", Value: "US", Combo: true,
		Options: []ChoiceOption{{Export: "US", Display: "United States"}, {Export: "CA", Display: "Canada"}},
		X0:      50, Y0: 150, X1: 250, Y1: 170,
	})
	doc.AddPage(300, 300, c)

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	s := buf.String()
	for _, want := range []string{"/FT /Ch", "(country)", "/Opt", "(United States)", "(US)", "/Ff 131072"} {
		if !strings.Contains(s, want) {
			t.Errorf("choice PDF missing %q", want)
		}
	}
	if bin, _ := exec.LookPath("pdfcpu"); bin != "" {
		p := filepath.Join(t.TempDir(), "choice.pdf")
		os.WriteFile(p, buf.Bytes(), 0o644)
		if out, err := exec.Command(bin, "validate", "-m", "strict", p).CombinedOutput(); err != nil {
			t.Fatalf("pdfcpu validate failed: %v\n%s", err, out)
		}
	}
}
