package seqparser

import (
	"strings"

	"gmaid/seqmodel"
)

// parseMessage parses `A->>B: text`, `A-->>B:text`, self-messages, and the
// optional +/- activation suffix on the arrow target.
func parseMessage(d *seqmodel.Diagram, line string) {
	// Split off the message text after the first ':'.
	text := ""
	head := line
	if colon := strings.Index(line, ":"); colon >= 0 {
		head = strings.TrimSpace(line[:colon])
		text = cleanText(strings.TrimSpace(line[colon+1:]))
	}

	from, to, arrow, ok := splitArrow(head)
	if !ok {
		return
	}

	ev := &seqmodel.Event{Kind: seqmodel.EvMessage, Text: text, Arrow: arrow}

	// activation suffix: trailing + activates recipient, - deactivates sender
	to = strings.TrimSpace(to)
	if strings.HasSuffix(to, "+") {
		ev.Activate = true
		to = strings.TrimSpace(strings.TrimSuffix(to, "+"))
	} else if strings.HasSuffix(to, "-") {
		ev.Deactivate = true
		to = strings.TrimSpace(strings.TrimSuffix(to, "-"))
	}

	from = strings.TrimSpace(from)
	ev.From = from
	ev.To = to
	d.EnsureParticipant(from)
	d.EnsureParticipant(to)
	d.Events = append(d.Events, ev)
}

// splitArrow finds the message operator in head and returns from, to, style.
func splitArrow(head string) (from, to string, style seqmodel.ArrowStyle, ok bool) {
	// Try each operator form, longest first.
	type opDef struct {
		op     string
		dotted bool
		head   seqmodel.HeadType
	}
	ops := []opDef{
		{"<<-->>", true, seqmodel.HeadBi},
		{"<<->>", false, seqmodel.HeadBi},
		{"-->>", true, seqmodel.HeadArrow},
		{"->>", false, seqmodel.HeadArrow},
		{"--x", true, seqmodel.HeadCross},
		{"-x", false, seqmodel.HeadCross},
		{"--)", true, seqmodel.HeadOpen},
		{"-)", false, seqmodel.HeadOpen},
		{"-->", true, seqmodel.HeadNone},
		{"->", false, seqmodel.HeadNone},
	}
	// Find the earliest-position, longest operator.
	bestIdx := -1
	var best opDef
	for _, o := range ops {
		if i := strings.Index(head, o.op); i >= 0 {
			// prefer the match that starts earliest; on tie, the longest op
			if bestIdx < 0 || i < bestIdx || (i == bestIdx && len(o.op) > len(best.op)) {
				bestIdx = i
				best = o
			}
		}
	}
	if bestIdx < 0 {
		return "", "", style, false
	}
	from = head[:bestIdx]
	to = head[bestIdx+len(best.op):]
	style = seqmodel.ArrowStyle{Dotted: best.dotted, Head: best.head}
	return from, to, style, true
}

// --- lexical helpers ---

func stripComment(line string) string {
	if i := strings.Index(line, "%%"); i >= 0 {
		line = line[:i]
	}
	return strings.TrimRight(line, " \t")
}

func fieldsFirst(line string) string {
	f := strings.Fields(line)
	if len(f) == 0 {
		return ""
	}
	return f[0]
}

// indexWord finds a case-insensitive " as " style delimiter.
func indexWord(s, delim string) int {
	return strings.Index(strings.ToLower(s), delim)
}

// cleanText normalizes message/label text: decode <br/>, entities, strip quotes.
func cleanText(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "\"")
	repl := strings.NewReplacer(
		"<br/>", "\n", "<br />", "\n", "<br>", "\n",
		"#59;", ";", "#35;", "#",
		"&lt;", "<", "&gt;", ">", "&amp;", "&", "&nbsp;", " ",
	)
	return strings.TrimSpace(repl.Replace(s))
}

// splitColorPrefix separates an optional leading color token (rgb(...),
// rgba(...), hsl(...), or a named color) from a rect/box label.
func splitColorPrefix(s string) (color, rest string) {
	s = strings.TrimSpace(s)
	lower := strings.ToLower(s)
	for _, fn := range []string{"rgba(", "rgb(", "hsla(", "hsl("} {
		if strings.HasPrefix(lower, fn) {
			if end := strings.Index(s, ")"); end >= 0 {
				return s[:end+1], strings.TrimSpace(s[end+1:])
			}
		}
	}
	// named color prefix: first token if it looks like a color word
	fields := strings.Fields(s)
	if len(fields) >= 1 {
		switch strings.ToLower(fields[0]) {
		case "transparent", "aqua", "red", "green", "blue", "yellow", "gray", "grey", "white", "black":
			return fields[0], strings.TrimSpace(s[len(fields[0]):])
		}
	}
	return "", s
}
