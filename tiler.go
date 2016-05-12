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

	"github.com/tajtiattila/photomap/clusterer"
	"github.com/tajtiattila/photomap/imagecache"
	"github.com/tajtiattila/photomap/quadtree"

	"go4.org/syncutil/singleflight"
)

const TileSize = 256

type TileMap struct {
	Lat, Long   float64 // center of boundary of all photos
	Dlat, Dlong float64 // size of boundary in lat/long direction

	ic     *imagecache.ImageCache
	images []imagecache.ImageInfo

	qt   *quadtree.Quadtree // for photo spots
	tree *clusterer.Tree    // for photo piles

	spotg  singleflight.Group
	photog singleflight.Group

	emptyTile []byte // empty tile in png format

	spot *image.RGBA // photo spot image
}

const photoMinSep = 5e-5 // ~5 meters on equator
const spotSize = 16

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

		emptyTile: pngBytes(image.NewNRGBA(image.Rect(0, 0, TileSize, TileSize))),
		spot:      blurrySpot(color.NRGBA{255, 0, 0, 64}, spotSize),
	}
	tm.findStartLocation()
	return tm
}

func (tm *TileMap) PhotoTile(x, y, zoom int) []byte {
	k := fmt.Sprintf("%d|%d|%d", x, y, zoom)

	r, _ := tm.photog.Do(k, func() (interface{}, error) {
		return tm.photoTile(x, y, zoom), nil
	})

	return r.([]byte)
}

func (tm *TileMap) SpotsTile(x, y, zoom int) []byte {
	k := fmt.Sprintf("%d|%d|%d", x, y, zoom)

	r, _ := tm.spotg.Do(k, func() (interface{}, error) {
		return tm.spotsTile(x, y, zoom), nil
	})

	return r.([]byte)
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

func (tm *TileMap) spotsTile(x, y, zoom int) []byte {
	t := makeTileInfo(x, y, zoom, spotSize)

	im := image.NewRGBA(image.Rect(0, 0, TileSize, TileSize))

	// draw spots
	ndrawn := 0
	tm.qt.NearFunc(t.lo0, lat2merc(t.la0), t.lo1, lat2merc(t.la1), func(i int) bool {
		ii := tm.images[i]
		px, py := t.pixel(ii.Lat, ii.Long)

		dx := tm.spot.Bounds().Dx()
		dy := tm.spot.Bounds().Dy()
		xo := int(px) - dx/2
		yo := int(py) - dy/2
		r := image.Rect(xo, yo, xo+dx, yo+dy)
		if r.Overlaps(im.Bounds()) {
			ndrawn++
			draw.Draw(im, r, tm.spot, tm.spot.Bounds().Min, draw.Over)
		}
		return true
	})

	if ndrawn == 0 {
		return tm.emptyTile
	}

	return pngBytes(im)
}

func (tm *TileMap) photoTile(x, y, zoom int) []byte {
	const thumbSize = 20

	t := makeTileInfo(x, y, zoom, thumbSize)

	im := image.NewRGBA(image.Rect(0, 0, TileSize, TileSize))

	// draw photo piles
	ndrawn := 0
	drawPhoto := func(px, py float64, ii imagecache.ImageInfo) {
		thumb, err := tm.ic.PhotoIcon(ii.Id)
		if err != nil {
			log.Printf("can't get photo icon for %s: %s", ii.Id, err)
			return
		}

		dx := thumb.Bounds().Dx()
		dy := thumb.Bounds().Dy()
		xo := int(px) - dx/2
		yo := int(py) - dy/2
		r := image.Rect(xo, yo, xo+dx, yo+dy)
		if r.Overlaps(im.Bounds()) {
			ndrawn++
			draw.Draw(im, r, thumb, thumb.Bounds().Min, draw.Over)
		}
	}
	tm.tree.Query(t.lo0, lat2merc(t.la0), t.lo1, lat2merc(t.la1), zoomdist(zoom),
		func(pt clusterer.Point, images []int) {
			px, py := t.pixel(merc2lat(pt.Y), pt.X)

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
	if ndrawn == 0 {
		return tm.emptyTile
	}
	return pngBytes(im)
}

type tileInfo struct {
	tiler
	xo, yo             float64
	la0, lo0, la1, lo1 float64
}

func makeTileInfo(x, y, zoom int, thumbSize float64) tileInfo {
	gap := (thumbSize * 1.5) / TileSize

	xo, yo := float64(x), float64(y)

	x0 := float64(x) - gap
	y0 := float64(y) - gap
	x1 := float64(x+1) + gap
	y1 := float64(y+1) + gap

	t := makeTiler(zoom)
	la0, lo0 := t.LatLong(x0, y1)
	la1, lo1 := t.LatLong(x1, y0)
	if la1 < la0 || lo1 < lo0 {
		panic("invalid")
	}

	return tileInfo{t, xo, yo, la0, lo0, la1, lo1}
}

func (t tileInfo) pixel(lat, long float64) (px, py float64) {
	x, y := t.Tile(lat, long)
	px = (x - t.xo) * TileSize
	py = (y - t.yo) * TileSize
	return
}

func zoomdist(zoom int) float64 {
	return photoMinSep * math.Pow(2, float64(21-zoom))
}

func pngBytes(im image.Image) []byte {
	buf := new(bytes.Buffer)
	if err := png.Encode(buf, im); err != nil {
		panic(err) // impossible
	}
	return buf.Bytes()
}

type iiByDate []imagecache.ImageInfo

func (s iiByDate) Len() int           { return len(s) }
func (s iiByDate) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s iiByDate) Less(i, j int) bool { return s[i].CreateTime.Before(s[j].CreateTime) }

type iiarr []imagecache.ImageInfo

func (s iiarr) Len() int                { return len(s) }
func (s iiarr) At(i int) (x, y float64) { return s[i].Long, lat2merc(s[i].Lat) }
func (s iiarr) Weight(i int) float64    { return 1 }

type tiler float64

func makeTiler(zoom int) tiler {
	return tiler(int(1) << uint(zoom))
}

func (t tiler) LatLong(x, y float64) (lat, long float64) {
	m := float64(t)
	long = x/m*360 - 180
	n := math.Pi - 2*math.Pi*y/m
	lat = 180 / math.Pi * math.Atan(0.5*(math.Exp(n)-math.Exp(-n)))
	return
}

func (t tiler) Tile(lat, long float64) (x, y float64) {
	m := float64(t)
	x = m * (long + 180) / 360
	y = m * (1 - math.Log(math.Tan(lat*math.Pi/180)+1/math.Cos(lat*math.Pi/180))/math.Pi) / 2
	return
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
