// Command icon rasterises internal/brand/logo.svg into the raster forms that
// cannot read an SVG or a stylesheet:
//
//	internal/brand/icon.ico       eight frames, embedded in the Windows binary
//	internal/brand/icon-32.png    the favicon the web page links to
//	internal/brand/icon-192.png   ditto, for touch icons
//
// The shape is the one the SVG and the Android vector drawable use; see the
// comment above the geometry below. Gradients are interpolated per pixel and
// edges are resolved by 4x4 supersampling, which is plenty for a mark made of
// straight lines and circles.
//
// After editing the SVG, also refresh the resources that carry it:
//
//	go run ./tools/icon
//	go run github.com/akavel/rsrc@latest -arch amd64 -ico internal/brand/icon.ico -o cmd/pantegnos/rsrc_windows_amd64.syso
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
)

type point struct{ x, y float64 }

// Geometry mirrors internal/brand/logo.svg, which mirrors
// android/app/src/main/res/drawable/ic_launcher_foreground.xml. The hexagon pair
// winds in opposite directions, and the keyhole is a real cut-out rather than a
// shape painted in the background colour, so the mark survives being placed on
// any surface.
var (
	hexagonOuter = []point{{54, 27}, {77.38, 40.5}, {77.38, 67.5}, {54, 81}, {30.62, 67.5}, {30.62, 40.5}}
	hexagonInner = []point{{54, 33.5}, {36.25, 43.75}, {36.25, 64.25}, {54, 74.5}, {71.75, 64.25}, {71.75, 43.75}}
	keyholeTail  = []point{{54, 52.4}, {57.2, 63.5}, {50.8, 63.5}}
)

const (
	viewBox    = 108.0
	lockCX     = 54.0
	lockCY     = 53.0 // the disc spans y 41..65
	lockOuter  = 12.0
	holeCX     = 54.0
	holeCY     = 50.0
	holeRadius = 3.4
)

var (
	gradA = color.RGBA{0xff, 0xd9, 0xa0, 0xff} // #ffd9a0
	gradM = color.RGBA{0xff, 0x9e, 0x3d, 0xff} // #ff9e3d
	gradB = color.RGBA{0xff, 0x6b, 0x1a, 0xff} // #ff6b1a
)

// inside reports whether a point is within a polygon, using the non-zero winding
// rule to match how the SVG is filled.
func inside(p point, poly []point) bool {
	winding := 0
	for i, a := range poly {
		b := poly[(i+1)%len(poly)]
		if a.y <= p.y {
			if b.y > p.y && cross(a, b, p) > 0 {
				winding++
			}
		} else if b.y <= p.y && cross(a, b, p) < 0 {
			winding--
		}
	}
	return winding != 0
}

func cross(a, b, p point) float64 {
	return (b.x-a.x)*(p.y-a.y) - (b.y-a.y)*(p.x-a.x)
}

func inCircle(p point, cx, cy, r float64) bool {
	dx, dy := p.x-cx, p.y-cy
	return dx*dx+dy*dy <= r*r
}

// covered is the silhouette of the mark: the hexagonal frame, and the lock
// body with the keyhole removed.
func covered(p point) bool {
	frame := inside(p, hexagonOuter) && !inside(p, hexagonInner)
	lock := inCircle(p, lockCX, lockCY, lockOuter)
	keyhole := inCircle(p, holeCX, holeCY, holeRadius) || inside(p, keyholeTail)
	return frame || (lock && !keyhole)
}

// shade returns the gradient colour along the same top-left to bottom-right
// diagonal the SVG uses, including its mid stop.
func shade(p point) color.RGBA {
	t := (p.x + p.y) / (2 * viewBox)
	switch {
	case t < 0:
		t = 0
	case t > 1:
		t = 1
	}
	var lo, hi color.RGBA
	var f float64
	if t < 0.45 {
		lo, hi, f = gradA, gradM, t/0.45
	} else {
		lo, hi, f = gradM, gradB, (t-0.45)/0.55
	}
	mix := func(a, b uint8) uint8 { return uint8(math.Round(float64(a) + (float64(b)-float64(a))*f)) }
	return color.RGBA{mix(lo.R, hi.R), mix(lo.G, hi.G), mix(lo.B, hi.B), 0xff}
}

