package layout

import (
	"github.com/megaproaktiv/gmaid/model"
	"github.com/megaproaktiv/gmaid/textmetrics"
)

// Engine performs layout for a graph.
type Engine struct {
	cfg  Config
	face *textmetrics.Face
}

// NewEngine builds an engine with the given config.
func NewEngine(cfg Config) *Engine {
	return &Engine{cfg: cfg, face: textmetrics.New(cfg.FontSize, false)}
}

// block is the laid-out result of a node or a subgraph, used as a unit in a
// parent group. It carries a local placement that is later translated.
type block struct {
	id    string
	isSub bool
	w, h  float64

	// for a real node:
	node  *model.Node
	lines []string

	// for a subgraph: fully solved internal content in local coords
	subTitle string
	subNodes []NodeBox
	subSubs  []SubBox
	// internal edges fully contained in this subgraph (routed in local coords)
	subEdges []EdgePath
	depth    int
}

// Layout computes the full diagram.
func (e *Engine) Layout(g *model.Graph) *Diagram {
	// Determine top-level members: nodes and subgraphs with empty parent.
	topSubs := []string{}
	for _, id := range g.SubOrder {
		if g.Subgraphs[id].Parent == "" {
			topSubs = append(topSubs, id)
		}
	}
	topNodes := []string{}
	for _, id := range g.NodeOrder {
		if g.Nodes[id].Parent == "" {
			topNodes = append(topNodes, id)
		}
	}

	blocks := map[string]*block{}
	for _, id := range topNodes {
		blocks[id] = e.nodeBlock(g.Nodes[id])
	}
	for _, id := range topSubs {
		blocks[id] = e.subgraphBlock(g, g.Subgraphs[id], 0)
	}

	// Map every node id to its top-level block id so inter-group edges can be
	// drawn between the enclosing blocks.
	owner := map[string]string{} // node id -> top-level block id
	for _, id := range topNodes {
		owner[id] = id
	}
	for _, sid := range topSubs {
		for _, nid := range descendantNodeIDs(g, sid) {
			owner[nid] = sid
		}
	}

	dia := e.solveGroup(g, string(g.Direction), blocks, owner, topNodes, topSubs)
	e.finalizeCanvas(dia)
	return dia
}

// nodeBlock measures a node and returns its block.
func (e *Engine) nodeBlock(n *model.Node) *block {
	lines := textmetrics.SplitLines(n.Label)
	tw, th := e.face.MeasureLines(lines, e.cfg.LineHeight)
	w := tw + 2*e.cfg.NodeHPad
	h := th + 2*e.cfg.NodeVPad
	// Shape-specific size adjustments.
	switch n.Shape {
	case model.ShapeDiamond:
		w *= 1.5
		h *= 1.7
	case model.ShapeCircle:
		d := maxf(w, h) * 1.1
		w, h = d, d
	case model.ShapeStadium, model.ShapeRound:
		w += e.cfg.NodeHPad
	case model.ShapeCylinder:
		h += 14
	case model.ShapeHexagon, model.ShapeParallelogram, model.ShapeParallelogramAlt,
		model.ShapeTrapezoid, model.ShapeTrapezoidAlt:
		w += 24
	}
	if w < e.cfg.MinNodeW {
		w = e.cfg.MinNodeW
	}
	if h < e.cfg.MinNodeH {
		h = e.cfg.MinNodeH
	}
	return &block{id: n.ID, node: n, lines: lines, w: w, h: h}
}

