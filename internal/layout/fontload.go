package layout

import (
	"encoding/base64"
	"fmt"

	"github.com/SwishHQ/waffle/internal/contract"
	"github.com/SwishHQ/waffle/internal/fetch"
	"github.com/SwishHQ/waffle/internal/fontstore"
	"github.com/SwishHQ/waffle/internal/stylesheet"
)

// buildFontStore registers all Font.register faces into a fontstore.Store for
// custom-font measurement and embedding. Sources may be data: URIs, $inline
// bytes, http(s) URLs, or local file paths. Faces that can't be
// fetched/decoded/parsed are skipped with a warning so a bad font degrades
// gracefully to the standard-font fallback. Returns nil when no fonts are
// registered.
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

// decodeFontSrc extracts font program bytes from a Font.register src: a data:
// URI, http(s) URL, or file path (string), or inline bytes ($inline). Returns
// (nil, nil) for an empty/unusable source so the caller skips the face.
func decodeFontSrc(src any) ([]byte, error) {
	switch v := src.(type) {
	case string:
		if v == "" {
			return nil, nil
		}
		return fetch.Bytes(v)
	case contract.InlineAsset:
		return base64.StdEncoding.DecodeString(v.Base64)
	}
	return nil, nil
}
