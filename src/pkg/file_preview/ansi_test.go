package filepreview

import (
	"fmt"
	"image"
	"image/color"
	"strings"
	"testing"

	"github.com/muesli/termenv"
)

// Keep the independent per-cell style implementation as an output oracle for
// the direct encoder, including its transparent-background escape sequence.
func referenceImageToANSI(img image.Image, background color.Color) string {
	var output strings.Builder
	backgroundHex := colorToHex(background)
	convert := func(pixel color.Color, fallback string) termenv.RGBColor {
		rgba, ok := color.RGBAModel.Convert(pixel).(color.RGBA)
		if !ok || rgba.A == 0 {
			return termenv.RGBColor(fallback)
		}
		return termenv.RGBColor(fmt.Sprintf("#%02x%02x%02x", rgba.R, rgba.G, rgba.B))
	}
	for y := 0; y < img.Bounds().Dy(); y += 2 {
		for x := range img.Bounds().Dx() {
			upper := convert(img.At(x, y), backgroundHex)
			lower := convert(background, "")
			if y+1 < img.Bounds().Dy() {
				lower = convert(img.At(x, y+1), backgroundHex)
			}
			output.WriteString(termenv.String("▄").Foreground(lower).Background(upper).String())
		}
		if y+2 < img.Bounds().Dy() {
			output.WriteByte('\n')
		}
	}
	return output.String()
}

func TestANSIEncoderMatchesStyledCells(t *testing.T) {
	for _, noColor := range []string{"", "1"} {
		t.Setenv("NO_COLOR", noColor)
		for _, dimensions := range []image.Point{{}, {X: 0, Y: 3}, {X: 3, Y: 0}, {X: 1, Y: 1}, {X: 2, Y: 2}, {X: 3, Y: 5}, {X: 79, Y: 53}} {
			bounds := image.Rectangle{Max: dimensions}
			for _, img := range []image.Image{newANSIReferenceImage(bounds, false), newANSIReferenceImage(bounds, true),
				image.NewGray(bounds), image.NewRGBA64(bounds), image.NewPaletted(bounds, color.Palette{color.Transparent})} {
				for _, background := range []color.Color{color.Black, color.White, color.RGBA{R: 127, G: 63, B: 211, A: 255},
					color.Transparent, color.NRGBA{R: 255, G: 129, B: 22, A: 77}} {
					want := referenceImageToANSI(img, background)
					got := ConvertImageToANSI(img, background)
					if got != want {
						t.Fatalf("encoding differs for image=%T bounds=%v background=%v NO_COLOR=%q",
							img, bounds, background, noColor)
					}
				}
			}
		}
	}
}

func TestANSIEncoderAllRGBChannels(t *testing.T) {
	img := image.NewRGBA64(image.Rect(0, 0, 256, 2))
	for x := range 256 {
		img.SetRGBA64(x, 0, color.RGBA64{R: uint16(x * 257), G: uint16((255 - x) * 257), B: 65535, A: 65535})
		// Below one eight-bit alpha unit, the existing encoder treats pixels
		// as fully transparent even when their sixteen-bit alpha is nonzero.
		img.SetRGBA64(x, 1, color.RGBA64{R: 255, G: 255, B: 255, A: uint16(x)})
	}
	for _, background := range []color.Color{color.Black, color.RGBA64{R: 65535, G: 32768, B: 16384, A: 1}} {
		if got, want := ConvertImageToANSI(img, background), referenceImageToANSI(img, background); got != want {
			t.Fatal("RGB channel rounding or sixteen-bit transparency changed")
		}
	}
}

func TestANSIEncoderRespectsImageOrigin(t *testing.T) {
	parent := newANSIReferenceImage(image.Rect(0, 0, 10, 10), false).(*image.RGBA)
	cropped := parent.SubImage(image.Rect(3, 2, 8, 7))
	normalized := image.NewRGBA(image.Rect(0, 0, 5, 5))
	for y := range 5 {
		for x := range 5 {
			normalized.Set(x, y, parent.At(3+x, 2+y))
		}
	}
	got := ConvertImageToANSI(cropped, color.Black)
	if want := referenceImageToANSI(normalized, color.Black); got != want {
		t.Fatal("subimage rendered pixels outside its bounds")
	}
}

func newANSIReferenceImage(bounds image.Rectangle, unpremultiplied bool) image.Image {
	if unpremultiplied {
		img := image.NewNRGBA(bounds)
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				img.SetNRGBA(x, y, color.NRGBA{
					R: byte(x * 29), G: byte(y * 37), B: byte(x*y + 17), A: byte((x + y) * 11),
				})
			}
		}
		return img
	}
	img := image.NewRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			img.SetRGBA(x, y, color.RGBA{R: byte(x * 29), G: byte(y * 37), B: byte(x*y + 17), A: 255})
		}
	}
	return img
}

func BenchmarkConvertImageToANSI(b *testing.B) {
	for _, kind := range []string{"flat", "gradient"} {
		b.Run(kind, func(b *testing.B) {
			img := newANSIReferenceImage(image.Rect(0, 0, 120, 80), false).(*image.RGBA)
			if kind == "flat" {
				for y := range 80 {
					for x := range 120 {
						img.SetRGBA(x, y, color.RGBA{R: 100, G: 150, B: 200, A: 255})
					}
				}
			}
			b.ReportAllocs()
			for b.Loop() {
				if ConvertImageToANSI(img, color.Black) == "" {
					b.Fatal("empty image encoding")
				}
			}
		})
	}
}

func BenchmarkANSIRenderer(b *testing.B) {
	img := newANSIReferenceImage(image.Rect(0, 0, 640, 480), false)
	p := &ImagePreviewer{}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := p.ANSIRenderer(img, "#000000", 120, 40); err != nil {
			b.Fatal(err)
		}
	}
}
