package source

import (
	"fmt"
	"io"
	"time"
)

type ImageInfo struct {
	CreateTime time.Time

	// image dimensions
	Width  int
	Height int

	// gps location
	Lat  float64
	Long float64
}

type ImageSource interface {
	// ModTimes returns all images that are candidates for inclusion
	// on the map, along with their modtimes.
	ModTimes() map[string]time.Time

	// Info returns the image info for the specified id.
	// An error is returned if the id is not known to this ImageSource
	// or the id does not refer to a geotagged image.
	Info(id string) (ImageInfo, error)

	// Open returns a io.ReadCloser for the image with id.
	Open(id string) (io.ReadCloser, error)

	// Close closes this source.
	Close() error
}

// Open opens the source registered with name using
// the argument provided.
func Open(name string, arg string) (ImageSource, error) {
	f, ok := sources[name]
	if !ok {
		return nil, fmt.Errorf("unknown image source %q", name)
	}
	return f(arg)
}

var sources map[string]NewSourceFunc

type NewSourceFunc func(arg string) (ImageSource, error)

func Register(name string, f NewSourceFunc) {
	if sources == nil {
		sources = make(map[string]NewSourceFunc)
	}
	if _, ok := sources[name]; ok {
		panic(fmt.Sprintf("source %q already exists", name))
	}
	sources[name] = f
}
