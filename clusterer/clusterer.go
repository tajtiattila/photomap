package clusterer

import (
	"math"

	"github.com/tajtiattila/photomap/quadtree"
)

// Interface is the subject of clustering.
type Interface interface {
	// Len is the number of elements.
	Len() int

	// At reports the position of the ith element.
	At(i int) (x, y float64)

	// Weight is the weight of the ith element.
	Weight(i int) float64
}

type Cluster struct {
	// center of cluster
	Center Point

	// indices of elements of this Cluster into Interface
	Elem []int
}

type Point struct {
	X, Y float64
}

// MakeClusters clusters intf using dist. It combines points
// that are less than dist apart, and then tries to subdivide
// those clusters so they are smaller than 2*dist in both
// directions.
func MakeClusters(intf Interface, dist float64) []Cluster {
	pts := make([]pent, intf.Len())
	for i := range pts {
		pts[i] = pent{Point: intfpt(intf, i), group: -1}
	}
	// find overlapping groups
	qt := quadtree.New(intf, quadtree.MinDist(dist/4))
	var grps [][]int
	for i := range pts {
		ei := &pts[i]
		qt.RectFunc(ei.X-dist, ei.Y-dist, ei.X+dist, ei.Y+dist, func(j int) bool {
			if i == j {
				return true
			}
			ej := &pts[j]
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
					pts[k].group = kgrp
				}
				ei.group, ej.group = kgrp, kgrp
				grps[ogrp] = nil
			}
			return true
		})
	}
	var res []Cluster
	// create groups
	for _, g := range grps {
		res = append(res, subdivideCluster(intf, g, dist)...)
	}
	// add ungrouped points
	for i, ent := range pts {
		if ent.group == -1 {
			res = append(res, Cluster{ent.Point, []int{i}})
		}
	}
	return res
}

func subdivideCluster(intf Interface, g []int, dist float64) []Cluster {
	switch len(g) {
	case 0:
		return nil
	case 1:
		return []Cluster{
			{Center: intfpt(intf, g[0]), Elem: g},
		}
	}
	pt0 := intfpt(intf, g[0])
	xmin, xmax := pt0.X, pt0.X
	ymin, ymax := pt0.Y, pt0.Y
	for i := 1; i < len(g); i++ {
		pt := intfpt(intf, g[i])
		xmin = math.Min(xmin, pt.X)
		ymin = math.Min(ymin, pt.Y)
		xmax = math.Max(xmax, pt.X)
		ymax = math.Max(ymax, pt.Y)
	}

	dx, dy := xmax-xmin, ymax-ymin

	var size float64
	horz := dx > dy
	if horz {
		size = dx
	} else {
		size = dy
	}

	center := pointsAvg(intf, g)
	if size < dist*2 {
		// can't subdivide
		return []Cluster{
			{Center: center, Elem: g},
		}
	}

	var first func(pt Point) bool
	if horz {
		first = func(pt Point) bool {
			return pt.X < center.X
		}
	} else {
		first = func(pt Point) bool {
			return pt.Y < center.Y
		}
	}

	var one, two []int
	for _, i := range g {
		if first(intfpt(intf, i)) {
			one = append(one, i)
		} else {
			two = append(two, i)
		}
	}

	if len(one) == 0 || len(two) == 0 {
		// subdivision failed
		return []Cluster{
			{Center: center, Elem: g},
		}
	}

	aone, atwo := pointsAvg(intf, one), pointsAvg(intf, two)
	var adist float64
	if horz {
		adist = aone.X - atwo.X
	} else {
		adist = aone.Y - atwo.Y
	}
	if math.Abs(adist) < dist {
		// subdivision failed
		return []Cluster{
			{Center: center, Elem: g},
		}
	}

	return append(
		subdivideCluster(intf, one, dist),
		subdivideCluster(intf, two, dist)...)
}

func pointsAvg(intf Interface, g []int) Point {
	var avg Point
	var w float64
	for _, i := range g {
		pt, wi := intfpt(intf, i), intf.Weight(i)
		if wi == 0 {
			panic("zero weight")
		}
		avg.X += pt.X * wi
		avg.Y += pt.Y * wi
		w += wi
	}
	avg.X /= w
	avg.Y /= w
	return avg
}

func intfpt(intf Interface, i int) Point {
	x, y := intf.At(i)
	return Point{x, y}
}

type pent struct {
	Point
	group int
}
