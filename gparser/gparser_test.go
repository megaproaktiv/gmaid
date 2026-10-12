package gparser

import (
	"os"
	"testing"
)

func TestParseGantt(t *testing.T) {
	for _, f := range []string{
		"../testdata/gantt-100.mmd",
		"../testdata/gantt-110.mmd",
	} {
		b, _ := os.ReadFile(f)
		c, err := Parse(string(b))
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("%s: title=%q df=%s af=%s tick=%d%s sections=%d tasks=%d range=%s..%s",
			f, c.Title, c.DateFormat, c.AxisFormat, c.TickEvery, c.TickUnit,
			len(c.Sections), len(c.Tasks), c.Min.Format("2006-01-02"), c.Max.Format("2006-01-02"))
		for _, s := range c.Sections {
			for _, tk := range s.Tasks {
				t.Logf("  [%s] %-28s tag=%d ms=%v %s -> %s", s.Name, tk.Label, tk.Tag, tk.Milestone,
					tk.Start.Format("2006-01-02"), tk.End.Format("2006-01-02"))
			}
		}
	}
}
