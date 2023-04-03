package pushback

import (
	"errors"
)

type byteChan chan byte

// ByteChannel is a channel of bytes with pushback.
type ByteChannel struct {
	// pushBackBuffer contains any bytes that have been pushed back.
	pushBackBuffer []byte
	// This is the source of the bytes.
	byteChan
}

// New creates a ByteChannelWithPushback containing the given byte channel.
// ch must be a *buffered* channel.
func New(ch chan byte) *ByteChannel {
	bc := ByteChannel{byteChan: ch}
	return &bc
}

// Close closes the channel.
func (bc *ByteChannel) Close() {
	close(bc.byteChan)
}

// get reads the next byte from the channel (or returns an error),
// ignoring any pushed back bytes.
func (bc *ByteChannel) get() (byte, error) {
	if bc.byteChan == nil {
		return 0, errors.New("channel is nil")
	}
	b, more := <-bc.byteChan
	if !more {
		return 0, errors.New("done")
	}
	return b, nil
}

// GetNextByte gets the next byte from the channel or, if the channel
// has been closed, returns an error.  If bytes have been pushed back,
// it returns the first of them instead.
func (bc *ByteChannel) GetNextByte() (byte, error) {
	// Check if there is anything in the push back buffer.  If so, remove the
	// first byte and return it.
	if len(bc.pushBackBuffer) > 0 {
		// There is something in the buffer.  Pull out the first byte and
		// return it.
		b := bc.pushBackBuffer[0]
		bc.pushBackBuffer = bc.pushBackBuffer[1:]
		return b, nil
	}

	// There is nothing in the pushback buffer.  fetch a byte from the channel.
	b, err := bc.get()
	return b, err
}

// Pushback pushes back a byte - the next call of FetchNextByte will read
// from the buffer rather than the channel.
func (bc *ByteChannel) PushBack(b byte) {
	if bc.pushBackBuffer == nil {
		// First call - create the buffer.
		bc.pushBackBuffer = make([]byte, 0)
	}
	bc.pushBackBuffer = append(bc.pushBackBuffer, b)
}
