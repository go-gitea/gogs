// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package queue

import "context"

// ByteFIFO defines a FIFO that takes a byte array
type ByteFIFO interface {
	// Len returns the length of the fifo
	Len(ctx context.Context) int64
	// PushFunc pushes data to the end of the fifo and calls the callback if it is added
	PushFunc(ctx context.Context, data []byte, fn func() error) error
	// Pop pops data from the start of the fifo
	Pop(ctx context.Context) ([]byte, error)
	// Close this fifo
	Close() error
	// PushBack pushes data back to the top of the fifo
	PushBack(ctx context.Context, data []byte) error
}

// UniqueByteFIFO defines a FIFO that Uniques its contents
type UniqueByteFIFO interface {
	ByteFIFO
	// Has returns whether the fifo contains this data
	Has(ctx context.Context, data []byte) (bool, error)
}

var _ ByteFIFO = &DummyByteFIFO{}

// DummyByteFIFO represents a dummy fifo
type DummyByteFIFO struct{}

// PushFunc returns nil
func (*DummyByteFIFO) PushFunc(ctx context.Context, data []byte, fn func() error) error {
	return nil
}

// Pop returns nil
func (*DummyByteFIFO) Pop(ctx context.Context) ([]byte, error) {
	return []byte{}, nil
}

// Close returns nil
func (*DummyByteFIFO) Close() error {
	return nil
}

// Len is always 0
func (*DummyByteFIFO) Len(ctx context.Context) int64 {
	return 0
}

// PushBack pushes data back to the top of the fifo
func (*DummyByteFIFO) PushBack(ctx context.Context, data []byte) error {
	return nil
}

var _ UniqueByteFIFO = &DummyUniqueByteFIFO{}

// DummyUniqueByteFIFO represents a dummy unique fifo
type DummyUniqueByteFIFO struct {
	DummyByteFIFO
}

// Has always returns false
func (*DummyUniqueByteFIFO) Has(ctx context.Context, data []byte) (bool, error) {
	return false, nil
}
