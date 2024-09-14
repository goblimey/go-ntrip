package message

import (
	"log/slog"
	"testing"

	"github.com/goblimey/go-ntrip/rtcm/header"
	"github.com/goblimey/go-ntrip/rtcm/testdata"
	"github.com/goblimey/go-ntrip/rtcm/type_msm7/satellite"
	"github.com/goblimey/go-ntrip/rtcm/type_msm7/signal"
	"github.com/goblimey/go-ntrip/rtcm/utils"

	"github.com/kylelemons/godebug/diff"
)

// TestString checks that String correctly displays a message.
func TestString(t *testing.T) {
	// These values and results are copied from tests of satellites, signal etc.
	const rangeWhole uint = 0x80       // 1000 0000 (128 millis)
	const rangeFractional uint = 0x200 // 10 bits 1000 ... (0.5 millis)
	const rangeDelta = int(0x40000)    // 20 bits 0100 ...
	const wavelength = utils.SpeedOfLightMS / utils.Freq2
	const phaseRangeDelta = 1
	const signalID1 uint = 16
	const signalID2 uint = 5
	const extendedInfo = 5

	const wholePhaseRangeRate = 6
	const phaseRangeRateDelta = 7000
	const lockTimeIndicator = 4
	const halfCycleAmbiguity = true

	invalidPhaseRangeRateBytes := []byte{0x80, 00} // 14 bits plus filler: 1000 0000 0000 00 filler 00
	invalidPhaseRangeRate := int(utils.GetBitsAsInt64(invalidPhaseRangeRateBytes, 0, 14))

	// Satellite mask with bits set for satellites 1 and 2.
	const satMask uint64 = 0xc00000000000000
	// Signal mask with bits set for signals 5 and 16
	const sigMask uint32 = 0x08010000
	// Cell mask for two satellites, two signals, with bits set for satellite 1
	// second signal, satellite 2 first signal and satellite 2 second signal.
	const cellMask uint64 = 7

	// Satellite and signal cells to match the masks.
	satelliteCell1 := satellite.New(1, rangeWhole, rangeFractional,
		extendedInfo, wholePhaseRangeRate,
		slog.LevelDebug)
	satelliteCell2 := satellite.New(2, utils.InvalidRange, rangeFractional,
		extendedInfo, invalidPhaseRangeRate,
		slog.LevelDebug)
	satellites := []satellite.Cell{*satelliteCell1, *satelliteCell2}
	signalCell1 := signal.New(signalID1, satelliteCell1, rangeDelta, phaseRangeDelta,
		lockTimeIndicator, halfCycleAmbiguity, 5, phaseRangeRateDelta, wavelength,
		slog.LevelDebug)
	signalCell2 := signal.New(signalID2, satelliteCell2, rangeDelta, phaseRangeDelta,
		lockTimeIndicator, halfCycleAmbiguity, 6, phaseRangeRateDelta, wavelength,
		slog.LevelDebug)
	signalCell3 := signal.New(signalID2, satelliteCell2, rangeDelta, phaseRangeDelta,
		7, false, 8, phaseRangeRateDelta, wavelength,
		slog.LevelDebug)

	signalRow1 := []signal.Cell{*signalCell1}
	signalRow2 := []signal.Cell{*signalCell2, *signalCell3}
	signals := [][]signal.Cell{signalRow1, signalRow2}

	const want = `stationID 2, single message, issue of data station 1
session transmit time 5, clock steering 6, external clock 7
divergence free smoothing true, smoothing interval 9
Satellite mask:
0000 1100 0000 0000  0000 0000 0000 0000  0000 0000 0000 0000  0000 0000 0000 0000
Signal mask: 0000 1000 0000 0001  0000 0000 0000 0000
cell mask: ft tt
2 satellites, 2 signal types, 3 signals
Satellite ID {approx range - whole, frac, millis, metres, extended info, phase range rate}:
 1 {128, 512, 128.500, 38523330.853, 5, 6}
 2 {invalid, 5, invalid}
Signals: sat ID sig ID {range m, phase range, phase range rate doppler Hz, phase range rate m/s, lock time ind, half cycle ambiguity, Carrier Noise Ratio, wavelength}:
 1 16 {(262144, 146.383, 38523477.236), (1, 157746600.001), -27.435, (7000, 0.700, 6.700), 4, true, 5, 0.244}
 2  5 {invalid, invalid, invalid, invalid, 4, true, 6, 0.244}
 2  5 {invalid, invalid, invalid, invalid, 7, false, 8, 0.244}
`

	header := header.New(
		1077, 2, 3, false, 1, 5, 6, 7, true, 9,
		satMask, sigMask, cellMask, slog.LevelDebug)

	message := New(header, satellites, signals, slog.LevelDebug)

	got := message.String()

	if want != got {
		t.Errorf("results differ:\n%s", diff.Diff(want, got))
	}
}

