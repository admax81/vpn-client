//go:build ignore

// gen-icon generates .ico file from the programmatic icon in the ui package.
// Usage: go run build/gen-icon/main.go [output.ico]
//
// The generated .ico can be used for the Windows installer and executable.
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
)

func main() {
	output := "build/windows/icon.ico"
	if len(os.Args) > 1 {
		output = os.Args[1]
	}

	// Generate a 32x32 BGRA icon (same palette as the app)
	sizes := []int{16, 32, 48, 256}
	images := make([][]byte, len(sizes))

	for i, size := range sizes {
		images[i] = generateMaskBMP(size, 30, 200, 90) // Green (connected state)
	}

	ico := buildICO(sizes, images)

	if err := os.MkdirAll("build/windows", 0755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(output, ico, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "write %s: %v\n", output, err)
		os.Exit(1)
	}

	fmt.Printf("Generated %s (%d bytes)\n", output, len(ico))
}

// buildICO creates a .ico file from BGRA pixel data at various sizes
func buildICO(sizes []int, images [][]byte) []byte {
	var buf bytes.Buffer
	n := len(sizes)

	// ICONDIR header: 6 bytes
	binary.Write(&buf, binary.LittleEndian, uint16(0)) // reserved
	binary.Write(&buf, binary.LittleEndian, uint16(1)) // type: icon
	binary.Write(&buf, binary.LittleEndian, uint16(n)) // count

	headerSize := 6 + n*16
	offset := headerSize

	// ICONDIRENTRY for each image
	for i, size := range sizes {
		w := byte(size)
		h := byte(size)
		if size == 256 {
			w, h = 0, 0 // 0 = 256 in ICO format
		}

		bmpSize := len(images[i])

		buf.WriteByte(w)                                    // width
		buf.WriteByte(h)                                    // height
		buf.WriteByte(0)                                    // color palette
		buf.WriteByte(0)                                    // reserved
		binary.Write(&buf, binary.LittleEndian, uint16(1))  // color planes
		binary.Write(&buf, binary.LittleEndian, uint16(32)) // bits per pixel
		binary.Write(&buf, binary.LittleEndian, uint32(bmpSize))
		binary.Write(&buf, binary.LittleEndian, uint32(offset))

		offset += bmpSize
	}

	// Image data
	for _, img := range images {
		buf.Write(img)
	}

	return buf.Bytes()
}

// generateMaskBMP renders a simplified shield/mask icon as BMP DIB data
func generateMaskBMP(size int, cr, cg, cb byte) []byte {
	pixels := make([]byte, size*size*4)

	cx, cy := float64(size)/2, float64(size)/2
	radius := float64(size) * 0.4

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx := float64(x) - cx
			dy := float64(y) - cy
			dist := dx*dx + dy*dy

			if dist < radius*radius {
				off := (y*size + x) * 4
				pixels[off+0] = cb // B
				pixels[off+1] = cg // G
				pixels[off+2] = cr // R
				pixels[off+3] = 255
			}
		}
	}

	// BMP DIB header (BITMAPINFOHEADER - 40 bytes)
	var buf bytes.Buffer
	binary.Write(&buf, binary.LittleEndian, uint32(40))          // header size
	binary.Write(&buf, binary.LittleEndian, int32(size))         // width
	binary.Write(&buf, binary.LittleEndian, int32(size*2))       // height (doubled for AND mask)
	binary.Write(&buf, binary.LittleEndian, uint16(1))           // planes
	binary.Write(&buf, binary.LittleEndian, uint16(32))          // bpp
	binary.Write(&buf, binary.LittleEndian, uint32(0))           // compression
	binary.Write(&buf, binary.LittleEndian, uint32(len(pixels))) // image size
	binary.Write(&buf, binary.LittleEndian, int32(0))            // X ppm
	binary.Write(&buf, binary.LittleEndian, int32(0))            // Y ppm
	binary.Write(&buf, binary.LittleEndian, uint32(0))           // colors used
	binary.Write(&buf, binary.LittleEndian, uint32(0))           // important colors

	// XOR mask (BGRA pixel data, bottom-up)
	for y := size - 1; y >= 0; y-- {
		for x := 0; x < size; x++ {
			off := (y*size + x) * 4
			buf.Write(pixels[off : off+4])
		}
	}

	// AND mask (1bpp, all zeros since we have alpha channel)
	andRowBytes := ((size + 31) / 32) * 4
	andMask := make([]byte, andRowBytes*size)
	buf.Write(andMask)

	return buf.Bytes()
}
