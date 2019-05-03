package imagecontainer

import (
	"bufio"
	"filter"
	"fmt"
	"image"
	"math"
	"os"
	pc "pixelcolor"
	"strings"
)

// Constants for instructions
const (
	IPixelMagnitude         = iota
	IMinimizeVerticalSeam   = iota
	IMinimizeHorizontalSeam = iota
	IRemoveRow              = iota
	IRemoveColumn           = iota
)

// ImageToProcess stores the information on an image and the filters being applied to it.
type ImageToProcess struct {
	OutputFileName         string
	CurrentImage           image.Image
	NewImage               *image.RGBA
	CumulativeMagnitude    [][]float32
	ImageCompressionBounds chan CompressionBounds
	CurrentStageComplete   chan interface{}
	InstructionsComplete   chan int
	TargetX                int
	TargetY                int
}

// GetPixelMagnitudes loops through each pixel of the padded image, gets the new pixel color by
// applying the specified filter and sets the new image's pixel to that new color.
// this returns the image of the gradient magnitudes to ensure the calculations are done correctly.
func (imageToProcess *ImageToProcess) GetPixelMagnitudes(compressionBounds CompressionBounds) image.Image {
	newImage := image.NewRGBA(image.Rect(imageToProcess.CurrentImage.Bounds().Min.X, imageToProcess.CurrentImage.Bounds().Min.X, imageToProcess.CurrentImage.Bounds().Max.X, imageToProcess.CurrentImage.Bounds().Max.Y))
	// Loop through padded image.
	for x := compressionBounds.MinX; x <= compressionBounds.MaxX; x++ {
		for y := compressionBounds.MinY; y <= compressionBounds.MaxY; y++ {
			// get the new pixel color
			xCoord, yCoord := x, y

			// This prevents striking edges caused by applying a filter that goes into the padding.
			// It assumes the last pixel is probably similar to the one before it.
			if xCoord+1 > imageToProcess.CurrentImage.Bounds().Max.X-1 {
				xCoord = imageToProcess.CurrentImage.Bounds().Max.X - 2
			}
			if yCoord+1 > imageToProcess.CurrentImage.Bounds().Max.Y-1 {
				yCoord = imageToProcess.CurrentImage.Bounds().Max.Y - 2
			}

			xGradientOfPixel := addFilterToPixel(xCoord, yCoord, filter.XGradientFilter(), imageToProcess.CurrentImage)
			yGradientOfPixel := addFilterToPixel(xCoord, yCoord, filter.YGradientFilter(), imageToProcess.CurrentImage)
			var gradientMagnitude pc.PixelColor
			imageToProcess.CumulativeMagnitude[y][x], gradientMagnitude = pc.GetGradientMagnitude(xGradientOfPixel, yGradientOfPixel)
			newImage.Set(x, y, gradientMagnitude.ToRGBA())
		}
	}
	return newImage
}

// addFilterToPixel multiplies the pixels color values through the filter and sums them up.
// It then removes negagive values and returns a pixelColor struct of the new rgba values
// for that pixel in the new image.
func addFilterToPixel(x, y int, filter [][]int32, paddedImage image.Image) pc.PixelColor {
	pixelColor := pc.PixelColor{}
	// a is preserved in the new image.
	_, _, _, a := paddedImage.At(x, y).RGBA()
	pixelColor.A = int32(a)

	// apply the filter over the 9 pixels around the target pixel.
	for sx := 0; sx < 3; sx++ {
		for sy := 0; sy < 3; sy++ {
			xCoord := x - 1 + sx
			yCoord := y - 1 + sy
			r, g, b, _ := paddedImage.At(xCoord, yCoord).RGBA()
			// Divided by 257 because the color is offset when stored as RGBA by 0x101 and this ultimate needs
			// to be 8 bits.
			pixelColor.R += int32(r/257) * filter[sx][sy]
			pixelColor.G += int32(g/257) * filter[sx][sy]
			pixelColor.B += int32(b/257) * filter[sx][sy]
		}
	}

	pixelColor.RemoveNegativeColors()
	return pixelColor
}

