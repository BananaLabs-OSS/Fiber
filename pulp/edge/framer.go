package edge

import (
	"encoding/binary"
	"errors"
	"fmt"
)

var (
	ErrFrameTooLarge = errors.New("edge frame exceeds configured maximum")
)

// LengthPrefixFramer incrementally splits a TCP byte stream containing
// big-endian uint32 length-prefixed frames. It handles partial headers,
// fragmented payloads, and multiple frames in one read without exposing an
// unbounded allocation surface.
type LengthPrefixFramer struct {
	maxFrame uint32
	buffer   []byte
}

func NewLengthPrefixFramer(maxFrame uint32) (*LengthPrefixFramer, error) {
	if maxFrame == 0 {
		return nil, fmt.Errorf("max frame must be positive")
	}
	return &LengthPrefixFramer{maxFrame: maxFrame}, nil
}

func (f *LengthPrefixFramer) Push(chunk []byte) ([][]byte, error) {
	f.buffer = append(f.buffer, chunk...)
	frames := make([][]byte, 0)
	for len(f.buffer) >= 4 {
		size := binary.BigEndian.Uint32(f.buffer[:4])
		if size > f.maxFrame {
			f.buffer = nil
			return nil, ErrFrameTooLarge
		}
		needed := uint64(size) + 4
		if uint64(len(f.buffer)) < needed {
			break
		}
		frame := append([]byte(nil), f.buffer[4:needed]...)
		frames = append(frames, frame)
		f.buffer = append(f.buffer[:0], f.buffer[needed:]...)
	}
	return frames, nil
}

func (f *LengthPrefixFramer) Buffered() int { return len(f.buffer) }
func (f *LengthPrefixFramer) Reset()        { f.buffer = nil }