// TestGetMessage checks GetMessage.
func TestGetMessage(t *testing.T) {

	gotMessage, gotError := GetMessage(
		testdata.MessageFrame1077, slog.LevelDebug)
	if gotError != nil {
		t.Errorf(gotError.Error())
	}

	if gotMessage.Header.MessageType != 1077 {
		t.Errorf("want %d got %d", 1077, gotMessage.Header.MessageType)
	}

}

// TestGetMessageWithErrors checks that GetMessage handles errors correctly.
func TestGetMessageWithErrors(t *testing.T) {
	// GetMessage responds with an error message at various points if the
	// bit stream is too short.

	var testData = []struct {
		Description string
		BitStream   []byte
		Want        string
	}{
		{
			"header too short", testdata.MessageFrameType1077[:27],
			"bitstream is too short for an MSM header - got 168 bits, expected at least 169",
		},
		{
			"satellite cells too short", testdata.MessageFrameType1077[:60],
			"overrun - not enough data for 8 MSM7 satellite cells - need 288 bits, got 271",
		},
		{
			"Signal cells too short", testdata.MessageFrameType1077[:72],
			"overrun - want at least one 80-bit signal cell when multiple message flag is set, got only 79 bits left",
		},
		{
			"notMSM7", testdata.MessageFrameType1074_1,
			"message type 1074 is not an MSM7",
		},
	}
	for _, td := range testData {
		gotMessage, gotError := GetMessage(
			td.BitStream, slog.LevelDebug)
		if gotMessage != nil {
			t.Error("On error, the message should be nil")
		}
		if gotError == nil {
			t.Error("expected an error")
		} else {
			if gotError.Error() != td.Want {
				t.Errorf("%s:\nwant %s\n got %s", td.Description, td.Want, gotError.Error())
			}
		}
	}
}

// TestDisplaySatelliteCellsWithError checks that DisplaySatelliteCells
// produces the correct result when there are no satellite cells.
func TestDisplaySatelliteCellsWithError(t *testing.T) {
	const want = "No satellites\n"

	// A message with a nil satellite slice.
	var message Message

	got1 := message.DisplaySatelliteCells()

	if want != got1 {
		t.Errorf("want %s, got %s", want, got1)
	}

	// Now with an empty slice.
	message.Satellites = make([]satellite.Cell, 0)

	got2 := message.DisplaySatelliteCells()

	if want != got2 {
		t.Errorf("want %s, got %s", want, got2)
	}
}

// TestDisplaySignalCellsWithError checks that DisplaySignalCells
// produces the correct result when there are no signal cells.
func TestDisplaySignalCellsWithError(t *testing.T) {
	const want = "No signals\n"

	// A message with a nil satellite slice.
	var message Message

	got1 := message.DisplaySignalCells()

	if want != got1 {
		t.Errorf("want %s, got %s", want, got1)
	}

	// Now with an empty slice.
	message.Satellites = make([]satellite.Cell, 0)

	got2 := message.DisplaySignalCells()

	if want != got2 {
		t.Errorf("want %s, got %s", want, got2)
	}
}

