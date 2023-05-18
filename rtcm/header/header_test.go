package header

import (
	"fmt"
	"testing"
	"time"

	"github.com/goblimey/go-ntrip/rtcm/utils"

	"github.com/goblimey/go-crc24q/crc24q"

	"github.com/kylelemons/godebug/diff"
)

// TestNew checks that New creates a header correctly.
func TestNew(t *testing.T) {

	const wantSatelliteMask = 3
	const wantSignalMask = 7
	const wantCellMask = 1
	const wantMessageType = 1074
	const wantStationID = 1
	const wantTimestamp = 2 * 1000 // 2 seconds.
	const wantMultipleMessage = true
	const wantIssue = 3
	const wantTransTime = 4
	const wantClockSteeringIndicator = 5
	const wantExternalClockSteeringIndicator = 6
	const wantSmoothing = true
	const wantSmoothingInterval = 7

	gotHeader := New(wantMessageType, wantStationID, wantTimestamp,
		wantMultipleMessage,
		wantIssue, wantTransTime, wantClockSteeringIndicator,
		wantExternalClockSteeringIndicator, true, wantSmoothingInterval,
		wantSatelliteMask, wantSignalMask, wantCellMask)

	if gotHeader.MessageType != wantMessageType {
		t.Errorf("want %d got %d", wantMessageType, gotHeader.MessageType)
	}

	if gotHeader.StationID != wantStationID {
		t.Errorf("want %d got %d", wantStationID, gotHeader.StationID)
	}

	if gotHeader.Timestamp != wantTimestamp {
		t.Errorf("want %d got %d", wantTimestamp, gotHeader.Timestamp)
	}

	if !gotHeader.MultipleMessage {
		t.Errorf("want %v got %v", wantMultipleMessage, gotHeader.MultipleMessage)
	}

	if gotHeader.IssueOfDataStation != wantIssue {
		t.Errorf("want %d got %d", wantIssue, gotHeader.IssueOfDataStation)
	}
	if gotHeader.Timestamp != wantTimestamp {
		t.Errorf("want %d got %d", wantTimestamp, gotHeader.Timestamp)
	}

	if gotHeader.ClockSteeringIndicator != wantClockSteeringIndicator {
		t.Errorf("want %d got %d", wantClockSteeringIndicator, gotHeader.ClockSteeringIndicator)
	}
	if gotHeader.ExternalClockSteeringIndicator != wantExternalClockSteeringIndicator {
		t.Errorf("want %d got %d", wantExternalClockSteeringIndicator, gotHeader.ExternalClockSteeringIndicator)
	}
	if !gotHeader.GNSSDivergenceFreeSmoothingIndicator {
		t.Errorf("want %v got %v", wantSmoothing, gotHeader.GNSSDivergenceFreeSmoothingIndicator)
	}
	if gotHeader.GNSSSmoothingInterval != wantSmoothingInterval {
		t.Errorf("want %d got %d", wantSmoothingInterval, gotHeader.GNSSSmoothingInterval)
	}
	if gotHeader.GNSSSmoothingInterval != wantSmoothingInterval {
		t.Errorf("want %d got %d", wantSmoothingInterval, gotHeader.GNSSSmoothingInterval)
	}
	if gotHeader.SatelliteMask != wantSatelliteMask {
		t.Errorf("want %d got %d", wantSatelliteMask, gotHeader.SatelliteMask)
	}
	if gotHeader.SignalMask != wantSignalMask {
		t.Errorf("want %d got %d", wantSignalMask, gotHeader.SignalMask)
	}
	if gotHeader.CellMask != wantCellMask {
		t.Errorf("want %d got %d", wantSmoothingInterval, gotHeader.CellMask)
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

	// The position in the bit stream after the message type has been read.
	// The message frame contains the leader, the embedded message and the CRC.
	// The message type is the first field of the message.
	//
	const posAfterMessageType = lenMessageType + utils.LeaderLengthBits

	var testData = []struct {
		MessageType       int
		WantError         string // "" means the error is nil
		WantPosition      int
		WantConstellation string
	}{
		{1074, "", posAfterMessageType, "GPS"},
		{1084, "", posAfterMessageType, "GLONASS"},
		{1094, "", posAfterMessageType, "Galileo"},
		{1104, "", posAfterMessageType, "SBAS"},
		{1114, "", posAfterMessageType, "QZSS"},
		{1124, "", posAfterMessageType, "Beidou"},
		{1134, "", posAfterMessageType, "NavIC/IRNSS"},
		{1077, "", posAfterMessageType, "GPS"},
		{1087, "", posAfterMessageType, "GLONASS"},
		{1097, "", posAfterMessageType, "Galileo"},
		{1107, "", posAfterMessageType, "SBAS"},
		{1117, "", posAfterMessageType, "QZSS"},
		{1127, "", posAfterMessageType, "Beidou"},
		{1137, "", posAfterMessageType, "NavIC/IRNSS"},

		// These message numbers are not for MSM messages
		{0, "message type 0 is not an MSM4 or an MSM7", 0, ""},
		{1, "message type 1 is not an MSM4 or an MSM7", 0, ""},
		{1073, "message type 1073 is not an MSM4 or an MSM7", 0, ""},
		{1075, "message type 1075 is not an MSM4 or an MSM7", 0, ""},
		{1076, "message type 1076 is not an MSM4 or an MSM7", 0, ""},
		{1138, "message type 1138 is not an MSM4 or an MSM7", 0, ""},
		{1023, "message type 1023 is not an MSM4 or an MSM7", 0, ""},
		{utils.MaxMessageType, errorForMaxMessageType, 40, ""},
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
			0xd3, 0, 22,
			byte(tp >> 8), byte(tp & 0xff),
			0, 0, 0, 0, 0,
			0, 0, 0, 0, 0,
			0, 0, 0, 0, 0,
			0, 0, 0, 0, 0,
		}

		crc := crc24q.Hash(bitStream)
		bitStream = append(bitStream, crc24q.HiByte(crc))
		bitStream = append(bitStream, crc24q.MiByte(crc))
		bitStream = append(bitStream, crc24q.LoByte(crc))

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

	// The bit stream is a 3-byte header, the embedded message and the CRC.
	bitStream := []byte{0xd3, 0, 0, 0xff, 0, 0, 0}

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
	// just the length of this bit stream consumed so far.)

	// 0100 0011  0101| 0000   0000 0001|  0000 0000   0000 0000
	//                                                       satellite mask:
	// 0000 0000  0000 10|1|0  11|00 0010  0|10|0 1|1|11   1|000 0000
	// 0000 0000   0000 0000   0000 0000   0000 0000   0000 0000
	//                           signal mask:
	// 0000 0000   0000 1110   1|000 0000   0000 0000   0000 0000
	//               cell mask:
	// 0001 0101   0|111 1100  0000 1|000
	//
	// The cell mask is 4X3 bits - 111, 110, 000, 001.

	bitStream := []byte{
		0xd3, 0, 32,
		0x43, 0x50, 0x01, 0x00, 0x00,
		0x00, 0x0a, 0xc2, 0x4f, 0x80,
		0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x0e, 0x80, 0x00, 0x00,
		0x15, 0x7c, 0x08, 0x00, 0x00,
		0x00, 0x0e, 0x80, 0x00, 0x00,
		0x00, 0x00,
	}

	crc := crc24q.Hash(bitStream)
	bitStream = append(bitStream, crc24q.HiByte(crc))
	bitStream = append(bitStream, crc24q.MiByte(crc))
	bitStream = append(bitStream, crc24q.LoByte(crc))

	// The length of the fixed-size fields of the cell mask.
	const lenHeaderWithoutCellMask = 169
	// The length of the cell mask (4 satellites, 3 signals).
	const lenCellMask = 12

	// Expect to be at this position in the bit stream after reading the header.
	const wantPosition = utils.LeaderLengthBits + lenHeaderWithoutCellMask + lenCellMask

	// The expected satellite IDs
	wantSatellites := []uint{60, 61, 62, 64}
	// The signal types we expect to observe
	wantSignals := []uint{27, 29, 31}
	// The expected broken-out cell mask values.
	wantCellBools := [][]bool{
		{true, true, true}, {true, true, false},
		{false, false, false}, {false, false, true},
	}

	wantHeader := Header{
		MessageType:                          1077,
		Constellation:                        "GPS",
		StationID:                            1,
		Timestamp:                            2,
		MultipleMessage:                      true,
		IssueOfDataStation:                   3,
		SessionTransmissionTime:              4,
		ClockSteeringIndicator:               2,
		ExternalClockSteeringIndicator:       1,
		GNSSDivergenceFreeSmoothingIndicator: true,
		GNSSSmoothingInterval:                7,
		SatelliteMask:                        0x001d,
		SignalMask:                           0x2a,
		CellMask:                             0xf810000000000000,
		NumSignalCells:                       12,
	}

	header, gotPos, err := GetMSMHeader(bitStream)

	if err != nil {
		t.Error(err)
		return
	}

	if gotPos != uint(wantPosition) {
		t.Errorf("got position %d, want %d", gotPos, wantPosition)
	}

	if header.Constellation != wantHeader.Constellation {
		t.Errorf("got type %s, want %s",
			header.Constellation, wantHeader.Constellation)
	}

	if header.StationID != wantHeader.StationID {
		t.Errorf("got type %d, want %d",
			header.StationID, wantHeader.StationID)
	}

	if header.Timestamp != wantHeader.Timestamp {
		t.Errorf("got timestamp %d, want %d",
			header.Timestamp, wantHeader.Timestamp)
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

	if header.ExternalClockSteeringIndicator != wantHeader.ExternalClockSteeringIndicator {
		t.Errorf("got external CLI %d, want %d",

			header.ExternalClockSteeringIndicator, wantHeader.ExternalClockSteeringIndicator)
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

	if len(wantSatellites) == len(header.Satellites) {
		for i := range wantSatellites {
			if wantSatellites[i] != header.Satellites[i] {
				t.Errorf("satellite %d want %d got %d",
					i, wantSatellites[i], header.Satellites[i])
			}
		}
	} else {
		t.Errorf("want %d satellites, got %d", len(wantSatellites), len(header.Satellites))
		return
	}

	if len(wantSignals) == len(header.Signals) {
		for i := range wantSignals {
			if wantSignals[i] != header.Signals[i] {
				t.Errorf("signal %d want %d got %d",
					i, wantSignals[i], header.Signals[i])
			}
		}
	} else {
		t.Errorf("want %d signals, got %d", len(wantSignals), len(header.Signals))
		return
	}

	// Check the cell mask.
	if len(wantCellBools) == len(header.Cells) {
		for i := range wantCellBools {
			if len(wantCellBools[i]) == len(header.Cells[i]) {
				for j := range wantCellBools[i] {
					if wantCellBools[i][j] != (header.Cells[i][j]) {
						t.Errorf("cellMask[%d][%d]: want %v, got %v",
							i, j, wantCellBools[i][j], header.Cells[i][j])
					}
				}
			} else {
				t.Errorf("cellMask[%d] want %d items, got %d",
					i, len(wantCellBools), len(header.Cells))
			}
		}
	} else {
		t.Errorf("cellMask: want %d items, got %d",
			len(wantCellBools), len(header.Cells))
	}
}

// TestGetMSMHeaderWithShortBitStream checks GetMSMHeader returns the correct
// error if the bit stream is too short.
func TestGetMSMHeaderWithShortBitStream(t *testing.T) {
	// The fixed-length firlds in a bit stream containing an MSM header must
	// be at least 169 bits in toatl.  This bit stream is 21 bytes, 168 bits,
	// so just too short.
	bitStream := []byte{
		0xd3, 0, 21, // leader
		0x43, 0x50, 0x01, 0x00, 0x00, // 21-byte embedded message
		0x00, 0x0a, 0xc2, 0x4f, 0x80,
		0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x0e, 0x80, 0x00, 0x00,
		0x15,
	}

	// Add the CRC to the message frame.
	crc := crc24q.Hash(bitStream)
	bitStream = append(bitStream, crc24q.HiByte(crc))
	bitStream = append(bitStream, crc24q.MiByte(crc))
	bitStream = append(bitStream, crc24q.LoByte(crc))

	const wantError = "bitstream is too short for an MSM header - got 168 bits, expected at least 169"
	const wantPos = 0

	gotHeader, gotPos, gotError := GetMSMHeader(bitStream)

	if gotError == nil {
		t.Error("expected an error")
		return
	}

	if gotError.Error() != wantError {
		t.Errorf("want error\n%s\ngot\n%s", wantError, gotError.Error())
		return
	}

	if wantPos != gotPos {
		t.Errorf("want position %d, got %d", wantPos, gotPos)
	}

	if gotHeader != nil {
		t.Error("want  nil header, got non-nil")
	}
}

// TestGetMSMHeaderWithShortCellMask checks GetMSMHeader returns the correct
// error if the cell mask is too short.
func TestGetMSMHeaderWithShortCellMask(t *testing.T) {

	// The cell mask is variable-length and comes at the end of the bit
	// stream.  the length is given by the number of bits set in the
	// satellite mask (4 in this case), multiplied by the number of bits in
	// the signal mask (3 in this case), ie the cell mask should be 12 bits
	// followed by three bits of padding.
	//
	// This is a correct bit stream, 181 bits plus 3 bits of padding
	// 0100 0011  0101| 0000   0000 0001|  0000 0000   0000 0000
	//                                                       satellite mask:
	// 0000 0000  0000 10|1|0  11|00 0010  0|10|0 1|1|11   1|000 0000
	// 0000 0000   0000 0000   0000 0000   0000 0000   0000 0000
	//                           signal mask:
	// 0000 0000   0000 1110   1|000 0000   0000 0000   0000 0000
	//               cell Mask:       padding:
	// 0001 0101   0|111 1100  0000 1|000

	// In this version the last byte is missing:
	bitStream := []byte{
		0xd3, 0, 22, // Leader
		0x43, 0x50, 0x01, 0x00, 0x00, // 22 byte message
		0x00, 0x0a, 0xc2, 0x4f, 0x80,
		0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x0e, 0x80, 0x00, 0x00,
		0x15, 0x7c,
	}

	// Add the CRC to the message frame.
	crc := crc24q.Hash(bitStream)
	bitStream = append(bitStream, crc24q.HiByte(crc))
	bitStream = append(bitStream, crc24q.MiByte(crc))
	bitStream = append(bitStream, crc24q.LoByte(crc))

	const wantError = "bitstream is too short for an MSM header with 12 cell mask bits - got 224 bits, expected at least 229"
	const wantPos = 0

	gotHeader, gotPos, gotError := GetMSMHeader(bitStream)

	if gotError == nil {
		t.Error("expected an error")
		return
	}

	if gotError.Error() != wantError {
		t.Errorf("want error\n%s\ngot\n%s", wantError, gotError.Error())
		return
	}

	if wantPos != gotPos {
		t.Errorf("want position %d, got %d", wantPos, gotPos)
	}

	if gotHeader != nil {
		t.Error("want  nil header, got non-nil")
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
		0xd3, 0, 41,
		byte(tp >> 8), byte(tp & 0xff), 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0,
	}

	// Add the CRC to the message frame.
	crc := crc24q.Hash(bitStream)
	bitStream = append(bitStream, crc24q.HiByte(crc))
	bitStream = append(bitStream, crc24q.MiByte(crc))
	bitStream = append(bitStream, crc24q.LoByte(crc))

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
		0xd3, 0, 32,
		0x43, 0x50, 0x01, 0x00, 0x00,
		0x00, 0x0a,
		0xc2, 0x4f, 0xbf, 0xf0, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x0e,
		0xff, 0x80, 0x00, 0x00, 0x7c,
		0x08, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00,
	}

	// Add the CRC to the message frame.
	crc := crc24q.Hash(bitStream)
	bitStream = append(bitStream, crc24q.HiByte(crc))
	bitStream = append(bitStream, crc24q.MiByte(crc))
	bitStream = append(bitStream, crc24q.LoByte(crc))

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

	var testData = []struct {
		messageType int
		want        string
	}{
		{1074, "GPS Full Pseudoranges and PhaseRanges plus Carrier to Noise Ratio"},
		{1084, "GLONASS Full Pseudoranges and PhaseRanges plus Carrier to Noise Ratio"},
		{1094, "Galileo Full Pseudoranges and PhaseRanges plus Carrier to Noise Ratio"},
		{1104, "SBAS Full Pseudoranges and PhaseRanges plus Carrier to Noise Ratio"},
		{1114, "QZSS Full Pseudoranges and PhaseRanges plus Carrier to Noise Ratio"},
		{1124, "BeiDou Full Pseudoranges and PhaseRanges plus Carrier to Noise Ratio"},
		{1134, "NavIC/IRNSS Full Pseudoranges and PhaseRanges plus Carrier to Noise Ratio"},
		{1077, "GPS Full Pseudoranges and PhaseRanges plus Carrier to Noise Ratio (high resolution)"},
		{1087, "GLONASS Full Pseudoranges and PhaseRanges plus Carrier to Noise Ratio (high resolution)"},
		{1097, "Galileo Full Pseudoranges and PhaseRanges plus Carrier to Noise Ratio (high resolution)"},
		{1107, "SBAS Full Pseudoranges and PhaseRanges plus Carrier to Noise Ratio (high resolution)"},
		{1117, "QZSS Full Pseudoranges and PhaseRanges plus Carrier to Noise Ratio (high resolution)"},
		{1127, "BeiDou Full Pseudoranges and PhaseRanges plus Carrier to Noise Ratio (high resolution)"},
		{1137, "NavIC/IRNSS Full Pseudoranges and PhaseRanges plus Carrier to Noise Ratio (high resolution)"},

		// These message numbers are not for MSM messages
		{0, "message type 0 is not known"},
		{1, "message type 1 is not known"},
		{utils.MaxMessageType, "Assigned to: Ashtech"},
	}
	for _, td := range testData {

		header := Header{MessageType: td.messageType}

		got := header.GetTitle()
		if got != td.want {
			t.Errorf("%d: want title \"%s\", got \"%s\"",
				td.messageType, td.want, got)
		}
	}
}

// TestString checks the String function.
func TestString(t *testing.T) {

	const wantSatelliteMask = 3
	const wantSignalMask = 7
	const wantCellMask = 1
	const wantMessageType = 1074
	const wantStationID = 1
	const wantGPSTimestamp = 2 * 1000                          // 2 seconds.
	const wantGlonassTimestamp = uint(1<<27) + (2 * 60 * 1000) // One day and two minutes.
	const wantIllegalGlonassTimestamp = uint(7 << 27)
	const wantMultipleMessage = true
	const wantIssue = 3
	const wantTransTime = 4
	const wantClockSteeringIndicator = 5
	const wantExternalClockSteeringIndicator = 6
	const wantSmoothing = true
	const wantSmoothingInterval = 7
	wantStartOfGPSWeek := time.Date(2023, time.May, 13, 23, 59, 42, 0, utils.LocationUTC)
	wantUTCTimeFromGPSTimestamp := time.Date(2023, time.May, 13, 23, 59, 44, 0, utils.LocationUTC)
	wantStartOfGlonassWeek := time.Date(2023, time.May, 14, 0, 0, 0, 0, utils.LocationMoscow)
	wantUTCTimeFromGlonassTimestamp := time.Date(2023, time.May, 15, 0, 0, 2, 0, utils.LocationMoscow)

	const wantGPSDisplay = `Sent at 2023-05-13 23:59:44 +0000 UTC
Start of GPS week 2023-05-13 23:59:42 +0000 UTC plus timestamp 2000 (0d 0h 0m 2s 0ms)
stationID 1, multiple message, issue of data station 3
session transmit time 4, clock steering 5, external clock 6
divergence free smoothing true, smoothing interval 7
2 satellites, 3 signal types, 6 signals
`

	const wantGlonassDisplay = `Sent at 2023-05-15 00:00:02 +0300 MSK
Start of GPS week 2023-05-14 00:00:00 +0300 MSK plus timestamp 2000 (0d 0h 0m 2s 0ms)
stationID 1, multiple message, issue of data station 3
session transmit time 4, clock steering 5, external clock 6
divergence free smoothing true, smoothing interval 7
2 satellites, 3 signal types, 6 signals
`

	const wantGlonassDisplayIllegalDay = `Sent at 2023-05-14 00:00:00 +0300 MSK
Start of GPS week 2023-05-14 00:00:00 +0300 MSK plus timestamp 2000 (0d 0h 0m 2s 0ms)
stationID 1, multiple message, issue of data station 3
session transmit time 4, clock steering 5, external clock 6
divergence free smoothing true, smoothing interval 7
2 satellites, 3 signal types, 6 signals
`

	var testData = []struct {
		description          string
		MessageType          int
		timestamp            uint
		startOfWeek          time.Time
		utcTimeFromTimestamp time.Time
		wantDisplay          string
	}{
		{"GPS", 1074, wantGPSTimestamp, wantStartOfGPSWeek, wantUTCTimeFromGPSTimestamp, wantGPSDisplay},
		{"legal Glonass", 1084, wantGlonassTimestamp, wantStartOfGlonassWeek, wantUTCTimeFromGlonassTimestamp, wantGlonassDisplay},
		// Glonass timestamp with illegal day.  The resulting UTC time is ignored.
		{"illegal Glonass", 1084, wantIllegalGlonassTimestamp, wantStartOfGlonassWeek, wantStartOfGlonassWeek, wantGlonassDisplayIllegalDay},
	}
	for _, td := range testData {
		header := New(wantMessageType, wantStationID, wantGPSTimestamp,
			wantMultipleMessage,
			wantIssue, wantTransTime, wantClockSteeringIndicator,
			wantExternalClockSteeringIndicator, wantSmoothing, wantSmoothingInterval,
			wantSatelliteMask, wantSignalMask, wantCellMask)

		header.StartOfWeek = td.startOfWeek
		header.UTCTimeFromTimestamp = td.utcTimeFromTimestamp

		gotDisplay := header.String()

		if td.wantDisplay != gotDisplay {
			t.Errorf("%s: %s", td.description, diff.Diff(td.wantDisplay, gotDisplay))
		}
	}
}

// TestStringMultipleFlag checks that String() correctly handles the multiple message flag.
func TestStringMultipleFlag(t *testing.T) {
	const satMask = 3
	const sigMask = 7
	const cellMask = 1
	wantStartOfWeek := time.Date(2023, time.May, 13, 23, 59, 42, 0, utils.LocationUTC)
	wantUTCTimeFromTimestamp := time.Date(2023, time.May, 13, 23, 59, 45, 0, utils.LocationUTC)

	// The result contains "single message" or "multiple message", depending
	// on the multiple message flag.
	resultTemplate := `Sent at 2023-05-13 23:59:45 +0000 UTC
Start of GPS week 2023-05-13 23:59:42 +0000 UTC plus timestamp 3 (0d 0h 0m 0s 3ms)
stationID 2, %s, issue of data station 1
session transmit time 5, clock steering 6, external clock 7
divergence free smoothing true, smoothing interval 9
2 satellites, 3 signal types, 6 signals
`

	var testData = []struct {
		hdr                  *Header
		startOfWeek          time.Time
		utcTimeFromTimestamp time.Time
		want                 string
	}{
		{
			New(1074, 2, 3,
				false, 1, 5, 6, 7, true, 9, satMask, sigMask, cellMask),
			wantStartOfWeek,
			wantUTCTimeFromTimestamp,
			fmt.Sprintf(resultTemplate, "single message"),
		},

		{
			New(1074, 2, 3, true, 1, 5, 6, 7, true, 9, satMask, sigMask, cellMask),
			wantStartOfWeek,
			wantUTCTimeFromTimestamp,
			fmt.Sprintf(resultTemplate, "multiple message"),
		},
	}

	for _, td := range testData {
		td.hdr.StartOfWeek = td.startOfWeek
		td.hdr.UTCTimeFromTimestamp = td.utcTimeFromTimestamp

		got := td.hdr.String()

		if td.want != got {
			t.Errorf(diff.Diff(td.want, got))
		}

	}
}

// TestTimestampInString checks that String()correctly interprets the timestamp
// in an MSM.
func TestTimestampInString(t *testing.T) {
	const satMask = 3
	const sigMask = 7
	const cellMask = 1
	const wantGPSTimestamp = 2 * 1000                          // 2 seconds.
	const wantGlonassTimestamp = uint(1<<27) + (2 * 60 * 1000) // One day and two minutes.
	const wantIllegalGlonassTimestamp = uint(7 << 27)
	wantStartOfGPSWeek := time.Date(2023, time.May, 13, 23, 59, 42, 0, utils.LocationUTC)
	wantUTCTimeFromGPSTimestamp := time.Date(2023, time.May, 13, 23, 59, 44, 0, utils.LocationUTC)
	wantStartOfGlonassWeek := time.Date(2023, time.May, 14, 0, 0, 0, 0, utils.LocationMoscow).In(utils.LocationUTC)
	wantUTCTimeFromGlonassTimestamp := time.Date(2023, time.May, 15, 0, 0, 2, 0, utils.LocationMoscow).In(utils.LocationUTC)

	wantGPSDisplay := `Sent at 2023-05-13 23:59:44 +0000 UTC
Start of GPS week 2023-05-13 23:59:42 +0000 UTC plus timestamp 2000 (0d 0h 0m 2s 0ms)
stationID 2, single message, issue of data station 1
session transmit time 5, clock steering 6, external clock 7
divergence free smoothing true, smoothing interval 9
2 satellites, 3 signal types, 6 signals
`

	wantGlonassDisplay := `Sent at 2023-05-14 21:00:02 +0000 UTC
Start of Glonass week 2023-05-13 21:00:00 +0000 UTC plus timestamp 134337728 (1d 0h 2m 0s 0ms)
stationID 2, single message, issue of data station 1
session transmit time 5, clock steering 6, external clock 7
divergence free smoothing true, smoothing interval 9
2 satellites, 3 signal types, 6 signals
`

	wantGlonassDisplayWithIllegalDay := `Sent at 2023-05-14 21:00:02 +0000 UTC
timestamp 939524096 - illegal Glonass day
stationID 2, single message, issue of data station 1
session transmit time 5, clock steering 6, external clock 7
divergence free smoothing true, smoothing interval 9
2 satellites, 3 signal types, 6 signals
`

	var testData = []struct {
		hdr                  *Header
		startOfWeek          time.Time
		utcTimeFromTimestamp time.Time
		want                 string
	}{
		{
			New(1074, 2, wantGPSTimestamp, false, 1, 5, 6, 7, true, 9, satMask, sigMask, cellMask),
			wantStartOfGPSWeek,
			wantUTCTimeFromGPSTimestamp,
			wantGPSDisplay,
		},

		{
			New(1084, 2, wantGlonassTimestamp, false, 1, 5, 6, 7, true, 9, satMask, sigMask, cellMask),
			wantStartOfGlonassWeek,
			wantUTCTimeFromGlonassTimestamp,
			wantGlonassDisplay,
		},

		{
			New(1084, 2, wantIllegalGlonassTimestamp, false, 1, 5, 6, 7, true, 9, satMask, sigMask, cellMask),
			wantStartOfGlonassWeek,
			wantUTCTimeFromGlonassTimestamp, // placeholder - ignored.
			wantGlonassDisplayWithIllegalDay,
		},
	}

	for _, td := range testData {

		td.hdr.StartOfWeek = td.startOfWeek
		td.hdr.UTCTimeFromTimestamp = td.utcTimeFromTimestamp

		got := td.hdr.String()

		if td.want != got {
			t.Errorf(diff.Diff(td.want, got))
		}
	}
}

// TestGetSatellites checks that getSatellites deciphers a bit mask correctly.
func TestGetSatellites(t *testing.T) {
	// 1    5    9    13   17   21   25   29   33   37   41   45   49   53   57   61
	// 1000 0000 0000 0000 0100 0000 0000 0000 0100 0000 0000 0000 0000 0000 0000 0010
	const satMask = 0x8000000040000002
	want := []uint{1, 34, 63}
	got := getSatellites(satMask)

	if !utils.SlicesEqual(want, got) {
		t.Errorf("want %v got  %v", want, got)
	}
}

// TestGetSignals checks that getSignals deciphers a bit mask correctly.
func TestGetSignals(t *testing.T) {
	// 1    5    9    13   17   21   25   29
	// 0100 0000 0000 0000 1111 0000 0000 0001
	const satMask = 0x4000f001
	want := []uint{2, 17, 18, 19, 20, 32}

	got := getSignals(satMask)

	if !utils.SlicesEqual(want, got) {
		t.Errorf("want %v got  %v", want, got)
	}
}

// TestGetCells checks that getCells deciphers a bit mask correctly.
func TestGetCells(t *testing.T) {

	// <- 3X2 mask ->
	// 0001 1011 0000 | 0000 0000 0000 0000 0000 .....
	const cellMask uint64 = 0x1b00000000000000
	want := [][]bool{
		{false, false},
		{false, true},
		{true, false},
		{true, true},
	}
	got := getCells(cellMask, 4, 2)

	for i, _ := range want {
		for j, _ := range want[i] {
			if want[i][j] != got[i][j] {
				t.Errorf("expected [%d][%d] to be %v", i, j, want[1][1])
			}
		}
	}
}
