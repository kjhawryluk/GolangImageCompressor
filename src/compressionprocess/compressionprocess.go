package compressionprocess

import (
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	r "regexp"
	"strconv"
	s "strings"
)

//Note: I used https://www.devdungeon.com/content/working-images-go
// and https://golang.org/pkg/image/png/ to help understand how to encode/decode images.

// getImageForFIltering opens an image and adds the necessary padding.
func getImageForFiltering(pathName string) (image.Image, error) {
	path, _ := filepath.Abs(pathName)
	// Read image from file that already exists
	existingImageFile, err := os.Open(path)
	defer existingImageFile.Close()

	// Check that image could be opened.
	if err != nil {
		fmt.Println(err)
		return nil, errors.New("Could Not Find Image")
	}

	// Try to decode image.
	loadedImage, err := png.Decode(existingImageFile)
	if err != nil {
		fmt.Println(err)
		return nil, errors.New("Could Not Decode Image")
	}

	// Adding padding to image.
	return loadedImage, nil
}

//ouputImage saves an image to the designated output path.
func outputImage(imageOutPath string, currentImage image.Image) {
	if imageOutPath == "" {
		return
	}
	// outputFile is a File type which satisfies Writer interface
	outputFile, err := os.Create(imageOutPath)
	defer outputFile.Close()
	if err != nil {
		fmt.Println("Output Error:", err, imageOutPath)
		return
	}
	// Encode image to png and write to file.
	png.Encode(outputFile, currentImage)
}

// splitLine reads in a line and makes sure that it has an input line, output line
// and at least one filter
func splitLine(line string) (lineData []string, err error) {
	re := r.MustCompile(`\s+`)
	line = re.ReplaceAllString(line, "")
	//Trying to catch bad lines.
	if line == "" {
		return lineData, errors.New("Empty Line")
	}

	//Split line to grab date.
	lineData = s.Split(line, ",")

	if len(lineData) < 2 {
		return lineData, errors.New("No Image In/Out Info" + line)
	}
	if len(lineData) < 4 {
		return lineData, errors.New("Missing Desired Dimensions" + line)
	}
	return lineData, err
}

// This gets the target dimensions bases on used input or target scale factors.
func getTargetDimensions(imageInPath, scaleRateX, scaleRateY string, currentImage image.Image) (targetX, targetY int, err error) {
	scaleFactorX, xErr := strconv.ParseFloat(scaleRateX, 64)
	scaleFactorY, yErr := strconv.ParseFloat(scaleRateY, 64)
	targetX = int(float64(currentImage.Bounds().Max.X) * scaleFactorX)
	targetY = int(float64(currentImage.Bounds().Max.Y) * scaleFactorY)
	if xErr != nil || yErr != nil || targetX > currentImage.Bounds().Max.X || targetY > currentImage.Bounds().Max.Y {
		fmt.Println(imageInPath, "- Invalid Scalng Rate:", scaleFactorX, scaleFactorY)
		err = errors.New("Invalid Target Dimensions.")
	}
	return targetX, targetY, err
}
