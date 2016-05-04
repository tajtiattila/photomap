package main

import (
	"image"
	"math"
)

const nRGBAchan = 4

// fast gaussian blur based on http://blog.ivank.net/fastest-gaussian-blur.html
func gaussianBlur(dst, src *image.RGBA, radius int) {
	boxes := determineBoxes(float64(radius), 3)
	tmp := image.NewRGBA(dst.Bounds())
	boxBlur3(dst, tmp, src, (boxes[0]-1)/2)
	boxBlur3(dst, tmp, dst, (boxes[1]-1)/2)
	boxBlur3(dst, tmp, dst, (boxes[2]-1)/2)
}

func boxBlur3(dst, scratch, src *image.RGBA, radius int) {
	if src == scratch || dst == scratch {
		panic("scratch must be diffrent than src and dst")
	}
	boxBlurH(scratch, src, radius)
	boxBlurV(dst, scratch, radius)
}

func boxBlurH(dst, src *image.RGBA, radius int) {
	so := src.PixOffset(xy(src.Bounds().Min))
	do := dst.PixOffset(xy(dst.Bounds().Min))
	w, h := whmin(src.Bounds(), dst.Bounds())
	r2 := 2*radius + 1
	var val [nRGBAchan]int
	for y := 0; y < h; y++ {
		fv := src.Pix[so:]
		lv := src.Pix[so+(w-1)*nRGBAchan:]
		for i := 0; i < nRGBAchan; i++ {
			val[i] = (radius + 1) * int(fv[i])
		}
		rp := so
		for x := 0; x < radius; x++ {
			for i := 0; i < nRGBAchan; i++ {
				val[i] += int(src.Pix[rp])
				rp++
			}
		}
		x, lp, dp := 0, so, do
		for ; x <= radius; x++ {
			for i := 0; i < nRGBAchan; i++ {
				val[i] += int(src.Pix[rp]) - int(fv[i])
				dst.Pix[dp] = uint8(val[i] / r2)
				rp++
				dp++
			}
		}
		for ; x < w-radius; x++ {
			for i := 0; i < nRGBAchan; i++ {
				val[i] += int(src.Pix[rp]) - int(src.Pix[lp])
				dst.Pix[dp] = uint8(val[i] / r2)
				lp++
				rp++
				dp++
			}
		}
		for ; x < w; x++ {
			for i := 0; i < nRGBAchan; i++ {
				val[i] += int(lv[i]) - int(src.Pix[lp])
				dst.Pix[dp] = uint8(val[i] / r2)
				lp++
				dp++
			}
		}
		so += src.Stride
		do += dst.Stride
	}
}

func boxBlurV(dst, src *image.RGBA, radius int) {
	so := src.PixOffset(xy(src.Bounds().Min))
	do := dst.PixOffset(xy(dst.Bounds().Min))
	w, h := whmin(src.Bounds(), dst.Bounds())
	r2 := 2*radius + 1
	var val [nRGBAchan]int
	for x := 0; x < w; x++ {
		fv := src.Pix[so:]
		lv := src.Pix[so+(h-1)*src.Stride:]
		for i := 0; i < nRGBAchan; i++ {
			val[i] = (radius + 1) * int(fv[i])
		}
		rp := so
		for y := 0; y < radius; y++ {
			for i := 0; i < nRGBAchan; i++ {
				val[i] += int(src.Pix[rp+i])
			}
			rp += src.Stride
		}
		y, lp, dp := 0, so, do
		for ; y <= radius; y++ {
			for i := 0; i < nRGBAchan; i++ {
				val[i] += int(src.Pix[rp+i]) - int(fv[i])
				dst.Pix[dp+i] = uint8(val[i] / r2)
			}
			rp += src.Stride
			dp += dst.Stride
		}
		for ; y < h-radius; y++ {
			for i := 0; i < nRGBAchan; i++ {
				val[i] += int(src.Pix[rp+i]) - int(src.Pix[lp+i])
				dst.Pix[dp+i] = uint8(val[i] / r2)
			}
			lp += src.Stride
			rp += src.Stride
			dp += dst.Stride
		}
		for ; y < h; y++ {
			for i := 0; i < nRGBAchan; i++ {
				val[i] += int(lv[i]) - int(src.Pix[lp+i])
				dst.Pix[dp+i] = uint8(val[i] / r2)
			}
			lp += src.Stride
			dp += dst.Stride
		}
		so += nRGBAchan
		do += nRGBAchan
	}
}

func determineBoxes(sigma float64, nbox int) []int {
	// standard deviation, number of boxes
	idealWeight := math.Sqrt((12 * sigma * sigma / float64(nbox)) + 1)
	wlo := int(math.Floor(idealWeight))
	if wlo%2 == 0 {
		wlo--
	}
	wup := wlo + 2

	idealMedian := (12*sigma*sigma - float64(nbox*wlo*wlo+4*nbox*wlo+3*nbox)) / (-4*float64(wlo) - 4)
	median := int(math.Floor(idealMedian + 0.5))

	boxsizes := make([]int, nbox)
	for i := range boxsizes {
		if i < median {
			boxsizes[i] = wlo
		} else {
			boxsizes[i] = wup
		}
	}
	return boxsizes
}

func xy(pt image.Point) (x, y int) {
	return pt.X, pt.Y
}

func whmin(r1, r2 image.Rectangle) (w, h int) {
	w1, w2 := r1.Dx(), r2.Dx()
	h1, h2 := r1.Dy(), r2.Dy()
	if w1 < w2 {
		w = w1
	} else {
		w = w2
	}
	if h1 < h2 {
		h = h1
	} else {
		h = h2
	}
	return w, h
}
