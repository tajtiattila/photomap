package main

import (
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg"
	"io"

	"github.com/nfnt/resize"
)

type Thumber struct {
	mw, mh int

	border int

	// shadow
	sdx   int
	sdy   int
	sblur int
}

func NewThumber() *Thumber {
	return &Thumber{
		mw: 20,
		mh: 20,

		border: 2,

		sdx:   0,
		sdy:   1,
		sblur: 4,
	}
}

func (t *Thumber) PhotoIcon(r io.Reader) (image.Image, error) {
	im, _, err := image.Decode(r)
	if err != nil {
		return nil, err
	}
	dx, dy := t.sizeFor(im.Bounds())
	thumb := resize.Resize(uint(dx), uint(dy), im, resize.Bilinear)

	tdx := thumb.Bounds().Dx()
	tdy := thumb.Bounds().Dy()

	// framed photo icon dimensions without shadow
	pdx := tdx + 2*t.border
	pdy := tdy + 2*t.border

	// full frame size, accounting for shadow shift and blur
	fx := pdx + 2*t.sblur + iabs(t.sdx)
	fy := pdy + 2*t.sblur + iabs(t.sdy)

	// thumb origin
	tx, ty := t.sblur+t.border, t.sblur+t.border
	if t.sdx < 0 {
		tx += t.sdx
	}
	if t.sdy < 0 {
		ty += t.sdy
	}

	framed := image.NewRGBA(image.Rect(0, 0, fx, fy))

	// paint shadow
	shadow := color.RGBA{0, 0, 0, 128}
	for y := 0; y < pdy; y++ {
		yy := ty - t.border + y + t.sdy
		for x := 0; x < pdx; x++ {
			xx := tx - t.border + x + t.sdx
			framed.SetRGBA(xx, yy, shadow)
		}
	}
	gaussianBlur(framed, framed, t.sblur)

	// paint white background
	white := color.RGBA{255, 255, 255, 255}
	for y := 0; y < pdy; y++ {
		for x := 0; x < pdx; x++ {
			framed.SetRGBA(tx+x-t.border, ty+y-t.border, white)
		}
	}

	// copy thumb to framed shadow
	draw.Draw(framed, image.Rect(tx, ty, tx+tdx, ty+tdy), thumb, image.Pt(0, 0), draw.Src)

	return framed, nil
}

func (t *Thumber) sizeFor(bounds image.Rectangle) (dx, dy int) {
	sx := bounds.Dx()
	sy := bounds.Dy()
	if t.mw <= 0 && t.mh <= 0 {
		panic("impossible")
	}
	scaleForWidth := false
	switch {
	case t.mw >= 0 && t.mh >= 0:
		// scale for both max width and height
		scalex := float64(sx) / float64(t.mw)
		scaley := float64(sy) / float64(t.mh)
		scaleForWidth = scalex > scaley
	case t.mw >= 0:
		// scale width
		scaleForWidth = true
	default:
		// scale height
	}

	if scaleForWidth {
		dx = t.mw
		dy = sy * dx / sx
	} else {
		dy = t.mh
		dx = sx * dy / sy
	}
	return dx, dy
}

func iabs(i int) int {
	if i < 0 {
		return -i
	}
	return i
}
