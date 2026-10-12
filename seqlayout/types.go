// Package seqlayout computes pixel geometry for a parsed sequence diagram:
// participant column positions, per-event vertical rows, note boxes, block
// frames and the overall canvas size. It uses a shared text-measuring face so
// sizes match what seqrender draws.
package seqlayout

import (
	"gmaid/seqmodel"
)

// Config holds spacing parameters (pixels, pre-scale), tuned mermaid-like.
type Config struct {
	Margin      float64 // outer canvas margin
	ActorW      float64 // min actor box width
	ActorH      float64 // actor box height
	ActorPadX   float64 // text padding inside actor box
	ActorGap    float64 // min horizontal gap between actor boxes
	TopGap      float64 // gap below top actor boxes before first event
	MsgGap      float64 // vertical gap between message rows
	SelfMsgGap  float64 // extra height for a self-message loop
	NoteGap     float64 // vertical padding around a note
	NotePadX    float64 // horizontal text padding in a note
	NotePadY    float64
	BlockPadX   float64 // horizontal inset of a block frame beyond its span
	BlockPadTop float64 // space for a block label row
	BlockGap    float64 // vertical padding inside a block
	FontSize    float64
	LineHeight  float64
	LabelFont   float64 // message/label font size
}

// DefaultConfig returns mermaid-like defaults.
func DefaultConfig() Config {
	return Config{
		Margin:      12,
		ActorW:      75,
		ActorH:      44,
		ActorPadX:   16,
		ActorGap:    12, // min edge-to-edge gap between boxes = the end margin
		TopGap:      38,
		MsgGap:      13,
		SelfMsgGap:  36,
		NoteGap:     12,
		NotePadX:    14,
		NotePadY:    10,
		BlockPadX:   8,
		BlockPadTop: 22,
		BlockGap:    10,
		FontSize:    14,
		LineHeight:  17,
		LabelFont:   13,
	}
}

// Actor is a positioned participant column.
type Actor struct {
	ID    string
	Label string
	Kind  seqmodel.ParticipantKind
	CX    float64 // center x of the lifeline
	W     float64 // box width
	Lines []string
}

// Message is a positioned message arrow.
type Message struct {
	Ev         *seqmodel.Event
	FromX, ToX float64
	Y          float64 // baseline y of the arrow
	Self       bool
	Number     int // autonumber (0 = none)
	Lines      []string
	LabelY     float64
}

// NoteBox is a positioned note.
type NoteBox struct {
	Ev         *seqmodel.Event
	X, Y, W, H float64
	Lines      []string
}

// Block is a positioned frame (loop/alt/opt/...).
type Block struct {
	Type       string
	Label      string
	Color      string
	X, Y, W, H float64
	Depth      int
	// Dividers are y positions of else/and/option lines plus their labels.
	Dividers []Divider
}

// Divider is an else/and/option separator inside a block.
type Divider struct {
	Y     float64
	Label string
}

// Diagram is the fully positioned sequence diagram.
type Diagram struct {
	Actors         []Actor
	Messages       []Message
	Notes          []NoteBox
	Blocks         []Block
	Width          float64
	Height         float64
	LifelineTop    float64 // y where lifelines start (below top actors)
	LifelineBottom float64 // y where lifelines end (above bottom actors)
}
