package header

import (
	"fmt"
	"testing"
	"time"
)

const maxMessageType = 4095 // the message type is a 12-bit unsigned quantity.

// TestNew checks that New creates a header correctly.
func TestNew(t *testing.T) {

	var testData = []struct {
		MessageType       int
		WantConstellation string
	}{
		{1137, "NavIC/IRNSS"},
		{1133, "unknown"},
	}

	for _, td := range testData {
		header := NewWithMessageType(td.MessageType)

		if td.MessageType != header.MessageType {
			t.Errorf("want %d, got %d", td.MessageType, header.MessageType)
		}

		if td.WantConstellation != header.Constellation {
			t.Errorf("want %s, got %s", td.WantConstellation, header.Constellation)
		}
	}
}

// TestGetMSMHeaderWithType checks that a bitstream containing the start of an
// MSM Header (containing a message type) is correctly interpreted.
func TestGetMSMType(t *testing.T) {
	// getMSMType is a helper function for GetMSMHeader.  The task
	// of figuring out the type values is messy, and needs careful testing.
	// Testing GetMSMHeader involves hand-crafting long bit streams.  If it
	// handled the work that getMSMHeaderType does, we would need to
	// hand-craft lots of them.

	const errorForMaxMessageType = "message type 4095 is not an MSM4 or an MSM7"

	var testData = []struct {
		MessageType       int
		WantError         string // "" means the error is nil
		WantPosition      int
		WantConstellation string
	}{
		{1074, "", lenMessageType, "GPS"},
		{1084, "", lenMessageType, "GLONASS"},
		{1094, "", lenMessageType, "Galileo"},
		{1104, "", lenMessageType, "SBAS"},
		{1114, "", lenMessageType, "QZSS"},
		{1124, "", lenMessageType, "BeiDou"},
		{1134, "", lenMessageType, "NavIC/IRNSS"},
		{1077, "", lenMessageType, "GPS"},
		{1087, "", lenMessageType, "GLONASS"},
		{1097, "", lenMessageType, "Galileo"},
		{1107, "", lenMessageType, "SBAS"},
		{1117, "", lenMessageType, "QZSS"},
		{1127, "", lenMessageType, "BeiDou"},
		{1137, "", lenMessageType, "NavIC/IRNSS"},

		// These message numbers are not for MSM messages
		{0, "message type 0 is not an MSM4 or an MSM7", 0, ""},
		{1, "message type 1 is not an MSM4 or an MSM7", 0, ""},
		{1073, "message type 1073 is not an MSM4 or an MSM7", 0, ""},
		{1075, "message type 1075 is not an MSM4 or an MSM7", 0, ""},
		{1076, "message type 1076 is not an MSM4 or an MSM7", 0, ""},
		{1138, "message type 1138 is not an MSM4 or an MSM7", 0, ""},
		{1023, "message type 1023 is not an MSM4 or an MSM7", 0, ""},
		{maxMessageType, errorForMaxMessageType, 40, ""},
	}
	for _, td := range testData {

		// GetMSMHeaderWithType expects at least 25 bytes (200 bits) but only
		// uses the first five bytes.  They should be a 3-byte leader, the
		// 12-bit message type and four trailing zero bits.
		// example 1074: 0100 0001 1110|0000 .....

		// Shift the type to give 16 bits with 4 trailing bits.
		tp := td.MessageType << 4

		// bitStream contains some stuff and a header starting at byte 5 (bit 40).
		bitStream := []byte{
			byte(tp >> 8), byte(tp & 0xff),
			0, 0, 0, 0, 0,
			0, 0, 0, 0, 0,
			0, 0, 0, 0, 0,
			0, 0, 0, 0, 0,
		}

		gotMSMType, gotPosition, err := getMSMType(bitStream)

		if td.WantError != "" {
			// This call is expected to fail and return an error.
			if err == nil {
				t.Errorf("%d: want error %s",
					td.MessageType, td.WantError)
			} else {
				if td.WantError != err.Error() {
					t.Errorf("%d: want error %s, got %s",
						td.MessageType, td.WantError, err.Error())
				}
			}

			// The checks below only make sense for valid calls and this one
			// is not valid.
			continue
		}

		// This call is expected to work.
		if err != nil {
			t.Errorf("%d: want no error, got %s",
				td.MessageType, err.Error())
			continue
		}

		if gotMSMType != td.MessageType {
			t.Errorf("want type %d, got %d",
				td.MessageType, gotMSMType)
			continue
		}

		if td.WantPosition != int(gotPosition) {
			t.Errorf("%d: want %d bits to be consumed, got %d",
				td.MessageType, lenMessageType, gotPosition)
			continue
		}
	}
}