// MinimzeHorizontalSeam loops through each pixel of the row, gets the min cumulative gradient to a parent pixel and updates
// the value of the given cumulativeMagnitude location to sum that path value plus the magnitude of the current pixel.
func (imageToProcess *ImageToProcess) MinimzeHorizontalSeam(compressionBounds CompressionBounds) {
	// Loop through padded image.
	for x := compressionBounds.MinX; x < compressionBounds.MaxX; x++ {
		for y := compressionBounds.MinY; y <= compressionBounds.MaxY; y++ {
			minX, minY := imageToProcess.getMinMag(x-1, y-1, x-1, y, x-1, y+1)
			minParentMag := imageToProcess.CumulativeMagnitude[minY][minX]
			imageToProcess.CumulativeMagnitude[y][x] = imageToProcess.CumulativeMagnitude[y][x] + minParentMag
		}
	}
}

// MinimzeVerticalSeam loops through each pixel of the column, gets the min cumulative gradient to a parent pixel and updates
// the value of the given cumulativeMagnitude location to sum that path value plus the magnitude of the current pixel.
func (imageToProcess *ImageToProcess) MinimzeVerticalSeam(compressionBounds CompressionBounds) {
	// Loop through padded image.
	for y := compressionBounds.MinY; y < compressionBounds.MaxY; y++ {
		for x := compressionBounds.MinX; x <= compressionBounds.MaxX; x++ {
			minX, minY := imageToProcess.getMinMag(x-1, y-1, x, y-1, x+1, y-1)
			minParentMag := imageToProcess.CumulativeMagnitude[minY][minX]
			imageToProcess.CumulativeMagnitude[y][x] = imageToProcess.CumulativeMagnitude[y][x] + minParentMag
		}
	}
}

// FindMinSeam loops through the bottom row or right column to find the minimum cumulative gradient value and returns the coordinates
func (imageToProcess *ImageToProcess) FindMinSeam(compressionBounds CompressionBounds) (minSeamX, minSeamY int) {
	var minSeamValue float32 = math.MaxFloat32

	for x := compressionBounds.MinX; x <= compressionBounds.MaxX && x < imageToProcess.CurrentImage.Bounds().Max.X; x++ {
		for y := compressionBounds.MinY; y <= compressionBounds.MaxY && y < imageToProcess.CurrentImage.Bounds().Max.Y; y++ {
			if imageToProcess.CumulativeMagnitude[y][x] < minSeamValue {
				minSeamValue = imageToProcess.CumulativeMagnitude[y][x]
				minSeamX = x
				minSeamY = y
			}
		}
	}
	return minSeamX, minSeamY
}

// MarkVerticalSeam loops to continuously find the parent above with the min gradient and mark it.
func (imageToProcess *ImageToProcess) MarkVerticalSeam(x, y int) {
	imageToProcess.CumulativeMagnitude[y][x] = -1
	for y > 0 && x >= 0 {
		x, y = imageToProcess.markMinMag(x-1, y-1, x, y-1, x+1, y-1)
	}
}

// MarkHorizontalSeam loops to continuously find the parent to the left with the min gradient and mark it.
func (imageToProcess *ImageToProcess) MarkHorizontalSeam(x, y int) {
	imageToProcess.CumulativeMagnitude[y][x] = -1
	for x > 0 && y >= 0 {
		x, y = imageToProcess.markMinMag(x-1, y-1, x-1, y, x-1, y+1)
	}
}

//Checks if x and y are within bounds of CumulativeMagnitude
func (imageToProcess *ImageToProcess) coordinatesInBounds(x, y int) bool {
	return (x > -1 && y > -1 && x < cap(imageToProcess.CumulativeMagnitude[0]) && y < cap(imageToProcess.CumulativeMagnitude))
}

//MarkMinMag marks the parent with the min magnitude with -1 and returns the coordinates.
func (imageToProcess *ImageToProcess) getMinMag(x1, y1, x2, y2, x3, y3 int) (minX, minY int) {
	if imageToProcess.coordinatesInBounds(x1, y1) {
		minX, minY = x1, y1
	} else {
		minX, minY = x3, y3
	}

	if imageToProcess.coordinatesInBounds(x2, y2) && imageToProcess.CumulativeMagnitude[minY][minX] > imageToProcess.CumulativeMagnitude[y2][x2] {
		minX, minY = x2, y2
	}

	if imageToProcess.coordinatesInBounds(x3, y3) && imageToProcess.CumulativeMagnitude[minY][minX] > imageToProcess.CumulativeMagnitude[y3][x3] {
		minX, minY = x3, y3
	}
	return minX, minY
}

