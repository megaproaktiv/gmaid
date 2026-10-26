// Package glayout computes pixel geometry for a parsed gantt chart: a linear
// date->x scale, one row per task, section bands, axis ticks/gridlines, the
// today marker and milestone positions.
package glayout

import (
	"time"

	"github.com/megaproaktiv/gmaid/gmodel"
	"github.com/megaproaktiv/gmaid/textmetrics"
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
	// LineSpacingPct adds extra vertical gap between wrapped label lines, as a
	// fraction of the base line height (e.g. 0.20 = +20%).
	LineSpacingPct float64
}

// DefaultConfig returns mermaid-like defaults, tuned for a wider left text
// column and an A4-landscape-friendly chart width.
func DefaultConfig() Config {
	return Config{
		Margin:         8,
		TitleH:         40,
		LeftPad:        170, // left column; long names auto-wrap (see grender)
		RightPad:       140,
		BarH:           20,
		BarGap:         4,
		AxisH:          34,
		TopPad:         12,
		FontSize:       12,
		SectionFont:    13,
		MinChartW:      900, // wider plotting area -> landscape-leaning output
		LineSpacingPct: 0.20,
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
	LabelLines []string // pre-wrapped section-name lines
	LineH      float64  // line height for the label lines (incl. spacing)
}

// Tick is an axis gridline + label.
type Tick struct {
	X     float64
	Label string
}

// LinkPath is a positioned dependency arrow: an orthogonal route from a source
// point (end/start of the predecessor bar) to a target point (start/end of the
// successor bar), with an arrowhead at the target.
type LinkPath struct {
	Pts []glayout_pt // polyline points (>=2); arrowhead drawn at the last segment
}

// glayout_pt is a 2D point (kept local to avoid importing a point type).
type glayout_pt = struct{ X, Y float64 }

// Diagram is the fully positioned gantt chart.
type Diagram struct {
	Title       string
	Bars        []Bar
	Sections    []SectionBand
	Ticks       []Tick
	Links       []LinkPath
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

	// Count rows, accounting for same-line tasks (which do NOT advance a row).
	nRows := 0
	for _, s := range c.Sections {
		for ti, t := range s.Tasks {
			if t.SameLine && ti > 0 {
				continue
			}
			nRows++
		}
	}
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

	// Section bands + bars. A running y-cursor lets a band grow taller than its
	// task rows when a wrapped section name needs more vertical space.
	sectionLineH := (cfg.SectionFont + 3) * (1 + cfg.LineSpacingPct)
	availLabelW := cfg.LeftPad - 10
	yCursor := out.ChartTop
	for si, s := range c.Sections {
		// How many real rows (same-line tasks don't add a row).
		taskRows := 0
		for ti, t := range s.Tasks {
			if t.SameLine && ti > 0 {
				continue
			}
			taskRows++
		}
		if taskRows == 0 {
			taskRows = 1
		}
		// Wrap the section name and measure how many lines it needs.
		labelLines := e.sect.WrapToWidth(textmetrics.SplitLines(s.Name), availLabelW)

		taskH := float64(taskRows) * rowH
		labelH := float64(len(labelLines)) * sectionLineH
		bandH := taskH
		if labelH > bandH {
			bandH = labelH
		}
		bandTop := yCursor
		// Bars are vertically centered within the band if it is taller than the
		// task rows (e.g. a tall wrapped label).
		rowsTop := bandTop + (bandH-taskH)/2

		rowInBand := 0
		for ti, t := range s.Tasks {
			sameLine := t.SameLine && ti > 0
			// A bar belongs to a shared row if it is a same-line follower OR
			// it is the anchor whose next sibling is same-line. Such labels are
			// centered over their own bar so they stay in the right column.
			inSharedRow := sameLine || (ti+1 < len(s.Tasks) && s.Tasks[ti+1].SameLine)
			var y float64
			if sameLine {
				y = rowsTop + float64(rowInBand-1)*rowH
			} else {
				y = rowsTop + float64(rowInBand)*rowH
			}
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
				switch {
				case lw+10 <= bar.W:
					bar.LabelInside = true
					bar.LabelX = x0 + bar.W/2
				case sameLine || inSharedRow:
					// On a shared row (e.g. monthly sums), keep the label
					// centered over its own bar even if it overflows, so it
					// stays in the correct column instead of jumping right
					// into the next bar's slot.
					bar.LabelInside = true
					bar.LabelX = x0 + bar.W/2
				default:
					bar.LabelInside = false
					bar.LabelX = x1 + 6
				}
			}
			out.Bars = append(out.Bars, bar)
			if !sameLine {
				rowInBand++
			}
		}
		bandBottom := bandTop + bandH
		out.Sections = append(out.Sections, SectionBand{
			Name: s.Name, X: cfg.Margin, Y: bandTop,
			W: out.ChartRight + cfg.RightPad - cfg.Margin, H: bandH,
			Index: si, LabelY: (bandTop + bandBottom) / 2,
			LabelLines: labelLines, LineH: sectionLineH,
		})
		yCursor = bandBottom
	}

	// Real chart bottom from the running cursor.
	out.ChartBottom = yCursor

	// Axis ticks by tickInterval.
	out.Ticks = e.ticks(c, min, max, dateX)

	// Dependency arrows (gmaid extension). Build a task-id -> bar index map.
	if len(c.Links) > 0 {
		byID := make(map[string]int, len(out.Bars))
		for i := range out.Bars {
			if id := out.Bars[i].Task.ID; id != "" {
				byID[id] = i
			}
		}
		for _, lk := range c.Links {
			fi, ok1 := byID[lk.From]
			ti, ok2 := byID[lk.To]
			if !ok1 || !ok2 {
				continue
			}
			out.Links = append(out.Links, linkPath(out.Bars[fi], out.Bars[ti], lk.Kind, cfg.BarH))
		}
	}

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

// barAnchor returns the (x,y) of a bar's start or end edge at its vertical
// center. For milestones, start==end==diamond center x.
func barAnchor(b Bar, atEnd bool) (float64, float64) {
	if b.Milestone {
		return b.MX, b.MY
	}
	y := b.Y + b.H/2
	if atEnd {
		return b.X + b.W, y
	}
	return b.X, y
}

// linkPath routes an orthogonal dependency arrow between two bars according to
// the link kind (which edges to connect): FS end->start, SS start->start,
// FF end->end, SF start->end. The route leaves the source horizontally, steps
// vertically at a small offset past the source, then enters the target
// horizontally so the arrowhead meets the target edge head-on.
func linkPath(from, to Bar, kind gmodel.LinkKind, barH float64) LinkPath {
	var srcEnd, dstEnd bool
	switch kind {
	case gmodel.LinkFS:
		srcEnd, dstEnd = true, false
	case gmodel.LinkSS:
		srcEnd, dstEnd = false, false
	case gmodel.LinkFF:
		srcEnd, dstEnd = true, true
	case gmodel.LinkSF:
		srcEnd, dstEnd = false, true
	}
	sx, sy := barAnchor(from, srcEnd)
	tx, ty := barAnchor(to, dstEnd)

	stub := barH * 0.6 // how far to stick out of the source edge
	var pts []glayout_pt
	// Leave the source edge horizontally (right for an end anchor, left for a
	// start anchor), then a vertical run, then approach the target edge.
	sdir := 1.0
	if !srcEnd {
		sdir = -1.0
	}
	ddir := -1.0 // approach a start edge from the left
	if dstEnd {
		ddir = 1.0 // approach an end edge from the right
	}
	midX := tx + ddir*stub
	pts = append(pts, glayout_pt{sx, sy})
	pts = append(pts, glayout_pt{sx + sdir*stub, sy})
	pts = append(pts, glayout_pt{sx + sdir*stub, ty}) // vertical run at source stub x
	// If target approach x differs, step across at target row height.
	pts = append(pts, glayout_pt{midX, ty})
	pts = append(pts, glayout_pt{tx, ty})
	return LinkPath{Pts: pts}
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
