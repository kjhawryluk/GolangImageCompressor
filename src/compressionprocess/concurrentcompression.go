package compressionprocess

import (
	"bufio"
	"fmt"
	"image"
	ic "imagecontainer"
	"io"
	"os"
	"path/filepath"
)

// imageProcessContext stores the channels and information needed to sync between threads.
type imageProcessContext struct {
	inputFileName               string
	queueManagementComplete     chan interface{}
	currentImageToProcess       *ic.ImageToProcess
	numberOfWorkerThreads       int
	imagesForOutput             chan ic.ImageToProcess
	compressionBoundsToProcesss chan ic.CompressionBounds
	outputCompleted             chan int
	lastImageOutput             chan interface{}
}

// getImageCompressionFileReader takes a path, opens it and returns a reader.
func (ctx *imageProcessContext) getImageCompressionFileReader() (reader *bufio.Reader, file *os.File, dir string) {
	path, _ := filepath.Abs(ctx.inputFileName)
	file, err := os.Open(path)
	dir = filepath.Dir(path)
	if err != nil {
		panic(err)
	}
	//Try to create a buffered reader.
	reader = bufio.NewReader(file)
	return reader, file, dir
}

// getImageToProcess opens up an image, and if there's no errors, it will create an ImageToProcess
// container, add the filters and return it for processing.
func getImageToProcess(inputPath, outputPath, scaleRateX, scaleRateY string) *ic.ImageToProcess {
	currentImage, err := getImageForFiltering(inputPath)
	if err != nil {
		fmt.Println("Cannot Get Image:", err)
		return nil
	}

	newX, newY, err := getTargetDimensions(inputPath, scaleRateX, scaleRateY, currentImage)
	if err != nil {
		return nil
	}
	// Enqueue for filtering.
	ImageToProcess := ic.ImageToProcess{
		OutputFileName: outputPath,
		CurrentImage:   currentImage,
		TargetX:        newX,
		TargetY:        newY}

	return &ImageToProcess
}

// There should be only one thread that manages the filters. The rest help with applying the filters
// and outputing the images.
func (ctx *imageProcessContext) launchProcessingThreads() {
	for x := 0; x < ctx.numberOfWorkerThreads-1; x++ {
		go ctx.processQueue()
	}
}

// enqueueVerticalCompressionBounds breaks the image up into vertical sections according the number of threads
// that are processing the image. It then creates a series of CompressionBounds according to those sections and
// adds them to the queue for threadsto process. It aso adds FilterInstructionsComplete counters so that we can
// know when the last section has been processed.
func (ctx *imageProcessContext) enqueueVerticalCompressionBounds(compressionBoundsToDivide ic.CompressionBounds) {
	xDivision := compressionBoundsToDivide.MaxX / ctx.numberOfWorkerThreads
	ctx.currentImageToProcess.CurrentStageComplete = make(chan interface{})
	ctx.currentImageToProcess.InstructionsComplete = make(chan int, ctx.numberOfWorkerThreads)
	ctx.currentImageToProcess.ImageCompressionBounds = make(chan ic.CompressionBounds, ctx.numberOfWorkerThreads)
	for thread := 0; thread < ctx.numberOfWorkerThreads; thread++ {
		// This tracks the number of data divisions. When a channel receives 0 from this,
		// the filter stage is complete.
		ctx.currentImageToProcess.InstructionsComplete <- ctx.numberOfWorkerThreads - thread - 1

		// In case number of threads doesn't evenly divide into the boundaries
		var maxX int
		if thread < ctx.numberOfWorkerThreads-1 {
			maxX = thread*xDivision + xDivision
		} else {
			maxX = compressionBoundsToDivide.MaxX
		}

		// Add the bounds to the channel
		ctx.currentImageToProcess.ImageCompressionBounds <- ic.CompressionBounds{
			MinX:        compressionBoundsToDivide.MinX + thread*xDivision,
			MinY:        compressionBoundsToDivide.MinY,
			MaxX:        maxX,
			MaxY:        compressionBoundsToDivide.MaxY,
			Instruction: compressionBoundsToDivide.Instruction}
	}
	close(ctx.currentImageToProcess.InstructionsComplete)
}

