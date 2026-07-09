package layout

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/swish/feast/internal/contract"
	"github.com/swish/feast/internal/fontstore"
	"github.com/swish/feast/internal/stylesheet"
)

// buildFontStore registers all Font.register faces into a fontstore.Store for
// custom-font measurement and embedding. Only inline sources (base64 data: URIs
// and $inline bytes) are decoded here; URL/file font fetching is a later step.
// Faces that can't be decoded/parsed are skipped with a warning so a bad font
// degrades gracefully to the standard-font fallback. Returns nil when no fonts
// are registered.
func buildFontStore(fonts []contract.FontRegistration) (*fontstore.Store, []string) {
	if len(fonts) == 0 {
		return nil, nil
	}
	store := fontstore.New()
	var warns []string
	registered := false
	for _, reg := range fonts {
		if reg.Family == "" {
			continue
		}
		var specs []fontstore.FaceSpec
		for _, face := range reg.Faces {
			data, err := decodeFontSrc(face.Src)
			if err != nil {
				warns = append(warns, fmt.Sprintf("font %q: %v", reg.Family, err))
				continue
			}
			if data == nil {
				continue // URL/file source not fetched yet
			}
			weight := 0
			if w, ok := stylesheet.ParseFontWeight(face.FontWeight); ok {
				weight = w
			}
			specs = append(specs, fontstore.FaceSpec{
				Data:   data,
				Weight: weight,
				Style:  fontstore.ParseStyle(face.FontStyle),
			})
		}
		if len(specs) == 0 {
			continue
		}
		if err := store.Register(reg.Family, specs...); err != nil {
			warns = append(warns, fmt.Sprintf("font %q: %v", reg.Family, err))
			continue
		}
		registered = true
	}
	if !registered {
		return nil, warns
	}
	return store, warns
}

// decodeFontSrc extracts font program bytes from a Font.register src: a base64
// data: URI or inline bytes ($inline). Returns (nil, nil) for URL/file sources
// (not fetched yet) so the caller falls back to a standard font.
func decodeFontSrc(src any) ([]byte, error) {
	switch v := src.(type) {
	case string:
		if strings.HasPrefix(v, "data:") {
			return decodeDataURIBytes(v)
		}
		return nil, nil
	case contract.InlineAsset:
		return base64.StdEncoding.DecodeString(v.Base64)
	}
	return nil, nil
}

// decodeDataURIBytes decodes the payload of a data: URI (base64 or raw).
func decodeDataURIBytes(uri string) ([]byte, error) {
	comma := strings.IndexByte(uri, ',')
	if comma < 0 {
		return nil, fmt.Errorf("malformed data URI")
	}
	meta, payload := uri[:comma], uri[comma+1:]
	if strings.Contains(meta, ";base64") {
		return base64.StdEncoding.DecodeString(payload)
	}
	return []byte(payload), nil
}
