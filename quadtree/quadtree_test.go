package quadtree

import (
	"fmt"
	"math/rand"
	"testing"
)

func TestQuadtree(t *testing.T) {
	pts := pointslice{
		pt(-2, -2),
		pt(0, 0),
		pt(1, 2),
		pt(2, 2),
	}
	n := 1000000
	if testing.Short() {
		n = 1000
	}
	for i := 0; i < n; i++ {
		pts = append(pts, pt(rand.Float64(), rand.Float64()))
	}

	qt := New(pts)

	lc := testTree(t, qt, &qt.root)
	if lc != len(pts) {
		t.Errorf("quadtree should have %d points, not %d", len(pts), lc)
	}

	testRect(t, qt, pts, rect{-2, -2, 0, 0}, 2)
	testRect(t, qt, pts, rect{1, 1, 2, 2}, 2)
	testRect(t, qt, pts, rect{0, 0, 0.1, 0.1}, -1)
	testRect(t, qt, pts, rect{-2, -2, 2, 2}, len(pts))
}

func testTree(t *testing.T, qt *Quadtree, n *qnode) (count int) {
	r := rect{n.min.x, n.min.y, n.max.x, n.max.y}
	for _, idx := range n.leaves {
		x, y := qt.src.At(idx)
		if !r.contains(pt(x, y)) {
			t.Errorf("%s is outside node %s", pt(x, y), r)
		}
	}
	count += len(n.leaves)
	for i := range n.children {
		count += testTree(t, qt, &n.children[i])
	}
	return count
}

func testRect(t *testing.T, qt *Quadtree, pts pointslice, r rect, npt int) {
	m := make(map[int]struct{})
	for _, idx := range qt.Rect(r.minx, r.miny, r.maxx, r.maxy, nil) {
		m[idx] = struct{}{}
	}
	if npt >= 0 && len(m) != npt {
		t.Errorf("%s should have returned %v, not %v point(s)", r, npt, len(m))
	}
	for i, pt := range pts {
		if _, ok := m[i]; ok != r.contains(pt) {
			if ok {
				t.Errorf("%s is in %s but should not be", pt, r)
			} else {
				t.Errorf("%s is not in %s but should be", pt, r)
			}
		}
	}
	if t.Failed() {
		dumpTree(qt, &qt.root, "")
	}
}

type rect struct {
	minx, miny, maxx, maxy float64
}

func (r rect) String() string {
	return fmt.Sprintf("rect(%v,%v,%v,%v)",
		r.minx, r.miny, r.maxx, r.maxy)
}

func (r rect) contains(pt tpoint) bool {
	return r.minx <= pt.x && pt.x <= r.maxx &&
		r.miny <= pt.y && pt.y <= r.maxy
}

func pt(x, y float64) tpoint {
	return tpoint{x, y}
}

type tpoint struct {
	x, y float64
}

func (p tpoint) String() string {
	return fmt.Sprintf("pt(%v,%v)", p.x, p.y)
}

type pointslice []tpoint

func (s pointslice) Len() int                { return len(s) }
func (s pointslice) At(i int) (x, y float64) { return s[i].x, s[i].y }

func dumpTree(qt *Quadtree, n *qnode, indent string) {
	fmt.Printf("%snode %v,%v,%v,%v", indent, n.min.x, n.min.y, n.max.x, n.max.y)
	if len(n.leaves) != 0 {
		fmt.Print(" [")
		for i, idx := range n.leaves {
			if i != 0 {
				fmt.Print(" ")
			}
			x, y := qt.src.At(idx)
			fmt.Printf("%v,%v", x, y)
		}
		fmt.Print("]")
	}
	if len(n.children) != 0 {
		fmt.Println(" {")
		for i := range n.children {
			dumpTree(qt, &n.children[i], indent+"  ")
		}
		fmt.Printf("%s}\n", indent)
	} else {
		fmt.Println()
	}
}