// enqueueHorizontalCompressionBounds breaks the image up into horizontal sections according the number of threads
// that are processing the image. It then creates a series of CompressionBounds according to those sections and
// adds them to the queue for threadsto process. It aso adds FilterInstructionsComplete counters so that we can
// know when the last section has been processed. This is only used for removing columns, as the process requires full
// control over a given row.
func (ctx *imageProcessContext) enqueueHorizontalCompressionBounds(compressionBoundsToDivide ic.CompressionBounds) {
	yDivision := compressionBoundsToDivide.MaxY / ctx.numberOfWorkerThreads
	ctx.currentImageToProcess.CurrentStageComplete = make(chan interface{})
	ctx.currentImageToProcess.InstructionsComplete = make(chan int, ctx.numberOfWorkerThreads)
	ctx.currentImageToProcess.ImageCompressionBounds = make(chan ic.CompressionBounds, ctx.numberOfWorkerThreads)
	for thread := 0; thread < ctx.numberOfWorkerThreads; thread++ {
		// This tracks the number of data divisions. When a channel receives 0 from this,
		// the filter stage is complete.
		ctx.currentImageToProcess.InstructionsComplete <- ctx.numberOfWorkerThreads - thread - 1

		// In case number of threads doesn't evenly divide into the boundaries
		var maxY int
		if thread < ctx.numberOfWorkerThreads-1 {
			maxY = thread*yDivision + yDivision
		} else {
			maxY = compressionBoundsToDivide.MaxY
		}

		// Add the bounds to the channel
		ctx.currentImageToProcess.ImageCompressionBounds <- ic.CompressionBounds{
			MinX:        compressionBoundsToDivide.MinX,
			MinY:        compressionBoundsToDivide.MinY + thread*yDivision,
			MaxX:        compressionBoundsToDivide.MaxX,
			MaxY:        maxY,
			Instruction: compressionBoundsToDivide.Instruction}
	}
	close(ctx.currentImageToProcess.InstructionsComplete)
}

func (ctx *imageProcessContext) mangeImageCompression(inputDone bool) {
	// Process all filters for the image.
	// Process until hit target dimensions
	for ctx.currentImageToProcess.TargetY < ctx.currentImageToProcess.CurrentImage.Bounds().Max.Y || ctx.currentImageToProcess.TargetX < ctx.currentImageToProcess.CurrentImage.Bounds().Max.X {
		if ctx.currentImageToProcess.TargetY < ctx.currentImageToProcess.CurrentImage.Bounds().Max.Y {
			ctx.currentImageToProcess.CurrentImage = ctx.conRemoveHorizontalSeam()
		}

		if ctx.currentImageToProcess.TargetX < ctx.currentImageToProcess.CurrentImage.Bounds().Max.X {
			ctx.currentImageToProcess.CurrentImage = ctx.conRemoveVerticalSeam()
		}
	}

	if inputDone {
		ctx.addLastImageForOutput()
	} else {
		ctx.imagesForOutput <- *ctx.currentImageToProcess
		ctx.outputCompleted <- 1
	}
}

// seqRemoveVerticalSeam identifies a vertcal seam in the image with the minmial gradient magnitude and then returns a new image with
// one less column that doesnt have those pixels.
func (ctx *imageProcessContext) conRemoveVerticalSeam() image.Image {
	currentImage := ctx.currentImageToProcess.CurrentImage
	compressionBounds := ic.CompressionBounds{MinX: 0, MaxX: currentImage.Bounds().Max.X - 1, MinY: 0, MaxY: currentImage.Bounds().Max.Y - 1}
	LastRowBounds := ic.CompressionBounds{MinY: currentImage.Bounds().Max.Y - 1, MaxX: currentImage.Bounds().Max.X - 1, MaxY: currentImage.Bounds().Max.Y - 1}
	ctx.currentImageToProcess.CumulativeMagnitude = ic.GetCumulativeMagnitudeSlice(currentImage.Bounds().Max.X, currentImage.Bounds().Max.Y)

	// Create gradient magnitude matrix
	compressionBounds.Instruction = ic.IPixelMagnitude
	ctx.enqueueVerticalCompressionBounds(compressionBounds)
	ctx.queueManagerProcessFilter()

	// Find lowest magnitude vertical paths.
	for y := 1; y < currentImage.Bounds().Max.Y; y++ {
		rowToMinimize := ic.CompressionBounds{MinX: 0, MaxX: currentImage.Bounds().Max.X - 1, MinY: y, MaxY: y + 1, Instruction: ic.IMinimizeVerticalSeam}
		ctx.enqueueVerticalCompressionBounds(rowToMinimize)
		ctx.queueManagerProcessFilter()
	}

	// Single threaded, mark pixels to remove.
	minX, minY := ctx.currentImageToProcess.FindMinSeam(LastRowBounds)
	ctx.currentImageToProcess.MarkVerticalSeam(minX, minY)

	// Multithreaded, update new image and ruturn it once it's built.
	ctx.currentImageToProcess.NewImage = image.NewRGBA(image.Rect(0, 0, currentImage.Bounds().Max.X-1, currentImage.Bounds().Max.Y))
	compressionBounds = ic.CompressionBounds{MinX: 0, MaxX: currentImage.Bounds().Max.X, MinY: 0, MaxY: currentImage.Bounds().Max.Y, Instruction: ic.IRemoveColumn}
	ctx.enqueueHorizontalCompressionBounds(compressionBounds)
	ctx.queueManagerProcessFilter()

	return ctx.currentImageToProcess.NewImage
}

