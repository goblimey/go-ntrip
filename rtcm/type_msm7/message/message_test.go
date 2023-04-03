package message

import (
	"fmt"
	"testing"

	"github.com/goblimey/go-ntrip/rtcm/header"
	"github.com/goblimey/go-ntrip/rtcm/type_msm7/satellite"
	"github.com/goblimey/go-ntrip/rtcm/type_msm7/signal"
	"github.com/goblimey/go-ntrip/rtcm/testdata"
	"github.com/goblimey/go-ntrip/rtcm/utils"

	"github.com/kylelemons/godebug/diff"
)

// TestString checks that String correctly displays a message.
func TestString(t *testing.T) {
	// These values and results are copied from tests of satellites, signal etc.
	const rangeWhole uint = 0x80       // 1000 0000 (128 millis)
	const rangeFractional uint = 0x200 // 10 bits 1000 ... (0.5 millis)
	const rangeDelta = int(0x40000)    // 20 bits 0100 ...
	const approxRange = 128.5
	const twoToPower11 = 0x800 // 1000 0000 0000
	const twoToPowerMinus11 = 1 / float64(twoToPower11)
	const twoToPower31 = 0x80000000 // 1000 0000 0000 0000 0000 0000 0000 0000
	const twoToPowerMinus31 = 1 / float64(twoToPower31)
	const rangeMilliseconds = approxRange + float64(twoToPowerMinus11)
	const wantApproxRangeInMetres = approxRange * utils.OneLightMillisecond
	const wantRangeInMetres = rangeMilliseconds * utils.OneLightMillisecond
	const wavelength = utils.SpeedOfLightMS / utils.Freq2
	const phaseRangeDelta = 1
	wantPhaseRange := (approxRange + (phaseRangeDelta * twoToPowerMinus31)) * utils.OneLightMillisecond / wavelength

	const signalID1 uint = 16
	const signalID2 uint = 5
	const extendedInfo = 5

	const wholePhaseRangeRate = 6
	const phaseRangeRateDelta = 7000
	const phaseRangeRate = 6.7
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
		extendedInfo, wholePhaseRangeRate)
	satelliteCell2 := satellite.New(2, utils.InvalidRange, rangeFractional,
		extendedInfo, invalidPhaseRangeRate)
	satellites := []satellite.Cell{*satelliteCell1, *satelliteCell2}
	signalCell1 := signal.New(signalID1, satelliteCell1, rangeDelta, phaseRangeDelta,
		lockTimeIndicator, halfCycleAmbiguity, 5, phaseRangeRateDelta, wavelength)
	signalCell2 := signal.New(signalID2, satelliteCell2, rangeDelta, phaseRangeDelta,
		lockTimeIndicator, halfCycleAmbiguity, 6, phaseRangeRateDelta, wavelength)
	signalCell3 := signal.New(signalID2, satelliteCell2, rangeDelta, phaseRangeDelta,
		7, false, 8, phaseRangeRateDelta, wavelength)

	signalRow1 := []signal.Cell{*signalCell1}
	signalRow2 := []signal.Cell{*signalCell2, *signalCell3}
	signals := [][]signal.Cell{signalRow1, signalRow2}

	resultTemplate := `type 1077 GPS Full Pseudoranges and PhaseRanges plus CNR (high resolution)
stationID 2, timestamp 3, single message, sequence number 1
session transmit time 5, clock steering 6, external clock 7
divergence free smoothing true, smoothing interval 9
2 satellites, 2 signal types, 4 signals
Satellite ID {approx range m, extended info, phase range rate}:
 1 {%.3f, 5, 6}
 2 {invalid, 5, invalid}
Signals: sat ID sig ID {range m, phase range, lock time ind, half cycle ambiguity, Carrier Noise Ratio, phase range rate}:
 1 16 {%.3f, %.3f, %d, true, 5, %.3f}
 2  5 {invalid, invalid, 4, true, 6, invalid}
 2  5 {invalid, invalid, 7, false, 8, invalid}
`
	want := fmt.Sprintf(resultTemplate,
		wantApproxRangeInMetres, wantRangeInMetres, wantPhaseRange,
		lockTimeIndicator, phaseRangeRate)

	header :=
		header.New(1077, 2, 3, false, 1, 5, 6, 7, true, 9, satMask, sigMask, cellMask)

	message := New(header, satellites, signals)

	got := message.String()

	if want != got {
		t.Errorf("results differ:\n%s", diff.Diff(want, got))
	}
}

