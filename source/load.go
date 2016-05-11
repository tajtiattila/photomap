package source

import (
	"bytes"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log"

	"github.com/rwcarlsen/goexif/exif"
)

func LoadImage(r io.Reader) (image.Image, error) {
	buf := new(bytes.Buffer)
	tr := io.TeeReader(r, buf)
	x, xerr := exif.Decode(tr)

	im, _, err := image.Decode(io.MultiReader(buf, r))
	if err != nil {
		return nil, err
	}

	if xerr != nil {
		// no exif
		return im, nil
	}

	tag, err := x.Get(exif.Orientation)
	if err != nil {
		log.Println("Exif has no orientation tag")
		return im, nil
	}

	orient, err := tag.Int(0)
	if err != nil {
		log.Println("Invalid exif orientation tag")
		return im, nil
	}

	// http://www.daveperrett.com/articles/2012/07/28/exif-orientation-handling-is-a-ghetto/
	switch orient {
	case 1:
		// pass
	case 2:
		im = fliph(im)
	case 3:
		im = rotate(im, 180)
	case 4:
		im = rotate(fliph(im), 180)
	case 5:
		im = rotate(fliph(im), -90)
	case 6:
		im = rotate(im, -90)
	case 7:
		im = rotate(fliph(im), 90)
	case 8:
		im = rotate(im, 90)
	}

	return im, nil
}

func fliph(im image.Image) image.Image {
	flipped := image.NewRGBA(im.Bounds())
	w, h := im.Bounds().Dx(), im.Bounds().Dy()
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			flipped.Set(x, y, im.At(w-1-y, x))
		}
	}
	return flipped
}

func rotate(im image.Image, angle int) image.Image {
	var rotated *image.RGBA
	// trigonometric (i.e counter clock-wise)
	switch angle {
	case 90:
		newH, newW := im.Bounds().Dx(), im.Bounds().Dy()
		rotated = image.NewRGBA(image.Rect(0, 0, newW, newH))
		for y := 0; y < newH; y++ {
			for x := 0; x < newW; x++ {
				rotated.Set(x, y, im.At(newH-1-y, x))
			}
		}
	case -90:
		newH, newW := im.Bounds().Dx(), im.Bounds().Dy()
		rotated = image.NewRGBA(image.Rect(0, 0, newW, newH))
		for y := 0; y < newH; y++ {
			for x := 0; x < newW; x++ {
				rotated.Set(x, y, im.At(y, newW-1-x))
			}
		}
	case 180, -180:
		newW, newH := im.Bounds().Dx(), im.Bounds().Dy()
		rotated = image.NewRGBA(image.Rect(0, 0, newW, newH))
		for y := 0; y < newH; y++ {
			for x := 0; x < newW; x++ {
				rotated.Set(x, y, im.At(newW-1-x, newH-1-y))
			}
		}
	default:
		return im
	}
	return rotated
}
