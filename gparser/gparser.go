// Package gparser turns Mermaid gantt source into a gmodel.Chart with resolved
// task dates.
//
// Supported subset (covers project testdata + common spec features):
//   - header: gantt
//   - title, dateFormat, axisFormat, tickInterval
//   - section <name>
//   - tasks:  Label : [tags,] [id,] <start>, <end|duration>
//     tags: done, active, crit, milestone (comma-separated, first)
//     start: explicit date (dateFormat), "after <id>", or omitted (sequential)
//     end:   duration like 7d/2w/1M, or explicit date
//   - %% comments
package gparser

import (
	"bufio"
	"strings"

	"github.com/megaproaktiv/gmaid/gmodel"
)

// Parse reads gantt source and returns a chart with resolved dates.
func Parse(src string) (*gmodel.Chart, error) {
	c := gmodel.New()
	sc := bufio.NewScanner(strings.NewReader(src))
	sc.Buffer(make([]byte, 0, 1<<20), 1<<20)
	first := true
	var curSection *gmodel.Section

	for sc.Scan() {
		line := strings.TrimSpace(stripComment(sc.Text()))
		if line == "" {
			continue
		}
		if first {
			first = false
			if strings.EqualFold(firstWord(line), "gantt") {
				continue
			}
		}
		lower := strings.ToLower(line)
		switch {
		case strings.HasPrefix(lower, "title "):
			c.Title = strings.TrimSpace(line[len("title "):])
		case strings.HasPrefix(lower, "dateformat "):
			c.DateFormat = strings.TrimSpace(line[len("dateformat "):])
		case strings.HasPrefix(lower, "axisformat "):
			c.AxisFormat = strings.TrimSpace(line[len("axisformat "):])
		case strings.HasPrefix(lower, "tickinterval "):
			parseTickInterval(c, strings.TrimSpace(line[len("tickinterval "):]))
		case strings.HasPrefix(lower, "excludes "), strings.HasPrefix(lower, "weekend "),
			strings.HasPrefix(lower, "todaymarker"), strings.HasPrefix(lower, "click "),
			strings.HasPrefix(lower, "vert "):
			// not needed for the target files; ignored
		case strings.HasPrefix(lower, "section "):
			curSection = &gmodel.Section{Name: strings.TrimSpace(line[len("section "):])}
			c.Sections = append(c.Sections, curSection)
		default:
			if t := parseTask(line); t != nil {
				if curSection == nil {
					curSection = &gmodel.Section{Name: ""}
					c.Sections = append(c.Sections, curSection)
				}
				t.Section = curSection.Name
				curSection.Tasks = append(curSection.Tasks, t)
				c.Tasks = append(c.Tasks, t)
			}
		}
	}
	resolveDates(c)
	return c, sc.Err()
}

// parseTask parses "Label : meta1, meta2, ..." into a Task with raw pieces.
func parseTask(line string) *gmodel.Task {
	colon := strings.Index(line, ":")
	if colon < 0 {
		return nil
	}
	label := strings.TrimSpace(line[:colon])
	meta := strings.TrimSpace(line[colon+1:])
	parts := splitTrim(meta, ",")
	if len(parts) == 0 {
		return nil
	}
	t := &gmodel.Task{Label: label}

	// Leading tags.
	for len(parts) > 0 {
		switch strings.ToLower(parts[0]) {
		case "done":
			t.Tag = gmodel.TagDone
			parts = parts[1:]
			continue
		case "active":
			t.Tag = gmodel.TagActive
			parts = parts[1:]
			continue
		case "crit":
			t.Tag = gmodel.TagCrit
			parts = parts[1:]
			continue
		case "milestone":
			t.Milestone = true
			parts = parts[1:]
			continue
		}
		break
	}

	// Remaining: [id,] start, end  OR  [id,] end  OR  start,end etc.
	// Heuristic: if the first remaining token is not a date / not "after" /
	// not a duration, treat it as the task id.
	if len(parts) >= 1 && isID(parts[0]) {
		t.ID = parts[0]
		parts = parts[1:]
	}
	switch len(parts) {
	case 0:
		// nothing
	case 1:
		t.SetRaw("", parts[0])
	default:
		t.SetRaw(parts[0], parts[1])
	}
	return t
}

func parseTickInterval(c *gmodel.Chart, s string) {
	// pattern: <n><unit>  e.g. 1week, 2day, 1month
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == 0 {
		return
	}
	n := atoi(s[:i])
	unit := strings.ToLower(strings.TrimSpace(s[i:]))
	unit = strings.TrimSuffix(unit, "s")
	if n > 0 {
		c.TickEvery = n
	}
	switch unit {
	case "millisecond", "second", "minute", "hour", "day", "week", "month":
		c.TickUnit = unit
	}
}

// --- helpers ---

func stripComment(line string) string {
	if i := strings.Index(line, "%%"); i >= 0 {
		line = line[:i]
	}
	return strings.TrimRight(line, " \t")
}

func firstWord(s string) string {
	f := strings.Fields(s)
	if len(f) == 0 {
		return ""
	}
	return f[0]
}

func splitTrim(s, sep string) []string {
	var out []string
	for _, p := range strings.Split(s, sep) {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// isID reports whether a token looks like a task identifier (not a date, not a
// duration, not an "after/until" clause).
func isID(tok string) bool {
	lower := strings.ToLower(tok)
	if strings.HasPrefix(lower, "after ") || strings.HasPrefix(lower, "until ") {
		return false
	}
	if isDuration(tok) {
		return false
	}
	if looksLikeDate(tok) {
		return false
	}
	return true
}

func looksLikeDate(tok string) bool {
	// crude: contains a digit and a '-' or '/' or '.' separator with >=2 groups
	digits := 0
	for _, r := range tok {
		if r >= '0' && r <= '9' {
			digits++
		}
	}
	return digits >= 4 && (strings.Count(tok, "-")+strings.Count(tok, "/")+strings.Count(tok, ".") >= 2)
}

func isDuration(tok string) bool {
	if tok == "" {
		return false
	}
	i := 0
	for i < len(tok) && ((tok[i] >= '0' && tok[i] <= '9') || tok[i] == '.') {
		i++
	}
	if i == 0 {
		return false
	}
	unit := tok[i:]
	switch unit {
	case "ms", "s", "m", "h", "d", "w", "M", "y":
		return true
	}
	return false
}

func atoi(s string) int {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			break
		}
		n = n*10 + int(r-'0')
	}
	return n
}
