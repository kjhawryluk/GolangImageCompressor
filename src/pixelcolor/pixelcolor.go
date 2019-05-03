package pixelcolor

import (
	"image/color"
	"math"
)

// PixelColor tores rgba values for filtering.
type PixelColor struct {
	R, G, B, A int32
}

// RemoveNegativeColors changes negative colors to 0.
func (pixelColor *PixelColor) RemoveNegativeColors() {
	if pixelColor.R < 0 {
		pixelColor.R = 0
	}
	if pixelColor.B < 0 {
		pixelColor.B = 0
	}
	if pixelColor.G < 0 {
		pixelColor.G = 0
	}
}

// ToRGBA creates a color.RGBA from the values in a PixelColor.
func (pixelColor *PixelColor) ToRGBA() color.RGBA {
	return color.RGBA{uint8(pixelColor.R), uint8(pixelColor.G), uint8(pixelColor.B), uint8(pixelColor.A)}
}

// GetGradientMagnitude returns a pixel color for gradient magnitude from the x and y gradient.
func GetGradientMagnitude(xGradient PixelColor, yGradient PixelColor) (float32, PixelColor) {
	r := float32(math.Sqrt(math.Pow(float64(xGradient.R), 2) + math.Pow(float64(yGradient.R), 2)))
	g := float32(math.Sqrt(math.Pow(float64(xGradient.G), 2) + math.Pow(float64(yGradient.G), 2)))
	b := float32(math.Sqrt(math.Pow(float64(xGradient.B), 2) + math.Pow(float64(yGradient.B), 2)))

	return r + g + b, PixelColor{
		R: int32(r),
		G: int32(g),
		B: int32(b),
		A: xGradient.A}
}
