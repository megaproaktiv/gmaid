// Package seqparser turns Mermaid sequenceDiagram source into a seqmodel.Diagram.
//
// Supported subset (covers all project testdata plus common spec features):
//   - header: sequenceDiagram
//   - autonumber
//   - participant/actor declarations with optional "as" alias
//   - messages: ->, -->, ->>, -->>, -x, --x, -), --), <<->>, <<-->>
//     with optional +/- activation suffix and self-messages
//   - Note over|left of|right of X[,Y]: text
//   - blocks: loop/alt/opt/par/critical/break/rect/box ... [else/and/option] ... end
//   - activate/deactivate
//   - %% comments
package seqparser

import (
	"bufio"
	"strings"

	"github.com/megaproaktiv/gmaid/seqmodel"
)

// Parse reads sequence-diagram source and returns the populated diagram.
func Parse(src string) (*seqmodel.Diagram, error) {
	d := seqmodel.New()
	sc := bufio.NewScanner(strings.NewReader(src))
	sc.Buffer(make([]byte, 0, 1024*1024), 1024*1024)
	first := true
	for sc.Scan() {
		line := strings.TrimSpace(stripComment(sc.Text()))
		if line == "" {
			continue
		}
		if first {
			first = false
			if strings.EqualFold(fieldsFirst(line), "sequenceDiagram") {
				continue // consume header
			}
			// no header; fall through and parse the line anyway
		}
		parseLine(d, line)
	}
	return d, sc.Err()
}

func parseLine(d *seqmodel.Diagram, line string) {
	lower := strings.ToLower(line)
	switch {
	case lower == "autonumber" || strings.HasPrefix(lower, "autonumber "):
		d.Autonumber = true
		return
	case strings.HasPrefix(lower, "participant "):
		parseParticipant(d, line[len("participant "):], seqmodel.KindParticipant)
		return
	case strings.HasPrefix(lower, "actor "):
		parseParticipant(d, line[len("actor "):], seqmodel.KindActor)
		return
	case strings.HasPrefix(lower, "note "):
		parseNote(d, line)
		return
	case strings.HasPrefix(lower, "activate "):
		d.Events = append(d.Events, &seqmodel.Event{Kind: seqmodel.EvActivate, Target: strings.TrimSpace(line[len("activate "):])})
		d.EnsureParticipant(strings.TrimSpace(line[len("activate "):]))
		return
	case strings.HasPrefix(lower, "deactivate "):
		d.Events = append(d.Events, &seqmodel.Event{Kind: seqmodel.EvDeactivate, Target: strings.TrimSpace(line[len("deactivate "):])})
		return
	case lower == "end":
		d.Events = append(d.Events, &seqmodel.Event{Kind: seqmodel.EvBlockEnd})
		return
	}

	if ev, ok := parseBlockStart(line); ok {
		d.Events = append(d.Events, ev)
		return
	}
	if ev, ok := parseBlockElse(line); ok {
		d.Events = append(d.Events, ev)
		return
	}

	// Otherwise: a message line.
	parseMessage(d, line)
}

// parseParticipant handles `X as Label` or `X`.
func parseParticipant(d *seqmodel.Diagram, rest string, kind seqmodel.ParticipantKind) {
	rest = strings.TrimSpace(rest)
	id, label := rest, rest
	if idx := indexWord(rest, " as "); idx >= 0 {
		id = strings.TrimSpace(rest[:idx])
		label = strings.TrimSpace(rest[idx+4:])
	}
	p := d.EnsureParticipant(id)
	p.Label = cleanText(label)
	p.Kind = kind
}

// parseNote handles `Note over A[,B]: text`, `Note left of A: text`,
// `Note right of A: text`.
func parseNote(d *seqmodel.Diagram, line string) {
	rest := strings.TrimSpace(line[len("note"):])
	lower := strings.ToLower(rest)
	ev := &seqmodel.Event{Kind: seqmodel.EvNote}
	var afterKw string
	switch {
	case strings.HasPrefix(lower, "over "):
		ev.Placement = seqmodel.NoteOver
		afterKw = rest[len("over "):]
	case strings.HasPrefix(lower, "left of "):
		ev.Placement = seqmodel.NoteLeftOf
		afterKw = rest[len("left of "):]
	case strings.HasPrefix(lower, "right of "):
		ev.Placement = seqmodel.NoteRightOf
		afterKw = rest[len("right of "):]
	default:
		return
	}
	// afterKw = "A[,B]: text"
	colon := strings.Index(afterKw, ":")
	if colon < 0 {
		return
	}
	actorsPart := strings.TrimSpace(afterKw[:colon])
	ev.Text = cleanText(strings.TrimSpace(afterKw[colon+1:]))
	for _, a := range strings.Split(actorsPart, ",") {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}
		d.EnsureParticipant(a)
		ev.Actors = append(ev.Actors, a)
	}
	d.Events = append(d.Events, ev)
}

// parseBlockStart recognizes a block opener and its label.
func parseBlockStart(line string) (*seqmodel.Event, bool) {
	for _, kw := range []string{"loop", "alt", "opt", "par", "critical", "break", "rect", "box"} {
		if line == kw || strings.HasPrefix(strings.ToLower(line), kw+" ") {
			text := strings.TrimSpace(line[len(kw):])
			ev := &seqmodel.Event{Kind: seqmodel.EvBlockStart, BlockType: kw}
			if kw == "rect" || kw == "box" {
				ev.BlockColor, text = splitColorPrefix(text)
			}
			ev.BlockText = cleanText(text)
			return ev, true
		}
	}
	return nil, false
}

// parseBlockElse recognizes else/and/option dividers.
func parseBlockElse(line string) (*seqmodel.Event, bool) {
	lower := strings.ToLower(line)
	for _, kw := range []string{"else", "and", "option"} {
		if line == kw || strings.HasPrefix(lower, kw+" ") {
			return &seqmodel.Event{Kind: seqmodel.EvBlockElse, BlockText: cleanText(strings.TrimSpace(line[len(kw):]))}, true
		}
	}
	return nil, false
}
