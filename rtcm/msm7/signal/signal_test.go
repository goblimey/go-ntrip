package signal

import (
	"fmt"
	"testing"

	"github.com/goblimey/go-ntrip/rtcm/header"
	"github.com/goblimey/go-ntrip/rtcm/msm7/satellite"
	"github.com/goblimey/go-ntrip/rtcm/utils"
)

// TestNew checks that New correctly creates an MSM7 signal
// cell, including when some values are invalid.
func TestNew(t *testing.T) {

	const satelliteID = 1
	const rangeWhole = 2
	const rangeFractional = 3
	const wavelength = 4.0
	const phaseRangeRate = 5
	const phaseRangeRateDelta = 6

	const rangeDelta = 7
	const phaseRangeDelta = 8
	const lockTimeIndicator = 9
	const halfCycleAmbiguity = true
	const cnr = 10

	satCell := satellite.New(satelliteID, rangeWhole, rangeFractional, 0, phaseRangeRate)

	var testData = []struct {
		Comment             string
		ID                  uint
		SatelliteCell       *satellite.Cell
		RangeDelta          int
		PhaseRangeDelta     int
		PhaseRangeRateDelta int
		LockTimeIndicator   uint
		HalfCycleAmbiguity  bool
		CNR                 uint
		Wavelength          float64
		Want                Cell // expected result
	}{
		{"all values", 1, satCell, rangeDelta, phaseRangeDelta, phaseRangeRateDelta, lockTimeIndicator, halfCycleAmbiguity, cnr, wavelength,
			Cell{SignalID: 1,
				SatelliteID:                            satCell.SatelliteID,
				RangeWholeMillisFromSatelliteCell:      rangeWhole,
				RangeFractionalMillisFromSatelliteCell: rangeFractional,
				PhaseRangeRateFromSatelliteCell:        phaseRangeRate,
				Wavelength:                             wavelength,
				RangeDelta:                             rangeDelta,
				PhaseRangeDelta:                        phaseRangeDelta,
				LockTimeIndicator:                      lockTimeIndicator,
				HalfCycleAmbiguity:                     halfCycleAmbiguity,
				CarrierToNoiseRatio:                    cnr,
				PhaseRangeRateDelta:                    phaseRangeRateDelta}},
		{"nil satellite", 2, nil, rangeDelta, phaseRangeDelta, phaseRangeRateDelta, lockTimeIndicator, halfCycleAmbiguity, cnr, 0.0,
			Cell{SignalID: 2, SatelliteID: 0,
				RangeWholeMillisFromSatelliteCell:      utils.InvalidRange,
				RangeFractionalMillisFromSatelliteCell: 0,
				PhaseRangeRateFromSatelliteCell:        InvalidPhaseRangeRate,
				Wavelength:                             0.0,
				RangeDelta:                             rangeDelta,
				PhaseRangeDelta:                        phaseRangeDelta,
				LockTimeIndicator:                      lockTimeIndicator,
				HalfCycleAmbiguity:                     halfCycleAmbiguity,
				CarrierToNoiseRatio:                    cnr,
				PhaseRangeRateDelta:                    phaseRangeRateDelta}},
	}
	for _, td := range testData {
		got := *New(
			td.ID, td.SatelliteCell, td.RangeDelta, td.PhaseRangeDelta, td.LockTimeIndicator,
			td.HalfCycleAmbiguity, td.Want.CarrierToNoiseRatio, td.PhaseRangeRateDelta, td.Wavelength)
		if got != td.Want {
			t.Errorf("%s: want %v, got %v", td.Comment, td.Want, got)
		}
	}
}

