package message1005

import (
	"testing"

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
ECEF coords in metres (  1.2345,  2.3456,  3.4567)
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

	// bitStream1005 contains a message type 1005:
	// message type:    Station ID:        ITRF year
	//                                             ign:   x:
	// 0011 1110   1101|0000   0000 0010|  0000 11|11  11|00 0000
	//                                                 ig y:
	// 0000 0000   0000 0001   1110 0010   0100 0000|  01|00 0000
	//                                                    z:
	// 0000 0000   0000 0011   1001 0100   0100 0111|  10|00 0000
	// 0000 0000   0000 0101   0100 0110   0100 1110
	var bitStream1005 = []byte{
		0x3e, 0xd0, 0x02, 0x0f, 0xc0,
		0x00, 0x01, 0xe2, 0x40, 0x40,
		0x00, 0x03, 0x94, 0x47, 0x80,
		0x00, 0x05, 0x46, 0x4e,
	}

	var testData = []struct {
		description string
		bitStream   []byte
		wantError   string
		wantMessage *Message
	}{
		{"complete", bitStream1005, "", New(1005, 2, 3, 0xf, 123456, 1, 234567, 2, 345678)},
		{"short", bitStream1005[:18], "overrun - expected 152 bits in a message type 1005, got 144", nil},
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
