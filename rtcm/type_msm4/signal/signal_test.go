package signal

import (
	"fmt"
	"testing"

	"github.com/goblimey/go-ntrip/rtcm/header"
	"github.com/goblimey/go-ntrip/rtcm/type_msm4/satellite"
	"github.com/goblimey/go-ntrip/rtcm/utils"

	"github.com/google/go-cmp/cmp"
)

// Tests for the handling of an MSM4 signal cell.

// TestNew checks that New correctly creates an MSM4 signal cell.
func TestNew(t *testing.T) {

	const rangeDelta = 5
	const phaseRangeDelta = 6
	const lockTimeIndicator = 7
	const halfCycleAmbiguity = true
	const cnr = 8
	const satelliteID = 1
	const rangeWhole = 2
	const rangeFractional = 3
	const wavelength = 4.0

	satCell := satellite.New(satelliteID, rangeWhole, rangeFractional)

	var testData = []struct {
		Comment            string
		ID                 uint
		SatelliteCell      *satellite.Cell
		RangeDelta         int
		PhaseRangeDelta    int
		LockTimeIndicator  uint
		HalfCycleAmbiguity bool
		CNR                uint
		Wavelength         float64
		Want               Cell // expected result
	}{
		{"all values", 1, satCell, rangeDelta, phaseRangeDelta, lockTimeIndicator, halfCycleAmbiguity, cnr, wavelength,
			Cell{ID: 1,
				Wavelength:          wavelength,
				RangeDelta:          rangeDelta,
				PhaseRangeDelta:     phaseRangeDelta,
				LockTimeIndicator:   lockTimeIndicator,
				HalfCycleAmbiguity:  halfCycleAmbiguity,
				CarrierToNoiseRatio: cnr,
				Satellite:           satCell}},
		{"nil satellite", 2, nil, rangeDelta, phaseRangeDelta, lockTimeIndicator, halfCycleAmbiguity, cnr, 0.0,
			Cell{ID: 2,
				Wavelength:          0.0,
				RangeDelta:          rangeDelta,
				PhaseRangeDelta:     phaseRangeDelta,
				LockTimeIndicator:   lockTimeIndicator,
				HalfCycleAmbiguity:  halfCycleAmbiguity,
				CarrierToNoiseRatio: cnr,
				Satellite:           nil}},
	}
	for _, td := range testData {
		got := *New(
			td.ID, td.SatelliteCell, td.RangeDelta, td.PhaseRangeDelta, td.LockTimeIndicator,
			td.HalfCycleAmbiguity, td.Want.CarrierToNoiseRatio, td.Wavelength)
		if got != td.Want {
			t.Errorf("%s: want %v, got %v", td.Comment, td.Want, got)
		}
	}
}

