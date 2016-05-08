package main

import (
	"fmt"
	"math"

	"github.com/tajtiattila/photomap/quadtree"
)

type point struct {
	x, y float64
}

type pent struct {
	point
	group int
}

type ptqs []pent

func (s ptqs) Len() int                { return len(s) }
func (s ptqs) At(i int) (x, y float64) { return s[i].x, s[i].y }

func groupNearbyPoints(pts []point, dist float64) []group {
	v := make([]pent, len(pts))
	for i := range pts {
		v[i] = pent{point: pts[i], group: -1}
	}
	// find overlapping groups
	qt := quadtree.New(ptqs(v))
	var grps [][]int
	for i := range v {
		ei := &v[i]
		qt.RectFunc(ei.x-dist, ei.y-dist, ei.x+dist, ei.y+dist, func(j int) bool {
			if i == j {
				return true
			}
			ej := &v[j]
			switch {
			case ei.group == ej.group:
				if ei.group == -1 {
					igrp := len(grps)
					grps = append(grps, []int{i, j})
					ei.group, ej.group = igrp, igrp
				}
			case ei.group == -1: // ej.group != -1
				jgrp := ej.group
				grps[jgrp] = append(grps[jgrp], i)
				ei.group = ej.group
			case ej.group == -1: // ei.group != -1
				igrp := ei.group
				grps[igrp] = append(grps[igrp], j)
				ej.group = ei.group
			default: // ei.group != ej.group, both != -1
				igrp, jgrp := ei.group, ej.group
				var kgrp, ogrp int
				if len(grps[igrp]) > len(grps[jgrp]) {
					kgrp, ogrp = igrp, jgrp
				} else {
					kgrp, ogrp = jgrp, igrp
				}
				n := len(grps[kgrp])
				grps[kgrp] = append(grps[kgrp], grps[ogrp]...)
				for _, k := range grps[kgrp][n:] {
					v[k].group = kgrp
				}
				ei.group, ej.group = kgrp, kgrp
				grps[ogrp] = nil
			}
			return true
		})
	}
	var res []group
	// create groups
	for _, g := range grps {
		res = append(res, subdivideGroup(pts, g, dist)...)
	}
	// add ungrouped points
	for i, pt := range pts {
		if v[i].group == -1 {
			res = append(res, group{pt, []int{i}})
		}
	}
	return res
}

type group struct {
	center point
	elems  []int
}

func subdivideGroup(pts []point, g []int, dist float64) []group {
	switch len(g) {
	case 0:
		return nil
	case 1:
		return []group{
			{center: pts[g[0]], elems: g},
		}
	}
	pt0 := pts[g[0]]
	xmin, xmax := pt0.x, pt0.x
	ymin, ymax := pt0.y, pt0.y
	for i := 1; i < len(g); i++ {
		pt := pts[g[i]]
		xmin = math.Min(xmin, pt.x)
		ymin = math.Min(ymin, pt.y)
		xmax = math.Max(xmax, pt.x)
		ymax = math.Max(ymax, pt.y)
	}

	center := point{
		(xmin + xmax) / 2,
		(ymin + ymax) / 2,
	}
	dx, dy := xmax-xmin, ymax-ymin

	var size float64
	horz := dx > dy
	if horz {
		size = dx
	} else {
		size = dy
	}

	if size < dist*2 {
		// can't subdivide
		return []group{
			{center: center, elems: g},
		}
	}

	var first func(pt point) bool
	if horz {
		first = func(pt point) bool {
			return pt.x < center.x
		}
	} else {
		first = func(pt point) bool {
			return pt.y < center.y
		}
	}

	var one, two []int
	for _, i := range g {
		if first(pts[i]) {
			one = append(one, i)
		} else {
			two = append(two, i)
		}
	}

	aone, atwo := pointsAvg(pts, one), pointsAvg(pts, two)
	var adist float64
	if horz {
		adist = aone.x - atwo.x
	} else {
		adist = aone.y - atwo.y
	}
	if math.Abs(adist) < dist {
		// subdivision failed
		return []group{
			{center: center, elems: g},
		}
	}

	return append(
		subdivideGroup(pts, one, dist),
		subdivideGroup(pts, two, dist)...)
}

