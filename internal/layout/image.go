package layout

import (
	"encoding/base64"
	"fmt"

	"github.com/SwishHQ/waffle/internal/contract"
	"github.com/SwishHQ/waffle/internal/flexbox"
	"github.com/SwishHQ/waffle/internal/imaging"
	"github.com/SwishHQ/waffle/internal/pdf"
	"github.com/SwishHQ/waffle/internal/tree"
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

// resolveImage decodes an Image node's source (data URI, inline bytes, http(s)
// URL, or file path) into a drawable spec. With a cache, the decoded spec is
// built once per source and reused across renders — decode and pixel
// compression are the dominant per-render cost for image-heavy documents.
// Failures are not cached, so a transient fetch error retries next render.
func resolveImage(node *tree.Node, style map[string]any, cache *Cache) *imageResolve {
	fit := str(style["objectFit"])
	key := imageSrcKey(node)
	if e := cache.image(key); e != nil {
		return &imageResolve{spec: e.spec, fit: fit, w: e.w, h: e.h}
	}
	dec, err := decodeImageSrc(node)
	if err != nil || dec == nil {
		return nil
	}
	e := &imageEntry{
		spec: &pdf.ImageSpec{
			Width: dec.Width, Height: dec.Height, ColorSpace: dec.ColorSpace,
			BitsPerComponent: dec.BitsPerComponent, Filter: dec.Filter,
			Data: dec.Data, SMask: dec.SMask,
		},
		w: float64(dec.Width),
		h: float64(dec.Height),
	}
	cache.storeImage(key, e)
	return &imageResolve{spec: e.spec, fit: fit, w: e.w, h: e.h}
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