// TestGetMSMTypeWithShortBitStream checks that getMSMType returns the
// correct error message when given a bit stream which is too short.
func TestGetMSMTypeWithShortBitStream(t *testing.T) {
	const wantError = "bit stream is 8 bits long, too short for a message type"
	bitStream := []byte{0xff}

	_, _, gotError := getMSMType(bitStream)

	if gotError == nil {
		t.Errorf("expected an error")
		return
	}

	if gotError.Error() != wantError {
		em := fmt.Sprintf("want \"%s\", got \"%s\"", wantError, gotError)
		t.Error(em)
		return
	}
}

// TestGetConstellation checks the getConstellation helper function, which should
// return an error if the message type is not an MSM.
func TestGetConstellation(t *testing.T) {
	// getConstellation is a helper function for GetMSMHeader.  The task of figuring
	// out the constellation value is messy and needs careful testing.

	var testData = []struct {
		MessageType       int
		WantConstellation string
	}{
		{1074, "GPS"},
		{1084, "GLONASS"},
		{1094, "Galileo"},
		{1104, "SBAS"},
		{1114, "QZSS"},
		{1124, "BeiDou"},
		{1134, "NavIC/IRNSS"},
		{1077, "GPS"},
		{1087, "GLONASS"},
		{1097, "Galileo"},
		{1107, "SBAS"},
		{1117, "QZSS"},
		{1127, "BeiDou"},
		{1137, "NavIC/IRNSS"},

		// These message numbers are not for MSM messages and provoke an error response.
		{0, "unknown"},
		{1, "unknown"},
		{1023, "unknown"},
		{1138, "unknown"},
		{1073, "unknown"},
		{1075, "unknown"},
		{1076, "unknown"},

		{maxMessageType, "unknown"},
	}

	for _, td := range testData {
		constellation := getConstellation(td.MessageType)
		if td.WantConstellation != constellation {
			t.Errorf("%d: want constellation %s, got %s",
				td.MessageType, td.WantConstellation, constellation)
		}
	}
}