// seqRemoveHorizontalSeam identifies a horizontal seam in the image with the minmial gradient magnitude and then returns a new image with
// one less row that doesnt have those pixels.
func (ctx *imageProcessContext) conRemoveHorizontalSeam() image.Image {
	currentImage := ctx.currentImageToProcess.CurrentImage
	compressionBounds := ic.CompressionBounds{MinX: 0, MaxX: currentImage.Bounds().Max.X - 1, MinY: 0, MaxY: currentImage.Bounds().Max.Y - 1}
	LastColumnBounds := ic.CompressionBounds{MinX: currentImage.Bounds().Max.X - 1, MaxX: currentImage.Bounds().Max.X - 1, MinY: 0, MaxY: currentImage.Bounds().Max.Y - 1}
	ctx.currentImageToProcess.CumulativeMagnitude = ic.GetCumulativeMagnitudeSlice(currentImage.Bounds().Max.X, currentImage.Bounds().Max.Y)

	// Create gradient magnitude matrix
	compressionBounds.Instruction = ic.IPixelMagnitude
	ctx.enqueueVerticalCompressionBounds(compressionBounds)
	ctx.queueManagerProcessFilter()

	// Find lowest magnitude horizontal paths.
	for x := 1; x < currentImage.Bounds().Max.X; x++ {
		columnToMinimize := ic.CompressionBounds{MinX: x, MaxX: x + 1, MinY: 0, MaxY: currentImage.Bounds().Max.Y - 1, Instruction: ic.IMinimizeHorizontalSeam}
		ctx.enqueueHorizontalCompressionBounds(columnToMinimize)
		ctx.queueManagerProcessFilter()
	}

	// Single threaded, mark pixels to remove
	minX, minY := ctx.currentImageToProcess.FindMinSeam(LastColumnBounds)
	ctx.currentImageToProcess.MarkHorizontalSeam(minX, minY)

	// Multithreaded, update new image and ruturn it once it's built.
	ctx.currentImageToProcess.NewImage = image.NewRGBA(image.Rect(0, 0, currentImage.Bounds().Max.X, currentImage.Bounds().Max.Y-1))
	compressionBounds = ic.CompressionBounds{MinX: 0, MaxX: currentImage.Bounds().Max.X, MinY: 0, MaxY: currentImage.Bounds().Max.Y, Instruction: ic.IRemoveRow}
	ctx.enqueueVerticalCompressionBounds(compressionBounds)
	ctx.queueManagerProcessFilter()
	return ctx.currentImageToProcess.NewImage
}

//finishExportingImages makes sure all of the images have been written to their files before closing the thread.
func (ctx *imageProcessContext) finishExportingImages() {
	// Finish exporting images.
	for {
		imageForOutput, moreOutput := <-ctx.imagesForOutput
		if moreOutput {
			outputImage(imageForOutput.OutputFileName, imageForOutput.NewImage)
			outputCompleted := <-ctx.outputCompleted
			if outputCompleted == -1 {
				return
			}
		} else {
			break
		}
	}
	<-ctx.lastImageOutput
	return
}

// addLastImageForOutput enqueues the image to be written out and closes the respective channels
// in order to prevent deadlock.
func (ctx *imageProcessContext) addLastImageForOutput() {
	close(ctx.compressionBoundsToProcesss)
	ctx.outputCompleted <- -1
	close(ctx.outputCompleted)
	ctx.imagesForOutput <- *ctx.currentImageToProcess
	close(ctx.imagesForOutput)
}

//closeChannels tells the other threads to expext no more additional work.
func (ctx *imageProcessContext) closeChannels() {
	close(ctx.compressionBoundsToProcesss)
	close(ctx.outputCompleted)
	close(ctx.imagesForOutput)
}

