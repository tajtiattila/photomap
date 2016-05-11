package source

import (
	"bytes"
	"errors"
	"image"
	"io"
	"log"
	"strings"
	"sync"
	"time"

	_ "image/jpeg"
	_ "image/png"

	"github.com/bradfitz/latlong"
	"github.com/rwcarlsen/goexif/exif"
	"github.com/rwcarlsen/goexif/tiff"
)

type ErrNoLoc struct {
	e error
}

func (e *ErrNoLoc) Error() string { return e.e.Error() }

func IsNoLoc(err error) bool {
	_, ok := err.(*ErrNoLoc)
	return ok
}

// InfoFromReader initializes an ImageInfo from r.
// If the source appears to be a valid image but has
// no location, then the CreateTime, Width and Height are
// initialized and an error of type *ErrNoLoc is returned.
func InfoFromReader(mt time.Time, r io.Reader) (ImageInfo, error) {
	buf := new(bytes.Buffer)
	tr := io.TeeReader(r, buf)
	cfg, _, err := image.DecodeConfig(tr)
	if err != nil {
		return ImageInfo{}, err
	}
	ii := ImageInfo{
		CreateTime: mt, // will be overwritten with exif metadata below

		Width:  cfg.Width,
		Height: cfg.Height,
	}

	x, err := exif.Decode(io.MultiReader(buf, r))
	if err != nil {
		return ii, &ErrNoLoc{err}
	}

	ct, err := x.DateTime()
	if err != nil {
		ii.CreateTime = ct
	}

	ii.Lat, ii.Long, err = x.LatLong()
	if err != nil {
		return ii, &ErrNoLoc{err}
	}

	// add CreateTime timezone based on exif lat/long
	if loc := lookupLocation(latlong.LookupZoneName(ii.Lat, ii.Long)); loc != nil {
		if t, err := exifDateTimeInLocation(x, loc); err == nil {
			ii.CreateTime = t
		}
	}

	return ii, nil
}

// This is basically a copy of the exif.Exif.DateTime() method, except:
//   * it takes a *time.Location to assume
//   * the caller already assumes there's no timezone offset or GPS time
//     in the EXIF, so any of that code can be ignored.
func exifDateTimeInLocation(x *exif.Exif, loc *time.Location) (time.Time, error) {
	tag, err := x.Get(exif.DateTimeOriginal)
	if err != nil {
		tag, err = x.Get(exif.DateTime)
		if err != nil {
			return time.Time{}, err
		}
	}
	if tag.Format() != tiff.StringVal {
		return time.Time{}, errors.New("DateTime[Original] not in string format")
	}
	const exifTimeLayout = "2006:01:02 15:04:05"
	dateStr := strings.TrimRight(string(tag.Val), "\x00")
	return time.ParseInLocation(exifTimeLayout, dateStr, loc)
}

var zoneCache struct {
	sync.RWMutex
	m map[string]*time.Location
}

func lookupLocation(zone string) *time.Location {
	if zone == "" {
		return nil
	}
	zoneCache.RLock()
	l, ok := zoneCache.m[zone]
	zoneCache.RUnlock()
	if ok {
		return l
	}
	// could use singleflight here, but doesn't really
	// matter if two callers both do this.
	loc, err := time.LoadLocation(zone)

	zoneCache.Lock()
	if zoneCache.m == nil {
		zoneCache.m = make(map[string]*time.Location)
	}
	zoneCache.m[zone] = loc // even if nil
	zoneCache.Unlock()

	if err != nil {
		log.Printf("failed to lookup timezone %q: %v", zone, err)
		return nil
	}
	return loc
}