// TestGetMSMHeader checks that a bitstream containing a header is
// correctly interpreted.
func TestGetMSMHeader(t *testing.T) {

	// The MSMHeader contains:
	//    a 12-bit unsigned message type (tested in detail separately)
	//    a 12-bit unsigned station ID
	//    a 30-bit unsigned timestamp
	//    a boolean multiple message flag
	//    a 3-bit unsigned sequence number
	//    a 7-bit unsigned session transmission time value
	//    a 2-bit unsigned clock steering indicator
	//    a 2-bit unsigned external clock indicator
	//    a boolean GNSS Divergence Free Smoothing Indicator
	//    a 3-bit GNSS Smoothing Interval
	//    a 64-bit satellite mask (one bit set per satellite observed)
	//    a 32-bit signal mask (one bit set per signal type observed)
	//    (169 bits up to this point)
	//    a cell mask (nSatellites X nSignals) bits long
	//
	// The function returns the broken out header and the bit position
	// of the start of the next part of the message. (In the real world
	// that comes immediately after this cell mask, so the position is
	// just the length of this bit stream, ignoring the final three
	// padding bits.)

	// 0100 0011  0101| 0000   0000 0001|  0000 0000   0000 0000
	// 0000 0000  0000 10|1|0  11|00 0010  0|10|0 1|1|11   1|000 0000
	// 0000 0000   0000 0000   0000 0000   0000 0000   0000 0000
	// 0000 0000   0000 1110   1|000 0000   0000 0000   0000 0000
	// 0001 0101   0|111 1100  0000 1|000

	bitStream := []byte{
		0x43, 0x50, 0x01, 0x00, 0x00,
		0x00, 0x0a, 0xc2, 0x4f, 0x80,
		0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x0e, 0x80, 0x00, 0x00,
		0x15, 0x7c, 0x08, 0x00, 0x00,
		0x00, 0x0e, 0x80, 0x00, 0x00,
		0x00, 0x00,
	}

	// The length of the fixed-size fields of the cell mask.
	const lenHeaderWithoutCellMask = 169
	// The length of the cell mask (4 satellites, 3 signals).
	const lenCellMask = 12
	// The expected satellite IDs
	satellites := []uint{60, 61, 62, 64}
	// The signal types we expect to observe
	signals := []uint{27, 29, 31}
	// The expected broken-out cell mask values.
	cellMask := [][]bool{
		{true, true, true}, {true, true, false},
		{false, false, false}, {false, false, true},
	}
	// Expect to be at this position of the bit stream after reading the header.
	const wantPos = lenHeaderWithoutCellMask
	// Expect this error message if the bit stream is short by two bytes
	// (so not all the fixed-length items are there).
	// wantMinLengthError :=
	// fmt.Sprintf("bitstream is too short for an MSM header - got %d bits, expected at least %d",
	// 	(len(bitStream)-2)*8, lenHeaderWithoutCellMask)
	// Expect this error if the bit stream is short by one byte
	// (so it includes part of the cell mask but is incomplete).
	//wantLengthError :=
	// fmt.Sprintf("bitstream is too short for an MSM header with %d cell mask bits - got %d bits, expected at least %d",
	// 	lenCellMask, (len(bitStream)-1)*8, wantPos)

	wantHeader := Header{
		MessageType:                          1077,
		Constellation:                        "GPS",
		StationID:                            1,
		EpochTime:                            2,
		MultipleMessage:                      true,
		IssueOfDataStation:                   3,
		SessionTransmissionTime:              4,
		ClockSteeringIndicator:               2,
		ExternalClockIndicator:               1,
		GNSSDivergenceFreeSmoothingIndicator: true,
		GNSSSmoothingInterval:                7,
		SatelliteMask:                        0x001d,
		SignalMask:                           0x2a,
		CellMask:                             0xf81,
		NumSignalCells:                       12,
	}

	// This bitstream is below the minimum length for an MSM header.
	//_, _, err := GetMSMHeader(bitStream[:len(bitStream)-2])

	// if minLengthError == nil {
	// 	t.Errorf("expected an error")
	// } else {

	// 	if minLengthError.Error() != wantMinLengthError {
	// 		t.Errorf("expected error \"%s\", got \"%s\"",
	// 			wantMinLengthError, minLengthError.Error())

	// 	}
	// }

	// This bitstream is above the minimum length but still short.
	// _, _, lengthError := GetMSMHeader(bitStream[:len(bitStream)-1])

	// if lengthError == nil {
	// 	t.Errorf("expected an error")
	// } else {
	// 	if lengthError.Error() != wantLengthError {
	// 		t.Errorf("expected error \"%s\", got \"%s\"",
	// 			wantLengthError, lengthError.Error())

	// 	}
	// }

	header, gotPos, err := GetMSMHeader(bitStream)

	if err != nil {
		t.Error(err)
		return
	}

	if gotPos != uint(wantPos) {
		t.Errorf("got position %d, want %d", gotPos, wantPos)
	}

	if header.Constellation != wantHeader.Constellation {
		t.Errorf("got type %s, want %s",
			header.Constellation, wantHeader.Constellation)
	}

	if header.StationID != wantHeader.StationID {
		t.Errorf("got type %d, want %d",
			header.StationID, wantHeader.StationID)
	}

	if header.EpochTime != wantHeader.EpochTime {
		t.Errorf("got epoch time %d, want %d",
			header.EpochTime, wantHeader.EpochTime)
	}

	// UTCTime time.Time

	if header.MultipleMessage != wantHeader.MultipleMessage {
		t.Errorf("got multiple message %v, want %v",
			header.MultipleMessage, wantHeader.MultipleMessage)
	}

	if header.IssueOfDataStation != wantHeader.IssueOfDataStation {
		t.Errorf("got sequence number %d, want %d",
			header.IssueOfDataStation, wantHeader.IssueOfDataStation)
	}

	if header.SessionTransmissionTime != wantHeader.SessionTransmissionTime {
		t.Errorf("got trans time %d, want %d",
			header.SessionTransmissionTime, wantHeader.SessionTransmissionTime)
	}

	if header.ClockSteeringIndicator != wantHeader.ClockSteeringIndicator {
		t.Errorf("got clock steering ind %d, want %d",
			header.ClockSteeringIndicator, wantHeader.ClockSteeringIndicator)
	}

	if header.GNSSDivergenceFreeSmoothingIndicator !=
		wantHeader.GNSSDivergenceFreeSmoothingIndicator {

		t.Errorf("got smoothing ind %v, want %v",
			header.GNSSDivergenceFreeSmoothingIndicator,
			wantHeader.GNSSDivergenceFreeSmoothingIndicator)
	}

	if header.GNSSSmoothingInterval != wantHeader.GNSSSmoothingInterval {
		t.Errorf("got smoothing interval %d, want %d",
			header.GNSSSmoothingInterval, wantHeader.GNSSSmoothingInterval)
	}

	if header.ExternalClockIndicator != wantHeader.ExternalClockIndicator {
		t.Errorf("got external CLI %d, want %d",
			header.ExternalClockIndicator, wantHeader.ExternalClockIndicator)
	}

	if header.SatelliteMask != wantHeader.SatelliteMask {
		t.Errorf("got satellite mask 0x%x, want 0x%x",
			header.SatelliteMask, wantHeader.SatelliteMask)
	}

	if header.SignalMask != wantHeader.SignalMask {
		t.Errorf("got signal mask 0x%x, want 0x%x",
			header.SignalMask, wantHeader.SignalMask)
	}

	if header.CellMask != wantHeader.CellMask {
		t.Errorf("got cell mask 0x%x, want 0x%x",
			header.CellMask, wantHeader.CellMask)
	}

	if header.NumSignalCells != wantHeader.NumSignalCells {
		t.Errorf("got number of signal cells %d, want %d",
			header.NumSignalCells, wantHeader.NumSignalCells)
	}

	if len(satellites) == len(header.Satellites) {
		for i := range satellites {
			if satellites[i] != header.Satellites[i] {
				t.Errorf("satellite %d want %d got %d",
					i, satellites[i], header.Satellites[i])
			}
		}
	} else {
		t.Errorf("want %d satellites, got %d", len(satellites), len(header.Satellites))
		return
	}

	if len(signals) == len(header.Signals) {
		for i := range signals {
			if signals[i] != header.Signals[i] {
				t.Errorf("signal %d want %d got %d",
					i, signals[i], header.Signals[i])
			}
		}
	} else {
		t.Errorf("want %d signals, got %d", len(signals), len(header.Signals))
		return
	}

	// Check the cell mask.
	if len(cellMask) == len(header.Cells) {
		for i := range cellMask {
			if len(cellMask[i]) == len(header.Cells[i]) {
				for j := range cellMask[i] {
					if cellMask[i][j] != (header.Cells[i][j]) {
						t.Errorf("cellMask[%d][%d]: want %v, got %v",
							i, j, cellMask[i][j], header.Cells[i][j])
					}
				}
			} else {
				t.Errorf("cellMask[%d] want %d items, got %d",
					i, len(cellMask), len(header.Cells))
			}
		}
	} else {
		t.Errorf("cellMask: want %d items, got %d",
			len(cellMask), len(header.Cells))
	}
}

