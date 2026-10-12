// Package glayout computes pixel geometry for a parsed gantt chart: a linear
// date->x scale, one row per task, section bands, axis ticks/gridlines, the
// today marker and milestone positions.
package glayout

import (
	"time"

	"gmaid/gmodel"
	"gmaid/textmetrics"
)

// Config holds spacing parameters (pixels, pre-scale), tuned mermaid-like.
type Config struct {
	Margin      float64
	TitleH      float64 // vertical space for the title
	LeftPad     float64 // space on the left for section names
	RightPad    float64 // space on the right for overflow labels
	BarH        float64 // task bar height
	BarGap      float64 // vertical gap between task rows
	AxisH       float64 // space at the bottom for the date axis
	TopPad      float64 // gap between title and first row
	FontSize    float64
	SectionFont float64
	MinChartW   float64
}

// DefaultConfig returns mermaid-like defaults.
func DefaultConfig() Config {
	return Config{
		Margin:      8,
		TitleH:      40,
		LeftPad:     100,
		RightPad:    120,
		BarH:        20,
		BarGap:      4,
		AxisH:       34,
		TopPad:      12,
		FontSize:    12,
		SectionFont: 13,
		MinChartW:   560,
	}
}

// Bar is a positioned task bar or milestone.
type Bar struct {
	Task        *gmodel.Task
	X, Y, W, H  float64
	Milestone   bool
	MX, MY      float64 // milestone diamond center
	Label       string
	LabelX      float64
	LabelInside bool // label fits inside the bar
}

// SectionBand is the alternating background band for a section.
type SectionBand struct {
	Name       string
	X, Y, W, H float64
	Index      int // for alternating colors
	LabelY     float64
}

// Tick is an axis gridline + label.
type Tick struct {
	X     float64
	Label string
}

// Diagram is the fully positioned gantt chart.
type Diagram struct {
	Title       string
	Bars        []Bar
	Sections    []SectionBand
	Ticks       []Tick
	Width       float64
	Height      float64
	ChartLeft   float64 // x where the plotting area begins
	ChartRight  float64
	ChartTop    float64 // y where the first band begins
	ChartBottom float64 // y where the axis sits (bands end)
	TodayX      float64 // x of the today marker, <0 if out of range
	AxisFormat  string
}

// Engine lays out gantt charts.
type Engine struct {
	cfg  Config
	face *textmetrics.Face
	sect *textmetrics.Face
}

// NewEngine builds an engine.
func NewEngine(cfg Config) *Engine {
	return &Engine{
		cfg:  cfg,
		face: textmetrics.New(cfg.FontSize, false),
		sect: textmetrics.New(cfg.SectionFont, true),
	}
}

// Layout computes geometry for the whole chart.
func (e *Engine) Layout(c *gmodel.Chart) *Diagram {
	cfg := e.cfg
	out := &Diagram{Title: c.Title, AxisFormat: c.AxisFormat}

	// Count rows (one per task).
	nRows := len(c.Tasks)
	if nRows == 0 {
		out.Width, out.Height = 1, 1
		return out
	}

	// Chart plotting area.
	out.ChartLeft = cfg.Margin + cfg.LeftPad
	chartW := cfg.MinChartW
	out.ChartRight = out.ChartLeft + chartW
	out.ChartTop = cfg.Margin + cfg.TitleH + cfg.TopPad

	rowH := cfg.BarH + cfg.BarGap
	out.ChartBottom = out.ChartTop + float64(nRows)*rowH

	// Date -> x scale over [Min, Max], with a little padding so bars don't
	// touch the edges.
	min := c.Min
	max := c.Max
	span := max.Sub(min).Seconds()
	if span <= 0 {
		span = 86400 // 1 day fallback
	}
	dateX := func(t time.Time) float64 {
		frac := t.Sub(min).Seconds() / span
		return out.ChartLeft + frac*(out.ChartRight-out.ChartLeft)
	}

	// Section bands + bars.
	rowIdx := 0
	for si, s := range c.Sections {
		bandTop := out.ChartTop + float64(rowIdx)*rowH
		for _, t := range s.Tasks {
			y := out.ChartTop + float64(rowIdx)*rowH
			bar := Bar{Task: t, Y: y, H: cfg.BarH, Label: t.Label}
			if t.Milestone {
				cx := dateX(midpoint(t.Start, t.End))
				bar.Milestone = true
				bar.MX = cx
				bar.MY = y + cfg.BarH/2
				bar.LabelX = cx + cfg.BarH/2 + 6
				bar.LabelInside = false
			} else {
				x0 := dateX(t.Start)
				x1 := dateX(t.End)
				if x1-x0 < 2 {
					x1 = x0 + 2
				}
				bar.X, bar.W = x0, x1-x0
				lw, _ := e.face.Measure(t.Label)
				if lw+10 <= bar.W {
					bar.LabelInside = true
					bar.LabelX = x0 + bar.W/2
				} else {
					bar.LabelInside = false
					bar.LabelX = x1 + 6
				}
			}
			out.Bars = append(out.Bars, bar)
			rowIdx++
		}
		bandBottom := out.ChartTop + float64(rowIdx)*rowH
		out.Sections = append(out.Sections, SectionBand{
			Name: s.Name, X: cfg.Margin, Y: bandTop,
			W: out.ChartRight + cfg.RightPad - cfg.Margin, H: bandBottom - bandTop,
			Index: si, LabelY: (bandTop + bandBottom) / 2,
		})
	}

	// Axis ticks by tickInterval.
	out.Ticks = e.ticks(c, min, max, dateX)

	// Today marker.
	now := time.Now()
	if !now.Before(min) && !now.After(max) {
		out.TodayX = dateX(now)
	} else {
		out.TodayX = -1
	}

	out.Width = out.ChartRight + cfg.RightPad + cfg.Margin
	out.Height = out.ChartBottom + cfg.AxisH + cfg.Margin
	return out
}

func (e *Engine) ticks(c *gmodel.Chart, min, max time.Time, dateX func(time.Time) float64) []Tick {
	var ticks []Tick
	// Start at the floor of min to a natural boundary for the unit.
	cur := alignDown(min, c.TickUnit)
	count := 0
	for cur.Before(max.AddDate(0, 0, 1)) && count < 500 {
		if !cur.Before(min) {
			ticks = append(ticks, Tick{X: dateX(cur), Label: formatAxis(cur, c.AxisFormat)})
		}
		cur = advance(cur, c.TickEvery, c.TickUnit)
		count++
	}
	return ticks
}

func midpoint(a, b time.Time) time.Time {
	return a.Add(b.Sub(a) / 2)
}

func alignDown(t time.Time, unit string) time.Time {
	y, m, d := t.Date()
	switch unit {
	case "month":
		return time.Date(y, m, 1, 0, 0, 0, 0, t.Location())
	case "week":
		// back up to Monday
		wd := int(t.Weekday())
		if wd == 0 {
			wd = 7
		}
		return time.Date(y, m, d, 0, 0, 0, 0, t.Location()).AddDate(0, 0, -(wd - 1))
	default: // day, hour, etc.
		return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
	}
}

func advance(t time.Time, n int, unit string) time.Time {
	switch unit {
	case "month":
		return t.AddDate(0, n, 0)
	case "week":
		return t.AddDate(0, 0, 7*n)
	case "hour":
		return t.Add(time.Duration(n) * time.Hour)
	case "minute":
		return t.Add(time.Duration(n) * time.Minute)
	default: // day
		return t.AddDate(0, 0, n)
	}
}
