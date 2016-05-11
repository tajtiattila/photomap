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
	"sort"
	"sync"

	"github.com/tajtiattila/photomap/clusterer"
	"github.com/tajtiattila/photomap/imagecache"
	"github.com/tajtiattila/photomap/quadtree"
)

const TileSize = 256

type TileMap struct {
	Lat, Long   float64 // center of boundary of all photos
	Dlat, Dlong float64 // size of boundary in lat/long direction

	ic     *imagecache.ImageCache
	images []imagecache.ImageInfo

	qt   *quadtree.Quadtree // for photo spots
	tree *clusterer.Tree    // for photo piles

	mtx sync.Mutex // protect gen
	gen map[string]*genTile

	spot *image.RGBA // photo spot image
}

const photoMinSep = 5e-5 // ~5 meters on equator

func NewTileMap(ic *imagecache.ImageCache) *TileMap {
	images := ic.Images()
	if len(images) == 0 {
		log.Fatal("empty image cache")
	}
	tm := &TileMap{
		ic:     ic,
		qt:     quadtree.New(iiarr(images), quadtree.MinDist(photoMinSep)),
		tree:   clusterer.NewTree(iiarr(images), photoMinSep),
		images: images,
		gen:    make(map[string]*genTile),
		spot:   blurrySpot(color.NRGBA{255, 0, 0, 64}, 16),
	}
	tm.findStartLocation()
	return tm
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

// PhotoPlaces returns clickable places with galleries within the requested boundary,
// along with
func (tm *TileMap) PhotoPlaces(la0, lo0, la1, lo1 float64, zoom int) ([]LatLong, float64) {
	zd := zoomdist(zoom)
	var r []LatLong
	tm.tree.Query(lo0, lat2merc(la0), lo1, lat2merc(la1), zd, func(pt clusterer.Point, images []int) {
		r = append(r, LatLong{merc2lat(pt.Y), pt.X})
	})
	return r, zd / 2
}

type LatLong struct {
	Lat  float64 `json:"lat"`
	Long float64 `json:"long"`
}

// Gallery returns the ids to show in a gallery at the given location.
func (tm *TileMap) Gallery(lat, long float64, zoom int) []string {
	zd := zoomdist(zoom)
	m := lat2merc(lat)
	r := zd / 2
	var im []int
	var bestdist float64
	tm.tree.Query(long-r, m-r, long+r, m+r, zd, func(pt clusterer.Point, images []int) {
		dx, dy := long-pt.X, m-pt.Y
		d := dx*dx + dy*dy
		if im == nil || d < bestdist {
			im = images
			bestdist = d
		}
	})
	if im == nil {
		return nil
	}
	iiv := make([]imagecache.ImageInfo, 0, len(im))
	for _, i := range im {
		iiv = append(iiv, tm.images[i])
	}
	sort.Sort(iiByDate(iiv))
	refs := make([]string, len(iiv))
	for i, ii := range iiv {
		refs[i] = ii.Id
	}
	return refs
}

func (tm *TileMap) findStartLocation() {
	w0 := tm.findStartLocationOfs(0, false)
	w180 := tm.findStartLocationOfs(180, false) // when photos are near date line
	if w0 <= w180 {
		tm.findStartLocationOfs(0, true)
	} else {
		tm.findStartLocationOfs(180, true)
	}
}

func (tm *TileMap) findStartLocationOfs(lofs float64, set bool) (width float64) {
	var x0, y0, x1, y1 float64
	for i, ii := range tm.ic.Images() {
		x, y := ii.Long+lofs, ii.Lat
		if i == 0 {
			x0, x1 = x, x
			y0, y1 = y, y
		} else {
			x0 = math.Min(x, x0)
			y0 = math.Min(y, y0)
			x1 = math.Max(x, x1)
			y1 = math.Max(y, y1)
		}
	}
	dx, dy := x1-x0, y1-y0
	if set {
		cx, cy := (x0+x1)/2, (y0+y1)/2
		tm.Lat = cy
		tm.Long = cx - lofs
		tm.Dlat = dy
		tm.Dlong = dx
	}
	return dx
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
	drawPhoto := func(px, py float64, ii imagecache.ImageInfo) {
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
	tm.tree.Query(lomi, lat2merc(lami), loma, lat2merc(lama), zoomdist(zoom), func(pt clusterer.Point, images []int) {
		x, y := t.Tile(merc2lat(pt.Y), pt.X)
		px := (x - xo) * TileSize
		py := (y - yo) * TileSize

		// have newest images first
		vii := make([]imagecache.ImageInfo, len(images))
		for i, x := range images {
			vii[i] = tm.images[x]
		}
		sort.Sort(sort.Reverse(iiByDate(vii)))

		if len(vii) > 1 {
			const (
				pileMax       = 10
				pileRadius    = thumbSize
				pilePhotoArea = pileRadius * pileRadius * math.Pi / pileMax
			)
			if len(vii) > pileMax {
				vii = vii[:pileMax]
			}
			area := float64(len(vii)) * pilePhotoArea
			rmax := math.Sqrt(float64(area) / math.Pi)
			rgen := newRgen(pt.X, pt.Y)
			for _, ii := range vii[1:] {
				sin, cos := math.Sincos(2 * math.Pi * rgen.Float64())
				r := math.Sqrt(rgen.Float64()) * rmax
				dx, dy := r*cos, r*sin
				drawPhoto(px+dx, py+dy, ii)
			}
		}
		drawPhoto(px, py, vii[0])
	})
	buf := new(bytes.Buffer)
	err := png.Encode(buf, im)
	if err != nil {
		panic(err) // impossible
	}
	return buf.Bytes()
}

func zoomdist(zoom int) float64 {
	return photoMinSep * math.Pow(2, float64(21-zoom))
}

type iiByDate []imagecache.ImageInfo

func (s iiByDate) Len() int           { return len(s) }
func (s iiByDate) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s iiByDate) Less(i, j int) bool { return s[i].CreateTime.Before(s[j].CreateTime) }

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
