package layout

import "testing"

func TestApplyTextTransform(t *testing.T) {
	cases := []struct {
		in, transform, want string
	}{
		{"Hello World", "uppercase", "HELLO WORLD"},
		{"Hello World", "lowercase", "hello world"},
		{"hello world", "capitalize", "Hello World"},
		{"hello world", "", "hello world"},
		{"hello world", "none", "hello world"},
		{"  spaced  out ", "capitalize", "  Spaced  Out "},
		{"1st place", "capitalize", "1st Place"}, // digit-led word: capitalize first letter
	}
	for _, c := range cases {
		if got := applyTextTransform(c.in, c.transform); got != c.want {
			t.Errorf("applyTextTransform(%q,%q) = %q, want %q", c.in, c.transform, got, c.want)
		}
	}
}
