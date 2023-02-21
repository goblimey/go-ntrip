package message

import (
	"fmt"
	"testing"
	"time"

	"github.com/goblimey/go-ntrip/rtcm/header"
	"github.com/goblimey/go-ntrip/rtcm/msm7/satellite"
	"github.com/goblimey/go-ntrip/rtcm/msm7/signal"
	"github.com/goblimey/go-ntrip/rtcm/utils"
	"github.com/kylelemons/godebug/diff"
)

var message1077Bitstream = []byte{

	// A real RTCM message captured from a UBlox GPS device.  Type 1077
	// (an MSM7) carrying signals from GPS satellites, padded with null
	// bytes at the end. Bytes 6 and 7 (ox62, 0x00) contain the multiple
	// message flag (true), and the sequence number (zero), so this is
	// the first of a sequence of messages covering the same scan and it
	// only contains some of the signal cells.
	//         |-- multiple message flag
	//         | |-- sequence number
	//         v v
	// 0110 00|1|0 00|00 0000
	//
	// The header is 185 bits long, with 16 cell mask bits.
	// 
	/* 0 */ 0x43, 0x50, 0x00, 0x67, 0x00, 0x97, 0x62, 0x00,
	//                   64 bit satellite mask
	// 0|00|0 0|0|00   0|000 1000   0100 0000   1010 0000
	/* 8 */ 0x00, 0x08, 0x40, 0xa0,
	// 0110 0101   0000 0000   0000 0000   0000 0000
	/* 12 */ 0x65, 0x00, 0x00, 0x00,
	//               32 bit signal mask
	// 0000 0000   0|010 0000   0000 0000    1000 0000
	/* 16 */ 0x00, 0x20, 0x00, 0x80,
	//
	//               64 bit cell mask                 Satellite cells
	// 0000 0000   0|11|0 1|10|1   1|11|1  1|11|1   1|010   1000
	/* 20 */ 0x00, 0x6d, 0xff, 0xa8,
	// 1|010 1010   0|010 0110   0|010 0011   1|010 0110
	/* 24 */ 0xaa, 0x26, 0x23, 0xa6,
	// 1|010 0010   0|010 0011   0|010 0100   0|000 0|000
	/* 28 */ 0xa2, 0x23, 0x24, 0x00,
	// 0|000 0|000   0|000 0|000   0|000 0|000   0|011 0110
	/* 32 */ 0x00, 0x00, 0x00, 0x36,
	// 011|0 1000
	/* 36 */ 0x68, 0xcb, 0x83, 0x7a, // 36
	/* 40 */ 0x6f, 0x9d, 0x7c, 0x04, 0x92, 0xfe, 0xf2, 0x05,
	/* 48 */ 0xb0, 0x4a, 0xa0, 0xec, 0x7b, 0x0e, 0x09, 0x27,
	//                   Signal cells
	/* 56 */ 0xd0, 0x3f, 0x23, 0x7c, 0xb9, 0x6f, 0xbd, 0x73,
	/* 64 */ 0xee, 0x1f, 0x01, 0x64, 0x96, 0xf5, 0x7b, 0x27,
	/* 72 */ 0x46, 0xf1, 0xf2, 0x1a, 0xbf, 0x19, 0xfa, 0x08,
	/* 80 */ 0x41, 0x08, 0x7b, 0xb1, 0x1b, 0x67, 0xe1, 0xa6,
	0x70, 0x71, 0xd9, 0xdf, 0x0c, 0x61, 0x7f, 0x19, // 88
	0x9c, 0x7e, 0x66, 0x66, 0xfb, 0x86, 0xc0, 0x04, // 96
	0xe9, 0xc7, 0x7d, 0x85, 0x83, 0x7d, 0xac, 0xad, // 104
	0xfc, 0xbe, 0x2b, 0xfc, 0x3c, 0x84, 0x02, 0x1d, // 112
	0xeb, 0x81, 0xa6, 0x9c, 0x87, 0x17, 0x5d, 0x86, // 120
	0xf5, 0x60, 0xfb, 0x66, 0x72, 0x7b, 0xfa, 0x2f, // 128
	0x48, 0xd2, 0x29, 0x67, 0x08, 0xc8, 0x72, 0x15, // 136
	0x0d, 0x37, 0xca, 0x92, 0xa4, 0xe9, 0x3a, 0x4e, // 144
	0x13, 0x80, 0x00, 0x14, 0x04, 0xc0, 0xe8, 0x50, // 152
	0x16, 0x04, 0xc1, 0x40, 0x46, 0x17, 0x05, 0x41, // 160
	0x70, 0x52, 0x17, 0x05, 0x01, 0xef, 0x4b, 0xde, // 168
	0x70, 0x4c, 0xb1, 0xaf, 0x84, 0x37, 0x08, 0x2a, // 176
	0x77, 0x95, 0xf1, 0x6e, 0x75, 0xe8, 0xea, 0x36, // 184
	0x1b, 0xdc, 0x3d, 0x7a, 0xbc, 0x75, 0x42, 0x80, // 192
	// Padding bytes.
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // 196
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00,
}