// TestGetSignalCells checks that getSignalCells correctly interprets a
// bit stream from an MSM4 message containing three signal cells.
func TestGetSignalCells(t *testing.T) {

	// Two satellites.
	const satelliteID0 = 42
	const satelliteID1 = 43
	// We receive up to 2 signals from each satellite.
	const signalID0 = 8
	const signalID1 = 3
	wavelength0 := utils.GetSignalWavelength("GPS", signalID0)
	wavelength1 := utils.GetSignalWavelength("GPS", signalID1)

	const rangeWhole0 = 0x81
	const rangeWhole1 = 1
	const rangeFractional0 = 0x21
	const rangeFractional1 = 2
	satellites := []uint{satelliteID0, satelliteID1}

	signals := []uint{signalID0, signalID1}
	// Satellite 42 received signals 5 and 7, satellite 43 received signal 5 only.
	cellMask := [][]bool{{true, true}, {true, false}}
	header := header.Header{MessageType: 1074, Constellation: "GPS",
		NumSignalCells: 3, Satellites: satellites, Signals: signals,
		Cells: cellMask}
	satData := []satellite.Cell{
		{ID: satelliteID0,
			RangeWholeMillis: rangeWhole0, RangeFractionalMillis: rangeFractional0},
		{ID: satelliteID1,
			RangeWholeMillis: rangeWhole1, RangeFractionalMillis: rangeFractional1},
	}

	// The signal cells start at bit 8 and contains three
	// 15-bit signed range delta (8193, -1, 0), followed by three 22-bit signed
	// phase range delta (-1, 0, 1), three 4-bit unsigned phase lock time
	// indicators (0xf, 0, 1), three single bit half-cycle ambiguity indicators
	// (t, f, t), three 6-bit unsigned GNSS Signal Carrier to Noise Ratio (CNR)
	// (33, 0, 32).  48 bits per signal, so 144 bits in all, set like so:
	// 0100 0000   0000 001|1  1111 1111   1111 11|00   0000 0000   0000 0|111
	// 1111 1111   1111 1111   111|00000   0000 0000    0000 0000   0|000 0000
	// 0000 0000   0000 001|1  111|0000|0   001|1|0|1|100001|0000   00|10 0000|
	// The 24-bit CRC follows.
	bitStream := []byte{
		0x00, 0x40, 0x03, 0xff, 0xfc, 0x00, 0x07, 0xff,
		0xff, 0xe0, 0x00, 0x00, 0x00, 0x00, 0x03, 0xe0,
		0x36, 0x10, 0x20,
		0, 0, 0,
	}
	const startPosition = 8

	// Wanted values from the three signal cells created below.
	// We received signals with 8 and 9 are from satellite 0, and
	// signal 8 from satellite 1.
	wantSatelliteID := []uint{satelliteID0, satelliteID1}
	wantRangeWhole := []uint{rangeWhole0, rangeWhole1}
	wantRangeFractional := []uint{rangeFractional0, rangeFractional1}

	satCell := []*satellite.Cell{
		satellite.New(wantSatelliteID[0], wantRangeWhole[0], wantRangeFractional[0]),
		satellite.New(wantSatelliteID[1], wantRangeWhole[1], wantRangeFractional[1]),
	}
	
	wantID := []uint{signalID0, signalID1, signalID0}
	wantWavelength := []float64{wavelength0, wavelength1, wavelength0}
	wantRangeDelta := []int{8193, -1, 0}
	wantPhaseRangeDelta := []int{-1, 0, 1}
	wantLockTimeIndicator := []uint{15, 0, 1}
	wantHalfCycleAmbiguity := []bool{true, false, true}
	wantCarrierToNoiseRatio := []uint{33, 0, 32}

	
	want := make([]Cell, 0)

	for i := range wantID {
		cell := Cell{
			ID:                  wantID[i],
			Wavelength:          wantWavelength[i],
			RangeDelta:          wantRangeDelta[i],
			PhaseRangeDelta:     wantPhaseRangeDelta[i],
			LockTimeIndicator:   wantLockTimeIndicator[i],
			HalfCycleAmbiguity:  wantHalfCycleAmbiguity[i],
			CarrierToNoiseRatio: wantCarrierToNoiseRatio[i],
		}
		want = append(want, cell)
	}

	want[0].Satellite = satCell[0]
	want[1].Satellite = satCell[0]
	want[2].Satellite = satCell[1]

	got, err := GetSignalCells(bitStream, startPosition, &header, satData)

	if err != nil {
		t.Errorf(err.Error())
	}

	if len(got) != 2 {
		t.Errorf("got %d outer slices, expected 2", len(got))
	}

	if len(got[0]) != 2 {
		t.Errorf("got[0] contains %d cells, expected 2", len(got[0]))
	}
	if len(got[1]) != 1 {
		t.Errorf("got[1] contains %d cells, expected 1", len(got[1]))
	}

	if !cmp.Equal(got[0][0], want[0]) {
		t.Errorf("expected [0][0]\n%v got\n%v", want[0], got[0][0])
	}

	if !cmp.Equal(got[0][1], want[1]) {
		t.Errorf("expected [0][1]\n%v got\n%v", want[1], got[0][1])
	}


	if !cmp.Equal(got[1][0], want[2]) {
		t.Errorf("expected [1][0]\n%v got\n%v", want[2], got[1][0])
	}
}

