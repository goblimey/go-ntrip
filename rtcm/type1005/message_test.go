package type1005

import (
	"log/slog"
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
		logLevel:            slog.LevelDebug,
	}

	got := New(2, 3, 0xf, 4, 1, 5, 2, 6, slog.LevelDebug)

	if want != *got {
		t.Errorf("want: %v\n got: %v\n", want, *got)
	}
}

func TestString(t *testing.T) {

	const wantDebug = `stationID 2, ITRF realisation year 3, unknown bits 1111,
x 12345, unknown bits 01, y 23456, unknown bits 10, z 34567,
ECEF coords in metres (1.2345, 2.3456, 3.4567)
`

	const wantInfo = `stationID 2, ITRF realisation year 3,
ECEF coords in metres (1.2345, 2.3456, 3.4567)

`

	messageDebug := New(2, 3, 0xf, 12345, 1, 23456, 2, 34567, slog.LevelDebug)

	gotDebug := messageDebug.String()

	if wantDebug != gotDebug {
		t.Error(diff.Diff(wantDebug, gotDebug))
	}

	messageInfo := New(2, 3, 0xf, 12345, 1, 23456, 2, 34567, slog.LevelInfo)

	gotInfo := messageInfo.String()

	if wantInfo != gotInfo {
		t.Error(diff.Diff(wantInfo, gotInfo))
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
		// This is a 3-byte leader, one byte of message data and a 3-byte CRC, which is not
		// enough data to get the message length - fails very quickly.
		{"very short", testdata.MessageFrameType1005[:7], "overrun - expected 152 bits in a message type 1005, got 8", nil},
		// This frame is too short but will fail further down the track.
		{"short", testdata.MessageFrameType1005[:24], "overrun - expected 152 bits in a message type 1005, got 144", nil},
		// This contains a 3 byte leader, 19 bytes of embedded message and a 3 byte CRC,
		// 25 bytes (160 bits) in all.
		{"complete", testdata.MessageFrameType1005, "",
			New(2, 3, 0xf, 123456, 1, 234567, 2, 345678, slog.LevelDebug)},
	}

	for _, td := range testData {
		gotMessage, gotError := GetMessage(td.bitStream, slog.LevelDebug)
		if len(td.wantError) > 0 {
			// Expect an error.
			if gotMessage != nil {
				t.Errorf("%s: expected a nil message", td.description)
			}

			if gotError == nil {
				t.Errorf("%s: expected the error ", td.description+td.wantError)
				continue
			}
			if td.wantError != gotError.Error() {
				t.Errorf("%s: want error %s got %s", td.description, td.wantError, gotError.Error())
			}
		} else {
			// Expect the call to work.
			if gotError != nil {
				t.Errorf("%s: unexpected error %s", td.description, gotError.Error())
				continue
			}

			if gotMessage == nil {
				t.Errorf("%s:expected a message", td.description)
				continue
			}

			if *td.wantMessage != *gotMessage {
				t.Errorf("%s: want: %v\n got: %v\n", td.description, *td.wantMessage, *gotMessage)
			}
		}
	}
}

// TestIncorrectMessageType checks an obscure case where GetMessage is called
// on a bit stream that doesn't contain a message of type 1005.
func TestIncorrectMessageType(t *testing.T) {
	const want = "expected message type 1005 got 1077"
	message, err := GetMessage(testdata.MessageFrameType1077, slog.LevelDebug)
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
