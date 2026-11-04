// menuicons draws the icons of the Explorer menu as .ico files. Each one is a few flat shapes on a 32 unit grid,
// rendered with 4x4 supersampling at the sizes Windows asks for at different scalings.
//
//	go run ./scripts/menuicons internal/explorer/icons
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

type shape interface{ dist(x, y float64) float64 }

type circle struct{ cx, cy, r float64 }

func (c circle) dist(x, y float64) float64 { return math.Hypot(x-c.cx, y-c.cy) - c.r }

type ring struct{ cx, cy, r, w float64 }

func (c ring) dist(x, y float64) float64 { return math.Abs(math.Hypot(x-c.cx, y-c.cy)-c.r) - c.w/2 }

type ellipse struct{ cx, cy, rx, ry float64 }

func (e ellipse) dist(x, y float64) float64 {
	return (math.Hypot((x-e.cx)/e.rx, (y-e.cy)/e.ry) - 1) * math.Min(e.rx, e.ry)
}

type seg struct{ x0, y0, x1, y1, w float64 }

func (s seg) dist(x, y float64) float64 {
	dx, dy := s.x1-s.x0, s.y1-s.y0
	t := 0.0
	if l := dx*dx + dy*dy; l > 0 {
		t = math.Max(0, math.Min(1, ((x-s.x0)*dx+(y-s.y0)*dy)/l))
	}
	return math.Hypot(x-(s.x0+t*dx), y-(s.y0+t*dy)) - s.w/2
}

type rect struct{ x0, y0, x1, y1, r float64 }

func (s rect) dist(x, y float64) float64 {
	cx, cy := (s.x0+s.x1)/2, (s.y0+s.y1)/2
	hx, hy := (s.x1-s.x0)/2-s.r, (s.y1-s.y0)/2-s.r
	dx, dy := math.Abs(x-cx)-hx, math.Abs(y-cy)-hy
	return math.Hypot(math.Max(dx, 0), math.Max(dy, 0)) + math.Min(math.Max(dx, dy), 0) - s.r
}

// arc is a stroke along a circle from angle a0 to a1 in degrees, clockwise on screen (0 is right, 90 is down).
type arc struct{ cx, cy, r, a0, a1, w float64 }

func (s arc) dist(x, y float64) float64 {
	a := math.Atan2(y-s.cy, x-s.cx) * 180 / math.Pi
	for a < s.a0 {
		a += 360
	}
	if a <= s.a1 {
		return math.Abs(math.Hypot(x-s.cx, y-s.cy)-s.r) - s.w/2
	}
	end := func(deg float64) float64 {
		r := deg * math.Pi / 180
		return math.Hypot(x-(s.cx+s.r*math.Cos(r)), y-(s.cy+s.r*math.Sin(r))) - s.w/2
	}
	return math.Min(end(s.a0), end(s.a1))
}

type tri struct{ ax, ay, bx, by, cx, cy float64 }

func (t tri) dist(x, y float64) float64 {
	side := func(px, py, qx, qy float64) float64 { return (qx-px)*(y-py) - (qy-py)*(x-px) }
	d1, d2, d3 := side(t.ax, t.ay, t.bx, t.by), side(t.bx, t.by, t.cx, t.cy), side(t.cx, t.cy, t.ax, t.ay)
	neg, pos := d1 < 0 || d2 < 0 || d3 < 0, d1 > 0 || d2 > 0 || d3 > 0
	if neg && pos {
		return 1
	}
	return -1
}

type layer struct {
	s shape
	c color.NRGBA // alpha 0 cuts a hole through what is under it
}

var cut = color.NRGBA{}

func hex(v uint32) color.NRGBA {
	return color.NRGBA{uint8(v >> 16), uint8(v >> 8), uint8(v), 255}
}

var (
	blue   = hex(0x3B82F6)
	green  = hex(0x22A34A)
	orange = hex(0xF0843C)
	red    = hex(0xE5484D)
	purple = hex(0x8B5CF6)
	gray   = hex(0x8A929D)
	yellow = hex(0xE0A31B)
	brown  = hex(0xB07A4A)
	teal   = hex(0x14A89A)
	light  = hex(0x5ED3C5)
)