// TestGetAggregateRange checks getAggregateRange.
func TestGetAggregateRange(t *testing.T) {
	// getAggregateRange takes the satellite and signal range values from an
	// MSM7SignalCell, combines those values and returns the range as a floating
	// point value in metres per second.  The data values can be marked as
	// invalid.

	const invalidWhole = 0xff   // 1111 1111
	const maxWhole = 0xfe       // 1111 1110
	const maxFractional = 0x3ff // 11 1111 1111

	// The invalid range value fr an MSM7 is 20 bits: 1000 0000 0000 0000 0000 filler 0000.
	invalidDeltaBytes := []byte{0x80, 0, 0, 0}
	invalidDelta := int(utils.GetBitsAsInt64(invalidDeltaBytes, 0, 20))

	// maxNoDelta is the result of combining the maximum whole and fractional
	// parts with a 0 delta:
	// 1 1111 1101 1111 1111 1000 0000 0000 0000 0000
	const maxNoDelta = 0x1fdff80000

	// maxDeltaOne is the result of combining the maximum whole and fractional
	// parts with a delta of 1:
	// 1 1111 1101 1111 1111 1000 0000 0000 0000 0001
	const maxDeltaOne = 0x1fdff80001

	// allOne is the result of combining three values, all 1:
	// 0 0000 0010 0000 0000 1000 0000 0000 0000 0001
	const allOne = 0x20080001

	CellWithInvalidRange := Cell{
		RangeWholeMillisFromSatelliteCell:      utils.InvalidRange,
		RangeFractionalMillisFromSatelliteCell: maxFractional,
		RangeDelta:                             invalidDelta,
	}

	cellWithInvalidRangeDelta := Cell{
		RangeWholeMillisFromSatelliteCell:      maxWhole,
		RangeFractionalMillisFromSatelliteCell: maxFractional,
		RangeDelta:                             invalidDelta,
	}

	cellWithRangeBothOneAndDeltaOne := Cell{
		RangeWholeMillisFromSatelliteCell:      1,
		RangeFractionalMillisFromSatelliteCell: 1,
		RangeDelta:                             1,
	}

	cellWithMaxRangeAndDeltaOne := Cell{
		RangeWholeMillisFromSatelliteCell:      maxWhole,
		RangeFractionalMillisFromSatelliteCell: maxFractional,
		RangeDelta:                             1,
	}

	var testData = []struct {
		Signal Cell
		Want   uint64 // Expected result.
	}{

		// If the whole milliseconds value is invalid, the result is zero.
		{CellWithInvalidRange, 0},
		// If the delta is invalid, the result is the approximate range.
		{cellWithInvalidRangeDelta, maxNoDelta},
		{cellWithRangeBothOneAndDeltaOne, allOne},
		{cellWithMaxRangeAndDeltaOne, maxDeltaOne},
		{cellWithRangeBothOneAndDeltaOne, allOne},
	}

	for _, td := range testData {

		got := td.Signal.GetAggregateRange()
		if got != td.Want {
			if td.Signal.RangeDelta < 0 {
				t.Errorf("(0x%x,0x%x,%d) want 0x%x, got 0x%x",
					td.Signal.RangeWholeMillisFromSatelliteCell,
					td.Signal.RangeFractionalMillisFromSatelliteCell,
					td.Signal.RangeDelta, td.Want, got)
			} else {
				t.Errorf("(0x%x,0x%x,0x%x) want 0x%x, got 0x%x",
					td.Signal.RangeWholeMillisFromSatelliteCell,
					td.Signal.RangeFractionalMillisFromSatelliteCell,
					td.Signal.RangeDelta, td.Want, got)
			}
		}
	}
}