// TestGetMS4SignalCellsWithShortBitStream checks that GetMSMSignalCells produces
// the correct error message if the bitstream is too short.
func TestGetMS4SignalCellsWithShortBitStream(t *testing.T) {
	const signalID1 = 7
	const satelliteID0 = 42
	const satelliteID1 = 43
	satellites := []uint{satelliteID0, satelliteID1}
	const signalID0 = 5
	signals := []uint{signalID0, signalID1}
	// Satellite 42 received signals 5 and 7, satellite 43 received signal 5 only.
	cellMask := [][]bool{{true, true}, {true, false}}
	headerForSingleMessage := header.Header{MessageType: 1074, MultipleMessage: false,
		NumSignalCells: 3, Satellites: satellites, Signals: signals, Cells: cellMask}
	headerForMultiMessage := header.Header{MessageType: 1074, MultipleMessage: true,
		NumSignalCells: 3, Satellites: satellites, Signals: signals, Cells: cellMask}
	satData := []satellite.Cell{
		satellite.Cell{ID: satelliteID0,
			RangeWholeMillis: 0x81, RangeFractionalMillis: 0x201},
		satellite.Cell{ID: satelliteID1,
			RangeWholeMillis: 1, RangeFractionalMillis: 2}}

	// The bit stream is taken from a working example.
	// It contains three signal cells - three 15-bit signed range
	// delta, followed by three 22-bit signed phase range delta, three 4-bit
	// unsigned phase lock time indicators, three single bit half-cycle ambiguity
	// indicators, three 6-bit unsigned GNSS Signal Carrier to Noise Ratio (CNR)
	// values (48 bits per signal, so 144 bits in all) set like so:
	// 0100 0000  0000 0011  1111 1111  1111 1100  0000 0000  0000 0111
	// 1111 1111  1111 1111  1110 0000  0000 0000  0000 0000  0000 0000
	// 0000 0000  0000 0011  1110 0000  0011 0110  0001 0000  0010 0000
	// The 24-bit CRC follows.
	bitStream := []byte{
		0x40, 0x03, 0xff, 0xfc, 0x00, 0x07,
		0xff, 0xff, 0xe0, 0x00, 0x00, 0x00,
		0x00, 0x03, 0xe0, 0x36, 0x10, 0x20,
		0, 0, 0,
	}

	// The test calls provide only part of the bitstream, to provoke an overrun error.
	var testData = []struct {
		description string
		header      *header.Header
		bitStream   []byte
		want        string
	}{
		{
			"single", &headerForSingleMessage, bitStream[:17],
			"overrun - want 3 MSM4 signals, got 2",
		},
		{
			"multiple", &headerForMultiMessage, bitStream[:8],
			"overrun - want at least one 48-bit signal cell when multiple message flag is set, got only 40 bits left",
		},
	}

	for _, td := range testData {

		// Expect an error.
		gotMessage, gotError := GetSignalCells(td.bitStream, 0, td.header, satData)

		if gotMessage != nil {
			t.Errorf("%s: expected a nil message and an error", td.description)
		}

		// Check the error.
		if gotError == nil {
			t.Errorf("%s: expected an overrun error", td.description)
			return
		}

		if gotError.Error() != td.want {
			t.Errorf("%s: expected the error\n\"%s\"\ngot \"%s\"",
				td.description, td.want, gotError.Error())
			return
		}
	}
}

