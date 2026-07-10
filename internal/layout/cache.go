package layout

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"

	"github.com/swish/waffle/internal/contract"
	"github.com/swish/waffle/internal/fontstore"
	"github.com/swish/waffle/internal/pdf"
	"github.com/swish/waffle/internal/tree"
)

// Cache memoizes render-invariant assets across renders of the same document:
// decoded images (whose pixel data is also already FlateDecode-compressed) and
// parsed font stores. A Template owns one, so repeated renders skip image
// decode + re-compression and font fetch + parse entirely — profiling shows
// those dominate render time on image-heavy documents.
//
// Entries are keyed by source (the data-URI/URL/path string, or a hash of
// inline bytes), so a file or URL source is fetched once per Cache lifetime;
// later changes to the underlying file are not observed. All methods are safe
// for concurrent use and are nil-receiver safe (a nil *Cache disables caching).
// On a cold concurrent start the same asset may be built more than once; the
// results are equivalent and the last one stored wins.
type Cache struct {
	images sync.Map // key string -> *imageEntry
	fonts  sync.Map // key string -> *fontsEntry
}

// NewCache returns an empty asset cache.
func NewCache() *Cache { return &Cache{} }

// imageEntry is a decoded, embed-ready image plus its intrinsic size.
type imageEntry struct {
	spec *pdf.ImageSpec
	w, h float64
}

// fontsEntry is a built font store plus the warnings its build produced (the
// same warnings are reported on every render, matching the uncached behavior).
type fontsEntry struct {
	store *fontstore.Store
	warns []string
}

// image returns the cached entry for key, or nil.
func (c *Cache) image(key string) *imageEntry {
	if c == nil || key == "" {
		return nil
	}
	if v, ok := c.images.Load(key); ok {
		return v.(*imageEntry)
	}
	return nil
}

// storeImage records a decoded image under key (no-op for nil cache/empty key).
func (c *Cache) storeImage(key string, e *imageEntry) {
	if c == nil || key == "" {
		return
	}
	c.images.Store(key, e)
}

// fontStore returns the font store for the given registrations, building it on
// first use and reusing it afterwards. A nil Cache builds a fresh store every
// time (the uncached one-shot path).
func (c *Cache) fontStore(fonts []contract.FontRegistration) (*fontstore.Store, []string) {
	if c == nil {
		return buildFontStore(fonts)
	}
	if len(fonts) == 0 {
		return nil, nil
	}
	key := fontsKey(fonts)
	if v, ok := c.fonts.Load(key); ok {
		e := v.(*fontsEntry)
		return e.store, e.warns
	}
	store, warns := buildFontStore(fonts)
	c.fonts.Store(key, &fontsEntry{store: store, warns: warns})
	return store, warns
}

// imageSrcKey derives a stable cache key from an Image node's source, mirroring
// decodeImageSrc's dispatch. Returns "" when the source is absent or of an
// uncacheable shape.
func imageSrcKey(node *tree.Node) string {
	src := node.Props["src"]
	if src == nil {
		src = node.Props["source"]
	}
	switch v := src.(type) {
	case string:
		return "s:" + srcKey(v)
	case contract.InlineAsset:
		return "i:" + srcKey(v.Base64)
	case map[string]any:
		if u, ok := v["uri"].(string); ok && u != "" {
			return "u:" + srcKey(u)
		}
	}
	return ""
}

// fontsKey fingerprints a Font.register set: every family with each face's
// weight, style, and source.
func fontsKey(fonts []contract.FontRegistration) string {
	var b strings.Builder
	for _, reg := range fonts {
		b.WriteString(reg.Family)
		b.WriteByte(0)
		for _, f := range reg.Faces {
			var src string
			switch v := f.Src.(type) {
			case string:
				src = srcKey(v)
			case contract.InlineAsset:
				src = srcKey(v.Base64)
			}
			fmt.Fprintf(&b, "%v|%s|%s;", f.FontWeight, f.FontStyle, src)
		}
		b.WriteByte(1)
	}
	return b.String()
}

// srcKey keeps short sources (paths, URLs) verbatim and hashes long ones (data
// URIs, inline base64) so map keys stay small.
func srcKey(s string) string {
	if len(s) <= 128 {
		return s
	}
	sum := sha256.Sum256([]byte(s))
	return "sha256:" + hex.EncodeToString(sum[:])
}
