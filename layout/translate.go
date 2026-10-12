package layout

// translateDiagram shifts every element of a diagram by (dx,dy) in place.
func translateDiagram(d *Diagram, dx, dy float64) {
	for i := range d.Nodes {
		d.Nodes[i].X += dx
		d.Nodes[i].Y += dy
	}
	for i := range d.Subs {
		d.Subs[i].X += dx
		d.Subs[i].Y += dy
	}
	for i := range d.Edges {
		for j := range d.Edges[i].Points {
			d.Edges[i].Points[j].X += dx
			d.Edges[i].Points[j].Y += dy
		}
		d.Edges[i].LabelPos.X += dx
		d.Edges[i].LabelPos.Y += dy
	}
}

// translatedSub is the result of embedding a subgraph block at an offset.
type translatedSub struct {
	sub   SubBox
	nodes []NodeBox
	subs  []SubBox
	edges []EdgePath
}

// translateSub moves a subgraph composite block so its top-left is at (x,y),
// returning the outer SubBox plus translated interior content.
func translateSub(b *block, x, y float64) translatedSub {
	ts := translatedSub{
		sub: SubBox{ID: b.id, Title: b.subTitle, X: x, Y: y, W: b.w, H: b.h, Depth: b.depth},
	}
	// Interior content was laid out starting at local origin; shift by (x,y).
	for _, n := range b.subNodes {
		n.X += x
		n.Y += y
		ts.nodes = append(ts.nodes, n)
	}
	for _, s := range b.subSubs {
		s.X += x
		s.Y += y
		ts.subs = append(ts.subs, s)
	}
	for _, ed := range b.subEdges {
		pts := make([]Point, len(ed.Points))
		for i, p := range ed.Points {
			pts[i] = Point{p.X + x, p.Y + y}
		}
		ed.Points = pts
		ed.LabelPos = Point{ed.LabelPos.X + x, ed.LabelPos.Y + y}
		ts.edges = append(ts.edges, ed)
	}
	return ts
}
