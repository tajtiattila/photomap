package main

import (
	"image"
	"image/draw"
	"image/png"
	"os"
	"testing"
)

func TestBlur(t *testing.T) {
	f, err := os.Open("testimg/cballs.png")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	im, _, err := image.Decode(f)
	if err != nil {
		t.Fatal(err)
	}

	var rgba *image.RGBA
	var ok bool
	if rgba, ok = im.(*image.RGBA); !ok {
		rgba = image.NewRGBA(im.Bounds())
		draw.Draw(rgba, rgba.Bounds(), im, im.Bounds().Min, draw.Src)
	}
	dst := image.NewRGBA(im.Bounds())
	gaussianBlur(dst, rgba, 5)

	f, err = os.Create("testimg/cballs_blur.png")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	if err := png.Encode(f, dst); err != nil {
		t.Fatal(err)
	}
}
