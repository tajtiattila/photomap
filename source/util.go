package source

import (
	"bytes"
	"image"
	"io"
	"time"

	_ "image/jpeg"
	_ "image/png"

	"github.com/rwcarlsen/goexif/exif"
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
// no location, then the ModTime, Width and Height are
// initialized and an error of type *ErrNoLoc is returned.
func InfoFromReader(mt time.Time, r io.Reader) (ImageInfo, error) {
	buf := new(bytes.Buffer)
	tr := io.TeeReader(r, buf)
	cfg, _, err := image.DecodeConfig(tr)
	if err != nil {
		return ImageInfo{}, err
	}
	ii := ImageInfo{
		ModTime: mt,
		Width:   cfg.Width,
		Height:  cfg.Height,
	}

	x, err := exif.Decode(io.MultiReader(buf, r))
	if err != nil {
		return ii, &ErrNoLoc{err}
	}
	ii.Lat, ii.Long, err = x.LatLong()
	if err != nil {
		return ii, &ErrNoLoc{err}
	}

	return ii, nil
}
