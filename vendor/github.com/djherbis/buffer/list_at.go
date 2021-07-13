package buffer

import "math"

// ListAt is a slice of BufferAt's, it's the backing for NewPartitionAt
type ListAt []BufferAt

// Len is the sum of the Len()'s of the BufferAt's in the list.
func (l *ListAt) Len() (n int64) {
	for _, buffer := range *l {
		if n > math.MaxInt64-buffer.Len() {
			return math.MaxInt64
		}
		n += buffer.Len()
	}
	return n
}

// Cap is the sum of the Cap()'s of the BufferAt's in the list.
func (l *ListAt) Cap() (n int64) {
	for _, buffer := range *l {
		if n > math.MaxInt64-buffer.Cap() {
			return math.MaxInt64
		}
		n += buffer.Cap()
	}
	return n
}

// Reset calls Reset() on each of the BufferAt's in the list.
func (l *ListAt) Reset() {
	for _, buffer := range *l {
		buffer.Reset()
	}
}

// Push adds a BufferAt to the end of the list
func (l *ListAt) Push(b BufferAt) {
	*l = append(*l, b)
}

// Pop removes and returns a BufferAt from the front of the list
func (l *ListAt) Pop() (b BufferAt) {
	b = (*l)[0]
	*l = (*l)[1:]
	return b
}
