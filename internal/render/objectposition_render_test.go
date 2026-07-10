package render

import (
	"bytes"
	"strings"
	"testing"
)

// The image CTM (dw 0 0 dh e f cm just before the Do) reflects objectPosition:
// a small objectFit:none image in a larger box is placed by the position
// fraction, so top-left and center produce different translations.
func imageDrawCTM(t *testing.T, position string) string {
	t.Helper()
	uri := pngDataURI(t, 4, 4) // tiny, leaves free space in a 40x40 box
	res := layoutFromJSON(t, `{"version":"feast-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[100,100]},"children":[
			{"type":"IMAGE","props":{"src":"`+uri+`","style":{"width":40,"height":40,"objectFit":"none","objectPosition":"`+position+`"}}}
		]}
	]}}`)
	var buf bytes.Buffer
	if err := Render(res, &buf, Options{}); err != nil {
		t.Fatal(err)
	}
	content := inflateStreams(t, buf.Bytes())
	i := strings.Index(content, " cm\n")
	if i < 0 {
		t.Fatalf("no image CTM (cm) found:\n%s", content)
	}
	start := strings.LastIndex(content[:i], "\n") + 1
	return content[start:i]
}

func TestRenderObjectPositionShiftsImage(t *testing.T) {
	center := imageDrawCTM(t, "center")
	topLeft := imageDrawCTM(t, "left top")
	if center == topLeft {
		t.Errorf("objectPosition should change image placement; both were %q", center)
	}
	// The image CTM is "dw 0 0 dh e f"; "left" pins the x translate (e) to the
	// box's left edge, which is x=0 for a box at the page origin.
	if f := strings.Fields(topLeft); len(f) != 6 || f[4] != "0" {
		t.Errorf("left position should translate x to 0, got CTM %q", topLeft)
	}
}
