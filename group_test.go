package main

import (
	"math/rand"
	"testing"
)

func TestMakeTree(t *testing.T) {
	var pts []point
	for i := 0; i < 1000; i++ {
		x := (rand.Float64() - 0.5) * 360
		y := (rand.Float64() - 0.5) * 360
		pts = append(pts, point{x, y})
	}

	grps := groupNearbyPoints(pts, 30)
	n := 0
	for _, g := range grps {
		n += len(g)
	}
	if n != len(pts) {
		t.Errorf("groupNearbyPoints yields %v, want %v", n, len(pts))
	}

	root := makeTree(pts, 5e-5)
	n = 0
	query(root, -180, -180, 180, 180, 0, func(pt point, vi []int) {
		n += len(vi)
	})
	if n != len(pts) {
		t.Errorf("query yields %v, want %v", n, len(pts))
	}
}
