package gparser

import "testing"

// Tests for gmaid gantt extensions: dependency links, summary tasks,
// same-line tasks, and <br> normalization.
func TestGanttExtensions(t *testing.T) {
	src := `gantt
	title Ext
	dateFormat YYYY-MM-DD
	section WP1<br/>Name<br/>40 PD
	Summary bar :summary, s1, 2026-01-01, 30d
	Task A      :a, 2026-01-01, 10d
	&Task B     :b, 2026-01-11, 10d
	link a FS b
	link a SS s1
	`
	c, err := Parse(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	// <br> in section name becomes newlines.
	if got := c.Sections[0].Name; got != "WP1\nName\n40 PD" {
		t.Errorf("section name = %q, want multi-line", got)
	}

	// summary + sameline flags
	var s1, a, b bool
	for _, tk := range c.Tasks {
		switch tk.ID {
		case "s1":
			if !tk.Summary {
				t.Error("s1 should be Summary")
			}
			s1 = true
		case "a":
			a = true
		case "b":
			if !tk.SameLine {
				t.Error("b should be SameLine (& prefix)")
			}
			b = true
		}
	}
	if !s1 || !a || !b {
		t.Fatalf("missing tasks: s1=%v a=%v b=%v", s1, a, b)
	}

	// links parsed
	if len(c.Links) != 2 {
		t.Fatalf("want 2 links, got %d", len(c.Links))
	}
	if c.Links[0].From != "a" || c.Links[0].To != "b" {
		t.Errorf("link0 = %+v", c.Links[0])
	}
}