// subgraphBlock lays out a subgraph's internal content and returns a composite
// block sized to contain it (plus padding and title space).
func (e *Engine) subgraphBlock(g *model.Graph, sg *model.Subgraph, depth int) *block {
	// Direct children: split into nodes and nested subgraphs.
	childNodes := []string{}
	childSubs := []string{}
	for _, cid := range sg.Children {
		if _, ok := g.Subgraphs[cid]; ok {
			childSubs = append(childSubs, cid)
		} else if _, ok := g.Nodes[cid]; ok {
			childNodes = append(childNodes, cid)
		}
	}

	inner := map[string]*block{}
	for _, id := range childNodes {
		inner[id] = e.nodeBlock(g.Nodes[id])
	}
	for _, id := range childSubs {
		inner[id] = e.subgraphBlock(g, g.Subgraphs[id], depth+1)
	}

	owner := map[string]string{}
	for _, id := range childNodes {
		owner[id] = id
	}
	for _, sid := range childSubs {
		for _, nid := range descendantNodeIDs(g, sid) {
			owner[nid] = sid
		}
	}

	dir := string(sg.Direction)
	if dir == "" {
		dir = string(g.Direction) // inherit
	}

	// Mermaid lays an isolated group (nodes that only connect outside the
	// subgraph) as a single lane perpendicular to the parent flow: a vertical
	// column for an LR parent, a horizontal row for a TB parent. Our layered
	// stacker always advances along the group's own rank axis, so when the
	// resolved direction is parallel to the parent flow we flip it to the
	// perpendicular axis to reproduce that column/row.
	if subgraphChildrenIsolated(g, childNodes, childSubs) {
		parent := string(g.Direction)
		if sg.Parent != "" {
			if psg := g.Subgraphs[sg.Parent]; psg != nil && psg.Direction != "" {
				parent = string(psg.Direction)
			}
		}
		dir = perpendicular(parent)
	}

	sub := e.solveGroup(g, dir, inner, owner, childNodes, childSubs)

	// Pad and reserve title space; shift content down/right by padding.
	pad := e.cfg.SubPad
	titleH := e.cfg.SubTitleH
	dx := pad
	dy := pad + titleH
	translateDiagram(sub, dx, dy)

	b := &block{
		id:       sg.ID,
		isSub:    true,
		subTitle: sg.Title,
		subNodes: sub.Nodes,
		subSubs:  sub.Subs,
		subEdges: sub.Edges,
		depth:    depth,
		w:        sub.Width + 2*pad,
		h:        sub.Height + 2*pad + titleH,
	}
	return b
}

