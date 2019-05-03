package filter

// createFilter takes three rows of filter values and returns a 2d array of them.
func createFilter(row1, row2, row3 []int32) [][]int32 {
	filter := [][]int32{}
	filter = append(filter, row1, row2, row3)
	return filter
}

//NOTE: These filter values were chosen based on the Sobel Filter that can be used to
// calculating gradient magnitude. https://en.wikipedia.org/wiki/Sobel_operator

//XGradientFilter applies a filter to identify horizontal differences in the image
func XGradientFilter() [][]int32 {
	row1 := []int32{1, 0, -1}
	row2 := []int32{2, 0, -2}
	row3 := []int32{1, 0, -1}
	return createFilter(row1, row2, row3)
}

//YGradientFilter applies a filter to identify vertical differences in the image.
func YGradientFilter() [][]int32 {
	row1 := []int32{1, 2, 1}
	row2 := []int32{0, 0, 0}
	row3 := []int32{-1, -2, -1}
	return createFilter(row1, row2, row3)
}