// TestGetAggregateRange checks the MSM4 signal cell's getAggregateRange.
func TestGetAggregateRange(t *testing.T) {
	// getAggregateRange takes the satellite and signal data from an MSM4,
	// combines the range values and returns the range as a floating
	// point value in metres per second.  The data values can be marked as
	// invalid.

	const invalidWhole = 0xff   // 1111 1111
	const maxWhole = 0xfe       // 1111 1110
	const maxFractional = 0x3ff // 11 1111 1111

	// The invalid range delta value for an MSM7 is 20 bits: 1000 0000 0000 0000 0000 filler 0000.
	invalidDeltaBytes := []byte{0x80, 0, 0, 0}
	// the invalid value for an MSM4 is just the first 15 bits of the MSM7 pattern.
	invalidDelta := int(utils.GetBitsAsInt64(invalidDeltaBytes, 0, 15))

	// maxNoDelta is the result of combining the maximum whole and fractional
	// parts with a 0 delta:
	// 1 1111 1101 1111 1111 1000 0000 0000 0000 0000
	const maxNoDelta = 0x1fdff80000

	/// maxDeltaNormalised is the result of combining the maximum whole and fractional
	// parts with a 15-bit delta of 1, normalised by GetAggregateRange to 20 bits:
	// 1 1111 1101 1111 1111 1000 0000 0000 0010 0000
	const maxDeltaNormalised = 0x1fdff80020

	satelliteWithInvalidWhole :=
		satellite.Cell{RangeWholeMillis: invalidWhole, RangeFractionalMillis: 1}

	satelliteWithMaxWholeAndMaxFrac :=
		satellite.Cell{RangeWholeMillis: maxWhole, RangeFractionalMillis: maxFractional}

	cellWithNoSatellite := Cell{
		RangeDelta: 1,
	}

	cellWithInvalidWhole := Cell{
		RangeDelta: 1,
		Satellite:  &satelliteWithInvalidWhole,
	}

	cellWithInvalidRangeDelta := Cell{
		RangeDelta: invalidDelta,
		Satellite:  &satelliteWithMaxWholeAndMaxFrac,
	}

	cellWithZeroRangeDelta := Cell{
		RangeDelta: 0,
		Satellite:  &satelliteWithMaxWholeAndMaxFrac,
	}

	cellWithRangeDeltaOne := Cell{
		RangeDelta: 1,
		Satellite:  &satelliteWithMaxWholeAndMaxFrac,
	}

	var testData = []struct {
		description string
		Signal      Cell
		Want        uint64 // Expected result.
	}{
		// If the satellite is not supplied, the result is always zero.
		{"no satellite", cellWithNoSatellite, 0},
		// If the whole milliseconds value is invalid, the result is always zero.
		{"whole invalid", cellWithInvalidWhole, 0},
		// If the delta is invalid, the result is the approximate range.
		{"delta invalid", cellWithInvalidRangeDelta, maxNoDelta},
		// If the delta is zero, the result is the approximate range.
		{"delta zero", cellWithZeroRangeDelta, maxNoDelta},
		{"max delta", cellWithRangeDeltaOne, maxDeltaNormalised},
	}

	for _, td := range testData {

		got := td.Signal.GetAggregateRange()
		if got != td.Want {
			if td.Signal.RangeDelta < 0 {
				t.Errorf("%s (0x%x,0x%x,%d) want 0x%x, got 0x%x",
					td.description, td.Signal.Satellite.RangeWholeMillis,
					td.Signal.Satellite.RangeFractionalMillis,
					td.Signal.RangeDelta, td.Want, got)
			} else {
				t.Errorf("%s (0x%x,0x%x,0x%x) want 0x%x, got 0x%x",
					td.description, td.Signal.Satellite.RangeWholeMillis,
					td.Signal.Satellite.RangeFractionalMillis,
					td.Signal.RangeDelta, td.Want, got)
			}
		}
	}
}

