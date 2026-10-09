package raster_test

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"math"
	"regexp"
	"strconv"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	mermaid "github.com/zkrebbekx/go-mermaid"
	"github.com/zkrebbekx/go-mermaid/raster"
)

func TestPNG(t *testing.T) {
	Convey("Given diagram source", t, func() {
		Convey("When rendering to PNG", func() {
			out, err := raster.PNG("graph TD\nA[Start] --> B[End]", 1)

			Convey("Then it returns a decodable PNG with positive size", func() {
				So(err, ShouldBeNil)
				So(out[:4], ShouldResemble, []byte{0x89, 'P', 'N', 'G'})
				img, derr := png.Decode(bytes.NewReader(out))
				So(derr, ShouldBeNil)
				So(img.Bounds().Dx(), ShouldBeGreaterThan, 0)
				So(img.Bounds().Dy(), ShouldBeGreaterThan, 0)
			})
		})

		Convey("When scaling 2x", func() {
			one, _ := raster.PNG("graph TD\nA --> B", 1)
			two, _ := raster.PNG("graph TD\nA --> B", 2)
			i1, _ := png.Decode(bytes.NewReader(one))
			i2, _ := png.Decode(bytes.NewReader(two))

			Convey("Then the 2x image is larger", func() {
				So(i2.Bounds().Dx(), ShouldBeGreaterThan, i1.Bounds().Dx())
			})
		})

		Convey("When the source is invalid", func() {
			_, err := raster.PNG("graph TD\nA[oops", 1)

			Convey("Then it propagates the render error", func() {
				So(err, ShouldNotBeNil)
			})
		})

		Convey("When passing options through", func() {
			out, err := raster.PNG("graph TD\nA --> B", 1, mermaid.WithTheme(mermaid.Dark))

			Convey("Then it still produces a valid PNG", func() {
				So(err, ShouldBeNil)
				_, derr := png.Decode(bytes.NewReader(out))
				So(derr, ShouldBeNil)
			})
		})
	})
}

// alpha returns the opacity of the pixel at p. The page itself is clear,
// because oksvg skips the percentage-sized background rect.
func alpha(img image.Image, p image.Point) uint32 {
	_, _, _, a := img.At(p.X, p.Y).RGBA()
	return a >> 8
}

func TestRoundedCorners(t *testing.T) {
	Convey("Given a hand-written SVG with corner radii", t, func() {
		svg := `<svg xmlns="http://www.w3.org/2000/svg" width="300" height="60" viewBox="0 0 300 60">` +
			`<rect x="10" y="10" width="80" height="40" rx="20" fill="#ff0000"/>` +
			`<rect x="110" y="10" width="80" height="40" ry="20" fill="#ff0000"/>` +
			`<rect x="210" y="10" width="80" height="40" fill="#ff0000"/>` +
			`</svg>`

		Convey("When rasterizing it", func() {
			out, err := raster.RasterizeSVG([]byte(svg), 1)
			So(err, ShouldBeNil)
			img, err := png.Decode(bytes.NewReader(out))
			So(err, ShouldBeNil)

			Convey("Then a lone rx or ry rounds the corners", func() {
				So(alpha(img, image.Pt(11, 11)), ShouldBeLessThan, 0x80)
				So(alpha(img, image.Pt(111, 11)), ShouldBeLessThan, 0x80)
				So(alpha(img, image.Pt(50, 30)), ShouldEqual, 0xff)
			})

			Convey("Then a rect without radii keeps square corners", func() {
				So(alpha(img, image.Pt(211, 11)), ShouldEqual, 0xff)
			})
		})
	})

	Convey("Given stadium, round and rect nodes", t, func() {
		const src = "flowchart LR\nA([stadium]) --> B(round) --> C[rect]"
		const scale = 2
		svg, err := mermaid.Render(src)
		So(err, ShouldBeNil)
		corners := nodeCorners(string(svg), scale)
		So(len(corners), ShouldEqual, 3)

		Convey("When rendering to PNG at scale 2", func() {
			out, err := raster.PNG(src, scale)
			So(err, ShouldBeNil)
			img, err := png.Decode(bytes.NewReader(out))
			So(err, ShouldBeNil)

			Convey("Then the stadium and round corners are rounded", func() {
				So(alpha(img, corners[0]), ShouldBeLessThan, 0x80)
				So(alpha(img, corners[1]), ShouldBeLessThan, 0x80)
			})

			Convey("Then the rect corner is square", func() {
				So(alpha(img, corners[2]), ShouldEqual, 0xff)
			})
		})
	})
}

var (
	translateRe = regexp.MustCompile(`<g transform="translate\(([0-9.]+),([0-9.]+)\)">`)
	nodeRectRe  = regexp.MustCompile(`<rect x="([0-9.]+)" y="([0-9.]+)"[^>]*fill="#ECECFF"`)
)

// nodeCorners returns, for each node box in document order, the output pixel
// just inside its top-left corner at the given scale.
func nodeCorners(svg string, scale float64) []image.Point {
	var gx, gy float64
	if m := translateRe.FindStringSubmatch(svg); m != nil {
		gx, _ = strconv.ParseFloat(m[1], 64)
		gy, _ = strconv.ParseFloat(m[2], 64)
	}
	px := func(v float64) int { return int(math.Floor(v*scale)) + 1 }
	var out []image.Point
	for _, m := range nodeRectRe.FindAllStringSubmatch(svg, -1) {
		x, _ := strconv.ParseFloat(m[1], 64)
		y, _ := strconv.ParseFloat(m[2], 64)
		out = append(out, image.Pt(px(x+gx), px(y+gy)))
	}
	return out
}

