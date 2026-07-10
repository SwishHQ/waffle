package render

import (
	"bytes"
	"strings"
	"testing"
)

// overflow:hidden clips the box's children to its frame (a Rect + W n before the
// children paint).
func TestRenderOverflowHiddenClips(t *testing.T) {
	doc := `{"version":"feast-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[200,200]},"children":[
			{"type":"VIEW","props":{"style":{"width":50,"height":50,"overflow":"hidden"}},"children":[
				{"type":"VIEW","props":{"style":{"width":80,"height":80,"backgroundColor":"#ff0000"}}}
			]}
		]}
	]}}`
	res := layoutFromJSON(t, doc)
	var out bytes.Buffer
	if err := Render(res, &out, Options{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(inflateStreams(t, out.Bytes()), "W\nn\n") {
		t.Error("overflow:hidden should clip children (W n)")
	}
}

// Without overflow:hidden, a plain nested View emits no clip.
func TestRenderNoOverflowNoClip(t *testing.T) {
	doc := `{"version":"feast-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[200,200]},"children":[
			{"type":"VIEW","props":{"style":{"width":50,"height":50}},"children":[
				{"type":"VIEW","props":{"style":{"width":80,"height":80,"backgroundColor":"#ff0000"}}}
			]}
		]}
	]}}`
	res := layoutFromJSON(t, doc)
	var out bytes.Buffer
	if err := Render(res, &out, Options{}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(inflateStreams(t, out.Bytes()), "W\nn\n") {
		t.Error("a box without overflow:hidden should not clip")
	}
}
