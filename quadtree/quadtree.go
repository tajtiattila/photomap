package quadtree

import (
	"math"
	"sort"
)

// Source provides data for a quadtree.
type Source interface {
	// Len reports the number of elements
	Len() int

	// At reports the position of ith element
	At(i int) (x, y float64)
}

// Quadtree is a data structure for efficient
// position based lookups.
type Quadtree struct {
	src Source

	root qnode

	maxnodelen int     // maximum element count when node should not be subdivided
	mindist    float64 // minimum node size (width or height) that should not be subdivided
}

// New creates a new Quadtree from src using opt.
func New(src Source, opt ...Option) *Quadtree {
	var min, max point

	// find boundary
	if src.Len() > 0 {
		x, y := src.At(0)
		min = point{x, y}
		max = point{x, y}
		for i := 1; i < src.Len(); i++ {
			x, y = src.At(i)
			min.x = math.Min(min.x, x)
			min.y = math.Min(min.y, y)
			max.x = math.Max(max.x, x)
			max.y = math.Max(max.y, y)
		}
	}

	// make sure root node is a square
	sx, sy := max.x-min.x, max.y-min.y
	var size float64
	if sx > sy {
		size = sx
		cy := (min.y + max.y) / 2
		min.y = math.Min(min.y, cy-size/2)
		max.y = math.Max(max.y, cy+size/2)
	} else {
		size = sy
		cx := (min.x + max.x) / 2
		min.x = math.Min(min.x, cx-size/2)
		max.x = math.Max(max.x, cx+size/2)
	}

	// prepare leaves
	leaves := make([]int, src.Len())
	for i := range leaves {
		leaves[i] = i
	}

	qt := &Quadtree{
		src: src,
		root: qnode{
			min:    min,
			max:    max,
			leaves: leaves,
		},
		maxnodelen: 16,
		mindist:    size / math.Pow(2, 24),
	}

	for _, o := range opt {
		o.set(qt)
	}

	qt.subdivide(&qt.root)

	return qt
}

// NearFunc is like RectFunc but f(i) might be called with
// indices for elements outside minx, miny, maxx, maxy.
func (qt *Quadtree) NearFunc(minx, miny, maxx, maxy float64, f func(i int) (ok bool)) {
	qt.root.query(nodequery{
		min: point{minx, miny},
		max: point{maxx, maxy},
		f:   f,
	})
}

// RectFunc calls f(i) for all indices that are within the bounds
// specified by the rectangle minx, miny, maxx, maxy.
func (qt *Quadtree) RectFunc(minx, miny, maxx, maxy float64, f func(i int) (ok bool)) {
	qt.root.query(nodequery{
		min: point{minx, miny},
		max: point{maxx, maxy},
		f: func(i int) bool {
			x, y := qt.src.At(i)
			if minx <= x && x <= maxx && miny <= y && y <= maxy {
				return f(i)
			}
			return true
		},
	})
}

// CircleFunc calls f(i) for all indices that are within the bounds
// specified by the circle at cx, cy having radius r.
func (qt *Quadtree) CircleFunc(cx, cy, r float64, f func(i int) (ok bool)) {
	qt.root.query(nodequery{
		min: point{cx - r, cy - r},
		max: point{cx + r, cy + r},
		f: func(i int) bool {
			x, y := qt.src.At(i)
			dx, dy := x-cx, y-cy
			if math.Sqrt(dx*dx+dy*dy) <= r {
				return f(i)
			}
			return true
		},
	})
}

func (qt *Quadtree) Near(minx, miny, maxx, maxy float64, p []int) []int {
	qt.NearFunc(minx, miny, maxx, maxy, func(i int) bool {
		p = append(p, i)
		return true
	})
	return p
}

// Rect appends all indices to p that are within the bounds
// specified by the rectangle minx, miny, maxx, maxy,
// and returns the resulting slice.
func (qt *Quadtree) Rect(minx, miny, maxx, maxy float64, p []int) []int {
	qt.RectFunc(minx, miny, maxx, maxy, func(i int) bool {
		p = append(p, i)
		return true
	})
	return p
}