// TestGetAggregatePhaseRange checks the MSM4 signal cell's getAggregateRange.
func TestGetAggregatePhaseRange(t *testing.T) {
	// getAggregateRange takes the satellite and signal data from a signal
	// cell, combines them and returns the range as a scaled integer.
	// Some values can be marked as invalid.

	const maxWhole = 0xfe       // 1111 1110
	const maxFractional = 0x3ff // 11 1111 1111

	// 24 bits: 1000 0000 0000 0000 0000 0000
	invalidDeltaBytes := []byte{0x80, 0, 0}
	// The invalid value for an MSM4 is just the first 22 bits of MSM7 pattern.
	invalidDelta22 := int(utils.GetBitsAsInt64(invalidDeltaBytes, 0, 22))

	// The incoming delta value is 24 bits signed and the delta and the fractional
	// part share 3 bits, producing a 39-bit result.
	//
	//     ------ Range -------
	//     whole     fractional
	//     876 5432 1098 7654 3210 9876 5432 1098 7654 3210
	//     www wwww wfff ffff fff0 0000 0000 0000 0000 0000
	//     + or -             dddd dddd dddd dddd dddd dddd <- phase range rate delta.

	// maxNoDelta is the result of combining the maximum whole and fractional
	// parts with a 0 24-bit delta:
	//     111 1111 0|111 1111 111|0
	//                         000 0000 0000 0000 0000 0000
	const maxNoDelta = 0x7f7fe00000

	/// maxDelta4 is the result of combining the maximum whole and fractional
	// parts with a 22-bit delta of 1, normalised to 24 bits:
	//     111 1111 0|111 1111 111|0
	//                         000 0000 0000 0000 0000 0100
	const maxDelta4 = 0x7f7fe00004

	satelliteWithMax := satellite.New(1, maxWhole, maxFractional)

	cellWithInvalidPhaseRangeDelta := Cell{
		PhaseRangeDelta: invalidDelta22,
		Satellite:       satelliteWithMax,
	}

	cellWithNoSatellite := Cell{
		PhaseRangeDelta: 1,
	}

	satelliteWithInvalidRange := satellite.New(2, utils.InvalidRange, 1)

	cellWithInvalidRange := Cell{
		PhaseRangeDelta: 1,
		Satellite:       satelliteWithInvalidRange,
	}

	cellWithPhaseRangeDeltaOne := Cell{
		PhaseRangeDelta: 1,
		Satellite:       satelliteWithMax,
	}

	var testData = []struct {
		ID     int
		Signal Cell
		Want   uint64 // Expected result.
	}{
		// If the satellite is not supplied, the result is always zero.
		{1, cellWithNoSatellite, 0},
		// If the whole milliseconds value is invalid, the result is always zero.
		{1, cellWithInvalidRange, 0},
		// If the delta is invalid, the result is the approximate range.
		{2, cellWithInvalidPhaseRangeDelta, maxNoDelta},
		// For an MSM4 message, the delta (if valid) is 4 times the value given.
		{3, cellWithPhaseRangeDeltaOne, maxDelta4},
	}

	for _, td := range testData {

		got := td.Signal.GetAggregatePhaseRange()
		if got != td.Want {
			if td.Signal.RangeDelta < 0 {
				t.Errorf("(%d 0x%x,0x%x,%d) want 0x%x, got 0x%x",
					td.ID,
					td.Signal.Satellite.RangeWholeMillis,
					td.Signal.Satellite.RangeFractionalMillis,
					td.Signal.RangeDelta, td.Want, got)
			} else {
				t.Errorf("%d (0x%x,0x%x,0x%x) want 0x%x, got 0x%x",
					td.ID,
					td.Signal.Satellite.RangeWholeMillis,
					td.Signal.Satellite.RangeFractionalMillis,
					td.Signal.RangeDelta, td.Want, got)
			}
		}
	}
}