// TestRangeInMetres checks that the correct range is calculated for an MSM7.
func TestRangeInMetres(t *testing.T) {

	const maxWhole = 0xfe                     // 1111 1110
	const maxFractional = 0x3ff               // 11 1111 1111
	var bigRangeMillisWhole uint = 0x80       // 1000 0000 (128 millis)
	var bigRangeMillisFractional uint = 0x200 // 10 bits 1000 ... (0.5 millis)
	const bigDelta = int(0x40000)             // 20 bits 0100 ...
	const twoToPower11 = 0x800                //                        1000 0000 0000
	const twoToPowerMinus11 = float64(1) / twoToPower11
	const twoToPower18 = 0x40000    //              0100 0000 0000 0000 0000
	const twoToPower29 = 0x20000000 // 10 0000 0000 0000 0000 0000 0000 0000

	const wantBig float64 = (128.5 + twoToPowerMinus11) * utils.OneLightMillisecond // 38523477.236036

	sigCellWithRangeOneMilli := Cell{
		RangeWholeMillisFromSatelliteCell:      1,
		RangeFractionalMillisFromSatelliteCell: 0,
		RangeDelta:                             0,
	}

	sigCellAllOnes := Cell{
		RangeWholeMillisFromSatelliteCell:      1,
		RangeFractionalMillisFromSatelliteCell: 1,
		RangeDelta:                             1,
	}

	sigCellWithSmallNegativeDelta := Cell{
		RangeWholeMillisFromSatelliteCell:      1,
		RangeFractionalMillisFromSatelliteCell: 1,
		RangeDelta:                             -1,
	}

	sigCellWithAllBigValues := Cell{
		RangeWholeMillisFromSatelliteCell:      bigRangeMillisWhole,
		RangeFractionalMillisFromSatelliteCell: bigRangeMillisFractional,
		RangeDelta:                             bigDelta,
	}

	sigCellWithMaxRangeAndDeltaOne := Cell{
		RangeWholeMillisFromSatelliteCell:      maxWhole,
		RangeFractionalMillisFromSatelliteCell: maxFractional,
		RangeDelta:                             1,
	}

	sigCellWithMaxRangeAndDeltaZero := Cell{
		RangeWholeMillisFromSatelliteCell:      maxWhole,
		RangeFractionalMillisFromSatelliteCell: maxFractional,
		RangeDelta:                             0,
	}

	// These values are taken from real data - an MSM7
	// converted to RINEX format to give the range.
	// {81, 435, -26835} -> 24410527.355

	sigCellWithRealDelta := Cell{
		RangeWholeMillisFromSatelliteCell:      81,
		RangeFractionalMillisFromSatelliteCell: 435,
		RangeDelta:                             -26835,
	}

	const wantResultFromReal = 24410527.355

	// The range delta is minus (half of a range fractional value of 1).
	sigCellWithLargeNegativeDelta := Cell{
		RangeWholeMillisFromSatelliteCell:      bigRangeMillisWhole, // 128
		RangeFractionalMillisFromSatelliteCell: 1,                   // 0.5 * twoToMinus10
		RangeDelta:                             twoToPower18,        // 0.25 8 twoToMinus10
	}

	var testData = []struct {
		Description string
		Signal      Cell
		Want        float64 // Expected result.
	}{

		{"1,0,0", sigCellWithRangeOneMilli, utils.OneLightMillisecond},
		{"1,1,1", sigCellAllOnes,
			(float64(1) + (float64(1) / 1024) + (float64(1) / twoToPower29)) * utils.OneLightMillisecond},
		{"1,1,small neg", sigCellWithSmallNegativeDelta,
			(float64(1) + (float64(1)/1024 - (float64(1) / twoToPower29))) * utils.OneLightMillisecond},
		{"1,1,large neg", sigCellWithLargeNegativeDelta,
			(float64(128) + (float64(1.5) / 1024)) * utils.OneLightMillisecond},
		{"big data", sigCellWithAllBigValues, wantBig},
		{"max,max,1", sigCellWithMaxRangeAndDeltaOne,
			(float64(maxWhole) + (float64(maxFractional) / 1024) + (float64(1) / twoToPower29)) * utils.OneLightMillisecond},
		{"max,max,0", sigCellWithMaxRangeAndDeltaZero,
			(float64(maxWhole) + (float64(maxFractional) / 1024)) * utils.OneLightMillisecond},
		{"real data", sigCellWithRealDelta, wantResultFromReal},
	}

	for _, td := range testData {

		got := td.Signal.RangeInMetres()

		if !utils.EqualWithin(3, td.Want, got) {
			t.Errorf("%s: want %f got %f", td.Description, td.Want, got)
		}

	}
}