// Circle appends all indices to p that are within the bounds
// specified by the circle at cx, cy having radius r,
// and returns the resulting slice.
func (qt *Quadtree) Circle(cx, cy, r float64, p []int) []int {
	qt.CircleFunc(cx, cy, r, func(i int) bool {
		p = append(p, i)
		return true
	})
	return p
}

func (qt *Quadtree) subdivide(n *qnode) {

	if len(n.leaves) < qt.maxnodelen || n.min.x+qt.mindist >= n.max.x {
		return // no need to or cannot subdivide
	}

	center := point{
		(n.min.x + n.max.x) / 2,
		(n.min.y + n.max.y) / 2,
	}

	ns := &nodeSort{c: center, src: qt.src, v: n.leaves}
	sort.Sort(ns)

	v := make([]qnode, 0, 4)
	lastqi := -1
	var child *qnode
	for i := range n.leaves {
		qi := ns.quad(i)
		if qi != lastqi {
			v = append(v, qnode{leaves: n.leaves[i : i+1]})
			child = &v[len(v)-1]
			switch qi {
			case 0:
				child.min = n.min
				child.max = center
			case 1:
				child.min.x = center.x
				child.min.y = n.min.y
				child.max.x = n.max.x
				child.max.y = center.y
			case 2:
				child.min.x = n.min.x
				child.min.y = center.y
				child.max.x = center.x
				child.max.y = n.max.y
			case 3:
				child.min = center
				child.max = n.max
			}
		} else {
			child.leaves = child.leaves[:len(child.leaves)+1]
		}
		lastqi = qi
	}

	n.children = append([]qnode(nil), v...)
	n.leaves = nil

	for i := range n.children {
		qt.subdivide(&n.children[i])
	}
}

type Option interface {
	set(qt *Quadtree)
}

// MaxNodeLen sets the maximum quad leaf length to l.
func MaxNodeLen(l int) Option { return maxNodeLen(l) }

type maxNodeLen int

func (o maxNodeLen) set(qt *Quadtree) {
	qt.maxnodelen = int(o)
}

// MaxDepth sets the maximum depth for the quadtree.
// Zero means no subdivision.
func MaxDepth(d int) Option { return maxDepth(d) }

type maxDepth int

func (o maxDepth) set(qt *Quadtree) {
	siz := qt.root.max.x - qt.root.min.x
	qt.mindist = siz / math.Pow(2, float64(o))
}

// MinDist sets the minimum quad size that can be further subdivided to d.
func MinDist(d float64) Option { return minDist(d) }

type minDist float64

func (o minDist) set(qt *Quadtree) {
	qt.mindist = float64(o)
}

type point struct {
	x, y float64
}

type qnode struct {
	min, max point
	children []qnode // 0 to 4 subtrees (empty ones omitted)
	leaves   []int   // indices for Quadtree.src.At()
}

func (n *qnode) query(q nodequery) bool {
	if q.max.x < n.min.x || n.max.x < q.min.x ||
		q.max.y < n.min.y || n.max.y < q.min.y {
		return true
	}
	if n.children != nil {
		for i := range n.children {
			if !n.children[i].query(q) {
				return false
			}
		}
	} else {
		for _, i := range n.leaves {
			if !q.f(i) {
				return false
			}
		}
	}
	return true
}

type nodequery struct {
	min, max point

	f func(i int) bool
}

// nodeSort sorts v according to which quad they are with
// relation to center c. The order of coordinates is X, Y:
// Nodes left of c (smaller X) comes first, then Nodes below c
// (smaller Y) within those blocks comes first.
type nodeSort struct {
	c   point
	src Source
	v   []int
}

func (s *nodeSort) Len() int           { return len(s.v) }
func (s *nodeSort) Swap(i, j int)      { s.v[i], s.v[j] = s.v[j], s.v[i] }
func (s *nodeSort) Less(i, j int) bool { return s.quad(i) < s.quad(j) }

// quad reports which quad v[i] is in
//
//  0 | 1
// ---+---
//  2 | 3
func (s *nodeSort) quad(i int) (qi int) {
	x, y := s.src.At(s.v[i])
	if s.c.x <= x {
		qi++
	}
	if s.c.y <= y {
		qi += 2
	}
	return qi
}
