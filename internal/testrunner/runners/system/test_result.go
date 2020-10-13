package system

// Document corresponds to the logs or metrics event stored in the data stream.
type Document struct {
	Error *struct {
		Message string
	}
}
