package main

import "time"

type ImageInfoCache interface {
	Get(id string) (*CachedImage, error)
	Put(id string, cim *CachedImage) error
	Close() error
}

type CachedImage struct {
	ModTime  time.Time
	Lat, Lng float64
}
