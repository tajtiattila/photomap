package main

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	_ "image/jpeg"
	"image/png"
	"log"
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/tajtiattila/photomap/quadtree"
)

const tmQtMinDist = 1e-6 // 6 digits of latitude is 11 cm on the equator

const TileSize = 256

type TileMap struct {
	ic *ImageCache
	qt *quadtree.Quadtree

	vii []idImageInfo

	mtx sync.Mutex // protect gentiles
	gen map[string]*genTile

	starttime time.Time
}

func NewTileMap(ic *ImageCache) *TileMap {
	m := ic.Images()

	v := make([]idImageInfo, 0, len(m))
	for k, ii := range m {
		v = append(v, idImageInfo{k, ii})
	}

	return &TileMap{
		ic:        ic,
		qt:        quadtree.New(imageInfoQS(v), quadtree.MinDist(tmQtMinDist)),
		vii:       v,
		gen:       make(map[string]*genTile),
		starttime: time.Now(),
	}
}

func (tm *TileMap) GetTile(x, y, zoom int) []byte {
	k := fmt.Sprintf("%d|%d|%d", x, y, zoom)

	tm.mtx.Lock()
	gt, ok := tm.gen[k]
	if !ok {
		gt = &genTile{x: x, y: y, zoom: zoom}
		tm.gen[k] = gt
	}
	tm.mtx.Unlock()

	gt.init(tm)
	return gt.image
}

func (tm *TileMap) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	q := req.URL.Query()
	if q.Get("x") == "" || q.Get("y") == "" || q.Get("zoom") == "" {
		http.Error(w, "parameter(s) missing: need x, y, zoom", http.StatusBadRequest)
		return
	}
	x, err := strconv.Atoi(q.Get("x"))
	if err != nil {
		http.Error(w, "x invalid: "+err.Error(), http.StatusBadRequest)
		return
	}
	y, err := strconv.Atoi(q.Get("y"))
	if err != nil {
		http.Error(w, "y invalid: "+err.Error(), http.StatusBadRequest)
		return
	}
	zoom, err := strconv.Atoi(q.Get("zoom"))
	if err != nil {
		http.Error(w, "zoom invalid: "+err.Error(), http.StatusBadRequest)
		return
	}
	rawimg := tm.GetTile(x, y, zoom)
	http.ServeContent(w, req, "tile.png", tm.starttime, bytes.NewReader(rawimg))
}

func (tm *TileMap) generate(x, y, zoom int) []byte {
	const radius = 20

	// safety gap for elements hanging over tile boundaries
	const gap = (radius * 1.5) / TileSize

	xo, yo := float64(x), float64(y)

	xmi := float64(x) - gap
	ymi := float64(y) - gap
	xma := float64(x+1) + gap
	yma := float64(y+1) + gap

	t := newTiler(zoom)
	lami, lomi := t.LatLong(xmi, yma)
	lama, loma := t.LatLong(xma, ymi)
	if lama < lami || loma < lomi {
		panic("invalid")
	}

	im := image.NewRGBA(image.Rect(0, 0, TileSize, TileSize))
	tm.qt.NearFunc(lami, lomi, lama, loma, func(i int) bool {
		ii := tm.vii[i]
		x, y := t.Tile(ii.Lat, ii.Long)
		px := int((x - xo) * TileSize)
		py := int((y - yo) * TileSize)

		thumb, err := tm.ic.PhotoIcon(ii.Id)
		if err != nil {
			log.Printf("can't get photo icon for %s: %s", ii.Id, err)
			return true
		}

		dx := thumb.Bounds().Dx()
		dy := thumb.Bounds().Dy()
		x0 := px - dx/2
		y0 := py - dy/2
		draw.Draw(im, image.Rect(x0, y0, x0+dx, y0+dy), thumb, thumb.Bounds().Min, draw.Over)
		return true
	})
	buf := new(bytes.Buffer)
	err := png.Encode(buf, im)
	if err != nil {
		panic(err) // impossible
	}
	return buf.Bytes()
}

type idImageInfo struct {
	Id string
	ImageInfo
}

type imageInfoQS []idImageInfo

func (s imageInfoQS) Len() int                { return len(s) }
func (s imageInfoQS) At(i int) (x, y float64) { return s[i].Lat, s[i].Long }

type tiler struct {
	m float64
}

func newTiler(zoom int) *tiler {
	return &tiler{
		m: float64(int(1) << uint(zoom)),
	}
}

func (t *tiler) LatLong(x, y float64) (lat, long float64) {
	long = x/t.m*360 - 180
	n := math.Pi - 2*math.Pi*y/t.m
	lat = 180 / math.Pi * math.Atan(0.5*(math.Exp(n)-math.Exp(-n)))
	return
}

func (t *tiler) Tile(lat, long float64) (x, y float64) {
	x = t.m * (long + 180) / 360
	y = t.m * (1 - math.Log(math.Tan(lat*math.Pi/180)+1/math.Cos(lat*math.Pi/180))/math.Pi) / 2
	return
}

type genTile struct {
	x, y, zoom int
	once       sync.Once
	image      []byte
}

func (gt *genTile) init(tm *TileMap) {
	gt.once.Do(func() {
		gt.image = tm.generate(gt.x, gt.y, gt.zoom)
	})
}
