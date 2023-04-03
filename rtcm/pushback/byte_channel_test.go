package pushback

import (
	"testing"
)

// TestGetNextByte checks that GetNextByte correctly gets the next byte from the channel.
func TestGetNextByte(t *testing.T) {

	const want = 'x'
	const wantError = "done"

	ch := make(chan byte, 1)
	bc := New(ch)

	// Put one character on the channel and close it.
	ch <- want
	bc.Close()

	// Get the byte back.
	got1, err := bc.GetNextByte()
	if err != nil {
		t.Error(err)
	}

	if want != got1 {
		t.Errorf("want %c got %c", want, got1)
	}

	// GetNextByte should produce an error.
	got2, gotError := bc.GetNextByte()
	if gotError == nil {
		t.Error("expected an error")
	}

	if got2 != 0 {
		t.Errorf("want 0 byte, got %c", got2)
	}

	if wantError != gotError.Error() {
		t.Errorf("want %s got %s", wantError, gotError.Error())
	}
}

// TestGetNextByteWithEmptyChannel checks that GetNextByte correctly handles a channel
// which is closed without sending any data into it.
func TestGetNextByteWithEmptyChannel(t *testing.T) {
	const wantError = "done"

	ch := make(chan byte, 1)
	bc := New(ch)

	// Close the channel.
	bc.Close()

	// GetNextByte should produce an error.
	gotByte, gotError := bc.GetNextByte()
	if gotError == nil {
		t.Error("expected an error")
	}

	if gotByte != 0 {
		t.Errorf("want 0 byte, got %c", gotByte)
	}

	if wantError != gotError.Error() {
		t.Errorf("want %s got %s", wantError, gotError.Error())
	}
}

// TestGetNextByteWithNilChannel checks that GetNextByte returns the correct error when
// the channel is nil.
func TestGetNextByteWithNilChannel(t *testing.T) {

	const wantByte = 0
	const wantError = "channel is nil"

	bc := New(nil)

	gotByte, gotError := bc.GetNextByte()

	if wantByte != gotByte {
		t.Errorf("want %d got %c (0x%x)", wantByte, gotByte, gotByte)
	}

	if wantError != gotError.Error() {
		t.Errorf("want %s got %s", wantError, gotError.Error())
	}

}

func TestPushBack(t *testing.T) {
	const want = "funk"

	buf := make([]byte, 0)
	ch := make(chan byte, 2)
	bc := New(ch)

	// Put bytes on the channel and close it.
	ch <- 'n'
	ch <- 'k'

	bc.Close()

	// Push back some bytes.
	bc.PushBack('f')
	bc.PushBack('u')

	// Read from the channel until it's exhausted and put the
	// result into the buffer.
	for {
		b, err := bc.GetNextByte()
		if err != nil {
			break
		}
		buf = append(buf, b)
	}

	got := string(buf)

	if want != got {
		t.Errorf("want %s got %s", want, got)
	}

}
