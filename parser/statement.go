package parser

import (
	"regexp"
	"strings"

	"github.com/megaproaktiv/gmaid/model"
)

// A more precise operator finder: we scan for the connector and classify it.
// Connector forms we handle:
//
//	solid:     ---, -->, --o, --x, ----, ---->
//	dotted:    -.-, -.->, -..-, -...->
//	thick:     ===, ==>, ====
//	invisible: ~~~
//
// Labels can be inline (`-- text -->`) or piped (`-->|text|`).

// parseStatement handles a full statement line, which may be a chain:
//
//	A --> B -->|lbl| C
//	A -- text --> B
//	X ~~~ Y
//	A[Label]   (bare node)
func (p *parser) parseStatement(line string) {
	segs, ops := splitChain(line)
	if len(segs) == 0 {
		return
	}
	if len(ops) == 0 {
		// bare node declaration (possibly several separated by & )
		for _, tok := range splitAmp(segs[0]) {
			tok = strings.TrimSpace(tok)
			if tok == "" {
				continue
			}
			id := p.declareNodeToken(tok)
			p.registerMembership(id)
		}
		return
	}

	// Resolve each segment to one or more node ids (handles `A & B`).
	nodeGroups := make([][]string, len(segs))
	for i, s := range segs {
		var ids []string
		for _, tok := range splitAmp(s) {
			tok = strings.TrimSpace(tok)
			if tok == "" {
				continue
			}
			id := p.declareNodeToken(tok)
			p.registerMembership(id)
			ids = append(ids, id)
		}
		nodeGroups[i] = ids
	}

	// Build edges between consecutive groups using the operator between them.
	for i := 0; i < len(ops); i++ {
		op := ops[i]
		from := nodeGroups[i]
		to := nodeGroups[i+1]
		for _, f := range from {
			for _, t := range to {
				e := &model.Edge{
					From:   f,
					To:     t,
					Label:  op.label,
					Line:   op.line,
					Arrow:  op.arrow,
					MinLen: op.minLen,
				}
				p.g.Edges = append(p.g.Edges, e)
			}
		}
	}
}

// splitAmp splits a segment on the `&` node-separator, but only when the `&`
// appears outside of any bracket wrapper or quoted string. This preserves
// ampersands that are part of a node label, e.g. WP2["Email & AI Routing"].
func splitAmp(s string) []string {
	var out []string
	var cur strings.Builder
	depth := 0
	inQuote := false
	for _, c := range s {
		switch {
		case inQuote:
			if c == '"' {
				inQuote = false
			}
			cur.WriteRune(c)
		case c == '"':
			inQuote = true
			cur.WriteRune(c)
		case c == '[' || c == '(' || c == '{':
			depth++
			cur.WriteRune(c)
		case c == ']' || c == ')' || c == '}':
			if depth > 0 {
				depth--
			}
			cur.WriteRune(c)
		case c == '&' && depth == 0:
			out = append(out, cur.String())
			cur.Reset()
		default:
			cur.WriteRune(c)
		}
	}
	out = append(out, cur.String())
	return out
}

type opInfo struct {
	label  string
	line   model.EdgeLineStyle
	arrow  model.ArrowType
	minLen int
}

// splitChain splits a statement into node segments and the operators between
// them, correctly skipping over bracketed label text so that dashes inside a
// node label are not mistaken for edge operators.
func splitChain(line string) (segs []string, ops []opInfo) {
	var cur strings.Builder
	i := 0
	depthBracket := 0 // [], (), {}
	inQuote := false

	flush := func() {
		segs = append(segs, strings.TrimSpace(cur.String()))
		cur.Reset()
	}

	runes := []rune(line)
	for i < len(runes) {
		c := runes[i]
		if inQuote {
			cur.WriteRune(c)
			if c == '"' {
				inQuote = false
			}
			i++
			continue
		}
		switch c {
		case '"':
			inQuote = true
			cur.WriteRune(c)
			i++
			continue
		case '[', '(', '{':
			depthBracket++
			cur.WriteRune(c)
			i++
			continue
		case ']', ')', '}':
			if depthBracket > 0 {
				depthBracket--
			}
			cur.WriteRune(c)
			i++
			continue
		}
		if depthBracket == 0 {
			if op, consumed, ok := matchOperatorAt(runes, i); ok {
				flush()
				ops = append(ops, op)
				i += consumed
				continue
			}
		}
		cur.WriteRune(c)
		i++
	}
	flush()
	return segs, ops
}