func arrowDown(x, top, bottom float64, c color.NRGBA) []layer {
	return []layer{{seg{x, top, x, bottom - 6, 4}, c}, {tri{x - 8, bottom - 9, x + 8, bottom - 9, x, bottom}, c}}
}

func arrowUp(x, bottom, top float64, c color.NRGBA) []layer {
	return []layer{{seg{x, bottom, x, top + 6, 4}, c}, {tri{x - 8, top + 9, x + 8, top + 9, x, top}, c}}
}

func cylinder(top, bottom float64) []layer {
	return []layer{
		{rect{6, top, 26, bottom, 0.01}, teal},
		{ellipse{16, bottom, 10, 3.5}, teal},
		{ellipse{16, top, 10, 3.5}, light},
	}
}

func doc(c color.NRGBA) []layer {
	return []layer{{rect{5, 3, 21, 27, 2.5}, c}, {seg{9, 10, 17, 10, 2}, cut}, {seg{9, 15, 17, 15, 2}, cut}, {seg{9, 20, 14, 20, 2}, cut}}
}

func lockBody(c color.NRGBA) []layer {
	return []layer{{rect{6, 15, 26, 29, 3}, c}, {circle{16, 21, 2.3}, cut}, {seg{16, 21, 16, 25, 2}, cut}}
}

