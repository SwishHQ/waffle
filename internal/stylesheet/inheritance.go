package stylesheet

import "strings"

// inheritedProps are the style properties that cascade from parent to child on
// non-SVG nodes. This mirrors react-pdf's resolveInheritance set (PLAN §2.3);
// verify against resolveInheritance.ts if a property's inheritance is ever in
// question (notably opacity, which inherits here unlike in CSS).
var inheritedProps = map[string]bool{
	"color":          true,
	"fontFamily":     true,
	"fontSize":       true,
	"fontStyle":      true,
	"fontWeight":     true,
	"letterSpacing":  true,
	"opacity":        true,
	"textTransform":  true,
	"lineHeight":     true,
	"textAlign":      true,
	"visibility":     true,
	"wordSpacing":    true,
	"textDecoration": true, // merged specially (see Inherit)
}

// textOnlyInherited are additionally inherited by Text nodes.
var textOnlyInherited = map[string]bool{
	"backgroundColor": true,
}

// Inherit returns the child's effective style map: inheritable properties from
// the parent's effective style fill in properties the child has not set (the
// child's own values always win). isText enables the Text-only inherited
// properties. textDecoration is merged so nested underline/line-through combine,
// unless the child sets "none". Apply this top-down, passing each node's returned
// effective style as the parent for its own children. SVG subtrees skip this
// pass entirely.
func Inherit(parent, child map[string]any, isText bool) map[string]any {
	out := make(map[string]any, len(child)+len(parent))
	for k, v := range child {
		out[k] = v
	}
	if parent == nil {
		return out
	}

	for k := range inheritedProps {
		if k == "textDecoration" {
			continue
		}
		if _, has := out[k]; !has {
			if pv, ok := parent[k]; ok {
				out[k] = pv
			}
		}
	}
	if isText {
		for k := range textOnlyInherited {
			if _, has := out[k]; !has {
				if pv, ok := parent[k]; ok {
					out[k] = pv
				}
			}
		}
	}

	if dec := mergeTextDecoration(parent["textDecoration"], child["textDecoration"]); dec != nil {
		out["textDecoration"] = dec
	} else {
		delete(out, "textDecoration")
	}
	return out
}

// mergeTextDecoration unions the parent and child decoration tokens. An explicit
// child "none" wins and suppresses inherited decorations. Returns nil when there
// is no decoration to set.
func mergeTextDecoration(parent, child any) any {
	childToks := decorationTokens(child)
	for _, t := range childToks {
		if t == "none" {
			return "none"
		}
	}
	seen := map[string]bool{}
	var order []string
	for _, t := range append(decorationTokens(parent), childToks...) {
		if t == "" || t == "none" || seen[t] {
			continue
		}
		seen[t] = true
		order = append(order, t)
	}
	if len(order) == 0 {
		return nil
	}
	return strings.Join(order, " ")
}

func decorationTokens(v any) []string {
	s, ok := v.(string)
	if !ok {
		return nil
	}
	return strings.Fields(s)
}
