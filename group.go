package main

import (
	"math"

	"github.com/tajtiattila/photomap/quadtree"
)

type point struct {
	x, y float64
}

type pent struct {
	point
	group *int
}

type ptqs []pent

func (s ptqs) Len() int                { return len(s) }
func (s ptqs) At(i int) (x, y float64) { return s[i].x, s[i].y }

type group struct {
	center point
	elems  []int
}

func groupNearby(pts []point, dist float64) []group {
	v := make([]pent, len(pts))
	for i := range pts {
		v[i] = pent{point: pts[i]}
	}
	// find overlapping groups
	qt := quadtree.New(ptqs(v))
	var grps [][]int
	for i, ei := range v {
		qt.RectFunc(ei.x-dist, ei.y-dist, ei.x+dist, ei.y+dist, func(j int) bool {
			ej := v[j]
			switch {
			case ei.group == ej.group:
				if ei.group == nil {
					igrp := len(grps)
					grps = append(grps, []int{i, j})
					ei.group, ej.group = &igrp, &igrp
				}
			case ei.group == nil: // ej.group != nil
				igrp := *ej.group
				grps[igrp] = append(grps[igrp], i)
				ei.group = ej.group
			case ej.group == nil: // ei.group != nil
				igrp := *ei.group
				grps[igrp] = append(grps[igrp], j)
				ej.group = ei.group
			default: // ei.group != ej.group, both non-nil
				igrp, jgrp := *ei.group, *ej.group
				grps[igrp] = append(grps[igrp], grps[jgrp]...)
				grps[jgrp] = nil
				*ej.group = igrp
			}
			return true
		})
	}
	// create groups by subdivision
	var res []group
	for _, g := range grps {
		res = append(res, subdivide(pts, g, dist)...)
	}
	return res
}

func subdivide(pts []point, g []int, dist float64) []group {
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
	for i := 1; i < len(pts); i++ {
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

	return append(
		subdivide(pts, one, dist),
		subdivide(pts, two, dist)...)
}

type rectangle struct {
	min, max point
}

func rect(x0, y0, x1, y1 float64) rectangle {
	return rectangle{point{x0, y0}, point{x1, y1}}
}

func (r *rectangle) add(q rectangle) {
	if q.min == q.max {
		*r = q
	} else {
		r.min.x = math.Min(r.min.x, q.min.x)
		r.min.y = math.Min(r.min.y, q.min.y)
		r.max.x = math.Max(r.max.x, q.max.x)
		r.max.y = math.Max(r.max.y, q.max.y)
	}
}