func (imageToProcess *ImageToProcess) markMinMag(x1, y1, x2, y2, x3, y3 int) (minX, minY int) {
	minX, minY = imageToProcess.getMinMag(x1, y1, x2, y2, x3, y3)
	imageToProcess.CumulativeMagnitude[minY][minX] = -1
	return minX, minY
}

// RemoveColumn Creates a new image with one fewer row and adds colors from the current image,
// skipping pixels marked as the seam to remove in the CumulativeMagnitude array.
func (imageToProcess *ImageToProcess) RemoveColumn(compressionBounds CompressionBounds) {
	newImageX, newImageY := compressionBounds.MinX, compressionBounds.MinY

	// Loop through current image.
	for y := compressionBounds.MinY; y < compressionBounds.MaxY; y++ {
		for x := compressionBounds.MinX; x < compressionBounds.MaxX; x++ {
			if imageToProcess.CumulativeMagnitude[y][x] > -1 {
				imageToProcess.NewImage.Set(newImageX, newImageY, imageToProcess.CurrentImage.At(x, y))
				newImageX++
			}
		}
		newImageX = 0
		newImageY++
	}
}

// RemoveRow Creates a new image with one fewer row and adds colors from the current image,
// skipping pixels marked as the seam to remove in the CumulativeMagnitude array.
func (imageToProcess *ImageToProcess) RemoveRow(compressionBounds CompressionBounds) {
	newImageX, newImageY := compressionBounds.MinX, compressionBounds.MinY
	// Loop through current image
	for x := compressionBounds.MinX; x < compressionBounds.MaxX; x++ {
		for y := compressionBounds.MinY; y < compressionBounds.MaxY; y++ {
			if imageToProcess.CumulativeMagnitude[y][x] > -1 {
				imageToProcess.NewImage.Set(newImageX, newImageY, imageToProcess.CurrentImage.At(x, y))
				newImageY++
			}
		}
		newImageY = 0
		newImageX++
	}
}

// ProcessInstruction takes a compressionBounds and executes the function identified by the instruction.
func (imageToProcess *ImageToProcess) ProcessInstruction(compressionBounds CompressionBounds) {
	switch compressionBounds.Instruction {
	case IPixelMagnitude:
		imageToProcess.GetPixelMagnitudes(compressionBounds)
	case IMinimizeVerticalSeam:
		imageToProcess.MinimzeVerticalSeam(compressionBounds)
	case IMinimizeHorizontalSeam:
		imageToProcess.MinimzeHorizontalSeam(compressionBounds)
	case IRemoveColumn:
		imageToProcess.RemoveColumn(compressionBounds)
	case IRemoveRow:
		imageToProcess.RemoveRow(compressionBounds)
	}
}

// Construct a 2d array for the cmulativeMagnitude slices
func GetCumulativeMagnitudeSlice(xBounds, yBounds int) [][]float32 {
	cumulativeMagnitude := make([][]float32, yBounds)
	for i := range cumulativeMagnitude {
		cumulativeMagnitude[i] = make([]float32, xBounds)
	}
	return cumulativeMagnitude
}

// WriteToCsv writes out the cumulative magnitude values at a given step during compression
// to a csv in order to check if paths are being correctly identified.
func (imageToProcess *ImageToProcess) WriteToCsv(path string, exit bool) {
	f, err := os.Create(path)
	fmt.Println(err)
	w := bufio.NewWriter(f)
	for _, row := range imageToProcess.CumulativeMagnitude {
		st := strings.Trim(strings.Join(strings.Fields(fmt.Sprint(row)), ", "), "[]") + "\n"
		w.WriteString(st)
	}
	w.Flush()
	f.Close()
	if exit {
		panic("CSV EXIT")
	}
}
