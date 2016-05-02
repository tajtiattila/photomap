package main

import (
	"io"
	"time"
)

type ImageInfo struct {
	ModTime time.Time
	Lat     float64
	Long    float64
}

type ImageSource interface {
	ModTimes() (map[string]time.Time, error)
	Info(id string) (ImageInfo, error)
	Open(id string) (io.ReadCloser, error)
	Close() error
}
