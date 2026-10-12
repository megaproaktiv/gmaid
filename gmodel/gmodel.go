// Package gmodel defines the in-memory representation of a parsed Mermaid gantt
// chart: the chart options, sections and tasks with resolved start/end dates.
//
// Separate from the flowchart and sequence models so gantt support evolves
// independently.
package gmodel

import "time"

// TaskTag is a status tag on a task.
type TaskTag int

const (
	TagNormal TaskTag = iota
	TagActive
	TagDone
	TagCrit
)

// Task is one bar (or milestone) in the chart.
type Task struct {
	ID        string
	Label     string
	Tag       TaskTag
	Milestone bool
	Start     time.Time
	End       time.Time
	Section   string
	// raw spec pieces kept for date resolution
	rawStart string // explicit date, "after X", or "" (sequential)
	rawEnd   string // duration (e.g. 7d) or explicit end date
}

// SetRaw stores the unresolved start/end spec strings for later resolution.
func (t *Task) SetRaw(start, end string) {
	t.rawStart = start
	t.rawEnd = end
}

// RawStart and RawEnd expose the stored spec strings to the resolver.
func (t *Task) RawStart() string { return t.rawStart }
func (t *Task) RawEnd() string   { return t.rawEnd }

// Section groups tasks under a named band.
type Section struct {
	Name  string
	Tasks []*Task
}

// Chart is a whole parsed gantt diagram.
type Chart struct {
	Title      string
	DateFormat string // input format, e.g. YYYY-MM-DD
	AxisFormat string // output strftime-ish, e.g. %d.%m
	TickEvery  int    // tick interval count
	TickUnit   string // "day","week","month",...
	Sections   []*Section
	Tasks      []*Task // all tasks in order (also referenced by sections)

	Min, Max time.Time // overall date range (set during resolution)
}

// New returns an empty chart with defaults.
func New() *Chart {
	return &Chart{
		DateFormat: "YYYY-MM-DD",
		AxisFormat: "%Y-%m-%d",
		TickEvery:  1,
		TickUnit:   "week",
	}
}
