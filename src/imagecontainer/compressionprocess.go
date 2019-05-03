package imagecontainer

// CompressionBounds contains the x and y coordinates used for applying a filter
// to a given part of an image.
type CompressionBounds struct {
	MinX, MinY, MaxX, MaxY, Instruction int
}

//IsNull checks if the struct is empty
func (cb *CompressionBounds) IsNull() bool {
	if cb.MinX == 0 && cb.MinY == 0 && cb.MaxX == 0 && cb.MaxY == 0 && cb.Instruction == 0 {
		return true
	}
	return false
}