// TestGetAggregatePhaseRange checks the MSM7 signal cell's
// getAggregatePhaseRange function.
func TestGetAggregatePhaseRange(t *testing.T) {
	// getAggregateRange takes the satellite and signal data from a signal
	// cell, combines them and returns the range as a scaled integer.
	// Some values can be marked as invalid.

	const invalidRangeWhole = 0xff   // 1111 1111
	const maxRangeWhole = 0xfe       // 1111 1110
	const maxRangeFractional = 0x3ff // 11 1111 1111

	// 24 bits: 1000 0000 0000 0000 0000 0000
	invalidDeltaBytes := []byte{0x80, 0, 0}
	invalidPhaseRangeDelta := int(utils.GetBitsAsInt64(invalidDeltaBytes, 0, 24))

	// Junk filler values.
	const filler2 = 2
	const filler3 = 3
	const filler4 = 4
	const filler5 = 5
	const filler6 = 6
	const fillerFalse = false
	const filler7 = 7
	const filler8 = 8
	const filler9 = 9
	// The incoming delta value is 24 bits signed and the delta and the fractional
	// part share 3 bits, producing a 39-bit result.
	//
	//     ------ Range -------
	//     whole     fractional
	//     876 5432 1098 7654 3210 9876 5432 1098 7654 3210
	//     www wwww wfff ffff fff0 0000 0000 0000 0000 0000
	//     + or -             dddd dddd dddd dddd dddd dddd <- phase range delta.

	// maxNoDelta is the result of combining the maximum whole and fractional
	// parts with a 0 24-bit delta:
	//     111 1111 0|111 1111 111|0
	//                         000 0000 0000 0000 0000 0000
	const maxNoDelta = 0x7f7fe00000

	// maxDeltaOne is the result of combining the maximum whole and fractional
	// parts with a delta of 1:
	//     111 1111 0|111 1111 111|0
	//                         000 0000 0000 0000 0000 0001
	const maxRangeDeltaOne = 0x7f7fe00001

	// allOne is the result of combining three values, all 1:
	//     000 0000 1|000 0000 001|0
	//                         000 0000 0000 0000 0000 0001
	const allOne = 0x80200001

	satelliteCellWithRangeBothOne := satellite.New(1, 1, 1, filler3, filler4)
	satelliteCellWithMaxValues :=
		satellite.New(1, maxRangeWhole, maxRangeFractional, filler3, filler4)
	satelliteCellWithInvalidRange :=
		satellite.New(1, invalidRangeWhole, maxRangeFractional, filler3, filler4)

	cellWithInvalidRange := New(1, satelliteCellWithInvalidRange, filler5, 1,
		filler6, fillerFalse, filler7, filler8, filler9)

	cellWithMaxRange := New(2, satelliteCellWithMaxValues, filler5, 0,
		filler6, fillerFalse, filler7, filler8, filler9)

	cellWithInvalidPhaseRangeDelta := New(3, satelliteCellWithMaxValues, filler5,
		invalidPhaseRangeDelta, filler6, fillerFalse, filler7, filler8, filler9)

	cellWithRangeAndPhaseRangeDeltaOne := New(3, satelliteCellWithRangeBothOne,
		filler5, 1, filler6, fillerFalse, filler7, filler8, filler9)

	cellWithMaxRangeAndPhaseRangeDeltaOne := New(4, satelliteCellWithMaxValues, filler5, 1,
		filler6, fillerFalse, filler7, filler8, filler9)

	var testData = []struct {
		Signal *Cell
		Want   uint64 // Expected result.
	}{
		// If the whole milliseconds value is invalid, the result is always zero.
		{cellWithInvalidRange, 0},
		// If the delta is invalid, the result is the approximate range.
		{cellWithInvalidPhaseRangeDelta, maxNoDelta},
		{cellWithMaxRange, maxNoDelta},
		{cellWithRangeAndPhaseRangeDeltaOne, allOne},
		{cellWithMaxRangeAndPhaseRangeDeltaOne, maxRangeDeltaOne},
	}

	for _, td := range testData {

		got := td.Signal.GetAggregatePhaseRange()
		if got != td.Want {
			if td.Signal.RangeDelta < 0 {
				t.Errorf("(%d 0x%x,0x%x,%d) want 0x%x, got 0x%x",
					td.Signal.SignalID,
					td.Signal.RangeWholeMillisFromSatelliteCell,
					td.Signal.RangeFractionalMillisFromSatelliteCell,
					td.Signal.PhaseRangeDelta, td.Want, got)
			} else {
				t.Errorf("%d (0x%x,0x%x,0x%x) want 0x%x, got 0x%x",
					td.Signal.SignalID,
					td.Signal.RangeWholeMillisFromSatelliteCell,
					td.Signal.RangeFractionalMillisFromSatelliteCell,
					td.Signal.PhaseRangeDelta, td.Want, got)
			}
		}
	}
}

