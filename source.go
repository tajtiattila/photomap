package main

import (
	"io"
	"time"
)

type Image interface {
	Id() string
	ModTime() time.Time
	LatLong() (lat, long float64)

	Open() io.ReadCloser
}

type ImageSource interface {
	GetImages() ([]Image, error)
	Close() error
}

func ErrReadCloser(err error) io.ReadCloser {
	return &errReadCloser{err}
}

type errReadCloser struct {
	err error
}

func (rc *errReadCloser) Read(p []byte) (n int, err error) {
	return 0, rc.err
}

func (rc *errReadCloser) Close() error {
	return nil
}
