package message1005

import (
	"testing"

	"github.com/goblimey/go-ntrip/rtcm/testdata"

	"github.com/kylelemons/godebug/diff"
)

func TestNew(t *testing.T) {

	want := Message{
		MessageType:         1005,
		StationID:           2,
		ITRFRealisationYear: 3,
		Ignored1:            0xf,
		AntennaRefX:         4,
		Ignored2:            1,
		AntennaRefY:         5,
		Ignored3:            2,
		AntennaRefZ:         6,
	}

	got := New(1005, 2, 3, 0xf, 4, 1, 5, 2, 6)

	if want != *got {
		t.Errorf("want: %v\n got: %v\n", want, *got)
	}
}

func TestString(t *testing.T) {

	const want = `message type 1005 - Base Station Information
stationID 2, ITRF realisation year 3, ignored 0xf,
x 12345, ignored 0x1, y 23456, ignored 0x2, z 34567,
ECEF coords in metres (1.2345, 2.3456, 3.4567)
`

	got := New(1005, 2, 3, 0xf, 12345, 1, 23456, 2, 34567)

	if want != got.String() {
		t.Errorf("want:\n%s\ngot:\n%s\n", want, got.String())
		t.Error(diff.Diff(want, got.String()))
	}
}

// TestGetMessage checks that GetMessage correctly interprets a
// bitstream containing a message type 1005, or returns an appropriate
// error message.
func TestGetMessage(t *testing.T) {

	var testData = []struct {
		description string
		bitStream   []byte
		wantError   string
		wantMessage *Message
	}{
		{"complete", testdata.MessageFrameType1005, "", New(1005, 2, 3, 0xf, 123456, 1, 234567, 2, 345678)},
		// This frame contains a 3 byte leader, 19 bytes of embedded message and a 3 byte CRC,
		// 25 bytes in all.
		{"short", testdata.MessageFrameType1005[:24], "overrun - expected 152 bits in a message type 1005, got 144", nil},
	}

	for _, td := range testData {
		gotMessage, gotError := GetMessage(td.bitStream)
		if len(td.wantError) > 0 {
			// Expect an error.
			if gotMessage != nil {
				t.Error("expected a nil message")
			}

			if gotError == nil {
				t.Error("expected the error " + td.wantError)
				continue
			}
			if td.wantError != gotError.Error() {
				t.Errorf("want error %s got %s", td.wantError, gotError.Error())
			}
		} else {
			// Expect the call to work.
			if gotError != nil {
				t.Errorf("%s: unexpected error", gotError.Error())
				continue
			}

			if gotMessage == nil {
				t.Error("expected a message.")
				continue
			}

			if *td.wantMessage != *gotMessage {
				t.Errorf("want: %v\n got: %v\n", *td.wantMessage, *gotMessage)
			}
		}
	}
}

// TestIncorrectMessageType checks an obscure case where GetMessage is called
// on a bit stream that doesn't contain a message of type 1005.
func TestIncorrectMessageType(t *testing.T) {
	const want = "expected message type 1005 got 1077"
	message, err := GetMessage(testdata.Message1077)
	if err == nil {
		t.Error("expected an error")
		return
	}
	if err.Error() != want {
		t.Errorf("want %s, got %s", want, err.Error())
	}
	if message != nil {
		t.Error("expected the message to be nil")
	}
}