// solveGroup lays out a set of blocks (nodes + subgraph composites) connected
// by the edges whose endpoints resolve (through owner) to these blocks. It
// returns a Diagram in local coordinates (origin at 0,0).
func (e *Engine) solveGroup(g *model.Graph, dir string, blocks map[string]*block, owner map[string]string, nodeIDs, subIDs []string) *Diagram {
	grp := &group{
		cfg:   e.cfg,
		flow:  dir,
		verts: map[string]*vertex{},
	}
	add := func(id string) {
		b := blocks[id]
		grp.verts[id] = &vertex{id: id, w: b.w, h: b.h}
		grp.order = append(grp.order, id)
	}
	for _, id := range nodeIDs {
		add(id)
	}
	for _, id := range subIDs {
		add(id)
	}

	// Build links between blocks from graph edges that live within this group.
	inGroup := func(id string) (string, bool) {
		// a node id belongs here if its owning top block is one of our verts
		if o, ok := owner[id]; ok {
			if _, has := grp.verts[o]; has {
				return o, true
			}
		}
		if _, has := grp.verts[id]; has {
			return id, true
		}
		return "", false
	}

	key := 0
	edgeByKey := map[int]*model.Edge{}
	endpoints := map[int][2]string{} // key -> block ids (for chain lookup)
	realEnds := map[int][2]string{}  // key -> actual node ids
	for _, ed := range g.Edges {
		fb, fok := inGroup(ed.From)
		tb, tok := inGroup(ed.To)
		if !fok || !tok || fb == tb {
			continue
		}
		if ed.Line == model.LineInvisible {
			// invisible: influences layout as a link but is not drawn
			grp.links = append(grp.links, link{from: fb, to: tb})
			continue
		}
		grp.links = append(grp.links, link{from: fb, to: tb, orig: &edgeRef{key: key}})
		edgeByKey[key] = ed
		endpoints[key] = [2]string{fb, tb}
		realEnds[key] = [2]string{ed.From, ed.To}
		key++
	}

	// Reserve rank-axis space for edge labels so they fit between ranks, as
	// mermaid does. For horizontal flows the label's width matters; for
	// vertical flows its (small) height does.
	horizontal := dir == "LR" || dir == "RL"
	maxLabel := 0.0
	for _, ed := range edgeByKey {
		if ed.Label == "" {
			continue
		}
		lines := textmetrics.SplitLines(ed.Label)
		if horizontal {
			w := 0.0
			for _, ln := range lines {
				lw, _ := e.face.Measure(ln)
				if lw > w {
					w = lw
				}
			}
			if w > maxLabel {
				maxLabel = w
			}
		} else {
			h := float64(len(lines)) * e.cfg.LineHeight
			if h > maxLabel {
				maxLabel = h
			}
		}
	}
	grp.labelExtent = maxLabel

	chains := grp.solve()

	// Compose the diagram in local coordinates.
	dia := &Diagram{}
	// place blocks
	placed := map[string]*block{}
	for _, id := range grp.order {
		v := grp.verts[id]
		if v.dummy {
			continue
		}
		b := blocks[id]
		if b == nil {
			continue
		}
		placed[id] = b
		if b.isSub {
			// translate the subgraph's internal content to v.x,v.y
			nb := translateSub(b, v.x, v.y)
			dia.Subs = append(dia.Subs, nb.sub)
			dia.Nodes = append(dia.Nodes, nb.nodes...)
			dia.Subs = append(dia.Subs, nb.subs...)
			dia.Edges = append(dia.Edges, nb.edges...)
		} else {
			dia.Nodes = append(dia.Nodes, NodeBox{
				ID: id, Node: b.node, X: v.x, Y: v.y, W: b.w, H: b.h, Lines: b.lines,
			})
		}
	}

	// Build an index of final absolute node boxes for accurate endpoint
	// clipping (important for edges that cross subgraph boundaries).
	nodeByID := map[string]*NodeBox{}
	for i := range dia.Nodes {
		nodeByID[dia.Nodes[i].ID] = &dia.Nodes[i]
	}

	// route edges along their chains, clipping the ends to the actual source
	// and target node boxes rather than the enclosing composite block.
	for k, ed := range edgeByKey {
		chain := chains[k]
		if len(chain) < 2 {
			ep := endpoints[k]
			chain = []string{ep[0], ep[1]}
		}
		re := realEnds[k]
		pts := e.routeChain(grp, nodeByID, chain, re[0], re[1])
		if len(pts) < 2 {
			continue
		}
		path := EdgePath{Edge: ed, Points: pts, Label: ed.Label}
		if ed.Label != "" {
			mid := pts[len(pts)/2]
			if len(pts)%2 == 0 {
				a, b := pts[len(pts)/2-1], pts[len(pts)/2]
				mid = Point{(a.X + b.X) / 2, (a.Y + b.Y) / 2}
			}
			path.LabelPos = mid
		}
		dia.Edges = append(dia.Edges, path)
	}

	// compute bounds
	e.computeBounds(dia)
	return dia
}

// routeChain converts a vertex-id chain into a polyline of points. The first
// and last points are the centers of the actual source/target node boxes
// (srcID/dstID) when known, so cross-subgraph edges connect real nodes. The
// intermediate points come from dummy vertex centers. Endpoints are clipped to
// the respective node-box borders.
func (e *Engine) routeChain(grp *group, nodeByID map[string]*NodeBox, chain []string, srcID, dstID string) []Point {
	pts := make([]Point, 0, len(chain))

	srcBox := nodeByID[srcID]
	dstBox := nodeByID[dstID]

	for i, id := range chain {
		switch {
		case i == 0 && srcBox != nil:
			pts = append(pts, srcBox.Center())
		case i == len(chain)-1 && dstBox != nil:
			pts = append(pts, dstBox.Center())
		default:
			v := grp.verts[id]
			if v == nil {
				continue
			}
			pts = append(pts, Point{v.x + v.w/2, v.y + v.h/2})
		}
	}
	if len(pts) < 2 {
		return pts
	}
	if srcBox != nil {
		pts[0] = clipBox(pts[0], pts[1], *srcBox)
	}
	if dstBox != nil {
		pts[len(pts)-1] = clipBox(pts[len(pts)-1], pts[len(pts)-2], *dstBox)
	}
	return pts
}