func icons() map[string][]layer {
	gear := []layer{{ring{16, 16, 8, 6}, gray}}
	for k := 0.0; k < 8; k++ {
		a := k * math.Pi / 4
		gear = append(gear, layer{seg{16 + 10*math.Cos(a), 16 + 10*math.Sin(a), 16 + 13*math.Cos(a), 16 + 13*math.Sin(a), 4.5}, gray})
	}
	gear = append(gear, layer{circle{16, 16, 3.5}, cut})

	m := map[string][]layer{
		"update":   arrowDown(16, 3, 29, green),
		"commit":   arrowUp(16, 29, 3, orange),
		"checkout": append([]layer{{seg{5, 21, 5, 28, 3}, blue}, {seg{5, 28, 27, 28, 3}, blue}, {seg{27, 28, 27, 21, 3}, blue}}, arrowDown(16, 2, 23, blue)...),
		"open":     {{ring{13, 13, 8, 3.5}, blue}, {seg{19, 19, 27, 27, 4.5}, blue}},
		"log":      {{rect{6, 3, 26, 29, 3}, gray}, {seg{10, 10, 22, 10, 2.5}, cut}, {seg{10, 16, 22, 16, 2.5}, cut}, {seg{10, 22, 18, 22, 2.5}, cut}},
		"repos":    cylinder(9, 24),
		"history":  {{ring{16, 16, 11, 3}, blue}, {seg{16, 16, 16, 9, 3}, blue}, {seg{16, 16, 21, 19, 3}, blue}},
		"revert":   {{arc{17, 18, 9, 180, 405, 3.5}, red}, {tri{3, 14, 13, 14, 8, 22}, red}},
		"cleanup": {
			{tri{16, 3, 19, 16, 13, 16}, yellow}, {tri{16, 29, 19, 16, 13, 16}, yellow},
			{tri{3, 16, 16, 13, 16, 19}, yellow}, {tri{29, 16, 16, 13, 16, 19}, yellow},
			{tri{26, 3, 27.5, 8, 24.5, 8}, yellow}, {tri{26, 13, 27.5, 8, 24.5, 8}, yellow},
			{tri{21, 8, 26, 6.5, 26, 9.5}, yellow}, {tri{31, 8, 26, 6.5, 26, 9.5}, yellow},
		},
		"resolve":   {{tri{16, 3, 30, 28, 2, 28}, red}, {seg{16, 11, 16, 19, 3}, cut}, {circle{16, 23.5, 1.9}, cut}},
		"branch":    {{seg{10, 7, 10, 25, 3}, purple}, {seg{23, 12, 23, 15, 3}, purple}, {seg{23, 15, 10, 22, 3}, purple}, {circle{10, 7, 3.8}, purple}, {circle{10, 25, 3.8}, purple}, {circle{23, 11, 3.8}, purple}},
		"switch":    {{seg{5, 10, 24, 10, 3}, teal}, {tri{21, 4, 21, 16, 29, 10}, teal}, {seg{27, 22, 8, 22, 3}, teal}, {tri{11, 16, 11, 28, 3, 22}, teal}},
		"merge":     {{seg{8, 4, 16, 16, 3.5}, purple}, {seg{24, 4, 16, 16, 3.5}, purple}, {seg{16, 16, 16, 28, 3.5}, purple}},
		"export":    {{seg{5, 10, 5, 27, 3}, gray}, {seg{5, 27, 22, 27, 3}, gray}, {seg{22, 27, 22, 20, 3}, gray}, {seg{5, 10, 12, 10, 3}, gray}, {seg{12, 20, 24, 8, 3.5}, orange}, {tri{18, 4, 28, 4, 28, 14}, orange}},
		"relocate":  {{circle{16, 12, 8.5}, blue}, {tri{8.5, 15, 23.5, 15, 16, 29}, blue}, {circle{16, 12, 3.3}, cut}},
		"add":       {{seg{16, 5, 16, 27, 5}, green}, {seg{5, 16, 27, 16, 5}, green}},
		"ignore":    {{ring{16, 16, 11, 3.5}, gray}, {seg{8.5, 8.5, 23.5, 23.5, 3.5}, gray}},
		"shelve":    {{rect{4, 13, 28, 28, 3}, brown}, {rect{3, 7, 29, 13.5, 2}, hex(0x8C5E37)}, {seg{13, 19, 19, 19, 2.5}, cut}},
		"unshelve":  append([]layer{{rect{4, 17, 28, 29, 3}, brown}, {seg{11, 22, 21, 22, 2.5}, cut}}, arrowUp(16, 17, 1, green)...),
		"patch":     append(doc(gray), layer{seg{24, 17, 24, 31, 4.5}, green}, layer{seg{17, 24, 31, 24, 4.5}, green}),
		"apply":     append(doc(gray), layer{seg{16, 24, 20, 28, 4}, green}, layer{seg{20, 28, 29, 17, 4}, green}),
		"props":     {{seg{5, 8, 27, 8, 2.5}, gray}, {seg{5, 16, 27, 16, 2.5}, gray}, {seg{5, 24, 27, 24, 2.5}, gray}, {circle{11, 8, 3.8}, blue}, {circle{21, 16, 3.8}, blue}, {circle{14, 24, 3.8}, blue}},
		"import":    append(cylinder(19, 27), arrowDown(16, 1, 17, green)...),
		"newrepo":   append(cylinder(8, 22), layer{circle{24, 24, 7.5}, green}, layer{seg{24, 20, 24, 28, 2.6}, cut}, layer{seg{20, 24, 28, 24, 2.6}, cut}),
		"upgrade":   {{circle{16, 16, 13}, green}, {seg{16, 24, 16, 14, 3.5}, cut}, {tri{9.5, 16, 22.5, 16, 16, 8}, cut}},
		"settings":  gear,
		"help":      {{circle{16, 16, 13}, blue}, {arc{16, 12.5, 5, 180, 420, 3}, cut}, {seg{18.5, 16.8, 16, 18, 3}, cut}, {seg{16, 18, 16, 19.5, 3}, cut}, {circle{16, 24, 1.9}, cut}},
		"about":     {{circle{16, 16, 13}, blue}, {circle{16, 9.5, 2.1}, cut}, {seg{16, 14, 16, 24, 3.5}, cut}},
		"diff":      {{rect{3, 4, 15, 28, 2}, red}, {seg{6.5, 16, 11.5, 16, 2.5}, cut}, {rect{17, 4, 29, 28, 2}, green}, {seg{23, 12.5, 23, 19.5, 2.5}, cut}, {seg{19.5, 16, 26.5, 16, 2.5}, cut}},
		"diffprev":  {{rect{3, 4, 15, 28, 2}, gray}, {seg{6.5, 16, 11.5, 16, 2.5}, cut}, {rect{17, 4, 29, 28, 2}, green}, {seg{23, 12.5, 23, 19.5, 2.5}, cut}, {seg{19.5, 16, 26.5, 16, 2.5}, cut}},
		"blame":     {{circle{16, 10, 6.5}, purple}, {ellipse{16, 29, 11.5, 9}, purple}},
		"lock":      append([]layer{{arc{16, 13, 6, 180, 360, 3.5}, yellow}, {seg{10, 13, 10, 16, 3.5}, yellow}, {seg{22, 13, 22, 16, 3.5}, yellow}}, lockBody(yellow)...),
		"unlock":    append([]layer{{arc{16, 8, 6, 180, 360, 3.5}, green}, {seg{22, 8, 22, 16, 3.5}, green}, {seg{10, 8, 10, 10, 3.5}, green}}, lockBody(green)...),
		"rename":    {{seg{8, 24, 23, 9, 6}, orange}, {seg{23, 9, 26, 6, 6}, hex(0xE58FA0)}, {tri{3, 29, 5, 21, 11, 27}, gray}},
		"delete":    {{seg{7, 7, 25, 25, 5}, red}, {seg{25, 7, 7, 25, 5}, red}},
		"unversion": {{seg{8, 8, 24, 24, 4}, gray}, {seg{24, 8, 8, 24, 4}, gray}, {ring{16, 16, 13, 2.5}, gray}},
	}
	return m
}

