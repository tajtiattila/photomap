package clusterer

import (
	"fmt"
	"math"
)

type Tree struct {
	root node
}

func NewTree(intf Interface, mindist float64) *Tree {
	return &Tree{makeTree(intf, mindist)}
}

func (t *Tree) Query(x0, y0, x1, y1, mindist float64, f func(p Point, elem []int)) {
	q := nodeQuery{bounds: Rectangle{x0, y0, x1, y1},
		mindist: mindist,
		cb:      f,
	}
	q.visit(&t.root)
}

type node struct {
	center Point
	weight int // total number of elements
	bounds Rectangle

	mindist float64 // children of this node are at least this far apart

	children []node

	elem []int // some image(s) representing this node
}

type nodeIntf []node

func (n nodeIntf) Len() int { return len(n) }

// At reports the position of the ith element.
func (n nodeIntf) At(i int) (x, y float64) {
	p := n[i].center
	return p.X, p.Y
}

// Weight is the weight of the ith element.
func (n nodeIntf) Weight(i int) float64 {
	return float64(n[i].weight)
}

func makeTree(intf Interface, mindist float64) node {
	dist := mindist
	var nodes []node
	for _, g := range MakeClusters(intf, dist) {
		nodes = append(nodes, node{
			center:  g.Center,
			weight:  len(g.Elem),
			bounds:  rectAround(g.Center, dist),
			mindist: dist,
			elem:    g.Elem,
		})
	}

	for len(nodes) > 1 {
		dist *= 2
		grps := MakeClusters(nodeIntf(nodes), dist)
		if len(grps) == len(nodes) {
			// nothing was merged
			continue
		}
		belownodes := nodes
		nodes = make([]node, 0, len(grps))
		for _, grp := range grps {
			g := grp.Elem
			if len(g) == 1 {
				node := belownodes[g[0]]
				node.mindist = dist
				node.bounds.Extend(rectAround(node.center, dist))
				nodes = append(nodes, node)
			} else {
				children := make([]node, 0, len(g))
				nimg := 0
				for _, i := range g {
					n := belownodes[i]
					children = append(children, n)
					nimg += len(n.elem)
				}
				var c Point
				var cw int
				var bounds Rectangle
				elem := make([]int, 0, nimg)
				for _, n := range children {
					c.X += n.center.X * float64(n.weight)
					c.Y += n.center.Y * float64(n.weight)
					cw += n.weight
					bounds.Extend(n.bounds)
					elem = append(elem, n.elem...)
				}
				c.X /= float64(cw)
				c.Y /= float64(cw)
				bounds.Extend(rectAround(c, dist))
				nodes = append(nodes, node{
					center:   c,
					weight:   cw,
					bounds:   bounds,
					mindist:  dist,
					children: children,
					elem:     elem,
				})
			}
		}
	}
	return nodes[0]
}

func dump(n *node, indent string) {
	fmt.Printf("%snode(%v,%v,%v,%v) @%v,%v: %v {\n", indent,
		n.bounds.X0, n.bounds.Y0, n.bounds.X1, n.bounds.Y1,
		n.center.X, n.center.Y, len(n.elem))
	for _, c := range n.children {
		dump(&c, indent+"  ")
	}
	fmt.Println(indent + "}")
}

type Rectangle struct {
	X0, Y0, X1, Y1 float64
}

func rectAround(c Point, r float64) Rectangle {
	return Rectangle{c.X - r, c.Y - r, c.X + r, c.Y + r}
}

func (r *Rectangle) Extend(s Rectangle) {
	if s.Empty() {
		return
	}
	if r.Empty() {
		*r = s
		return
	}
	r.X0 = math.Min(r.X0, s.X0)
	r.Y0 = math.Min(r.Y0, s.Y0)
	r.X1 = math.Max(r.X1, s.X1)
	r.Y1 = math.Max(r.Y1, s.Y1)
}

func (r Rectangle) Empty() bool {
	return r.X0 >= r.X1 || r.Y0 >= r.Y1
}

func (r Rectangle) Overlaps(s Rectangle) bool {
	return !r.Empty() && !s.Empty() &&
		r.X0 < s.X1 && s.X0 < r.X1 &&
		r.Y0 < s.Y1 && s.Y0 < r.Y1
}

type nodeQuery struct {
	bounds  Rectangle
	mindist float64

	n int

	cb func(pt Point, elem []int)
}

func (q *nodeQuery) visit(n *node) {
	if !q.bounds.Overlaps(n.bounds) {
		return
	}
	showChildren := true
	for _, c := range n.children {
		if c.mindist < q.mindist {
			showChildren = false
			break
		}
	}
	if !showChildren || len(n.children) == 0 {
		q.n++
		q.cb(n.center, n.elem)
		return
	}
	for i := range n.children {
		q.visit(&n.children[i])
	}
}