// TestGetMessageFromRealData is a regression test using real data.
func TestGetMessageFromRealData(t *testing.T) {

	const want = testdata.WantDebugHeaderFromMessageFrameType1077 + "\n" +
		testdata.WantSatelliteListFromMessageFrameType1077 + "\n" +
		testdata.WantDebugSignalListFromMessageFrameType1077 + "\n"

	message, err := GetMessage(
		testdata.MessageFrameType1077, slog.LevelDebug)
	if err != nil {
		t.Fatal(err)
	}

	got := message.String()

	if want != got {
		t.Errorf("results differ:\n%s", diff.Diff(want, got))
	}
}

// TestGlonassBugFix tests the fix for a bug in Glonass handling.
func TestGlonassBugFix(t *testing.T) {
	// This bitstream resulted in infinite values for the phase range
	// and phase range rate doppler.
	bitStream := []byte{
		0xd3, 0x00, 0xc3, 0x43, 0xf0, 0x00, 0xa2, 0x93, 0x7c, 0x26, 0x00, 0x00, 0x60, 0x41, 0xe0, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x20, 0x80, 0x00, 0x00, 0x7f, 0xfe, 0x94, 0x90, 0x96, 0x92, 0x88, 0x82,
		0x85, 0x06, 0xae, 0xd6, 0x85, 0x73, 0x4c, 0x65, 0xe0, 0xef, 0x37, 0xcf, 0x3e, 0xfd, 0x9e, 0x11,
		0x20, 0x47, 0x3e, 0x7b, 0xfe, 0xed, 0xf6, 0x68, 0x25, 0xc1, 0x31, 0x10, 0x12, 0xc3, 0xbe, 0xb4,
		0x19, 0xea, 0x40, 0x98, 0x63, 0x45, 0x83, 0x52, 0x65, 0xd2, 0x02, 0x5d, 0x20, 0x86, 0xc1, 0x6c,
		0x6a, 0xf3, 0xe3, 0x4c, 0x34, 0x32, 0x0b, 0x9a, 0x17, 0x07, 0xa1, 0x9d, 0x20, 0x55, 0x26, 0x00,
		0x4e, 0xcb, 0x3f, 0xb6, 0x65, 0x1f, 0xa9, 0x0d, 0x7e, 0x12, 0x2b, 0x3e, 0x0c, 0xef, 0xc1, 0x69,
		0xb8, 0xe1, 0x65, 0x84, 0x41, 0xbf, 0x6d, 0x21, 0x5c, 0x81, 0x80, 0xdf, 0x58, 0x60, 0xc0, 0x6e,
		0x5e, 0x96, 0x8d, 0xbe, 0xa0, 0x14, 0xd2, 0xa4, 0xa7, 0x14, 0x44, 0xf1, 0x1c, 0x47, 0x3b, 0xce,
		0xf2, 0xec, 0xb9, 0x38, 0xce, 0x32, 0xcc, 0xb2, 0x00, 0x05, 0x21, 0x20, 0x62, 0x14, 0x05, 0x41,
		0x58, 0x60, 0x15, 0x85, 0x81, 0x48, 0x64, 0x16, 0x06, 0x01, 0x58, 0x77, 0xb2, 0xef, 0x50, 0x91,
		0xb5, 0x22, 0x29, 0xd9, 0x73, 0xbc, 0xd7, 0x00, 0x2e, 0x05, 0xf8, 0x45, 0xf0, 0xab, 0x8f, 0xe7,
		0x22, 0x39, 0x8d, 0x43, 0x18, 0x40, 0x1c, 0x54, 0xe8,
	}

	m, err := GetMessage(bitStream, slog.LevelDebug)
	if err != nil {
		t.Error(err)
	}

	s := m.String()

	_ = s
}