func render(layers []layer, size int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	const ss = 4
	scale := 32.0 / float64(size)
	for py := range size {
		for px := range size {
			var r, g, b, a float64
			for sy := range ss {
				for sx := range ss {
					x := (float64(px) + (float64(sx)+0.5)/ss) * scale
					y := (float64(py) + (float64(sy)+0.5)/ss) * scale
					var c color.NRGBA
					for _, l := range layers {
						if l.s.dist(x, y) <= 0 {
							c = l.c
						}
					}
					fa := float64(c.A) / 255
					r, g, b, a = r+float64(c.R)*fa, g+float64(c.G)*fa, b+float64(c.B)*fa, a+fa
				}
			}
			if a == 0 {
				continue
			}
			img.SetNRGBA(px, py, color.NRGBA{uint8(r / a), uint8(g / a), uint8(b / a), uint8(255 * a / ss / ss)})
		}
	}
	return img
}

// ico packs PNG images into one .ico, which Windows has read since Vista.
func ico(sizes []int, layers []layer) ([]byte, error) {
	var pngs [][]byte
	for _, s := range sizes {
		var buf bytes.Buffer
		if err := png.Encode(&buf, render(layers, s)); err != nil {
			return nil, err
		}
		pngs = append(pngs, buf.Bytes())
	}
	var out bytes.Buffer
	binary.Write(&out, binary.LittleEndian, [3]uint16{0, 1, uint16(len(sizes))})
	offset := 6 + 16*len(sizes)
	for i, s := range sizes {
		binary.Write(&out, binary.LittleEndian, struct {
			W, H, Colors, Reserved uint8
			Planes, Bits           uint16
			Size, Offset           uint32
		}{uint8(s), uint8(s), 0, 0, 1, 32, uint32(len(pngs[i])), uint32(offset)})
		offset += len(pngs[i])
	}
	for _, p := range pngs {
		out.Write(p)
	}
	return out.Bytes(), nil
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: menuicons <out dir>")
		os.Exit(2)
	}
	dir := os.Args[1]
	sizes := []int{16, 20, 24, 32, 48}
	for name, layers := range icons() {
		data, err := ico(sizes, layers)
		if err == nil {
			err = os.WriteFile(filepath.Join(dir, name+".ico"), data, 0o644)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, name, err)
			os.Exit(1)
		}
		// A 64 px preview for looking at the set; not shipped.
		if prev := os.Getenv("MENUICONS_PREVIEW"); prev != "" {
			f, _ := os.Create(filepath.Join(prev, name+".png"))
			png.Encode(f, render(layers, 64))
			f.Close()
		}
	}
}
