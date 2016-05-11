package imagecache

import (
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg"

	"github.com/nfnt/resize"
	"github.com/tajtiattila/blur"
)

// Scaler scales images to given maximum width/height
type Scaler struct {
	mw, mh int
}

func MakeScaler(mw, mh int) Scaler {
	if mw <= 0 || mh <= 0 {
		panic("invalid scaler width/height")
	}
	return Scaler{
		mw: mw,
		mh: mh,
	}
}

func (s Scaler) Scale(im image.Image) image.Image {
	dx, dy := s.CalcSize(im.Bounds().Dx(), im.Bounds().Dy())
	thumb := resize.Resize(uint(dx), uint(dy), im, resize.Bilinear)
	return thumb
}

// calculate thumb dimensions
func (s *Scaler) CalcSize(sx, sy int) (tx, ty int) {
	if s.mw <= 0 && s.mh <= 0 {
		panic("impossible")
	}
	scaleForWidth := false
	switch {
	case s.mw >= 0 && s.mh >= 0:
		// scale for both max width and height
		scalex := float64(sx) / float64(s.mw)
		scaley := float64(sy) / float64(s.mh)
		scaleForWidth = scalex > scaley
	case s.mw >= 0:
		// scale width
		scaleForWidth = true
	default:
		// scale height
	}

	if scaleForWidth {
		tx = s.mw
		ty = sy * tx / sx
	} else {
		ty = s.mh
		tx = sx * ty / sy
	}
	return tx, ty
}

func Frame(im image.Image, w int, col color.RGBA) image.Image {
	if w <= 0 {
		return im
	}

	dx := im.Bounds().Dx()
	dy := im.Bounds().Dy()

	fx := dx + 2*w
	fy := dy + 2*w

	framed := image.NewRGBA(image.Rect(0, 0, fx, fy))

	for i := 0; i < w; i++ {
		for y := 0; y < fy; y++ {
			framed.SetRGBA(i, y, col)
			framed.SetRGBA(fx-1-i, y, col)
		}
		ex := fx - w
		for x := w; x < ex; x++ {
			framed.SetRGBA(x, i, col)
			framed.SetRGBA(x, fy-1-i, col)
		}
	}

	draw.Draw(framed, image.Rect(w, w, w+dx, w+dy), im, image.Pt(0, 0), draw.Src)
	return framed
}

// Shadow can add shadow effect to images.
type Shadow struct {
	Color color.RGBA // shadow color

	Dx, Dy int // shadow offset

	Blur int // shadow blur radius
}

// Apply applies s to im, increasing image size as needed.
func (s Shadow) Apply(im image.Image) image.Image {
	if s.Dx == 0 && s.Dy == 0 && s.Blur == 0 {
		return im
	}

	dx := im.Bounds().Dx()
	dy := im.Bounds().Dy()

	// full frame size, accounting for shadow shift and blur
	fx := dx + 4*s.Blur + iabs(s.Dx)
	fy := dy + 4*s.Blur + iabs(s.Dy)

	shadow := image.NewRGBA(image.Rect(0, 0, fx, fy))

	// im origin
	tx, ty := 2*s.Blur, 2*s.Blur
	if s.Dx < 0 {
		tx += s.Dx
	}
	if s.Dy < 0 {
		ty += s.Dy
	}

	// shadow origin
	sx, sy := tx+s.Dx, ty+s.Dy

	// paint shadow
	for y := 0; y < dy; y++ {
		for x := 0; x < dx; x++ {
			srcalpha := uint32(color.RGBAModel.Convert(im.At(x, y)).(color.RGBA).A)
			r := uint8((uint32(s.Color.R) * srcalpha) >> 8)
			g := uint8((uint32(s.Color.G) * srcalpha) >> 8)
			b := uint8((uint32(s.Color.B) * srcalpha) >> 8)
			a := uint8((uint32(s.Color.A) * srcalpha) >> 8)
			shadow.SetRGBA(sx+x, sy+y, color.RGBA{r, g, b, a})
		}
	}
	shadow = blur.Gaussian(shadow, s.Blur, blur.ReuseSrc)

	// copy im over shadow
	draw.Draw(shadow, image.Rect(tx, ty, tx+dx, ty+dy), im, image.Pt(0, 0), draw.Src)

	return shadow
}

func iabs(i int) int {
	if i < 0 {
		return -i
	}
	return i
}
