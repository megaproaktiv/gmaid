// Package gcss parses the small CSS subset gmaid supports for styling gantt
// task bars, mirroring how mermaid-cli (mmdc) is driven with an external
// stylesheet via `-C style.css`.
//
// Mermaid renders every gantt task bar as an SVG <rect> whose id is
// "my-svg-<taskid>" (the SVG id prefix plus the task id). A stylesheet
// therefore targets bars with attribute selectors on that id, e.g.
//
//	rect[id$="-wp4_ai"] { fill:#d62728; stroke:#7a1516; }
//	rect[id*="-engc_"]  { fill:#2ca02c !important; }
//
// gmaid has no DOM, so this package re-implements just enough: it parses the
// rules and, given a task id, returns the winning fill/stroke by emulating the
// same id string ("my-svg-" + taskID) and matching the attribute operator.
//
// Supported subset (sufficient for the gantt styling workflow):
//   - selector:   rect[id<op>"VALUE"]   with op in  = ^= $= *=
//     (the leading "rect" and the attribute name "id" are optional/ignored;
//     a bare [id$="x"] works too)
//   - declarations: fill: <color>;  stroke: <color>;   (others ignored)
//   - !important is accepted and treated as higher priority
//   - /* block comments */ are stripped
//
// Later-defined rules win on equal priority (CSS source order); !important
// always beats non-important. This is a pragmatic ordering, not full CSS
// cascade/specificity, but it matches how the plan stylesheet is written.
package gcss

import (
	"image/color"
	"strings"
)

// op is an attribute-selector operator.
type op int

const (
	opExact  op = iota // [id="x"]
	opPrefix           // [id^="x"]
	opSuffix           // [id$="x"]
	opSubstr           // [id*="x"]
)

// decl is one parsed rule: a selector plus the fill/stroke it sets.
type decl struct {
	o         op
	value     string
	fill      color.Color
	hasFill   bool
	stroke    color.Color
	hasStroke bool
	important bool
	order     int // source order, later wins on ties
}

// Sheet is a parsed stylesheet.
type Sheet struct {
	rules []decl
}

// idPrefix mirrors the SVG element id mermaid assigns to each task rect.
const idPrefix = "my-svg-"

// Parse reads a CSS subset and returns a Sheet. Unparseable rules are skipped.
func Parse(src string) *Sheet {
	src = stripComments(src)
	s := &Sheet{}
	order := 0
	// Split into "selector { body }" chunks.
	for len(src) > 0 {
		open := strings.IndexByte(src, '{')
		if open < 0 {
			break
		}
		close := strings.IndexByte(src, '}')
		if close < 0 || close < open {
			break
		}
		selector := strings.TrimSpace(src[:open])
		body := src[open+1 : close]
		src = src[close+1:]

		// A rule may list several comma-separated selectors sharing one body.
		var parsedBody *decl
		for _, selPart := range strings.Split(selector, ",") {
			selPart = strings.TrimSpace(selPart)
			if selPart == "" {
				continue
			}
			o, val, ok := parseSelector(selPart)
			if !ok {
				continue
			}
			d := decl{o: o, value: val, order: order}
			if parsedBody == nil {
				parseBody(body, &d)
				parsedBody = &d
			} else {
				d.fill, d.hasFill = parsedBody.fill, parsedBody.hasFill
				d.stroke, d.hasStroke = parsedBody.stroke, parsedBody.hasStroke
				d.important = parsedBody.important
			}
			if d.hasFill || d.hasStroke {
				s.rules = append(s.rules, d)
			}
		}
		order++
	}
	return s
}

// Match returns the fill/stroke a task id resolves to, emulating the SVG id
// "my-svg-<taskID>". ok reports whether any rule matched.
func (s *Sheet) Match(taskID string) (fill, stroke color.Color, hasFill, hasStroke, ok bool) {
	if s == nil {
		return nil, nil, false, false, false
	}
	full := idPrefix + taskID
	// Track the best (highest priority) fill and stroke independently.
	bestFillPri, bestStrokePri := -1, -1
	for _, r := range s.rules {
		if !matches(r.o, full, r.value) {
			continue
		}
		pri := r.order
		if r.important {
			pri += 1 << 20 // important always outranks non-important
		}
		if r.hasFill && pri > bestFillPri {
			fill, hasFill, bestFillPri = r.fill, true, pri
		}
		if r.hasStroke && pri > bestStrokePri {
			stroke, hasStroke, bestStrokePri = r.stroke, true, pri
		}
	}
	ok = hasFill || hasStroke
	return fill, stroke, hasFill, hasStroke, ok
}

func matches(o op, full, val string) bool {
	switch o {
	case opExact:
		return full == val || full == idPrefix+val
	case opPrefix:
		return strings.HasPrefix(full, val)
	case opSuffix:
		return strings.HasSuffix(full, val)
	case opSubstr:
		return strings.Contains(full, val)
	}
	return false
}

// parseSelector extracts the attribute operator and value from a selector like
// `rect[id$="-wp4_ai"]`. The element name and `id` attribute name are optional.
func parseSelector(sel string) (op, string, bool) {
	lb := strings.IndexByte(sel, '[')
	rb := strings.IndexByte(sel, ']')
	if lb < 0 || rb < 0 || rb < lb {
		return 0, "", false
	}
	inner := sel[lb+1 : rb] // e.g.  id$="-wp4_ai"
	eq := strings.IndexByte(inner, '=')
	if eq < 0 {
		return 0, "", false
	}
	var o op
	switch {
	case eq > 0 && inner[eq-1] == '^':
		o = opPrefix
	case eq > 0 && inner[eq-1] == '$':
		o = opSuffix
	case eq > 0 && inner[eq-1] == '*':
		o = opSubstr
	default:
		o = opExact
	}
	val := strings.TrimSpace(inner[eq+1:])
	val = strings.Trim(val, `"'`)
	if val == "" {
		return 0, "", false
	}
	return o, val, true
}

func parseBody(body string, d *decl) {
	for _, stmt := range strings.Split(body, ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		important := false
		if i := strings.Index(strings.ToLower(stmt), "!important"); i >= 0 {
			important = true
			stmt = strings.TrimSpace(stmt[:i])
		}
		colon := strings.IndexByte(stmt, ':')
		if colon < 0 {
			continue
		}
		prop := strings.ToLower(strings.TrimSpace(stmt[:colon]))
		val := strings.TrimSpace(stmt[colon+1:])
		switch prop {
		case "fill":
			if c, ok := ParseColor(val); ok {
				d.fill, d.hasFill = c, true
			}
		case "stroke":
			if c, ok := ParseColor(val); ok {
				d.stroke, d.hasStroke = c, true
			}
		}
		if important {
			d.important = true
		}
	}
}

func stripComments(s string) string {
	for {
		i := strings.Index(s, "/*")
		if i < 0 {
			return s
		}
		j := strings.Index(s[i+2:], "*/")
		if j < 0 {
			return s[:i]
		}
		s = s[:i] + s[i+2+j+2:]
	}
}
