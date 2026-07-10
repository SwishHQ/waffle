package layout

import (
	"encoding/base64"
	"fmt"

	"github.com/swish/waffle/internal/contract"
	"github.com/swish/waffle/internal/flexbox"
	"github.com/swish/waffle/internal/imaging"
	"github.com/swish/waffle/internal/pdf"
	"github.com/swish/waffle/internal/tree"
)

// imageResolve holds a decoded image ready to lay out and draw.
type imageResolve struct {
	spec *pdf.ImageSpec
	fit  string
	w, h float64 // intrinsic size (pixels treated as points)
}

func (im *imageResolve) measure(availW, availH float64) flexbox.Size {
	return flexbox.Size{W: im.w, H: im.h}
}

// resolveImage decodes an Image node's source into a drawable spec. Only data
// URIs and inline bytes are supported here; URL/file fetching is a later async
// asset-resolution step.
func resolveImage(node *tree.Node, style map[string]any) *imageResolve {
	dec, err := decodeImageSrc(node)
	if err != nil || dec == nil {
		return nil
	}
	return &imageResolve{
		spec: &pdf.ImageSpec{
			Width: dec.Width, Height: dec.Height, ColorSpace: dec.ColorSpace,
			BitsPerComponent: dec.BitsPerComponent, Filter: dec.Filter,
			Data: dec.Data, SMask: dec.SMask,
		},
		fit: str(style["objectFit"]),
		w:   float64(dec.Width),
		h:   float64(dec.Height),
	}
}

func decodeImageSrc(node *tree.Node) (*imaging.Image, error) {
	src := node.Props["src"]
	if src == nil {
		src = node.Props["source"]
	}
	switch v := src.(type) {
	case string:
		// data: URI, http(s) URL, or filesystem path — imaging.Load dispatches.
		return imaging.Load(v)
	case contract.InlineAsset:
		raw, err := base64.StdEncoding.DecodeString(v.Base64)
		if err != nil {
			return nil, err
		}
		return imaging.Decode(raw)
	case map[string]any:
		if u, ok := v["uri"].(string); ok && u != "" {
			return imaging.Load(u)
		}
	}
	return nil, fmt.Errorf("layout: unsupported image source")
}
