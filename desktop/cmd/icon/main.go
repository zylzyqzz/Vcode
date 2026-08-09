// Command icon generates the committed desktop PNG and Windows ICO assets from
// the Vcode vector geometry. Run with: go run ./cmd/icon
package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
)

var iconSizes = []int{16, 24, 32, 48, 64, 256}

func main() {
	if err := os.MkdirAll("build/windows", 0o755); err != nil {
		panic(err)
	}
	if err := writePNG("build/appicon.png", drawIcon(1024)); err != nil {
		panic(err)
	}

	images := make([][]byte, 0, len(iconSizes))
	for _, size := range iconSizes {
		var encoded bytes.Buffer
		if err := png.Encode(&encoded, drawIcon(size)); err != nil {
			panic(err)
		}
		images = append(images, encoded.Bytes())
	}
	if err := os.WriteFile(filepath.Join("build", "windows", "icon.ico"), encodeICO(images), 0o644); err != nil {
		panic(err)
	}
}

func writePNG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

func drawIcon(size int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	radius := 80 * size / 400
	blue := color.NRGBA{R: 0x01, G: 0x53, B: 0xe5, A: 0xff}
	for y := range size {
		for x := range size {
			if inRoundedSquare(x, y, size, radius) {
				img.SetNRGBA(x, y, blue)
			}
		}
	}

	points := []image.Point{
		{X: 120 * size / 400, Y: 120 * size / 400},
		{X: 160 * size / 400, Y: 120 * size / 400},
		{X: 220 * size / 400, Y: 220 * size / 400},
		{X: 280 * size / 400, Y: 120 * size / 400},
		{X: 320 * size / 400, Y: 120 * size / 400},
		{X: 220 * size / 400, Y: 280 * size / 400},
	}
	for y := range size {
		for x := range size {
			if inPolygon(x, y, points) {
				img.SetNRGBA(x, y, color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff})
			}
		}
	}
	return img
}

func inRoundedSquare(x, y, size, radius int) bool {
	if x >= radius && x < size-radius || y >= radius && y < size-radius {
		return true
	}
	cx, cy := radius, radius
	if x >= size-radius {
		cx = size - radius - 1
	}
	if y >= size-radius {
		cy = size - radius - 1
	}
	dx, dy := x-cx, y-cy
	return dx*dx+dy*dy <= radius*radius
}

func inPolygon(x, y int, points []image.Point) bool {
	inside := false
	for i, current := range points {
		previous := points[(i+len(points)-1)%len(points)]
		crosses := (current.Y > y) != (previous.Y > y)
		if crosses && x < (previous.X-current.X)*(y-current.Y)/(previous.Y-current.Y)+current.X {
			inside = !inside
		}
	}
	return inside
}

func encodeICO(images [][]byte) []byte {
	const headerSize = 6
	const entrySize = 16
	data := make([]byte, headerSize+entrySize*len(images))
	binary.LittleEndian.PutUint16(data[2:], 1)
	binary.LittleEndian.PutUint16(data[4:], uint16(len(images)))
	offset := len(data)
	for i, size := range iconSizes {
		entry := data[headerSize+i*entrySize : headerSize+(i+1)*entrySize]
		if size != 256 {
			entry[0], entry[1] = byte(size), byte(size)
		}
		binary.LittleEndian.PutUint16(entry[4:], 1)
		binary.LittleEndian.PutUint16(entry[6:], 32)
		binary.LittleEndian.PutUint32(entry[8:], uint32(len(images[i])))
		binary.LittleEndian.PutUint32(entry[12:], uint32(offset))
		data = append(data, images[i]...)
		offset += len(images[i])
	}
	return data
}
