package contract

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const majorPrefix = "waffle-tree/v"

// Parse decodes a waffle-tree/v1 document. Numbers are preserved as json.Number
// so downstream style parsing sees exact values. Unknown node types and props
// are tolerated and surfaced as warnings rather than errors, so a newer producer
// degrades gracefully against an older engine.
func Parse(data []byte) (*Tree, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var root map[string]any
	if err := dec.Decode(&root); err != nil {
		return nil, fmt.Errorf("contract: invalid JSON: %w", err)
	}

	version, _ := root["version"].(string)
	if err := checkVersion(version); err != nil {
		return nil, err
	}

	p := &parser{}
	tree := &Tree{Version: version}

	tree.Callbacks = stringSlice(root["callbacks"])

	docRaw, ok := root["document"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("contract: missing or invalid \"document\"")
	}
	tree.Document = p.document(docRaw)
	tree.Warnings = p.warnings

	p.checkCallbackConsistency(tree)
	return tree, nil
}

func checkVersion(v string) error {
	if v == "" {
		return fmt.Errorf("contract: missing \"version\"")
	}
	if !strings.HasPrefix(v, majorPrefix) {
		return fmt.Errorf("contract: unrecognized version %q (want %s...)", v, majorPrefix)
	}
	major := strings.TrimPrefix(v, majorPrefix)
	if i := strings.IndexByte(major, '.'); i >= 0 {
		major = major[:i]
	}
	if major != "1" {
		return fmt.Errorf("contract: unsupported version %q; this build implements %s", v, Version)
	}
	return nil
}

type parser struct {
	warnings []string
}

func (p *parser) warnf(format string, args ...any) {
	p.warnings = append(p.warnings, fmt.Sprintf(format, args...))
}

func (p *parser) document(raw map[string]any) *Document {
	d := &Document{}
	if props, ok := raw["props"].(map[string]any); ok {
		d.Props = convertMap(props)
	}
	d.Fonts = parseFonts(raw["fonts"])
	d.EmojiSource = parseEmoji(raw["emojiSource"])
	if cb, ok := convertValue(raw["hyphenationCallback"]).(CallbackRef); ok {
		d.HyphenationCallback = &cb
	}
	if kids, ok := raw["children"].([]any); ok {
		for _, k := range kids {
			if km, ok := k.(map[string]any); ok {
				d.Children = append(d.Children, p.node(km))
			}
		}
	}
	return d
}

func (p *parser) node(raw map[string]any) *Node {
	n := &Node{}
	if t, ok := raw["type"].(string); ok {
		n.Type = t
	}
	if v, ok := raw["value"].(string); ok {
		n.Value = v
	}
	if props, ok := raw["props"].(map[string]any); ok {
		n.Props = convertMap(props)
	}
	if kids, ok := raw["children"].([]any); ok {
		for _, k := range kids {
			if km, ok := k.(map[string]any); ok {
				n.Children = append(n.Children, p.node(km))
			}
		}
	}
	if n.Type == "" {
		p.warnf("node with empty type")
	} else if !KnownType(n.Type) {
		p.warnf("unknown node type %q", n.Type)
	}
	return n
}

// checkCallbackConsistency warns when referenced callback ids and the declared
// callbacks list disagree.
func (p *parser) checkCallbackConsistency(t *Tree) {
	declared := map[string]bool{}
	for _, id := range t.Callbacks {
		declared[id] = true
	}
	referenced := map[string]bool{}
	for _, id := range t.referencedCallbacks() {
		referenced[id] = true
		if !declared[id] {
			t.Warnings = append(t.Warnings, fmt.Sprintf("callback %q referenced but not declared", id))
		}
	}
	var undeclaredUse []string
	for id := range declared {
		if !referenced[id] {
			undeclaredUse = append(undeclaredUse, id)
		}
	}
	sort.Strings(undeclaredUse)
	for _, id := range undeclaredUse {
		t.Warnings = append(t.Warnings, fmt.Sprintf("callback %q declared but not referenced", id))
	}
}

func parseFonts(raw any) []FontRegistration {
	arr, ok := raw.([]any)
	if !ok {
		return nil
	}
	var out []FontRegistration
	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		fr := FontRegistration{}
		fr.Family, _ = m["family"].(string)
		if faces, ok := m["fonts"].([]any); ok {
			for _, fc := range faces {
				fm, ok := fc.(map[string]any)
				if !ok {
					continue
				}
				fr.Faces = append(fr.Faces, FontFace{
					Src:        convertValue(fm["src"]),
					FontWeight: fm["fontWeight"],
					FontStyle:  asString(fm["fontStyle"]),
				})
			}
		}
		out = append(out, fr)
	}
	return out
}

func parseEmoji(raw any) *EmojiSource {
	m, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	return &EmojiSource{
		URL:    asString(m["url"]),
		Format: asString(m["format"]),
	}
}

// convertValue replaces $cb/$inline sentinel objects with typed markers and
// recurses through maps and slices. Scalars (including json.Number) pass through.
func convertValue(v any) any {
	switch t := v.(type) {
	case map[string]any:
		if len(t) == 1 {
			if id, ok := t["$cb"]; ok {
				if s, ok := id.(string); ok {
					return CallbackRef{ID: s}
				}
			}
			if b, ok := t["$inline"]; ok {
				if s, ok := b.(string); ok {
					return InlineAsset{Base64: s}
				}
			}
		}
		return convertMap(t)
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			out[i] = convertValue(val)
		}
		return out
	default:
		return v
	}
}

func convertMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = convertValue(v)
	}
	return out
}

func stringSlice(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}