func pointsAvg(pts []point, g []int) point {
	var avg point
	for _, i := range g {
		avg.x += pts[i].x
		avg.y += pts[i].y
	}
	avg.x /= float64(len(g))
	avg.y /= float64(len(g))
	return avg
}

func query(root *node, x0, y0, x1, y1, mindist float64, cb func(pt point, images []int)) {
	q := nodeQuery{bounds: Rectangle{x0, y0, x1, y1},
		mindist: mindist,
		cb:      cb,
	}
	q.visit(root)
}

type nodeQuery struct {
	bounds  Rectangle
	mindist float64

	n int

	cb func(pt point, images []int)
}

func (q *nodeQuery) visit(n *node) {
	if !q.bounds.overlaps(n.bounds) {
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
		q.cb(n.center, n.images)
		return
	}
	for i := range n.children {
		q.visit(&n.children[i])
	}
}

type node struct {
	center point
	weight int // total number of images
	bounds Rectangle

	mindist float64 // children of this node are at least this far apart

	children []node

	images []int // some image(s) representing this node
}

func dump(n *node, indent string) {
	fmt.Printf("%snode(%v,%v,%v,%v) @%v,%v: %v {\n", indent,
		n.bounds.x0, n.bounds.y0, n.bounds.x1, n.bounds.y1,
		n.center.x, n.center.y, len(n.images))
	for _, c := range n.children {
		dump(&c, indent+"  ")
	}
	fmt.Println(indent + "}")
}

func makeTree(pts []point, mindist float64) *node {
	dist := mindist
	var nodes []node
	for _, g := range groupNearbyPoints(pts, dist) {
		nodes = append(nodes, node{
			center:  g.center,
			weight:  len(g.elems),
			bounds:  rectAround(g.center, dist),
			mindist: dist,
			images:  g.elems,
		})
	}

	for len(nodes) > 1 {
		dist *= 2
		abovepts := make([]point, len(nodes))
		for i, n := range nodes {
			abovepts[i] = n.center
		}
		grps := groupNearbyPoints(abovepts, dist)
		if len(grps) == len(nodes) {
			// nothing was merged
			continue
		}
		belownodes := nodes
		nodes = make([]node, 0, len(grps))
		for _, grp := range grps {
			g := grp.elems
			if len(g) == 1 {
				node := belownodes[g[0]]
				node.mindist = dist
				node.bounds.extend(rectAround(node.center, dist))
				nodes = append(nodes, node)
			} else {
				children := make([]node, 0, len(g))
				nimg := 0
				for _, i := range g {
					n := belownodes[i]
					children = append(children, n)
					nimg += len(n.images)
				}
				var c point
				var cw int
				var bounds Rectangle
				images := make([]int, 0, nimg)
				for _, n := range children {
					c.x += n.center.x * float64(n.weight)
					c.y += n.center.y * float64(n.weight)
					cw += n.weight
					bounds.extend(n.bounds)
					images = append(images, n.images...)
				}
				c.x /= float64(cw)
				c.y /= float64(cw)
				bounds.extend(rectAround(c, dist))
				nodes = append(nodes, node{
					center:   c,
					weight:   cw,
					bounds:   bounds,
					mindist:  dist,
					children: children,
					images:   images,
				})
			}
		}
	}
	return &nodes[0]
}

type Rectangle struct {
	x0, y0, x1, y1 float64
}

func rectAround(c point, r float64) Rectangle {
	return Rectangle{c.x - r, c.y - r, c.x + r, c.y + r}
}

func (r *Rectangle) extend(s Rectangle) {
	if s.empty() {
		return
	}
	if r.empty() {
		*r = s
		return
	}
	r.x0 = math.Min(r.x0, s.x0)
	r.y0 = math.Min(r.y0, s.y0)
	r.x1 = math.Max(r.x1, s.x1)
	r.y1 = math.Max(r.y1, s.y1)
}

func (r Rectangle) empty() bool {
	return r.x0 >= r.x1 || r.y0 >= r.y1
}

func (r Rectangle) overlaps(s Rectangle) bool {
	return !r.empty() && !s.empty() &&
		r.x0 < s.x1 && s.x0 < r.x1 &&
		r.y0 < s.y1 && s.y0 < r.y1
}