// clipBox returns the point on the border of box b along the line from the box
// center toward target.
func clipBox(center, target Point, b NodeBox) Point {
	cx, cy := b.X+b.W/2, b.Y+b.H/2
	dx, dy := target.X-cx, target.Y-cy
	if dx == 0 && dy == 0 {
		return center
	}
	hw, hh := b.W/2, b.H/2
	scale := 1e18
	if dx != 0 {
		scale = minf(scale, hw/absf(dx))
	}
	if dy != 0 {
		scale = minf(scale, hh/absf(dy))
	}
	return Point{cx + dx*scale, cy + dy*scale}
}

func (e *Engine) computeBounds(dia *Diagram) {
	minX, minY := 1e18, 1e18
	maxX, maxY := -1e18, -1e18
	upd := func(x, y, w, h float64) {
		if x < minX {
			minX = x
		}
		if y < minY {
			minY = y
		}
		if x+w > maxX {
			maxX = x + w
		}
		if y+h > maxY {
			maxY = y + h
		}
	}
	for _, n := range dia.Nodes {
		upd(n.X, n.Y, n.W, n.H)
	}
	for _, s := range dia.Subs {
		upd(s.X, s.Y, s.W, s.H)
	}
	for _, ed := range dia.Edges {
		for _, p := range ed.Points {
			upd(p.X, p.Y, 0, 0)
		}
		if ed.Label != "" {
			upd(ed.LabelPos.X-20, ed.LabelPos.Y-10, 40, 20)
		}
	}
	if minX == 1e18 {
		minX, minY, maxX, maxY = 0, 0, 0, 0
	}
	// shift so content starts at 0,0
	translateDiagram(dia, -minX, -minY)
	dia.Width = maxX - minX
	dia.Height = maxY - minY
}

func (e *Engine) finalizeCanvas(dia *Diagram) {
	m := e.cfg.Margin
	translateDiagram(dia, m, m)
	dia.Width += 2 * m
	dia.Height += 2 * m
}

// subgraphChildrenIsolated reports whether none of the subgraph's direct
// members are connected to each other (every edge from a member goes to a node
// outside this subgraph). Such groups are laid out as a single perpendicular
// lane, matching mermaid.
func subgraphChildrenIsolated(g *model.Graph, childNodes, childSubs []string) bool {
	if len(childNodes)+len(childSubs) < 2 {
		return false
	}
	member := map[string]bool{}
	for _, id := range childNodes {
		member[id] = true
	}
	subOf := map[string]string{}
	for _, sid := range childSubs {
		for _, nid := range descendantNodeIDs(g, sid) {
			subOf[nid] = sid
		}
	}
	resolve := func(id string) (string, bool) {
		if member[id] {
			return id, true
		}
		if s, ok := subOf[id]; ok {
			return s, true
		}
		return "", false
	}
	for _, ed := range g.Edges {
		f, fok := resolve(ed.From)
		t, tok := resolve(ed.To)
		if fok && tok && f != t {
			return false // an internal edge exists -> not isolated
		}
	}
	return true
}

// perpendicular returns the flow direction rotated 90 degrees: horizontal
// flows map to TB and vertical flows map to LR.
func perpendicular(dir string) string {
	switch dir {
	case "LR", "RL":
		return "TB"
	default:
		return "LR"
	}
}

// descendantNodeIDs returns all node ids contained (recursively) in a subgraph.
func descendantNodeIDs(g *model.Graph, sid string) []string {
	var out []string
	sg := g.Subgraphs[sid]
	if sg == nil {
		return out
	}
	for _, cid := range sg.Children {
		if _, ok := g.Subgraphs[cid]; ok {
			out = append(out, descendantNodeIDs(g, cid)...)
		} else if _, ok := g.Nodes[cid]; ok {
			out = append(out, cid)
		}
	}
	return out
}

func maxf(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
func minf(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
func absf(a float64) float64 {
	if a < 0 {
		return -a
	}
	return a
}