// TestGetPhaseRange checks that the MSM4 signal's GetPhaseRange function works.
func TestGetPhaseRange(t *testing.T) {
	const rangeMillisWhole uint = 0x80       // 1000 0000
	const rangeMillisFractional uint = 0x200 // 10 0000 0000
	const phaseRangeDelta int = 1

	// The 39-bit aggregate values works like so:
	//     whole     fractional
	//     876 5432 1098 7654 3210 9876 5432 1098 7654 3210
	//     www wwww wfff ffff fff0 0000 0000 0000 0000 0000
	//     + or -             dddd dddd dddd dddd dddd dd| <- phase range delta.
	//   0|100 0000 0100 0000 000|
	//                        0000 0000 0000 0000 0000 01|00
	const wantAggregate = 0x4040000004

	const twoToPower29 = 0x20000000 // 10 0000 0000 0000 0000 0000 0000 0000
	const twoToPowerMinus29 = 1 / float64(twoToPower29)
	const twoToPower31 = 0x80000000 // 1000 0000 0000 0000 0000 0000 0000 0000
	const twoToPowerMinus31 = 1 / float64(twoToPower31)
	const rangeMilliseconds = 128.5 + float64(twoToPowerMinus29)
	const wantWavelength = utils.SpeedOfLightMS / utils.Freq2

	rangeLM := rangeMilliseconds * utils.OneLightMillisecond
	var signalID uint = 16

	wantPhaseRange := rangeLM / wantWavelength

	wavelength := utils.GetSignalWavelength("GPS", signalID)

	if wavelength != wantWavelength {
		if wantWavelength != wavelength {
			t.Errorf("want wavelength %f got %f", wavelength, wantWavelength)
			return
		}
	}

	satelliteCell := satellite.New(1, rangeMillisWhole, rangeMillisFractional)
	signalCell := Cell{
		ID:              signalID,
		Wavelength:      wavelength,
		PhaseRangeDelta: phaseRangeDelta,
		Satellite:       satelliteCell,
	}

	agg := signalCell.GetAggregatePhaseRange()

	if agg != wantAggregate {
		t.Errorf("want aggregate 0x%x got 0x%x", wantAggregate, agg)
		return
	}

	r := utils.GetPhaseRangeMilliseconds(agg)
	if !utils.EqualWithin(6, r, rangeMilliseconds) {
		t.Errorf("want range %f got %f", rangeMilliseconds, r)
		return
	}

	rlm := utils.GetPhaseRangeLightMilliseconds(r)
	if !utils.EqualWithin(3, rangeLM, rlm) {
		t.Errorf("want range %f got %f", rangeLM, rlm)
		return
	}

	if wantWavelength != wavelength {
		t.Errorf("want wavelength %f got %f", wantWavelength, wavelength)
		return
	}

	gotPhaseRange := signalCell.PhaseRange()

	if !utils.EqualWithin(3, wantPhaseRange, gotPhaseRange) {
		t.Errorf("expected %f got %f", wantPhaseRange, gotPhaseRange)
		return
	}

	// Try the biggest positive MSM4 delta: 01 0000 0000 0000 0000 0000
	const biggestDelta int = 0x100000
	// That will be normalised to the MSM7 size of 24 bits.
	const biggestDeltaNormalised = biggestDelta * 4

	const biggestDeltaRangeMilliseconds = 128.5 + float64(biggestDeltaNormalised)*float64(twoToPowerMinus31)

	const biggestDeltaRangeLM = biggestDeltaRangeMilliseconds * utils.OneLightMillisecond

	wantBiggestPhaseRange := biggestDeltaRangeLM / wantWavelength

	signalCell.PhaseRangeDelta = biggestDelta

	gotPhaseRange2 := signalCell.PhaseRange()

	if !utils.EqualWithin(3, wantBiggestPhaseRange, gotPhaseRange2) {
		t.Errorf("expected %f got %f", wantBiggestPhaseRange, gotPhaseRange2)
		return
	}
}