func TestLinkHeadMarkers(t *testing.T) {
	Convey("Given lines ending in each marker", t, func() {
		line := func(y int, start, end string) string {
			return fmt.Sprintf(`<path d="M10,%d L90,%d" stroke="#000000" stroke-width="1.5" marker-start="url(#%s)" marker-end="url(#%s)"/>`, y, y, start, end)
		}
		svg := `<svg xmlns="http://www.w3.org/2000/svg" width="100" height="60" viewBox="0 0 100 60">` +
			line(10, "circle", "circle") + line(30, "cross", "cross") + line(50, "arrow", "arrow") +
			`</svg>`
		// Curved edges repeat each end point as a control point.
		curved := `<svg xmlns="http://www.w3.org/2000/svg" width="100" height="20" viewBox="0 0 100 20">` +
			`<path d="M10,10 Q10,10 50,10 Q90,10 90,10" stroke="#000000" stroke-width="1.5" marker-start="url(#circle)" marker-end="url(#circle)"/>` +
			`</svg>`

		Convey("When rasterizing it", func() {
			out, err := raster.RasterizeSVG([]byte(svg), 1)
			So(err, ShouldBeNil)
			img, err := png.Decode(bytes.NewReader(out))
			So(err, ShouldBeNil)

			Convey("Then a circle is drawn 4 units back from both ends", func() {
				So(opacityAt(img, image.Pt(85, 13)), ShouldEqual, 0xff)
				So(opacityAt(img, image.Pt(14, 7)), ShouldEqual, 0xff)
			})

			Convey("Then a cross's arms reach off the line at both ends", func() {
				So(opacityAt(img, image.Pt(83, 33)), ShouldBeGreaterThan, 0x80)
				So(opacityAt(img, image.Pt(16, 27)), ShouldBeGreaterThan, 0x80)
			})

			Convey("Then a curved path gets markers at both ends", func() {
				out, err := raster.RasterizeSVG([]byte(curved), 1)
				So(err, ShouldBeNil)
				img, err := png.Decode(bytes.NewReader(out))
				So(err, ShouldBeNil)
				So(opacityAt(img, image.Pt(85, 13)), ShouldEqual, 0xff)
				So(opacityAt(img, image.Pt(14, 13)), ShouldEqual, 0xff)
			})

			Convey("Then an arrowhead fills the line near both tips", func() {
				So(opacityAt(img, image.Pt(84, 51)), ShouldEqual, 0xff)
				So(opacityAt(img, image.Pt(15, 48)), ShouldEqual, 0xff)
			})
		})
	})
}

// opacityAt returns the alpha of the pixel at p; the page itself is clear.
func opacityAt(img image.Image, p image.Point) uint32 {
	_, _, _, a := img.At(p.X, p.Y).RGBA()
	return a >> 8
}

// coverage sums the opacity down column x from y0 to y1, in pixels: a line
// of width w crossing the column scores about w however it is antialiased.
func coverage(img image.Image, x, y0, y1 int) float64 {
	var sum float64
	for y := y0; y <= y1; y++ {
		_, _, _, a := img.At(x, y).RGBA()
		sum += float64(a) / 0xffff
	}
	return sum
}

// runs returns the lengths of the alternating opaque and clear stretches
// along row y, from x0 to x1, starting at the first opaque pixel.
func runs(img image.Image, y, x0, x1 int) []int {
	var out []int
	for x := x0; x <= x1; x++ {
		_, _, _, a := img.At(x, y).RGBA()
		on := a > 0x8000
		// Odd-length out means the last run is an opaque one.
		lastOn := len(out)%2 == 1
		switch {
		case len(out) == 0 && !on:
		case on == lastOn:
			out[len(out)-1]++
		default:
			out = append(out, 1)
		}
	}
	return out
}

func TestStrokesScale(t *testing.T) {
	Convey("Given a 2-unit line, a dashed line and a line with no width", t, func() {
		svg := `<svg xmlns="http://www.w3.org/2000/svg" width="100" height="40" viewBox="0 0 100 40">` +
			`<path d="M0,10 L100,10" stroke="#000000" stroke-width="2"/>` +
			`<path d="M2,30 L98,30" stroke="#000000" stroke-width="2" stroke-dasharray="4,4"/>` +
			`<path d="M0,20 L100,20" stroke="#000000"/>` +
			`</svg>`

		for _, scale := range []float64{1, 2, 4} {
			Convey(fmt.Sprintf("When rasterizing at scale %g", scale), func() {
				out, err := raster.RasterizeSVG([]byte(svg), scale)
				So(err, ShouldBeNil)
				img, err := png.Decode(bytes.NewReader(out))
				So(err, ShouldBeNil)
				px := func(v float64) int { return int(v * scale) }

				Convey("Then the line is 2 units wide in output pixels", func() {
					So(coverage(img, px(50), px(5), px(15)), ShouldAlmostEqual, 2*scale, 0.5)
				})

				Convey("Then a line with no width uses SVG's default of 1 unit", func() {
					So(coverage(img, px(50), px(16), px(24)), ShouldAlmostEqual, 1*scale, 0.5)
				})

				Convey("Then the dashes are 4 units on and 4 off", func() {
					r := runs(img, px(30), px(2), px(98))
					So(len(r), ShouldBeGreaterThan, 4)
					for _, n := range r[:4] {
						So(float64(n), ShouldAlmostEqual, 4*scale, 1)
					}
				})
			})
		}
	})
}
