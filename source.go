package main

type Image struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

type ImageSource interface {
	GetImages() ([]Image, error)
	Close() error
}