// TestGetMSMHeaderWithWrongMessageType checks that getMSMHeader return the correct
// error when the message type is not MSM.
func TestGetMSMHeaderWithWrongMessageType(t *testing.T) {

	// GetMSMHeaderWithType expects at least 169 bits (21 bytes) but for this test
	// we only need to set the first five bytes.  They should be a 3-byte leader, the
	// 12-bit message type and four trailing zero bits.
	// For example 1074 (ignoring the leader): |0100 0001 1110|0000

	messageType := 1073 // Not MSM

	// Shift the type to give 16 bits with 4 trailing bits.
	tp := messageType << 4

	// bitStream contains some stuff and a header starting at byte 5 (bit 40).
	bitStream := []byte{
		byte(tp >> 8), byte(tp & 0xff), 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0,
	}

	const wantError = "message type 1073 is not an MSM4 or an MSM7"

	_, _, err := GetMSMHeader(bitStream)

	// The call is expected to fail and return an error.
	if err == nil {
		t.Errorf("%d: want error %s",
			messageType, wantError)
	} else {
		if wantError != err.Error() {
			t.Errorf("want error %s, got %s",
				wantError, err.Error())
		}
	}
}

// TestGetMSMHeaderWithCellMaskTooLong checks that a bitstream containing a header
// with a long cell mask produces the correct error.
func TestGetMSMHeaderWithCellMaskTooLong(t *testing.T) {

	// The bitstream contains a 3-byte message leader,followed by the MSM Header.
	//
	// The length of the cell mask must be 64 bits or less.  Provoke
	// an error by creating a bitStream containing valid values but
	// with 10 satellite cells and eight signal cells.  That gives a
	// cell mask 80 bits long.

	// 0100 0011  0101| 0000   0000 0001|  0000 0000   0000 0000
	// 0000 0000 0000 10|1|0
	//                               satellite mask with 10 bits set:
	// 11|00 0010  0|10|0 1|1|11   1|011 1111   1111 0000   0000 0000
	// 0000 0000   0000 0000   0000 0000   0000 0000   0000 1110
	//   signal mask with 8 bits set:
	// 1|111 1111   1000 0000   0000 0000   0000 0000   0|111 1100
	// 0000 1|000 .......

	bitStream := []byte{
		0x43, 0x50, 0x01, 0x00, 0x00,
		0x00, 0x0a,
		0xc2, 0x4f, 0xbf, 0xf0, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x0e,
		0xff, 0x80, 0x00, 0x00, 0x7c,
		0x08, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00,
	}

	_ = []byte{
		0x43, 0x50, 0x01, 0x00, 0x00,
		0x00, 0x0a, 0xc2, 0x4f, 0x80,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}

	wantError := "GetMSMHeader: cellMask is 112 bits - expected <= 64"

	_, _, err := GetMSMHeader(bitStream)

	// The call is expected to fail and return an error.
	if err == nil {
		t.Errorf("want error %s", wantError)
	} else {
		if wantError != err.Error() {
			t.Errorf("want error %s, got %s",
				wantError, err.Error())
		}
	}
}