// Operator forms, anchored at the current scan position. Order matters: the
// forms carrying an inline label (`-- text -->`) are tried before the bare
// connectors so the label is not mistaken for a node.
//
// Capture group 1 (where present) is the inline label text.
var (
	// invisible link: ~~~ (never carries a label/arrow)
	reInvisible = regexp.MustCompile(`^~~~+`)

	// inline-label forms: <open-run> <label> <close-run>
	// solid:  -- label --[>ox]?   (also handles --- with no arrow)
	reSolidLabel = regexp.MustCompile(`^<?-+\s+([^-=.<>][^-]*?)\s+-+([>ox]?)`)
	// thick:  == label ==[>]?
	reThickLabel = regexp.MustCompile(`^<?=+\s+([^=<>][^=]*?)\s+=+([>]?)`)
	// dotted: -. label .-[>]?
	reDotLabel = regexp.MustCompile(`^<?-?\.\s+([^.<>][^.]*?)\s+\.-+([>]?)`)

	// bare connectors (optionally with |piped label| handled by caller):
	reDotted = regexp.MustCompile(`^<?-\.+-+([>ox]?)`)
	reThick  = regexp.MustCompile(`^<?=+([>ox]?)`)
	reSolid  = regexp.MustCompile(`^<?-+([>ox]?)`)
)

// matchOperatorAt tries to recognize an edge operator starting at runes[i].
// It returns the parsed operator, the number of runes consumed, and ok.
func matchOperatorAt(runes []rune, i int) (opInfo, int, bool) {
	switch runes[i] {
	case '-', '=', '~', '<':
	default:
		return opInfo{}, 0, false
	}
	s := string(runes[i:])

	// Invisible link first.
	if m := reInvisible.FindString(s); m != "" {
		return opInfo{line: model.LineInvisible, arrow: model.ArrowNone, minLen: 1}, len([]rune(m)), true
	}

	// Inline-label forms.
	if m := reDotLabel.FindStringSubmatch(s); m != nil {
		op := opInfo{line: model.LineDotted, label: cleanLabel(m[1]), minLen: 1}
		op.arrow = arrowFromHead(m[2])
		return op, len([]rune(m[0])), true
	}
	if m := reThickLabel.FindStringSubmatch(s); m != nil {
		op := opInfo{line: model.LineThick, label: cleanLabel(m[1]), minLen: 1}
		op.arrow = arrowFromHead(m[2])
		return op, len([]rune(m[0])), true
	}
	if m := reSolidLabel.FindStringSubmatch(s); m != nil {
		op := opInfo{line: model.LineSolid, label: cleanLabel(m[1]), minLen: 1}
		op.arrow = arrowFromHead(m[2])
		return op, len([]rune(m[0])), true
	}

	// Bare connectors, optionally followed by a |piped label|.
	var raw string
	var line model.EdgeLineStyle
	var head string
	switch {
	case reDotted.MatchString(s):
		m := reDotted.FindStringSubmatch(s)
		raw, line, head = m[0], model.LineDotted, m[1]
	case reThick.MatchString(s) && strings.HasPrefix(strings.TrimPrefix(s, "<"), "=="):
		m := reThick.FindStringSubmatch(s)
		raw, line, head = m[0], model.LineThick, m[1]
	case reSolid.MatchString(s) && len(stripLT(s)) >= 2 && stripLT(s)[1] == '-':
		m := reSolid.FindStringSubmatch(s)
		raw, line, head = m[0], model.LineSolid, m[1]
	default:
		return opInfo{}, 0, false
	}

	consumed := len([]rune(raw))
	op := opInfo{line: line, arrow: arrowFromHead(head), minLen: operatorMinLen(raw)}
	if line == model.LineInvisible {
		op.arrow = model.ArrowNone
	}

	// Optional piped label immediately after the connector.
	after := s[len(raw):]
	if strings.HasPrefix(after, "|") {
		if end := strings.IndexByte(after[1:], '|'); end >= 0 {
			op.label = cleanLabel(after[1 : 1+end])
			consumed += len([]rune(after[:1+end+1]))
		}
	}
	return op, consumed, true
}

func stripLT(s string) string { return strings.TrimPrefix(s, "<") }

func arrowFromHead(h string) model.ArrowType {
	switch h {
	case ">":
		return model.ArrowNormal
	case "o":
		return model.ArrowCircle
	case "x":
		return model.ArrowCross
	default:
		return model.ArrowNone
	}
}

// operatorMinLen computes the requested rank span from the number of
// dash/equal characters. Base operators (-->, ---, ==>) are length 1.
func operatorMinLen(raw string) int {
	body := strings.TrimRight(raw, ">ox")
	switch {
	case strings.HasPrefix(body, "=="):
		n := strings.Count(body, "=")
		if n <= 2 {
			return 1
		}
		return n - 1
	case strings.Contains(body, "."):
		// dotted: -.-, -..-, -...-  -> dots beyond 1
		dots := strings.Count(body, ".")
		if dots <= 1 {
			return 1
		}
		return dots
	default:
		n := strings.Count(body, "-")
		if n <= 2 {
			return 1
		}
		return n - 1
	}
}
