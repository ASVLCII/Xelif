package filepreview

import (
	"fmt"
	"image"
	"image/color"
	"strconv"
	"strings"
)

// Two RGB SGR colors, one half-block rune, and a reset sequence.
const (
	maxANSICellBytes = 43
	ansiPixelsPerRow = 2
)

// Formatting an RGB byte through hex and termenv for every pixel involves
// several temporary strings. Each channel has only 256 possible values.
//
//nolint:gochecknoglobals // Immutable lookup table shared by image encoders.
var ansiColorByte = func() [256]string {
	var values [256]string
	for value := range values {
		// Preserve termenv's hex -> float RGB -> byte rounding, including the
		// handful of channels that round down by one during that conversion.
		normalized := float64(value) * (1.0 / alphaOpaque)
		values[value] = strconv.Itoa(int(uint8(normalized * alphaOpaque)))
	}
	return values
}()

// ConvertImageToANSI converts an image to ANSI escape codes with proper aspect ratio
func ConvertImageToANSI(img image.Image, defaultBGColor color.Color) string {
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()

	// TODO: Use renderer here to prevent newline management,and overflows
	var output strings.Builder
	rows := height/ansiPixelsPerRow + height%ansiPixelsPerRow
	maxInt := int(^uint(0) >> 1)
	if width > 0 && rows > 0 && width <= (maxInt-rows+1)/rows/maxANSICellBytes {
		// Bounds avoid overflowing the estimate for arbitrary image.Image
		// implementations while normal previews allocate their output once.
		output.Grow(width*rows*maxANSICellBytes + rows - 1)
	}
	defaultR, defaultG, defaultB, defaultA := defaultBGColor.RGBA()
	fallback := color.RGBA{R: uint8((defaultR >> rgbShift8) & rgbMask), G: uint8((defaultG >> rgbShift8) & rgbMask),
		B: uint8((defaultB >> rgbShift8) & rgbMask), A: alphaOpaque}

	for y := 0; y < height; y += 2 {
		for x := range width {
			upperColor := ansiPixelColor(img, bounds.Min.X+x, bounds.Min.Y+y, fallback)
			lowerColor := fallback
			lowerPresent := defaultA>>rgbShift8 != 0

			if y+1 < height {
				lowerColor = ansiPixelColor(img, bounds.Min.X+x, bounds.Min.Y+y+1, fallback)
				lowerPresent = true
			}

			// Using the "▄" character which fills the lower half
			output.WriteString("\x1b[")
			if lowerPresent {
				writeANSIColor(&output, "38;2;", lowerColor)
			}
			output.WriteByte(';')
			writeANSIColor(&output, "48;2;", upperColor)
			output.WriteString("m▄\x1b[0m")
		}
		// Only add newline if this is not the last row
		if y+2 < height {
			output.WriteByte('\n')
		}
	}

	return output.String()
}

func ansiPixelColor(img image.Image, x, y int, fallback color.RGBA) color.RGBA {
	var r, g, b, a uint32
	switch pixels := img.(type) {
	case *image.RGBA:
		pixel := pixels.RGBAAt(x, y)
		if pixel.A == 0 {
			return fallback
		}
		return pixel
	case *image.NRGBA:
		r, g, b, a = pixels.NRGBAAt(x, y).RGBA()
	default:
		r, g, b, a = img.At(x, y).RGBA()
	}
	if a>>rgbShift8 == 0 {
		return fallback
	}
	return color.RGBA{R: uint8((r >> rgbShift8) & rgbMask), G: uint8((g >> rgbShift8) & rgbMask),
		B: uint8((b >> rgbShift8) & rgbMask), A: alphaOpaque}
}

func writeANSIColor(output *strings.Builder, prefix string, col color.RGBA) {
	output.WriteString(prefix)
	output.WriteString(ansiColorByte[col.R])
	output.WriteByte(';')
	output.WriteString(ansiColorByte[col.G])
	output.WriteByte(';')
	output.WriteString(ansiColorByte[col.B])
}

// Convert image to ansi
func (p *ImagePreviewer) ANSIRenderer(img image.Image, defaultBGColor string,
	maxWidth int, maxHeight int) (string, error) {
	bgColor, err := hexToColor(defaultBGColor)
	if err != nil {
		return "", fmt.Errorf("invalid background color: %w", err)
	}

	// For ANSI rendering, resize image appropriately
	fittedImg := resizeForANSI(img, maxWidth, maxHeight)
	return ConvertImageToANSI(fittedImg, bgColor), nil
}
