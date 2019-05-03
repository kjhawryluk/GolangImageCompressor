Project Description:
I created this project for my parrallel programming class in golang using dynamic programming and seam carving for image compression. In other words, given a CSV of paths to images and target rate of compression for each dimension, my program reads in each image and then continuously iterates over the image to identify the vertical and horizontal paths with the least gradient magnitude (i.e. least busy) from one side of the image to the other. It then removes these paths as it creates a new image. It does this until the image reaches the target dimensions, and then it exports the compressed image. By removing the last busy paths through the image, the application tries to minimize image distortion caused by compression and preserve the most important features of the image.

To run my code, you need to create a CSV with the following columns and no headers:
	Input location of a png image to compress
	Output location
	Rate to Compress X dimensions (should be between 0 and 1)
	Rate to Compress Y dimensions (shoulde be between 0 and 1)

You can then run my code sequentially with the following command:
go run src/editor/editor.go path_to_csv

To run the concurrent version provide the flag p after the file path to use the default number of threads
or p={some number of threads}
go run src/editor/editor.go path_to_csv p=2

Test Scripts
In Test Scripts I have bash commands and a Python script for running the script multiple times to compare (and graph) its performance on various numbers of threads. I also have a sample CSV to demonstrate how to format the input. 

Note:
If you try to compress an image to smaller than 3 pixels in any direction, you may receive errors. 