// TestString checks that String correctly displays an MSM4 signal cell.
func TestString(t *testing.T) {

	const satelliteID = 1
	const rangeWhole = 2
	const rangeFractional = 0x200 // 0.5

	const rangeDelta = 0x2000 // 1/2048 - 15 bit signed 00100000 00000000
	// 22 bits signed  01 0000 0000 0000 0000 0000, will be shifted up to 24 bits
	// to give 0100 0000 0000 0000 0000 0000.  Which is a delta of 1/512.
	const phaseRangeDelta = 0x100000
	const lockTimeIndicator = 7
	const halfCycleAmbiguity = true
	const cnr = 8
	const rangeMilliseconds = (2.5 + 1.0/2048.0)
	const phaseRangeMilliseconds = 2.5 + 1.0/512.0

	wantRange := rangeMilliseconds * utils.OneLightMillisecond
	phaseRangeMetres := phaseRangeMilliseconds * utils.OneLightMillisecond
	var signalID uint = 16

	wavelength := utils.GetSignalWavelength("GPS", signalID)

	wantPhaseRange := phaseRangeMetres / wavelength

	satCell := satellite.New(satelliteID, rangeWhole, rangeFractional)

	var testData = []struct {
		Comment            string
		ID                 uint
		SatelliteCell      *satellite.Cell
		RangeDelta         int
		PhaseRangeDelta    int
		LockTimeIndicator  uint
		HalfCycleAmbiguity bool
		CNR                uint
		Wavelength         float64
		Want               string // expected result
	}{
		{"all values", 2, satCell, rangeDelta, phaseRangeDelta, lockTimeIndicator, halfCycleAmbiguity, cnr, wavelength,
			fmt.Sprintf(" 1  2 {%.3f, %.3f, 7, true, 8}", wantRange, wantPhaseRange)},
		{"nil satellite", 2, nil, rangeDelta, phaseRangeDelta, lockTimeIndicator, halfCycleAmbiguity, cnr, 0.0,
			"<nil>  2 {invalid, invalid, 7, true, 8}"},
	}
	for _, td := range testData {
		cell := *New(
			td.ID, td.SatelliteCell, td.RangeDelta, td.PhaseRangeDelta, td.LockTimeIndicator,
			td.HalfCycleAmbiguity, td.CNR, wavelength)
		got := cell.String()
		if got != td.Want {
			t.Errorf("%s: want %s, got %s", td.Comment, td.Want, got)
		}
	}
}

// signalsEqual compares two signals for equality including the contents of the
// satellites that the signal objects point to.
func signalsEqual(cell, otherCell Cell) bool {

	if cell.ID != otherCell.ID {
		return false
	}

	if cell.Wavelength != otherCell.Wavelength {
		return false
	}

	if cell.RangeDelta != otherCell.RangeDelta {
		return false
	}

	if cell.PhaseRangeDelta != otherCell.PhaseRangeDelta {
		return false
	}

	if cell.LockTimeIndicator != otherCell.LockTimeIndicator {
		return false
	}

	if cell.HalfCycleAmbiguity != otherCell.HalfCycleAmbiguity {
		return false
	}

	if cell.CarrierToNoiseRatio != otherCell.CarrierToNoiseRatio {
		return false
	}

	if cell.Satellite == otherCell.Satellite {
		return false
	}

	// All fields equal.
	return true
}

// TestEqual checks the Equal method field by field
func TestEqual(t *testing.T) {
	testSatellite := satellite.New(1, 2, 3)
	testSignal := New(6, testSatellite, 7, 8, 9, true, 10, 11.0)

	satelliteCell := []*satellite.Cell{
		satellite.New(1, 2, 3),  // Same as the test satellite.
		satellite.New(1, 20, 3), // Different from the test satellite.
	}

	// These each differ from the test signal by one field.
	signalCell := []*Cell{
		New(20, satelliteCell[0], 7, 8, 9, true, 10, 11.0),
		New(6, satelliteCell[1], 7, 8, 9, true, 10, 11.0),
		New(6, satelliteCell[0], 20, 8, 9, true, 10, 11.0),
		New(6, satelliteCell[0], 7, 20, 9, true, 10, 11.0),
		New(6, satelliteCell[0], 7, 8, 20, true, 10, 11.0),
		New(6, satelliteCell[0], 7, 8, 9, false, 10, 11.0),
		New(6, satelliteCell[0], 7, 8, 9, true, 20, 11.0),
		New(6, satelliteCell[0], 7, 8, 9, true, 10, 20.0),
	}

	for i := range signalCell {
		if cmp.Equal(testSignal, signalCell[i]) {
			t.Errorf("cell %d should not be equal", i)
		}
	}
}
