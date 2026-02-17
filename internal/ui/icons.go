package ui

// GetIcon returns the icon data for the given state
func GetIcon(state string) []byte {
	switch state {
	case "connected":
		return GenerateMaskIcon(30, 200, 90) // Green
	case "connecting":
		return GenerateMaskIcon(240, 190, 30) // Yellow/Amber
	case "error":
		return GenerateMaskIcon(220, 55, 55) // Red
	default:
		return GenerateMaskIcon(160, 160, 160) // Gray (disconnected)
	}
}

// GenerateMaskIcon renders a Guy Fawkes / Anonymous mask icon at 32x32 with transparent background
func GenerateMaskIcon(cr, cg, cb byte) []byte {
	const size = 32
	pixels := make([]byte, size*size*4)

	setPx := func(x, y int, r, g, b, a byte) {
		if x < 0 || x >= size || y < 0 || y >= size {
			return
		}
		off := ((size-1-y)*size + x) * 4
		ea := float64(pixels[off+3]) / 255.0
		na := float64(a) / 255.0
		oa := na + ea*(1-na)
		if oa > 0 {
			pixels[off+0] = byte((float64(b)*na + float64(pixels[off+0])*ea*(1-na)) / oa)
			pixels[off+1] = byte((float64(g)*na + float64(pixels[off+1])*ea*(1-na)) / oa)
			pixels[off+2] = byte((float64(r)*na + float64(pixels[off+2])*ea*(1-na)) / oa)
			pixels[off+3] = byte(oa * 255)
		}
	}

	abs := func(v float64) float64 {
		if v < 0 {
			return -v
		}
		return v
	}

	sqrt := func(v float64) float64 {
		if v <= 0 {
			return 0
		}
		r := v
		for i := 0; i < 20; i++ {
			r = (r + v/r) / 2
		}
		return r
	}

	// Dark variant for features
	var dr, dg, db byte
	lum := int(cr)*299 + int(cg)*587 + int(cb)*114
	if lum > 128000 {
		dr, dg, db = 40, 40, 40
	} else {
		dr, dg, db = cr/3, cg/3, cb/3
	}

	// Center of mask
	cx := 15.5

	// ======== 1. Face outline (egg/shield shape) ========
	// Symmetric face: wide at eyes (y~10), narrow pointed chin (y~29)
	for y := 1; y <= 30; y++ {
		fy := float64(y)
		var halfW float64

		if fy <= 4 {
			// Forehead top - dome
			t := (fy - 1) / 3.0
			hw := 8.0 + 5.0*t
			if hw > 13.0 {
				hw = 13.0
			}
			halfW = hw
		} else if fy <= 12 {
			// Wide face area
			halfW = 13.0
		} else if fy <= 18 {
			// Cheeks narrowing slightly
			t := (fy - 12) / 6.0
			halfW = 13.0 - 2.0*t
		} else if fy <= 24 {
			// Jaw narrowing
			t := (fy - 18) / 6.0
			halfW = 11.0 - 5.0*t
		} else {
			// Chin/beard pointed
			t := (fy - 24) / 6.0
			halfW = 6.0 - 5.5*t
			if halfW < 0.5 {
				halfW = 0.5
			}
		}

		for x := 0; x < size; x++ {
			fx := float64(x) + 0.5
			d := abs(fx - cx)
			if d <= halfW {
				a := byte(255)
				if d > halfW-1.0 {
					a = byte((halfW - d) * 255)
				}
				setPx(x, y, cr, cg, cb, a)
			}
		}
	}

	// ======== 2. Eyes - two angled slits (Guy Fawkes style) ========
	// Left eye: angled line from (7,10) to (12,12)
	// Right eye: mirrored (19,12) to (24,10)
	for i := 0; i <= 20; i++ {
		t := float64(i) / 20.0
		// Left eye
		lx := 7.0 + 5.0*t
		ly := 10.0 + 2.0*t
		for dy := -1; dy <= 1; dy++ {
			a := byte(255)
			if dy != 0 {
				a = 180
			}
			setPx(int(lx), int(ly)+dy, dr, dg, db, a)
		}
		// Right eye (perfectly mirrored)
		rx := 31.0 - lx
		ry := ly
		for dy := -1; dy <= 1; dy++ {
			a := byte(255)
			if dy != 0 {
				a = 180
			}
			setPx(int(rx), int(ry)+dy, dr, dg, db, a)
		}
	}

	// ======== 3. Eyebrows - arched above eyes ========
	for i := 0; i <= 20; i++ {
		t := float64(i) / 20.0
		// Left eyebrow
		lx := 6.0 + 7.0*t
		ly := 9.0 - 2.0*t*(1-t)*4 // arch up
		setPx(int(lx), int(ly), dr, dg, db, 220)
		setPx(int(lx), int(ly)-1, dr, dg, db, 100)
		// Right eyebrow (mirrored)
		rx := 31.0 - lx
		ry := ly
		setPx(int(rx), int(ry), dr, dg, db, 220)
		setPx(int(rx), int(ry)-1, dr, dg, db, 100)
	}

	// ======== 4. Nose - small triangle ========
	for y := 14; y <= 17; y++ {
		w := float64(y-14)*0.7 + 0.5
		for x := 0; x < size; x++ {
			fx := float64(x) + 0.5
			d := abs(fx - cx)
			if d <= w {
				a := byte(200)
				if d > w-0.5 {
					a = 120
				}
				setPx(x, y, dr, dg, db, a)
			}
		}
	}

	// ======== 5. Mustache - Guy Fawkes thin curled mustache ========
	for i := 0; i <= 30; i++ {
		t := float64(i) / 30.0
		// Left side of mustache: from center going left and curling up
		lx := cx - t*11.0
		ly := 19.0 + t*t*3.0 - t*4.0 // curves down then up
		if t > 0.7 {
			ly -= (t - 0.7) * 6.0 // curl up at tips
		}
		setPx(int(lx), int(ly), dr, dg, db, 255)
		setPx(int(lx), int(ly)+1, dr, dg, db, 150)

		// Right side (mirrored)
		rx := cx + t*11.0
		ry := ly
		setPx(int(rx), int(ry), dr, dg, db, 255)
		setPx(int(rx), int(ry)+1, dr, dg, db, 150)
	}

	// ======== 6. Mouth - small smile under mustache ========
	for i := 0; i <= 16; i++ {
		t := float64(i) / 16.0
		x := cx - 3.0 + 6.0*t
		y := 21.0 + t*(1-t)*3.0 // gentle smile curve
		setPx(int(x), int(y), dr, dg, db, 200)
	}

	// ======== 7. Goatee / beard point ========
	for y := 23; y <= 27; y++ {
		w := 2.5 - float64(y-23)*0.5
		if w < 0.3 {
			w = 0.3
		}
		for x := 0; x < size; x++ {
			fx := float64(x) + 0.5
			d := abs(fx - cx)
			if d <= w {
				a := byte(220)
				if d > w-0.5 {
					a = 120
				}
				setPx(x, y, dr, dg, db, a)
			}
		}
	}

	// ======== 8. Cheek lines (Guy Fawkes style creases) ========
	for i := 0; i <= 12; i++ {
		t := float64(i) / 12.0
		// Left cheek crease
		lx := 8.0 + t*3.0
		ly := 14.0 + t*6.0
		setPx(int(lx), int(ly), dr, dg, db, 120)
		// Right cheek crease (mirrored from center)
		rx := 31.0 - lx
		ry := ly
		setPx(int(rx), int(ry), dr, dg, db, 120)
	}

	// ======== 9. Face outline edge (darker border) ========
	for y := 1; y <= 30; y++ {
		fy := float64(y)
		var halfW float64

		if fy <= 4 {
			t := (fy - 1) / 3.0
			hw := 8.0 + 5.0*t
			if hw > 13.0 {
				hw = 13.0
			}
			halfW = hw
		} else if fy <= 12 {
			halfW = 13.0
		} else if fy <= 18 {
			t := (fy - 12) / 6.0
			halfW = 13.0 - 2.0*t
		} else if fy <= 24 {
			t := (fy - 18) / 6.0
			halfW = 11.0 - 5.0*t
		} else {
			t := (fy - 24) / 6.0
			halfW = 6.0 - 5.5*t
			if halfW < 0.5 {
				halfW = 0.5
			}
		}

		// Draw left and right border pixels
		lBorder := int(cx - halfW + 0.5)
		rBorder := int(cx + halfW - 0.5)
		ew := sqrt(halfW * 0.1)
		_ = ew
		setPx(lBorder, y, dr, dg, db, 160)
		setPx(lBorder+1, y, dr, dg, db, 60)
		setPx(rBorder, y, dr, dg, db, 160)
		setPx(rBorder-1, y, dr, dg, db, 60)
	}

	return buildICO(size, pixels)
}

// buildICO creates a valid ICO file from BGRA pixel data
func buildICO(size int, pixels []byte) []byte {
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

	// Pixel data (already bottom-up BGRA)
	buf = append(buf, pixels...)

	// AND mask
	for i := 0; i < maskSize; i++ {
		buf = append(buf, 0)
	}

	return buf
}

// Placeholder icons (pre-generated for startup speed)
var (
	GrayIcon   = GenerateMaskIcon(160, 160, 160)
	GreenIcon  = GenerateMaskIcon(30, 200, 90)
	YellowIcon = GenerateMaskIcon(240, 190, 30)
	RedIcon    = GenerateMaskIcon(220, 55, 55)
)
