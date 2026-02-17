//go:build windows

package ui

import (
	"image"
	"image/color"
	"math"

	"github.com/lxn/walk"
)

var statusDotCache = make(map[string]*walk.Bitmap)

func getStatusDot(state string) *walk.Bitmap {
	if bmp, ok := statusDotCache[state]; ok {
		return bmp
	}
	var cr, cg, cb uint8
	switch state {
	case "connected":
		cr, cg, cb = 30, 200, 90
	case "connecting", "disconnecting":
		cr, cg, cb = 240, 190, 30
	case "error":
		cr, cg, cb = 220, 55, 55
	default:
		cr, cg, cb = 160, 160, 160
	}
	bmp := createColoredDot(cr, cg, cb, 12)
	if bmp != nil {
		statusDotCache[state] = bmp
	}
	return bmp
}

func cwSetDot(state string) {
	if cwDotImage == nil {
		return
	}
	if dot := getStatusDot(state); dot != nil {
		cwDotImage.SetImage(dot)
	}
}

func createColoredDot(cr, cg, cb uint8, size int) *walk.Bitmap {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	c := color.RGBA{R: cr, G: cg, B: cb, A: 255}
	center := float64(size) / 2.0
	radius := center - 1.0
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx := float64(x) + 0.5 - center
			dy := float64(y) + 0.5 - center
			if math.Sqrt(dx*dx+dy*dy) <= radius {
				img.Set(x, y, c)
			}
		}
	}
	bmp, err := walk.NewBitmapFromImage(img)
	if err != nil {
		return nil
	}
	return bmp
}
