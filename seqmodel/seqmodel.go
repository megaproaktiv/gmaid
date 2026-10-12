// Package seqmodel defines the in-memory representation of a parsed Mermaid
// sequence diagram: participants, ordered events (messages, notes, block
// boundaries) and diagram-level options.
//
// It is intentionally separate from the flowchart model so sequence support
// can evolve without touching the flowchart implementation.
package seqmodel

// ParticipantKind is the symbol used to draw a participant.
type ParticipantKind int

const (
	KindParticipant ParticipantKind = iota // rectangle box
	KindActor                              // stick figure
)

// Participant is a lifeline column.
type Participant struct {
	ID    string // identifier used in messages
	Label string // display name (alias), defaults to ID
	Kind  ParticipantKind
}

// ArrowStyle describes the line + head of a message.
type ArrowStyle struct {
	Dotted bool // --> style (dashed line)
	Head   HeadType
}

// HeadType is the terminator drawn at the destination end of a message.
type HeadType int

const (
	HeadArrow HeadType = iota // >> filled arrowhead
	HeadOpen                  // ) async open arrow
	HeadCross                 // x cross
	HeadNone                  // plain line (-> / -->)
	HeadBi                    // <<->> bidirectional
)

// EventKind discriminates the Event union.
type EventKind int

const (
	EvMessage EventKind = iota
	EvNote
	EvBlockStart // loop/alt/opt/par/critical/rect/break/box begin
	EvBlockElse  // else / and / option divider
	EvBlockEnd   // end
	EvActivate
	EvDeactivate
)

// NotePlacement is where a note sits relative to its participant(s).
type NotePlacement int

const (
	NoteOver NotePlacement = iota
	NoteLeftOf
	NoteRightOf
)

// Event is one ordered item in the diagram body.
type Event struct {
	Kind EventKind

	// Message fields (Kind == EvMessage)
	From  string
	To    string
	Text  string
	Arrow ArrowStyle
	// activation suffix on the message (+ activates To, - deactivates From)
	Activate   bool
	Deactivate bool

	// Note fields (Kind == EvNote)
	Placement NotePlacement
	Actors    []string // one or two participant ids

	// Block fields (EvBlockStart / EvBlockElse)
	BlockType  string // "loop","alt","opt","par","critical","break","rect","box"
	BlockText  string // the label/condition
	BlockColor string // for rect/box background color, if any

	// Activate/Deactivate target (EvActivate/EvDeactivate)
	Target string
}

// Diagram is a whole parsed sequence diagram.
type Diagram struct {
	Participants []*Participant
	byID         map[string]*Participant
	Events       []*Event
	Autonumber   bool
}

// New returns an initialized empty diagram.
func New() *Diagram {
	return &Diagram{byID: map[string]*Participant{}}
}

// EnsureParticipant returns the participant with id, creating it (implicit
// declaration via first use in a message) if needed.
func (d *Diagram) EnsureParticipant(id string) *Participant {
	if p, ok := d.byID[id]; ok {
		return p
	}
	p := &Participant{ID: id, Label: id, Kind: KindParticipant}
	d.byID[id] = p
	d.Participants = append(d.Participants, p)
	return p
}

// Participant returns the participant by id (nil if absent).
func (d *Diagram) Participant(id string) *Participant {
	return d.byID[id]
}

// Index returns the column index of a participant id, or -1.
func (d *Diagram) Index(id string) int {
	for i, p := range d.Participants {
		if p.ID == id {
			return i
		}
	}
	return -1
}
