package source

import (
	"fmt"
	"io"
	"time"
)

type ImageInfo struct {
	ModTime time.Time

	// image dimensions
	Width  int
	Height int

	// gps location
	Lat  float64
	Long float64
}

type ImageSource interface {
	// ModTimes returns all images and their modtimes.
	ModTimes() (map[string]time.Time, error)

	// Info returns the image info for the specified id.
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
