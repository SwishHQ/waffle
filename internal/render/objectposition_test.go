package render

import "testing"

func TestObjectPositionOf(t *testing.T) {
	cases := []struct {
		v     string
		wantX float64
		wantY float64
	}{
		{"", 0.5, 0.5},
		{"center", 0.5, 0.5},
		{"left", 0, 0.5},
		{"right", 1, 0.5},
		{"top", 0.5, 0},
		{"bottom", 0.5, 1},
		{"left top", 0, 0},
		{"top left", 0, 0}, // keywords resolve by axis regardless of order
		{"right bottom", 1, 1},
		{"25% 75%", 0.25, 0.75},
		{"50%", 0.5, 0.5},
		{"0% 100%", 0, 1},
		{"right 25%", 1, 0.25}, // keyword sets x, percent fills y
	}
	for _, c := range cases {
		px, py := objectPositionOf(map[string]any{"objectPosition": c.v})
		if px != c.wantX || py != c.wantY {
			t.Errorf("objectPositionOf(%q) = (%v,%v), want (%v,%v)", c.v, px, py, c.wantX, c.wantY)
		}
	}
}