// TestGetAggregatePhaseRangeRate checks the MSM7 signal cell's
// getAggregatePhaseRangeRate function.
func TestGetAggregatePhaseRangeRate(t *testing.T) {
	// getAggregatePhaseRangeRate takes the satellite and signal data from a signal
	// cell, combines them and returns the range as a scaled integer.  Some values can
	// be marked as invalid.  The whole milliseconds value from the satellite cell
	// is a 14 bit twos complement int and the delta value in the signal cell is a
	// 15 bit two's complement int, one in ten thousand of a millisecond.  In both
	// cases a 1 bit at the top followed by all zeros marks the value as invalid.

	// The invalid value for the whole phase range rate is 14 bits 1000 0000 0000 00|00
	invalidWholeBytes := []byte{0x80, 0x00}
	invalidWhole := int(utils.GetBitsAsInt64(invalidWholeBytes, 0, 14))
	// The maximum value for the phase range rate is 0001 1111 1111 1111
	const maxWhole = int(0x1fff)
	// The invalid delta value is 15 bits 1000 0000 0000 000|0
	invalidDeltaBytes := []byte{0x80, 0x00}
	invalidDelta := int(utils.GetBitsAsInt64(invalidDeltaBytes, 0, 15))
	// The maximum delta value is 0010 0000 0000 0000
	const maxDelta = int(0x2000)

	// If the whole is at the max value and the delta is zero, the result should be:
	const wantMaxWholeNoDelta = int64(maxWhole * 10000)
	// If the whole is at the max value and the delta is one, the result should be:
	const wantMaxWholeAndDeltaOne = int64(maxWhole*10000) + 1
	// If the whole and the delta value are at their max, the result should be:
	const wantBothMax = wantMaxWholeNoDelta + int64(maxDelta)
	// If the whole and the delta are both 1, the result should be:
	const wantBothOne = 10001
	// If the whole and the delta are both -1, the result should be:
	const wantBothNeg = -10001

	cellWithInvalidWhole := Cell{
		PhaseRangeRateFromSatelliteCell: invalidWhole,
		PhaseRangeRateDelta:             1,
	}

	cellWithMaxWholeAndDeltaZero := Cell{
		PhaseRangeRateFromSatelliteCell: maxWhole,
		PhaseRangeRateDelta:             0,
	}

	cellWithMaxWholeAndInvalidDelta := Cell{
		PhaseRangeRateFromSatelliteCell: maxWhole,
		PhaseRangeRateDelta:             invalidDelta,
	}

	cellWithBothOne := Cell{
		PhaseRangeRateFromSatelliteCell: 1,
		PhaseRangeRateDelta:             1,
	}
	cellWithMaxRangeAndDeltaOne := Cell{
		PhaseRangeRateFromSatelliteCell: maxWhole,
		PhaseRangeRateDelta:             1,
	}

	cellWithBothMax := Cell{
		PhaseRangeRateFromSatelliteCell: maxWhole,
		PhaseRangeRateDelta:             maxDelta,
	}

	cellWithBothNegative := Cell{
		PhaseRangeRateFromSatelliteCell: -1,
		PhaseRangeRateDelta:             -1,
	}

	cellWithBothInvalid := Cell{
		PhaseRangeRateFromSatelliteCell: invalidWhole,
		PhaseRangeRateDelta:             invalidDelta,
	}

	var testData = []struct {
		ID     int
		Signal Cell
		Want   int64 // Expected result.
	}{
		// If the whole milliseconds value is invalid, the result is always zero.
		{1, cellWithInvalidWhole, 0},
		// If the delta is invalid, the result is the approximate range.
		{2, cellWithMaxWholeAndDeltaZero, wantMaxWholeNoDelta},
		{3, cellWithMaxWholeAndInvalidDelta, wantMaxWholeNoDelta},
		{4, cellWithBothOne, wantBothOne},
		{5, cellWithMaxRangeAndDeltaOne, wantMaxWholeAndDeltaOne},
		{6, cellWithBothMax, wantBothMax},
		{7, cellWithBothNegative, wantBothNeg},
		{8, cellWithBothInvalid, 0},
	}

	for _, td := range testData {

		got := td.Signal.GetAggregatePhaseRangeRate()
		if got != td.Want {
			t.Errorf("(%d %d,%d) want %d, got %d",
				td.ID,
				td.Signal.PhaseRangeRateFromSatelliteCell,
				td.Signal.PhaseRangeRateDelta, td.Want, got)
		}
	}
}