const samples = 4

// render draws the mark at the given size with supersampled edges.
func render(size int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	scale := viewBox / float64(size)
	step := 1 / float64(samples)
	offset := step / 2

	for py := range size {
		for px := range size {
			hits := 0
			var sumR, sumG, sumB float64
			for sy := range samples {
				for sx := range samples {
					p := point{
						x: (float64(px) + float64(sx)*step + offset) * scale,
						y: (float64(py) + float64(sy)*step + offset) * scale,
					}
					if !covered(p) {
						continue
					}
					hits++
					c := shade(p)
					sumR += float64(c.R)
					sumG += float64(c.G)
					sumB += float64(c.B)
				}
			}
			if hits == 0 {
				continue
			}
			n := float64(hits)
			img.SetNRGBA(px, py, color.NRGBA{
				R: uint8(math.Round(sumR / n)),
				G: uint8(math.Round(sumG / n)),
				B: uint8(math.Round(sumB / n)),
				A: uint8(math.Round(255 * n / (samples * samples))),
			})
		}
	}
	return img
}

func encodePNG(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	err := png.Encode(&buf, img)
	return buf.Bytes(), err
}

// iconSizes are the frames packed into the .ico. Windows picks whichever suits
// the shell it is drawn in, so all of them ship inside the one file.
var iconSizes = []int{16, 24, 32, 48, 64, 128, 192, 256}

// standaloneSizes are also written next to the .ico, because the browser cannot
// read an .ico and the web page links to these two directly.
var standaloneSizes = []int{32, 192}

// writeICO packs PNG-compressed frames into a single .ico, which every Windows
// shell since Vista understands.
func writeICO(path string, frames [][]byte, sizes []int) error {
	if len(frames) != len(sizes) {
		return fmt.Errorf("frames/sizes mismatch")
	}
	var out bytes.Buffer
	binary.Write(&out, binary.LittleEndian, uint16(0)) // reserved
	binary.Write(&out, binary.LittleEndian, uint16(1)) // type: icon
	binary.Write(&out, binary.LittleEndian, uint16(len(frames)))

	headerLen := 6 + 16*len(frames)
	offset := headerLen
	for i, size := range sizes {
		dim := byte(size)
		if size >= 256 {
			dim = 0 // 0 means 256 in the ICO directory
		}
		out.Write([]byte{dim, dim, 0, 0})
		binary.Write(&out, binary.LittleEndian, uint16(1))  // colour planes
		binary.Write(&out, binary.LittleEndian, uint16(32)) // bits per pixel
		binary.Write(&out, binary.LittleEndian, uint32(len(frames[i])))
		binary.Write(&out, binary.LittleEndian, uint32(offset))
		offset += len(frames[i])
	}
	for _, frame := range frames {
		out.Write(frame)
	}
	return os.WriteFile(path, out.Bytes(), 0o644)
}

// repoRoot walks up from the working directory until it finds go.mod, so the
// tool behaves the same whether it is run from the repo root or from tools/icon.
func repoRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			panic("could not find go.mod above " + dir)
		}
		dir = parent
	}
}

func main() {
	assets := filepath.Join(repoRoot(), "internal", "brand")
	if err := os.MkdirAll(assets, 0o755); err != nil {
		panic(err)
	}

	var frames [][]byte
	for _, size := range iconSizes {
		data, err := encodePNG(render(size))
		if err != nil {
			panic(err)
		}
		frames = append(frames, data)
	}

	ico := filepath.Join(assets, "icon.ico")
	if err := writeICO(ico, frames, iconSizes); err != nil {
		panic(err)
	}
	info, err := os.Stat(ico)
	if err != nil {
		panic(err)
	}
	fmt.Printf("wrote %s (%d bytes, %d frames)\n", ico, info.Size(), len(frames))

	// Only the frames the web page actually references are written separately.
	bySize := make(map[int][]byte, len(iconSizes))
	for i, size := range iconSizes {
		bySize[size] = frames[i]
	}
	for _, size := range standaloneSizes {
		name := filepath.Join(assets, fmt.Sprintf("icon-%d.png", size))
		if err := os.WriteFile(name, bySize[size], 0o644); err != nil {
			panic(err)
		}
		fmt.Printf("wrote %s\n", name)
	}
}