// TestGetMessage checks GetMessage.
func TestGetMessage(t *testing.T) {

	gotMessage, gotError := GetMessage(testdata.MessageFrame1077)
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
			"header too short", testdata.Message1077[:27],
			"bitstream is too short for an MSM header - got 168 bits, expected at least 169",
		},
		{
			"satellite cells too short", testdata.Message1077[:60],
			"overrun - not enough data for 8 MSM7 satellite cells - need 288 bits, got 271",
		},
		{
			"Signal cells too short", testdata.Message1077[:72],
			"overrun - want at least one 80-bit signal cell when multiple message flag is set, got only 79 bits left",
		},
		{
			"notMSM7", testdata.MessageType1074,
			"message type 1074 is not an MSM7",
		},
	}
	for _, td := range testData {
		gotMessage, gotError := GetMessage(td.BitStream)
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

	// This is just copied from the first run of the test.  Given that
	// the rest of the tests exercise the code properly, it's assumed
	// that this result is correct.  The purpose of the test is to
	// detect any future changes that break it.
	const want = `type 1077 GPS Full Pseudoranges and PhaseRanges plus CNR (high resolution)
stationID 0, timestamp 432023000, multiple message, sequence number 0
session transmit time 0, clock steering 0, external clock 0
divergence free smoothing false, smoothing interval 0
8 satellites, 2 signal types, 16 signals
Satellite ID {approx range m, extended info, phase range rate}:
 4 {24410542.339, 0, -135}
 9 {25264833.738, 0, 182}
16 {22915678.774, 0, 597}
18 {21506595.669, 0, 472}
25 {23345166.602, 0, -633}
26 {20661965.550, 0, 292}
29 {21135953.821, 0, -383}
31 {21670837.435, 0, -442}
Signals: sat ID sig ID {range m, phase range, lock time ind, half cycle ambiguity, Carrier Noise Ratio, phase range rate}:
 4  2 {24410527.355, 128278179.264, 582, false, 640, -135.107}
 4 16 {24410523.313, 99956970.352, 581, false, 608, -135.107}
 9 16 {25264751.952, 103454935.508, 179, false, 464, 182.123}
16  2 {22915780.724, 120423177.179, 529, false, 640, 597.345}
18  2 {21506547.550, 113017684.727, 579, false, 704, 472.432}
18 16 {21506542.760, 88065739.822, 578, false, 608, 472.418}
25  2 {23345103.037, 122679365.321, 646, false, 640, -633.216}
25 16 {23345100.838, 95594272.692, 623, false, 560, -633.187}
26  2 {20662003.308, 108579565.367, 596, false, 736, 292.755}
26 16 {20662000.914, 84607418.613, 596, false, 672, 292.749}
29  2 {21136079.188, 111070868.860, 628, false, 736, -383.775}
29 16 {21136074.598, 86548719.034, 628, false, 656, -383.770}
31  2 {21670772.711, 113880577.055, 624, false, 736, -442.539}
31 16 {21670767.783, 88738155.231, 624, false, 640, -442.550}
`
	// message, err := GetMessage(message1077Bitstream)
	message, err := GetMessage(testdata.Message1077)
	if err != nil {
		t.Fatal(err)
	}

	got := message.String()

	if want != got {
		t.Errorf("results differ:\n%s", diff.Diff(want, got))
	}
}