// TestString checks that String correctly displays a message.
func TestString(t *testing.T) {
	// These values and results are copied from tests of satellites, signal etc.
	const rangeWhole uint = 0x80       // 1000 0000 (128 millis)
	const rangeFractional uint = 0x200 // 10 bits 1000 ... (0.5 millis)
	const rangeDelta = int(0x40000)    // 20 bits 0100 ...
	const rangeMS = 128.5
	const twoToPower11 = 0x800 //                        1000 0000 0000
	const twoToPowerMinus11 = float64(1) / twoToPower11
	const twoToPower31 = 0x80000000 // 1000 0000 0000 0000 0000 0000 0000 0000
	const twoToPowerMinus31 = 1 / float64(twoToPower31)
	const rangeMilliseconds = 128.5 + float64(twoToPowerMinus31)
	const wavelength = utils.SpeedOfLightMS / utils.Freq2
	const signalID1 uint = 16
	const signalID2 uint = 5
	const extendedInfo = 5
	const phaseRangeDelta int = 1
	const wholePhaseRangeRate = 6
	const phaseRangeRateDelta = 7000
	const phaseRangeRate = 6.7
	const lockTimeIndicator = 4
	const halfCycleAmbiguity = true
	const wantRange float64 = (128.5 + twoToPowerMinus11) * utils.OneLightMillisecond // 11 1111 1111

	invalidPhaseRangeRateBytes := []byte{0x80, 00} // 14 bits plus filler: 1000 0000 0000 00 filler 00
	invalidPhaseRangeRate := int(utils.GetBitsAsInt64(invalidPhaseRangeRateBytes, 0, 14))

	rangeLM := rangeMilliseconds * utils.OneLightMillisecond
	wantPhaseRange := rangeLM / wavelength

	utc, _ := time.LoadLocation("UTC")
	utcTime := time.Date(2023, time.February, 14, 1, 2, 3, int(4*time.Millisecond), utc)

	// Satellite mask with bits set for satellites 1 and 2.
	const satMask uint64 = 0xc00000000000000
	// Signal mask with bits set for signals 5 and 16
	const sigMask uint32 = 0x08010000
	// Cell mask for two satellites, two siganls, with bits set for satellite 1
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
time 2023-02-14 01:02:03.004 +0000 UTC (epoch time 3)
stationID 2, single message, sequence number 1, session transmit time 5
clock steering 6, external clock 7
divergence free smoothing true, smoothing interval 9
2 satellites, 2 signal types, 4 signals
Satellite ID {range m, extended info, phase range rate}:
 1 {%.3f, 5, 6}
 2 {invalid, 5, invalid}
Signals: sat ID sig ID {range, phase range, lock time ind, half cycle ambiguity, Carrier Noise Ratio, phase range rate}:
 1 16 {%.3f, %.3f, %d, true, 5, %.3f}
 2  5 {invalid, invalid, 4, true, 6, invalid}
 2  5 {invalid, invalid, 7, false, 8, invalid}
`
	want := fmt.Sprintf(resultTemplate,
		rangeMS, wantRange, wantPhaseRange, lockTimeIndicator, phaseRangeRate)

	header :=
		header.New(1077, 2, 3, false, 1, 5, 6, 7, true, 9, satMask, sigMask, cellMask)
	header.UTCTime = utcTime

	message := New(header, satellites, signals)

	got := message.String()

	if want != got {
		t.Errorf("results differ:\n%s", diff.Diff(want, got))
	}
}

// TestGetMessageWithErrors checks that GetMessage handles errors correctly.
func TestGetMessageWithErrors(t *testing.T) {
	// GetMessage responds with an error message at various points if the
	// bitstream is too short.

	var testData = []struct {
		Description string
		BitStream   []byte
		Want        string
	}{
		{
			"header too short", message1077Bitstream[:21],
			"bitstream is too short for an MSM header - got 168 bits, expected at least 169",
		},
		{
			"satellite cells too short", message1077Bitstream[:57],
			"overrun - not enough data for 8 MSM7 satellite cells - need 288 bits, got 271",
		},
		{
			"Signal cells too short", message1077Bitstream[:69],
			"overrun - want at least one 80-bit signal cell when multiple message flag is set, got only 79 bits left",
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
time 2023-02-14 01:02:03.004 +0000 UTC (epoch time 432023000)
stationID 0, multiple message, sequence number 0, session transmit time 0
clock steering 0, external clock 0
divergence free smoothing false, smoothing interval 0
8 satellites, 2 signal types, 16 signals
Satellite ID {range m, extended info, phase range rate}:
 4 {81.425, 0, -135}
 9 {84.274, 0, 182}
16 {76.438, 0, 597}
18 {71.738, 0, 472}
25 {77.871, 0, -633}
26 {68.921, 0, 292}
29 {70.502, 0, -383}
31 {72.286, 0, -442}
Signals: sat ID sig ID {range, phase range, lock time ind, half cycle ambiguity, Carrier Noise Ratio, phase range rate}:
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
	message, err := GetMessage(message1077Bitstream)
	if err != nil {
		t.Fatal(err)
	}

	// An arbitrary UTC time value - does not match the epoch date.
	utc, _ := time.LoadLocation("UTC")
	utcTime := time.Date(2023, time.February, 14, 1, 2, 3, int(4*time.Millisecond), utc)
	message.Header.UTCTime = utcTime

	got := message.String()

	if want != got {
		t.Errorf("results differ:\n%s", diff.Diff(want, got))
	}
}