// TestGetTitle checks that getTitle return the correct title for display.
func TestGetTitle(t *testing.T) {

	const titleMSM4 = "Full Pseudoranges and PhaseRanges plus CNR"
	const titleMSM7 = "Full Pseudoranges and PhaseRanges plus CNR (high resolution)"
	const titleError = "Unknown MSM type"

	var testData = []struct {
		MessageType int
		WantTitle   string
	}{
		{1074, titleMSM4},
		{1084, titleMSM4},
		{1094, titleMSM4},
		{1104, titleMSM4},
		{1114, titleMSM4},
		{1124, titleMSM4},
		{1134, titleMSM4},
		{1077, titleMSM7},
		{1087, titleMSM7},
		{1097, titleMSM7},
		{1107, titleMSM7},
		{1117, titleMSM7},
		{1127, titleMSM7},
		{1137, titleMSM7},

		// These message numbers are not for MSM messages
		{0, titleError},
		{1, titleError},
		{1073, titleError},
		{1075, titleError},
		{1076, titleError},
		{1138, titleError},
		{1023, titleError},
		{maxMessageType, titleError},
	}
	for _, td := range testData {

		header := Header{MessageType: td.MessageType}

		title := header.getTitle()
		if title != td.WantTitle {
			t.Errorf("%d: want title %s, got %s",
				td.MessageType, td.WantTitle, title)
		}
	}
}

func TestString(t *testing.T) {
	const satMask = 3
	const sigMask = 7
	const cellMask = 1

	header := New(1074, 2, 3, false, 1, 5, 6, 7, true, 9, satMask, sigMask, cellMask)

	utc, _ := time.LoadLocation("UTC")
	header.UTCTime = time.Date(2023, time.February, 14, 1, 2, 3, int(4*time.Millisecond), utc)
	const want = `type 1074 GPS Full Pseudoranges and PhaseRanges plus CNR
time 2023-02-14 01:02:03.004 +0000 UTC (epoch time 3)
stationID 2, single message, sequence number 1, session transmit time 5
clock steering 6, external clock 7
divergence free smoothing true, smoothing interval 9
2 satellites, 3 signal types, 6 signals
`

	got := header.String()

	if want != got {
		t.Errorf("want\n%s\ngot\n%s", want, got)
	}
}

func TestGetSatellites(t *testing.T) {
	const satMask = 0x0000000000000001
}