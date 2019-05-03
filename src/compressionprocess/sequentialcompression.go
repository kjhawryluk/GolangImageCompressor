package compressionprocess

import (
	"bufio"
	"fmt"
	"image"
	ic "imagecontainer"
	"io"
	"os"
	"path/filepath"
	r "regexp"
)

// Takes the line input and applies the appropriate commands to the image.
func processLine(imageInPath, imageOutPath, scaleRateX, scaleRateY string) {
	currentImage, err := getImageForFiltering(imageInPath)
	if err != nil {
		fmt.Println(err)
		return
	}
	newX, newY, err := getTargetDimensions(imageInPath, scaleRateX, scaleRateY, currentImage)
	if err != nil {
		return
	}
	// Process until hit target dimensions
	for newY < currentImage.Bounds().Max.Y || newX < currentImage.Bounds().Max.X {
		if newY < currentImage.Bounds().Max.Y {
			currentImage = seqRemoveHorizontalSeam(currentImage)
		}
		if newX < currentImage.Bounds().Max.X {
			currentImage = seqRemoveVerticalSeam(currentImage)
		}
	}

	outputImage(imageOutPath, currentImage)
}

// seqRemoveVerticalSeam identifies a vertcal seam in the image with the minmial gradient magnitude and then returns a new image with
// one less column that doesnt have those pixels.
func seqRemoveVerticalSeam(currentImage image.Image) image.Image {
	compressionBounds := ic.CompressionBounds{MinX: 0, MaxX: currentImage.Bounds().Max.X - 1, MinY: 0, MaxY: currentImage.Bounds().Max.Y - 1}
	LastRowBounds := ic.CompressionBounds{MinY: currentImage.Bounds().Max.Y - 1, MaxX: currentImage.Bounds().Max.X - 1, MaxY: currentImage.Bounds().Max.Y - 1}
	imageToProcess := ic.ImageToProcess{
		CurrentImage:        currentImage,
		CumulativeMagnitude: ic.GetCumulativeMagnitudeSlice(currentImage.Bounds().Max.X, currentImage.Bounds().Max.Y)}
	imageToProcess.GetPixelMagnitudes(compressionBounds)

	//Update CumulativeMagnitudes to show the vertical paths that minimize cumulative gradient magnitude
	compressionBounds.MinY = 1
	imageToProcess.MinimzeVerticalSeam(compressionBounds)

	//Find the best seam to remove and mark it for removal.
	minX, minY := imageToProcess.FindMinSeam(LastRowBounds)
	imageToProcess.MarkVerticalSeam(minX, minY)

	//Remove the column.
	imageToProcess.NewImage = image.NewRGBA(image.Rect(0, 0, imageToProcess.CurrentImage.Bounds().Max.X-1, imageToProcess.CurrentImage.Bounds().Max.Y))
	compressionBounds = ic.CompressionBounds{MinX: 0, MaxX: currentImage.Bounds().Max.X, MinY: 0, MaxY: currentImage.Bounds().Max.Y}
	imageToProcess.RemoveColumn(compressionBounds)
	return imageToProcess.NewImage
}

// seqRemoveHorizontalSeam identifies a horizontal seam in the image with the minmial gradient magnitude and then returns a new image with
// one less row that doesnt have those pixels.
func seqRemoveHorizontalSeam(currentImage image.Image) image.Image {
	compressionBounds := ic.CompressionBounds{MinX: 0, MaxX: currentImage.Bounds().Max.X - 1, MinY: 0, MaxY: currentImage.Bounds().Max.Y - 1}
	LastColumnBounds := ic.CompressionBounds{MinX: currentImage.Bounds().Max.X - 1, MaxX: currentImage.Bounds().Max.X - 1, MinY: 0, MaxY: currentImage.Bounds().Max.Y - 1}

	imageToProcess := ic.ImageToProcess{
		CurrentImage:        currentImage,
		CumulativeMagnitude: ic.GetCumulativeMagnitudeSlice(currentImage.Bounds().Max.X, currentImage.Bounds().Max.Y)}
	imageToProcess.GetPixelMagnitudes(compressionBounds)

	//Update CumulativeMagnitudes to show the horizontal paths that minimize cumulative gradient magnitude
	compressionBounds.MinX = 1
	imageToProcess.MinimzeHorizontalSeam(compressionBounds)

	//Find the best seam to remove and mark it for removal.
	minX, minY := imageToProcess.FindMinSeam(LastColumnBounds)
	imageToProcess.MarkHorizontalSeam(minX, minY)

	//Remove the row.
	imageToProcess.NewImage = image.NewRGBA(image.Rect(0, 0, imageToProcess.CurrentImage.Bounds().Max.X, imageToProcess.CurrentImage.Bounds().Max.Y-1))
	compressionBounds = ic.CompressionBounds{MinX: 0, MaxX: currentImage.Bounds().Max.X, MinY: 0, MaxY: currentImage.Bounds().Max.Y}
	imageToProcess.RemoveRow(compressionBounds)
	return imageToProcess.NewImage
}

// LaunchSeqApplication reads a file and processes the filter commands
func LaunchSeqApplication(fileName string) {
	//Try to open the file.
	re := r.MustCompile(`\s+`)
	path, _ := filepath.Abs(fileName)
	file, err := os.Open(path)
	dir := filepath.Dir(path)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	//Try to create a buffered reader.
	reader := bufio.NewReader(file)

	for true {
		//Stop looping when there are no more lines left.
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			break
		}
		line = re.ReplaceAllString(line, "")
		lineValues, parseErr := splitLine(line)
		if parseErr != nil {
			fmt.Println(parseErr)
			break
		}

		// If there's an input/output location, do the work.
		if lineValues[0] != "" && lineValues[1] != "" {
			processLine(dir+"/"+lineValues[0], dir+"/"+lineValues[1], lineValues[2], lineValues[3])
		}
		if err == io.EOF {
			break
		}
	}
}
