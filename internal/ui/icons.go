package ui

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"runtime"
)

// GetIcon returns the icon data for the given state
func GetIcon(state string) []byte {
	switch state {
	case "connected":
		return GenerateShieldIcon(30, 200, 90) // Green
	case "connecting":
		return GenerateShieldIcon(240, 190, 30) // Yellow/Amber
	case "error":
		return GenerateShieldIcon(220, 55, 55) // Red
	default:
		return GenerateShieldIcon(160, 160, 160) // Gray (disconnected)
	}
}

// GenerateShieldIcon renders a simple filled shield icon at 32x32
func GenerateShieldIcon(cr, cg, cb byte) []byte {
	const size = 32
	img := image.NewRGBA(image.Rect(0, 0, size, size))

	cx := float64(size) / 2.0

	abs := func(v float64) float64 {
		if v < 0 {
			return -v
		}
		return v
	}

	// Shield shape: rounded top, pointed bottom
	for y := 0; y < size; y++ {
		fy := float64(y)
		var halfW float64

		if fy <= 3 {
			// Top edge — slight dome
			t := fy / 3.0
			halfW = 10.0 + 4.0*t
		} else if fy <= 14 {
			// Wide body
			halfW = 14.0
		} else {
			// Narrowing to point
			t := (fy - 14.0) / 17.0
			halfW = 14.0 * (1.0 - t*t)
			if halfW < 0.5 {
				halfW = 0.5
			}
		}

		for x := 0; x < size; x++ {
			fx := float64(x) + 0.5
			d := abs(fx - cx)
			if d <= halfW {
				a := byte(255)
				// Anti-alias the edge
				if d > halfW-1.2 {
					a = byte((halfW - d) / 1.2 * 255)
				}
				img.SetRGBA(x, y, color.RGBA{R: cr, G: cg, B: cb, A: a})
			}
		}
	}

	// Darker border
	var dr, dg, db byte
	lum := int(cr)*299 + int(cg)*587 + int(cb)*114
	if lum > 128000 {
		dr, dg, db = byte(float64(cr)*0.5), byte(float64(cg)*0.5), byte(float64(cb)*0.5)
	} else {
		dr, dg, db = cr/3, cg/3, cb/3
	}

	for y := 0; y < size; y++ {
		fy := float64(y)
		var halfW float64

		if fy <= 3 {
			t := fy / 3.0
			halfW = 10.0 + 4.0*t
		} else if fy <= 14 {
			halfW = 14.0
		} else {
			t := (fy - 14.0) / 17.0
			halfW = 14.0 * (1.0 - t*t)
			if halfW < 0.5 {
				halfW = 0.5
			}
		}

		for x := 0; x < size; x++ {
			fx := float64(x) + 0.5
			d := abs(fx - cx)
			// Draw border ring (1.5px thick at the edge)
			if d <= halfW && d > halfW-1.8 {
				a := byte(200)
				img.SetRGBA(x, y, color.RGBA{R: dr, G: dg, B: db, A: a})
			}
		}
	}

	// Small lock/checkmark mark in center — a simple "V" for VPN
	// Draw a vertical line (center of shield)
	for y := 8; y <= 20; y++ {
		mx := int(cx)
		a := byte(255)
		img.SetRGBA(mx, y, color.RGBA{R: 255, G: 255, B: 255, A: a})
		img.SetRGBA(mx-1, y, color.RGBA{R: 255, G: 255, B: 255, A: 120})
		img.SetRGBA(mx+1, y, color.RGBA{R: 255, G: 255, B: 255, A: 120})
	}
	// Horizontal bar of a keyhole shape
	for x := int(cx) - 4; x <= int(cx)+4; x++ {
		a := byte(255)
		img.SetRGBA(x, 12, color.RGBA{R: 255, G: 255, B: 255, A: a})
		img.SetRGBA(x, 13, color.RGBA{R: 255, G: 255, B: 255, A: 180})
	}

	if runtime.GOOS == "darwin" {
		return buildPNG(img)
	}
	return buildICOFromImage(img)
}

// buildPNG encodes an RGBA image as PNG (for macOS systray)
func buildPNG(img *image.RGBA) []byte {
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

// buildICOFromImage creates a valid ICO file from an RGBA image (for Windows systray)
func buildICOFromImage(img *image.RGBA) []byte {
	size := img.Bounds().Dx()

	// Convert to bottom-up BGRA
	pixels := make([]byte, size*size*4)
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			c := img.RGBAAt(x, y)
			off := ((size - 1 - y) * size + x) * 4
			pixels[off+0] = c.B
			pixels[off+1] = c.G
			pixels[off+2] = c.R
			pixels[off+3] = c.A
		}
	}

	const dibHeaderSize = 40
	pixelDataSize := size * size * 4
	maskRowSize := ((size + 31) / 32) * 4
	maskSize := maskRowSize * size
	imageDataSize := dibHeaderSize + pixelDataSize + maskSize
	headerSize := 6 + 16

	buf := make([]byte, 0, headerSize+imageDataSize)

	// ICONDIR
	buf = append(buf, 0, 0)
	buf = append(buf, 1, 0) // ICO type
	buf = append(buf, 1, 0) // 1 image

	// ICONDIRENTRY
	buf = append(buf, byte(size))
	buf = append(buf, byte(size))
	buf = append(buf, 0)     // No palette
	buf = append(buf, 0)     // Reserved
	buf = append(buf, 1, 0)  // Planes
	buf = append(buf, 32, 0) // BPP

	imgSize := uint32(imageDataSize)
	buf = append(buf, byte(imgSize), byte(imgSize>>8), byte(imgSize>>16), byte(imgSize>>24))
	off := uint32(headerSize)
	buf = append(buf, byte(off), byte(off>>8), byte(off>>16), byte(off>>24))

	// BITMAPINFOHEADER
	buf = append(buf, 40, 0, 0, 0)
	buf = append(buf, byte(size), 0, 0, 0)
	h2 := size * 2
	buf = append(buf, byte(h2), 0, 0, 0)
	buf = append(buf, 1, 0)
	buf = append(buf, 32, 0)
	buf = append(buf, 0, 0, 0, 0) // No compression
	pxSize := uint32(pixelDataSize)
	buf = append(buf, byte(pxSize), byte(pxSize>>8), byte(pxSize>>16), byte(pxSize>>24))
	buf = append(buf, 0, 0, 0, 0)
	buf = append(buf, 0, 0, 0, 0)
	buf = append(buf, 0, 0, 0, 0)
	buf = append(buf, 0, 0, 0, 0)

	// Pixel data (bottom-up BGRA)
	buf = append(buf, pixels...)

	// AND mask
	for i := 0; i < maskSize; i++ {
		buf = append(buf, 0)
	}

	return buf
}

// Placeholder icons (pre-generated for startup speed)
var (
	GrayIcon   = GenerateShieldIcon(160, 160, 160)
	GreenIcon  = GenerateShieldIcon(30, 200, 90)
	YellowIcon = GenerateShieldIcon(240, 190, 30)
	RedIcon    = GenerateShieldIcon(220, 55, 55)
)