// TestGetPhaseRange checks GetPhaseRange.
func TestGetPhaseRange(t *testing.T) {
	const rangeMillisWhole uint = 0x80       // 1000 0000
	const rangeMillisFractional uint = 0x200 // 10 0000 0000
	const phaseRangeDelta int = 1
	// Junk filler values.
	const satelliteID = 2
	const extendedInfo = 3
	const phaseRangeRate = 4
	const rangeDelta = 5
	const lockTimeIndicator = 6
	const halfCycleAmbiguity = false
	const cnr = 7
	const phaseRangeRateDelta = 8

	// The 39-bit aggregate values works like so:
	//     whole     fractional
	//     876 5432 1098 7654 3210 9876 5432 1098 7654 3210
	//     www wwww wfff ffff fff0 0000 0000 0000 0000 0000
	//     + or -             dddd dddd dddd dddd dddd dddd <- phase range rate delta.
	//   0|100 0000 0100 0000 000|
	//                        0000 0000 0000 0000 0000 0001
	const wantAggregate = 0x4040000001

	const twoToPower31 = 0x80000000 // 1000 0000 0000 0000 0000 0000 0000 0000
	const twoToPowerMinus31 = 1 / float64(twoToPower31)
	const rangeMilliseconds = 128.5 + float64(twoToPowerMinus31)
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

	satelliteCell := satellite.New(1, rangeMillisWhole, rangeMillisFractional,
		extendedInfo, phaseRangeRate)

	signalCell := New(signalID, satelliteCell, rangeDelta, phaseRangeDelta,
		lockTimeIndicator, halfCycleAmbiguity, cnr, phaseRangeRateDelta,
		wavelength)

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

	// Try the biggest positive delta: 0100 0000 0000 0000 0000 0000
	const biggestDelta int = 0x400000

	const biggestDeltaRangeMilliseconds = 128.5 + float64(biggestDelta)*float64(twoToPowerMinus31)

	const biggestDeltaRangeLM = biggestDeltaRangeMilliseconds * utils.OneLightMillisecond

	wantBiggestPhaseRange := biggestDeltaRangeLM / wantWavelength

	signalCell.PhaseRangeDelta = biggestDelta

	gotPhaseRange2 := signalCell.PhaseRange()

	if !utils.EqualWithin(3, wantBiggestPhaseRange, gotPhaseRange2) {
		t.Errorf("expected %f got %f", wantBiggestPhaseRange, gotPhaseRange2)
		return
	}
}

// TestGetPhaseRangeRealValues tests getPhaseRange with real data.
func TestGetPhaseRangeRealValues(t *testing.T) {

	// These data were captured from equipment and converted to RINEX format.
	const signalID = 2
	// wantPhaseRange is taken from the resulting RINEX file.
	const wantPhaseRange = 128278179.264

	wavelength := utils.GetSignalWavelength("GPS", signalID)

	signalCell := Cell{
		SignalID:                               signalID,
		RangeWholeMillisFromSatelliteCell:      81,
		RangeFractionalMillisFromSatelliteCell: 435,
		Wavelength:                             wavelength,
		PhaseRangeDelta:                        -117960}

	gotCycles := signalCell.PhaseRange()

	if !utils.EqualWithin(3, wantPhaseRange, gotCycles) {
		t.Errorf("expected %f got %f", wantPhaseRange, gotCycles)
		return
	}
}

// TestMSM7DopplerWithRealData checks that getMSM7Doppler works using real data.
func TestMSM7DopplerWithRealData(t *testing.T) {
	// The input data were collected from a UBlox device.
	// The want value is from a RINEX file created from those data.

	const signalID = 2

	const want = float64(709.992)

	wavelength := utils.GetSignalWavelength("GPS", signalID)

	sigCell := Cell{
		SignalID:                        2,
		PhaseRangeRateFromSatelliteCell: -135,
		Wavelength:                      wavelength,
		PhaseRangeRateDelta:             -1070}

	got := sigCell.GetMSM7Doppler()

	if !utils.EqualWithin(3, want, got) {
		t.Errorf("expected %f got %f", want, got)
		return
	}
}

