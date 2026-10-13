package gparser

import (
	"strconv"
	"strings"
	"time"

	"github.com/megaproaktiv/gmaid/gmodel"
)

// resolveDates computes each task's Start/End from its raw spec, handling
// explicit dates, durations, "after <id>", and sequential defaults, then sets
// the chart's overall Min/Max range.
func resolveDates(c *gmodel.Chart) {
	layout := goLayout(c.DateFormat)
	byID := map[string]*gmodel.Task{}
	var prev *gmodel.Task

	for _, t := range c.Tasks {
		start := resolveStart(t.RawStart(), layout, prev, byID)
		end := resolveEnd(t.RawEnd(), layout, start, prev, byID)
		if end.Before(start) {
			end = start
		}
		t.Start, t.End = start, end
		if t.ID != "" {
			byID[t.ID] = t
		}
		prev = t
	}

	// overall range
	first := true
	for _, t := range c.Tasks {
		if first {
			c.Min, c.Max = t.Start, t.End
			first = false
			continue
		}
		if t.Start.Before(c.Min) {
			c.Min = t.Start
		}
		if t.End.After(c.Max) {
			c.Max = t.End
		}
	}
	if first {
		c.Min = time.Now()
		c.Max = c.Min.AddDate(0, 0, 1)
	}
}

func resolveStart(raw, layout string, prev *gmodel.Task, byID map[string]*gmodel.Task) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		if prev != nil {
			return prev.End
		}
		return time.Now()
	}
	lower := strings.ToLower(raw)
	if strings.HasPrefix(lower, "after ") {
		latest := time.Time{}
		for _, id := range strings.Fields(raw[len("after "):]) {
			if ot, ok := byID[id]; ok && ot.End.After(latest) {
				latest = ot.End
			}
		}
		if !latest.IsZero() {
			return latest
		}
		if prev != nil {
			return prev.End
		}
		return time.Now()
	}
	if d, ok := parseDate(raw, layout); ok {
		return d
	}
	if prev != nil {
		return prev.End
	}
	return time.Now()
}

func resolveEnd(raw, layout string, start time.Time, prev *gmodel.Task, byID map[string]*gmodel.Task) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return start
	}
	lower := strings.ToLower(raw)
	if strings.HasPrefix(lower, "until ") {
		for _, id := range strings.Fields(raw[len("until "):]) {
			if ot, ok := byID[id]; ok {
				return ot.Start
			}
		}
		return start
	}
	if dur, ok := parseDuration(raw); ok {
		return addDuration(start, dur.n, dur.unit)
	}
	if d, ok := parseDate(raw, layout); ok {
		return d
	}
	return start
}

type dur struct {
	n    float64
	unit string
}

func parseDuration(tok string) (dur, bool) {
	i := 0
	for i < len(tok) && ((tok[i] >= '0' && tok[i] <= '9') || tok[i] == '.') {
		i++
	}
	if i == 0 {
		return dur{}, false
	}
	n, err := strconv.ParseFloat(tok[:i], 64)
	if err != nil {
		return dur{}, false
	}
	unit := tok[i:]
	switch unit {
	case "ms", "s", "m", "h", "d", "w", "M", "y":
		return dur{n: n, unit: unit}, true
	}
	return dur{}, false
}

func addDuration(t time.Time, n float64, unit string) time.Time {
	switch unit {
	case "ms":
		return t.Add(time.Duration(n * float64(time.Millisecond)))
	case "s":
		return t.Add(time.Duration(n * float64(time.Second)))
	case "m":
		return t.Add(time.Duration(n * float64(time.Minute)))
	case "h":
		return t.Add(time.Duration(n * float64(time.Hour)))
	case "d":
		return t.Add(time.Duration(n * 24 * float64(time.Hour)))
	case "w":
		return t.Add(time.Duration(n * 7 * 24 * float64(time.Hour)))
	case "M":
		return t.AddDate(0, int(n), 0)
	case "y":
		return t.AddDate(int(n), 0, 0)
	}
	return t
}

// parseDate parses a date using the Go layout derived from dateFormat.
func parseDate(s, layout string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if t, err := time.Parse(layout, s); err == nil {
		return t, true
	}
	// fall back to a few common layouts
	for _, l := range []string{"2006-01-02", "02-01-2006", "01/02/2006", "2006/01/02"} {
		if t, err := time.Parse(l, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// goLayout converts a Mermaid/dayjs dateFormat into a Go reference layout.
func goLayout(f string) string {
	repl := strings.NewReplacer(
		"YYYY", "2006",
		"YY", "06",
		"MMMM", "January",
		"MMM", "Jan",
		"MM", "01",
		"DD", "02",
		"Do", "02",
		"HH", "15",
		"mm", "04",
		"ss", "05",
	)
	out := repl.Replace(f)
	if out == "" {
		return "2006-01-02"
	}
	return out
}
