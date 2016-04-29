package main

type Image struct {
	Lat, Lng float64
}

type ImageSource interface {
	GetImages() []Image
}