// TestGetSignalCells checks that getSignalCells correctly interprets a
// bit stream from an MSM7 message containing two signal cells.
func TestGetSignalCells(t *testing.T) {
	const signalID0 = 5
	const signalID1 = 7
	const satelliteID0 = 42
	const satelliteID1 = 43

	wavelength0 := utils.GetSignalWavelength("GPS", signalID0)
	wavelength1 := utils.GetSignalWavelength("GPS", signalID1)

	satellites := []uint{satelliteID0, satelliteID1}

	signals := []uint{signalID0, signalID1}
	// Satellite 42 received signals 5 and 7, satellite 43 received signal 5 only.
	cellMask := [][]bool{{true, true}, {true, false}}
	header := header.Header{MessageType: 1077, NumSignalCells: 3,
		Satellites: satellites, Signals: signals, Cells: cellMask}
	satData := []satellite.Cell{
		satellite.Cell{SatelliteID: satelliteID0},
		satellite.Cell{SatelliteID: satelliteID1},
	}

	// The bit stream starts at byte 6 and contains three signal cells - three
	// 20-bit signed range deltas, followed by three 24-bit signed phase range
	// deltas, three 10-bit unsigned phase lock time indicators, three single bit
	// half-cycle ambiguity indicators, three 10-bit unsigned GNSS Signal Carrier
	// to Noise Ratio (CNR) values and three 15-bit signed phase range rate delta
	// values. 80 bits per signal, so 240 bits in all, set like so:
	// 0000 0000   0000 0000   0000|1111    1111 1111   1111 1111|  0100 0000     48
	// 0000 0000   0001|1111   1111 1111    1111 1111   1111|0000   0000 0000     96
	// 0000 0000   0000|0000   0000 0000    0000 0000   0101| 1111  1111 11|00   144
	// 0000 0000|  0000 0000   01|011|000   0000 000|1  1111  1111  1|0000001    172
	// 010|11111   1111 1111   11|00 0000   0000 0000   0|000 0000  0000 1101|   240
	bitStream := []byte{
		0x01, 0x02, 0x03, 0x04, 0x05, 0x06,
		// Start of message:
		0x00, 0x00, 0x0f, 0xff, 0xff, 0x40,
		0x00, 0x1f, 0xff, 0xff, 0xf0, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x5f, 0xfc,
		0x00, 0x00, 0x58, 0x01, 0xff, 0x81,
		0x5f, 0xff, 0xc0, 0x00, 0x00, 0x0d,
	}

	const startPosition = 48 // Byte 6.

	want0 := Cell{
		SignalID:    signalID0,
		SatelliteID: satelliteID0,
		Wavelength:  wavelength0,
		RangeDelta:  0, PhaseRangeDelta: -1,
		LockTimeIndicator: 1023, HalfCycleAmbiguity: false, CarrierToNoiseRatio: 0,
		PhaseRangeRateDelta: -1,
	}

	want1 := Cell{
		SignalID:    signalID1,
		SatelliteID: satelliteID0,
		Wavelength:  wavelength1,
		RangeDelta:  -1, PhaseRangeDelta: 0,
		LockTimeIndicator: 0, HalfCycleAmbiguity: true, CarrierToNoiseRatio: 1023,
		PhaseRangeRateDelta: 0,
	}

	want2 := Cell{
		SignalID:    signalID0,
		SatelliteID: satelliteID1,
		Wavelength:  wavelength0,
		RangeDelta:  262145, PhaseRangeDelta: 5,
		LockTimeIndicator: 1, HalfCycleAmbiguity: true, CarrierToNoiseRatio: 10,
		PhaseRangeRateDelta: 13,
	}

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

	if got[0][0] != want0 {
		t.Errorf("expected [0][0]\n%v got\n%v", want0, got[0][0])
	}

	if got[0][1] != want1 {
		t.Errorf("expected [0][1]\n%v got\n%v", want1, got[0][1])
	}

	if got[1][0] != want2 {
		t.Errorf("expected [1][0]\n%v got\n%v", want2, got[1][0])
	}
}

