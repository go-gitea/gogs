package buffer

import "math"

// List is a slice of Buffers, it's the backing for NewPartition
type List []Buffer

// Len is the sum of the Len()'s of the Buffers in the List.
func (l *List) Len() (n int64) {
	for _, buffer := range *l {
		if n > math.MaxInt64-buffer.Len() {
			return math.MaxInt64
		}
		n += buffer.Len()
	}
	return n
}

// Cap is the sum of the Cap()'s of the Buffers in the List.
func (l *List) Cap() (n int64) {
	for _, buffer := range *l {
		if n > math.MaxInt64-buffer.Cap() {
			return math.MaxInt64
		}
		n += buffer.Cap()
	}
	return n
}

// Reset calls Reset() on each of the Buffers in the list.
func (l *List) Reset() {
	for _, buffer := range *l {
		buffer.Reset()
	}
}

// Push adds a Buffer to the end of the List
func (l *List) Push(b Buffer) {
	*l = append(*l, b)
}

// Pop removes and returns a Buffer from the front of the List
func (l *List) Pop() (b Buffer) {
	b = (*l)[0]
	*l = (*l)[1:]
	return b
}