// manageQueue keeps track of what image is being processed and which filter at a given time.
func (ctx *imageProcessContext) manageQueue() {
	ctx.queueManagementComplete = make(chan interface{})
	// Process image convolutions.
	//Try to open the file.
	reader, file, dir := ctx.getImageCompressionFileReader()
	defer file.Close()
	var inputDone bool
	line, err := reader.ReadString('\n')
	//First line of file unreadable.
	if err != nil && err != io.EOF && line != "" {
		fmt.Println("First line contains no data. Exiting function.")
		return
	}
	var nextLine string
	var nextLineErr error
	//Stop looping when there are no more lines left.
	for true {
		lineValues, parseErr := splitLine(line)

		if parseErr != nil {
			fmt.Println("Split error:", parseErr)
			ctx.closeChannels()
			break
		}
		nextLine, nextLineErr = reader.ReadString('\n')
		// If there's an input/output location, start compression.
		if lineValues[0] != "" && lineValues[1] != "" {
			ctx.currentImageToProcess = getImageToProcess(dir+"/"+lineValues[0], dir+"/"+lineValues[1], lineValues[2], lineValues[3])
			if ctx.currentImageToProcess != nil {
				if err == io.EOF || nextLine == "" || (nextLineErr != nil && nextLineErr != io.EOF) {
					inputDone = true
				}
				ctx.mangeImageCompression(inputDone)
			}
		}
		if inputDone {
			break
		}
		line = nextLine
		err = nextLineErr
	}

	ctx.finishExportingImages()
}

// queueManagerProcessFilter allows the queue manager to also apply filters and to add bounds from the
// image context to the communal queue of work to be done.
func (ctx *imageProcessContext) queueManagerProcessFilter() {
	for {
		select {
		// Move the instructions from the ImageToProcess to the imageProcessContext so they can be done.
		case compressionBounds, moreBounds := <-ctx.currentImageToProcess.ImageCompressionBounds:
			if !moreBounds {
				return
			}
			ctx.compressionBoundsToProcesss <- compressionBounds
		// Apply a filter.
		case compressionBounds := <-ctx.compressionBoundsToProcesss:
			ctx.currentImageToProcess.ProcessInstruction(compressionBounds)
			instructionsLeft := <-ctx.currentImageToProcess.InstructionsComplete
			// The last section of the image has been processed.
			if instructionsLeft == 0 {
				close(ctx.currentImageToProcess.CurrentStageComplete)
				return
			}
		// Another thread completed the last filter section.
		case <-ctx.currentImageToProcess.CurrentStageComplete:
			return
		}
	}
}

// processQueue allows the general threads (and the reader thread) to apply filters and
// output images.
func (ctx *imageProcessContext) processQueue() {
	for {
		select {
		// Apply a filter
		case compressionBounds, moreBounds := <-ctx.compressionBoundsToProcesss:
			if !moreBounds {
				break
			}
			if !compressionBounds.IsNull() {
				ctx.currentImageToProcess.ProcessInstruction(compressionBounds)
				filtersLeft := <-ctx.currentImageToProcess.InstructionsComplete
				// The last section of the image has been processed.
				if filtersLeft == 0 {
					close(ctx.currentImageToProcess.CurrentStageComplete)
				}
			}

		// Output an image.
		case imageForOutput, more := <-ctx.imagesForOutput:
			if more {
				outputImage(imageForOutput.OutputFileName, imageForOutput.NewImage)
				outputCompleted := <-ctx.outputCompleted
				if outputCompleted == -1 {
					ctx.lastImageOutput <- true
					fmt.Println("Finished last one. Leaving worker thread...")
					return
				}
			} else {
				fmt.Println("None to do. Leaving worker thread...")
				return
			}
		}
	}
}

// LaunchConcurrentApplication creates the imageProcessContext and launches the threads to do work.
func LaunchConcurrentApplication(numberOfWorkerThreads int, inputFileName string) {
	ctx := imageProcessContext{
		inputFileName:               inputFileName,
		numberOfWorkerThreads:       numberOfWorkerThreads,
		imagesForOutput:             make(chan ic.ImageToProcess, numberOfWorkerThreads),
		compressionBoundsToProcesss: make(chan ic.CompressionBounds, numberOfWorkerThreads),
		outputCompleted:             make(chan int, numberOfWorkerThreads),
		lastImageOutput:             make(chan interface{})}
	ctx.launchProcessingThreads()
	ctx.manageQueue()
}