// TestGetSignalCellsWithShortBitStream checks that getSignalCells produces
// the correct error message if the bitstream is too short.
func TestGetSignalCellsWithShortBitStream(t *testing.T) {
	const signalID1 = 7
	const satelliteID0 = 42
	const satelliteID1 = 43
	satellites := []uint{satelliteID0, satelliteID1}
	const signalID0 = 5
	signals := []uint{signalID0, signalID1}
	// Satellite 42 received signals 5 and 7, satellite 43 received signal 5 only.
	cellMask := [][]bool{{true, true}, {true, false}}
	headerForSingleMessage := header.Header{MessageType: 1077, MultipleMessage: false,
		NumSignalCells: 3, Satellites: satellites, Signals: signals, Cells: cellMask}
	headerForMultiMessage := header.Header{MessageType: 1077, MultipleMessage: true,
		NumSignalCells: 3, Satellites: satellites, Signals: signals, Cells: cellMask}
	satData := []satellite.Cell{
		satellite.Cell{SatelliteID: satelliteID0,
			RangeWholeMillis: 0x81, RangeFractionalMillis: 0x201},
		satellite.Cell{SatelliteID: satelliteID1,
			RangeWholeMillis: 1, RangeFractionalMillis: 2}}

	// The bit stream contains three MSM7 signal cells - three
	// 20-bit signed range deltas, followed by three 24-bit signed phase range
	// deltas, three 10-bit unsigned phase lock time indicators, three single bit
	// half-cycle ambiguity indicators, three 10-bit unsigned GNSS Signal Carrier
	// to Noise Ratio (CNR) values and three 15-bit signed phase range rate delta
	// values. 80 bits per signal, so 240 bits in all, set like so:
	// 0000 0000   0000 0000   0000|1111    1111 1111   1111 1111|  0100 0000     48
	// 0000 0000   0001|1111   1111 1111    1111 1111   1111|0000   0000 0000     96
	// 0000 0000   0000|0000   0000 0000    0000 0000   0101| 1111  1111 11|00   144
	// 0000 0000|  0000 0000   01|011|000   0000 000|1  1111  1111  1|0000001    172
	// 010|11111   1111 1111   11|00 0000   0000 0000   0|000 0000  0000 1101|   240
	bitStream := []byte{
		0x00, 0x00, 0x0f, 0xff, 0xff, 0x40,
		0x00, 0x1f, 0xff, 0xff, 0xf0, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x5f, 0xfc,
		0x00, 0x00, 0x58, 0x01, 0xff, 0x81,
		0x5f, 0xff, 0xc0, 0x00, 0x00, 0x0d,
	}

	// The test provides only part of the bitstream, to provoke an overrun error.
	var testData = []struct {
		description string
		header      *header.Header
		bitStream   []byte
		want        string
	}{
		{
			"single", &headerForSingleMessage, bitStream[:24],
			"overrun - want 3 MSM7 signals, got 2",
		},
		{
			"multiple", &headerForMultiMessage, bitStream[:9],
			"overrun - want at least one 80-bit signal cell when multiple message flag is set, got only 72 bits left",
		},
	}

	for _, td := range testData {

		// Expect an error.
		gotMessage, gotError := GetSignalCells(td.bitStream, 0, td.header, satData)

		if gotMessage != nil {
			t.Error("expected a nil message with an error")
		}

		// Check the error.
		if gotError == nil {
			t.Error("expected an overrun error")
			return
		}

		if gotError.Error() != td.want {
			t.Errorf("expected the error\n\"%s\"\ngot \"%s\"",
				td.want, gotError.Error())
			return
		}
	}
}

func TestString(t *testing.T) {

	// These values and results are copied from some of the above tests.
	const rangeWhole uint = 0x80       // 1000 0000 (128 millis)
	const rangeFractional uint = 0x200 // 10 bits 1000 ... (0.5 millis)
	const rangeDelta = int(0x40000)    // 20 bits 0100 ...
	const twoToPower11 = 0x800         //                        1000 0000 0000
	const twoToPowerMinus11 = float64(1) / twoToPower11
	const twoToPower31 = 0x80000000 // 1000 0000 0000 0000 0000 0000 0000 0000
	const twoToPowerMinus31 = 1 / float64(twoToPower31)
	const rangeMilliseconds = 128.5 + float64(twoToPowerMinus31)
	const wavelength = utils.SpeedOfLightMS / utils.Freq2
	const signalID uint = 16
	const extendedInfo = 5
	const wholePhaseRangeRate = 6
	const phaseRangeDelta int = 1
	const phaseRangeRateDelta = 7890
	const wantPhaseRangeRate = 6.789
	const lockTimeIndicator = 4
	const halfCycleAmbiguity = true
	const cnr = 5
	const wantRange float64 = (128.5 + twoToPowerMinus11) * utils.OneLightMillisecond // 11 1111 1111

	rangeLM := rangeMilliseconds * utils.OneLightMillisecond
	wantPhaseRange := rangeLM / wavelength

	want := fmt.Sprintf(" 1 16 {%8.3f, %8.3f, %d, %v, %d, %.3f}",
		wantRange, wantPhaseRange, lockTimeIndicator, halfCycleAmbiguity,
		cnr, wantPhaseRangeRate)

	satelliteCell := satellite.New(1, rangeWhole, rangeFractional,
		extendedInfo, wholePhaseRangeRate)

	signalCell := New(signalID, satelliteCell, rangeDelta, phaseRangeDelta,
		lockTimeIndicator, halfCycleAmbiguity, cnr, phaseRangeRateDelta, wavelength)

	got := signalCell.String()

	if want != got {
		t.Errorf("\nwant %s\n got %s", want, got)
	}
}
