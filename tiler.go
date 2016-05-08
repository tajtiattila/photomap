package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg"
	"image/png"
	"log"
	"math"
	"math/rand"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/tajtiattila/photomap/clusterer"
	"github.com/tajtiattila/photomap/imagecache"
	"github.com/tajtiattila/photomap/quadtree"
)

const TileSize = 256

type TileMap struct {
	ic     *imagecache.ImageCache
	images []imagecache.ImageInfo

	qt   *quadtree.Quadtree // for photo spots
	tree *clusterer.Tree    // for photo piles

	mtx sync.Mutex // protect gen
	gen map[string]*genTile

	starttime time.Time
	spot      *image.RGBA // photo spot image
}

const photoMinSep = 5e-5 // ~5 meters on equator

func NewTileMap(ic *imagecache.ImageCache) *TileMap {
	images := ic.Images()
	pts := make([]point, len(images))
	for i, im := range images {
		pts[i] = point{im.Long, lat2merc(im.Lat)}
	}
	return &TileMap{
		ic:        ic,
		qt:        quadtree.New(iiarr(images), quadtree.MinDist(photoMinSep)),
		tree:      clusterer.NewTree(iiarr(images), photoMinSep),
		images:    images,
		gen:       make(map[string]*genTile),
		starttime: time.Now(),
		spot:      blurrySpot(color.NRGBA{255, 0, 0, 64}, 16),
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
	xmask := (1 << uint(zoom)) - 1
	x = x & xmask
	rawimg := tm.GetTile(x, y, zoom)
	http.ServeContent(w, req, "tile.png", tm.starttime, bytes.NewReader(rawimg))
}

func (tm *TileMap) generate(x, y, zoom int) []byte {
	const thumbSize = 20

	// safety gap for elements hanging over tile boundaries
	const gap = (thumbSize * 1.5) / TileSize

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

	mindist := photoMinSep * math.Pow(2, float64(21-zoom))
	im := image.NewRGBA(image.Rect(0, 0, TileSize, TileSize))

	// draw spots
	tm.qt.NearFunc(lomi, lat2merc(lami), loma, lat2merc(lama), func(i int) bool {
		ii := tm.images[i]
		x, y := t.Tile(ii.Lat, ii.Long)
		px := int((x - xo) * TileSize)
		py := int((y - yo) * TileSize)

		dx := tm.spot.Bounds().Dx()
		dy := tm.spot.Bounds().Dy()
		x0 := px - dx/2
		y0 := py - dy/2
		draw.Draw(im, image.Rect(x0, y0, x0+dx, y0+dy), tm.spot, tm.spot.Bounds().Min, draw.Over)
		return true
	})
	setAlpha(im, 127)

	// draw photo piles
	drawPhoto := func(px, py float64, i int) {
		ii := tm.images[i]
		thumb, err := tm.ic.PhotoIcon(ii.Id)
		if err != nil {
			log.Printf("can't get photo icon for %s: %s", ii.Id, err)
			return
		}

		dx := thumb.Bounds().Dx()
		dy := thumb.Bounds().Dy()
		x0 := int(px) - dx/2
		y0 := int(py) - dy/2
		draw.Draw(im, image.Rect(x0, y0, x0+dx, y0+dy), thumb, thumb.Bounds().Min, draw.Over)
	}
	tm.tree.Query(lomi, lat2merc(lami), loma, lat2merc(lama), mindist, func(pt clusterer.Point, images []int) {
		x, y := t.Tile(merc2lat(pt.Y), pt.X)
		px := (x - xo) * TileSize
		py := (y - yo) * TileSize

		if len(images) > 1 {
			const (
				pileMax       = 10
				pileRadius    = thumbSize
				pilePhotoArea = pileRadius * pileRadius * math.Pi / pileMax
			)
			if len(images) > pileMax {
				images = images[:pileMax]
			}
			area := float64(len(images)) * pilePhotoArea
			rmax := math.Sqrt(float64(area) / math.Pi)
			rgen := newRgen(pt.X, pt.Y)
			for _, i := range images[1:] {
				sin, cos := math.Sincos(2 * math.Pi * rgen.Float64())
				r := math.Sqrt(rgen.Float64()) * rmax
				dx, dy := r*cos, r*sin
				drawPhoto(px+dx, py+dy, i)
			}
		}
		drawPhoto(px, py, images[0])
	})
	buf := new(bytes.Buffer)
	err := png.Encode(buf, im)
	if err != nil {
		panic(err) // impossible
	}
	return buf.Bytes()
}

type iiarr []imagecache.ImageInfo

func (s iiarr) Len() int                { return len(s) }
func (s iiarr) At(i int) (x, y float64) { return s[i].Long, lat2merc(s[i].Lat) }
func (s iiarr) Weight(i int) float64    { return 1 }

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

// change global image alpha
func setAlpha(im *image.RGBA, alpha uint8) {
	dx, dy := im.Bounds().Dx(), im.Bounds().Dy()
	p0 := im.PixOffset(im.Bounds().Min.X, im.Bounds().Min.Y)
	a := uint32(alpha)
	for y := 0; y < dy; y++ {
		ps, pe := p0, p0+4*dx
		for i := ps; i < pe; i++ {
			p := &im.Pix[i]
			*p = uint8((uint32(*p) * a) >> 8)
		}
		p0 += im.Stride
	}
}

func blurrySpot(clr color.NRGBA, siz int) *image.RGBA {
	im := image.NewRGBA(image.Rect(0, 0, siz, siz))
	c := float64(siz) / 2
	for xi := 0; xi < siz; xi++ {
		for yi := 0; yi < siz; yi++ {
			x, y := float64(xi), float64(yi)
			dx, dy := x-c, y-c
			r := math.Sqrt(dx*dx + dy*dy)
			intens := math.Max(0, 1-(r/c))
			cp := clr
			cp.A = uint8(float64(clr.A) * intens)
			im.Set(xi, yi, cp)
		}
	}
	return im
}

// x, y in -180..180
func newRgen(x, y float64) *rand.Rand {
	xf, yf := math.Floor(x), math.Floor(y)
	const m = 65536
	xv, yv := int(m*(x-xf)), int(m*(y-yf))
	return rand.New(rand.NewSource(int64(yv*m + xv)))
}
