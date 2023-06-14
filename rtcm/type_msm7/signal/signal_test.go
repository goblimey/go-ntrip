package signal

import (
	"testing"

	"github.com/goblimey/go-ntrip/rtcm/header"
	"github.com/goblimey/go-ntrip/rtcm/type_msm7/satellite"
	"github.com/goblimey/go-ntrip/rtcm/utils"

	"github.com/google/go-cmp/cmp"
	"github.com/kylelemons/godebug/diff"
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
			Cell{ID: 1,
				Wavelength:          wavelength,
				RangeDelta:          rangeDelta,
				PhaseRangeDelta:     phaseRangeDelta,
				LockTimeIndicator:   lockTimeIndicator,
				HalfCycleAmbiguity:  halfCycleAmbiguity,
				CarrierToNoiseRatio: cnr,
				PhaseRangeRateDelta: phaseRangeRateDelta,
				Satellite:           satCell,
			},
		},
		{"nil satellite", 2, nil, rangeDelta, phaseRangeDelta, phaseRangeRateDelta, lockTimeIndicator, halfCycleAmbiguity, cnr, 0.0,
			Cell{
				ID:                  2,
				Wavelength:          0.0,
				RangeDelta:          rangeDelta,
				PhaseRangeDelta:     phaseRangeDelta,
				LockTimeIndicator:   lockTimeIndicator,
				HalfCycleAmbiguity:  halfCycleAmbiguity,
				CarrierToNoiseRatio: cnr,
				PhaseRangeRateDelta: phaseRangeRateDelta,
				Satellite:           nil,
			},
		},
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

	satelliteWithInvalidRange := satellite.New(1, utils.InvalidRange, maxFractional, 3, 4)
	satelliteWithMaxValues := satellite.New(2, maxWhole, maxFractional, 3, 4)
	satelliteWithRangeOne := satellite.New(5, 1, 1, 3, 4)

	cellWithNoSatellite := Cell{RangeDelta: invalidDelta}

	cellWithInvalidRange := Cell{
		RangeDelta: invalidDelta,
		Satellite:  satelliteWithInvalidRange,
	}

	cellWithInvalidRangeDelta := Cell{
		RangeDelta: invalidDelta,
		Satellite:  satelliteWithMaxValues,
	}

	cellWithRangeBothOneAndDeltaOne := Cell{
		RangeDelta: 1,
		Satellite:  satelliteWithRangeOne,
	}

	cellWithMaxRangeAndDeltaOne := Cell{
		RangeDelta: 1,
		Satellite:  satelliteWithMaxValues,
	}

	var testData = []struct {
		Signal Cell
		Want   uint64 // Expected result.
	}{
		// If there is no satellite, the result is zro.
		{cellWithNoSatellite, 0},
		// If the whole milliseconds value is invalid, the result is zero.
		{cellWithInvalidRange, 0},
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
					td.Signal.Satellite.RangeWholeMillis,
					td.Signal.Satellite.RangeFractionalMillis,
					td.Signal.RangeDelta, td.Want, got)
			} else {
				t.Errorf("(0x%x,0x%x,0x%x) want 0x%x, got 0x%x",
					td.Signal.Satellite.RangeWholeMillis,
					td.Signal.Satellite.RangeFractionalMillis,
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

	satelliteRangeOneFracZero := satellite.New(1, 1, 0, 3, 4)
	satelliteRangeOneFracOne := satellite.New(2, 1, 1, 3, 4)
	satelliteBigValues := satellite.New(3, bigRangeMillisWhole, bigRangeMillisFractional, 3, 4)
	satelliteMaxValues := satellite.New(4, maxWhole, maxFractional, 3, 4)
	satelliteRealValues := satellite.New(5, 81, 435, 3, 4)
	satelliteBigWholeFracOne := satellite.New(3, bigRangeMillisWhole, 1, 3, 4)

	sigCellWithRangeOneMilli := Cell{
		RangeDelta: 0,
		Satellite:  satelliteRangeOneFracZero,
	}

	sigCellAllOnes := Cell{
		RangeDelta: 1,
		Satellite:  satelliteRangeOneFracOne,
	}

	sigCellWithSmallNegativeDelta := Cell{
		RangeDelta: -1,
		Satellite:  satelliteRangeOneFracOne,
	}

	sigCellWithAllBigValues := Cell{
		RangeDelta: bigDelta,
		Satellite:  satelliteBigValues,
	}

	sigCellWithMaxRangeAndDeltaOne := Cell{
		RangeDelta: 1,
		Satellite:  satelliteMaxValues,
	}

	sigCellWithMaxRangeAndDeltaZero := Cell{
		RangeDelta: 0,
		Satellite:  satelliteMaxValues,
	}

	// These values are taken from real data - an MSM7
	// converted to RINEX format to give the range.
	// {81, 435, -26835} -> 24410527.355

	sigCellWithRealDelta := Cell{
		RangeDelta: -26835,
		Satellite:  satelliteRealValues,
	}

	const wantResultFromReal = 24410527.355

	// The range whole  is 128, fractional is 0.5 * twoToMinus10
	// The range delta is minus (half of a range fractional value of 1).
	sigCellWithLargeNegativeDelta := Cell{
		RangeDelta: twoToPower18, // 0.25 * twoToMinus10
		Satellite:  satelliteBigWholeFracOne,
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
					td.Signal.ID,
					td.Signal.Satellite.RangeWholeMillis,
					td.Signal.Satellite.RangeFractionalMillis,
					td.Signal.PhaseRangeDelta, td.Want, got)
			} else {
				t.Errorf("%d (0x%x,0x%x,0x%x) want 0x%x, got 0x%x",
					td.Signal.ID,
					td.Signal.Satellite.RangeWholeMillis,
					td.Signal.Satellite.RangeFractionalMillis,
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
	invalidPhaseRangeRateBytes := []byte{0x80, 0x00}
	invalidPhaseRangeRate := int(utils.GetBitsAsInt64(invalidPhaseRangeRateBytes, 0, 14))
	// The maximum value for the phase range rate is 0001 1111 1111 1111
	const maxPhaseRangeRate = int(0x1fff)
	// The invalid delta value is 15 bits 1000 0000 0000 000|0
	invalidDeltaBytes := []byte{0x80, 0x00}
	invalidDelta := int(utils.GetBitsAsInt64(invalidDeltaBytes, 0, 15))
	// The maximum delta value is 0010 0000 0000 0000
	const maxDelta = int(0x2000)

	// If the whole is at the max value and the delta is zero, the result should be:
	const wantMaxWholeNoDelta = int64(maxPhaseRangeRate * 10000)
	// If the whole is at the max value and the delta is one, the result should be:
	const wantMaxWholeAndDeltaOne = int64(maxPhaseRangeRate*10000) + 1
	// If the whole and the delta value are at their max, the result should be:
	const wantBothMax = wantMaxWholeNoDelta + int64(maxDelta)
	// If the whole and the delta are both 1, the result should be:
	const wantBothOne = 10001
	// If the whole and the delta are both -1, the result should be:
	const wantBothNeg = -10001

	satWithInvalidPhaseRangeRate := satellite.New(1, 2, 3, 0, invalidPhaseRangeRate)

	cellWithNoSatellite := Cell{PhaseRangeRateDelta: 1}

	cellWithInvalidPhaseRangeRate := Cell{
		PhaseRangeRateDelta: 1,
		Satellite:           satWithInvalidPhaseRangeRate,
	}

	satWithMaxPhaseRangeRate := satellite.New(1, 2, 3, 0, maxPhaseRangeRate)

	cellWithMaxWholeAndDeltaZero := Cell{
		PhaseRangeRateDelta: 0,
		Satellite:           satWithMaxPhaseRangeRate,
	}

	cellWithMaxWholeAndInvalidDelta := Cell{
		PhaseRangeRateDelta: invalidDelta,
		Satellite:           satWithMaxPhaseRangeRate,
	}

	satWithPhaseRangeRateOne := satellite.New(1, 2, 3, 0, 1)

	cellWithBothOne := Cell{
		PhaseRangeRateDelta: 1,
		Satellite:           satWithPhaseRangeRateOne,
	}

	cellWithMaxRateAndDeltaOne := Cell{
		PhaseRangeRateDelta: 1,
		Satellite:           satWithMaxPhaseRangeRate,
	}

	cellWithBothMax := Cell{
		PhaseRangeRateDelta: maxDelta,
		Satellite:           satWithMaxPhaseRangeRate,
	}

	satWithNegativePhaseRangeRate := satellite.New(1, 2, 3, 0, -1)

	cellWithBothNegative := Cell{
		PhaseRangeRateDelta: -1,
		Satellite:           satWithNegativePhaseRangeRate,
	}

	cellWithBothInvalid := Cell{
		PhaseRangeRateDelta: invalidDelta,
		Satellite:           satWithInvalidPhaseRangeRate,
	}

	var testData = []struct {
		ID     int
		Signal Cell
		Want   int64 // Expected result.
	}{
		// If the whole milliseconds value is invalid, the result is always zero.
		{1, cellWithInvalidPhaseRangeRate, 0},
		// If the delta is invalid, the result is the approximate range.
		{2, cellWithMaxWholeAndDeltaZero, wantMaxWholeNoDelta},
		{3, cellWithMaxWholeAndInvalidDelta, wantMaxWholeNoDelta},
		{4, cellWithBothOne, wantBothOne},
		{5, cellWithMaxRateAndDeltaOne, wantMaxWholeAndDeltaOne},
		{6, cellWithBothMax, wantBothMax},
		{7, cellWithBothNegative, wantBothNeg},
		{8, cellWithBothInvalid, 0},
		// If there is no satellite, the result is always zero.
		{9, cellWithNoSatellite, 0},
	}

	for _, td := range testData {

		got := td.Signal.GetAggregatePhaseRangeRate()
		if got != td.Want {
			t.Errorf("(%d %d,%d) want %d, got %d",
				td.ID,
				td.Signal.Satellite.PhaseRangeRate,
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
	//     + or -             dddd dddd dddd dddd dddd dddd <- 24-bit signed phase range delta.
	//   0|100 0000 0100 0000 000|
	//                        0000 0000 0000 0000 0000 0001
	const wantAggregate = 0x4040000001

	const twoToPower31 = 0x80000000 // 1000 0000 0000 0000 0000 0000 0000 0000
	const twoToPowerMinus31 = 1 / float64(twoToPower31)
	const phaseRangeMilliseconds = 128.5 + float64(twoToPowerMinus31)
	const wantWavelength = utils.SpeedOfLightMS / utils.Freq2

	phaseRangeLightMillis := phaseRangeMilliseconds * utils.OneLightMillisecond
	var signalID uint = 16

	wantPhaseRange := phaseRangeLightMillis / wantWavelength

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
	if !utils.EqualWithin(6, r, phaseRangeMilliseconds) {
		t.Errorf("want range %f got %f", phaseRangeMilliseconds, r)
		return
	}

	rlm := utils.GetPhaseRangeLightMilliseconds(r)
	if !utils.EqualWithin(3, phaseRangeLightMillis, rlm) {
		t.Errorf("want range %f got %f", phaseRangeLightMillis, rlm)
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

	satelliteWithRealValues := satellite.New(1, 81, 435, 3, 4)
	signalCell := Cell{
		ID:              signalID,
		Wavelength:      wavelength,
		PhaseRangeDelta: -117960,
		Satellite:       satelliteWithRealValues,
	}

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

	satelliteCell := satellite.New(1, 2, 3, 4, -135)
	sigCell := Cell{
		ID: 2,
		// PhaseRangeRateFromSatelliteCell: -135,
		Wavelength:          wavelength,
		PhaseRangeRateDelta: -1070,
		Satellite:           satelliteCell,
	}

	got := sigCell.PhaseRangeRateDoppler()

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
		{ID: satelliteID0},
		{ID: satelliteID1},
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

	satCell := []*satellite.Cell{
		satellite.New(satelliteID0, 0, 0, 0, 0),
		satellite.New(satelliteID1, 0, 0, 0, 0),
	}
	want := []Cell{
		{
			ID:         signalID0,
			Wavelength: wavelength0,
			RangeDelta: 0, PhaseRangeDelta: -1,
			LockTimeIndicator: 1023, HalfCycleAmbiguity: false, CarrierToNoiseRatio: 0,
			PhaseRangeRateDelta: -1,
			Satellite:           satCell[0],
		},
		{
			ID:         signalID1,
			Wavelength: wavelength1,
			RangeDelta: -1, PhaseRangeDelta: 0,
			LockTimeIndicator: 0, HalfCycleAmbiguity: true, CarrierToNoiseRatio: 1023,
			PhaseRangeRateDelta: 0,
			Satellite:           satCell[0],
		},
		{
			ID:         signalID0,
			Wavelength: wavelength0,
			RangeDelta: 262145, PhaseRangeDelta: 5,
			LockTimeIndicator: 1, HalfCycleAmbiguity: true, CarrierToNoiseRatio: 10,
			PhaseRangeRateDelta: 13,
			Satellite:           satCell[1],
		},
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
		satellite.Cell{ID: satelliteID0,
			RangeWholeMillis: 0x81, RangeFractionalMillis: 0x201},
		satellite.Cell{ID: satelliteID1,
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

	const signalID uint = 16
	const extendedInfo = 5
	const rangeWhole uint = 0x80       // 1000 0000 (128 millis)
	const rangeWholeInvalid = 255      // This marks the range value as invalid.
	const rangeFractional uint = 0x200 // 10 bits 1000 0000 ... (0.5 millis)
	const rangeDelta = int(0x40000)    // 20 bits 0100 0000 ... (1/2048 millis)
	const phaseRangeDelta int = 1
	const approxPhaseRangeRate = 6
	const phaseRangeRateDelta = 7890
	const lockTimeIndicator = 4
	const halfCycleAmbiguity = true
	const cnr = 5
	const wavelength = utils.SpeedOfLightMS / utils.Freq2

	const wantDisplayAllValid = " 1 16 {(262144, 146.383, 38523477.236), (1, 157746600.001), -27.800, (7890, 0.789, 6.789), 4, true, 5}"

	const wantDisplayInvalidRange = " 1 16 {invalid, invalid, -27.800, (7890, 0.789, 6.789), 4, true, 5}"

	const wantInvalidPhaseRangeRate = " 1 16 {(262144, 146.383, 38523477.236), (1, 157746600.001), invalid, invalid, 4, true, 5}"

	const wantDisplayZeroWavelength = " 1 16 {(262144, 146.383, 38523477.236), no wavelength, no wavelength, no wavelength, 4, true, 5}"

	invalidRangeDeltaBytes := []byte{0x80, 0x00, 0x00} // 20 bits plus filler: 1000 0000 0000 0000 0000 filler 0000
	invalidRangeDelta := int(utils.GetBitsAsInt64(invalidRangeDeltaBytes, 0, 20))

	invalidPhaseRangeRateBytes := []byte{0x80, 0x00} // 14 bits plus filler: 1000 0000 0000 00 filler 00
	invalidPhaseRangeRate := int(utils.GetBitsAsInt64(invalidPhaseRangeRateBytes, 0, 14))

	// A satellite cell with valid range and phase range rate.
	satelliteCellAllValid := satellite.New(1, rangeWhole, rangeFractional,
		extendedInfo, approxPhaseRangeRate)

	// A satellite cell with invalid range.
	satelliteCellWithInvalidRange := satellite.New(1, rangeWholeInvalid, rangeFractional,
		extendedInfo, approxPhaseRangeRate)

	// A satellite cell with invalid phase range rate
	satelliteCellWithInvalidPhaseRangeRate :=
		satellite.New(1, rangeWhole, rangeFractional,
			extendedInfo, invalidPhaseRangeRate)

	signalCellAllValid := New(signalID, satelliteCellAllValid, rangeDelta, phaseRangeDelta,
		lockTimeIndicator, halfCycleAmbiguity, cnr, phaseRangeRateDelta, wavelength)

	signalCellWithInvalidRange := New(signalID, satelliteCellWithInvalidRange,
		invalidRangeDelta, phaseRangeDelta, lockTimeIndicator, halfCycleAmbiguity,
		cnr, phaseRangeRateDelta, wavelength)

	signalCellWithInvalidPhaseRangeRate := New(signalID, satelliteCellWithInvalidPhaseRangeRate,
		rangeDelta, phaseRangeDelta, lockTimeIndicator, halfCycleAmbiguity, cnr,
		phaseRangeRateDelta, wavelength)

	signalCellWithZeroWavelength := New(signalID, satelliteCellAllValid, rangeDelta, phaseRangeDelta,
		lockTimeIndicator, halfCycleAmbiguity, cnr, phaseRangeRateDelta, 0)

	var testData = []struct {
		description string
		cell        *Cell
		want        string
	}{
		{
			"all valid", signalCellAllValid, wantDisplayAllValid,
		},
		{
			"invalid range", signalCellWithInvalidRange, wantDisplayInvalidRange,
		},
		{
			"invalid phase range rate", signalCellWithInvalidPhaseRangeRate, wantInvalidPhaseRangeRate,
		},
		{
			"zero wavelength", signalCellWithZeroWavelength, wantDisplayZeroWavelength,
		},
	}

	for _, td := range testData {

		got := td.cell.String()

		if td.want != got {
			t.Errorf("%s\n%s", td.description, diff.Diff(td.want, got))
		}
	}
}

// TestEqual checks the Equal method field by field
func TestEqual(t *testing.T) {
	testSatellite := satellite.New(1, 2, 3, 4, 5)
	testSignal := New(6, testSatellite, 7, 8, 9, true, 10, 11, 12.0)

	satelliteCell := []*satellite.Cell{
		satellite.New(1, 2, 3, 4, 5),  // Same as the test satellite.
		satellite.New(1, 20, 3, 4, 5), // Different from the test satellite.
	}

	signalCell := []*Cell{
		// This is the same as testSignal.
		New(6, satelliteCell[0], 7, 8, 9, true, 10, 11, 12.0),
		// These each differ from the test signal by one field.
		New(20, satelliteCell[0], 7, 8, 9, true, 10, 11, 12.0),
		New(6, satelliteCell[1], 7, 8, 9, true, 10, 11, 12.0),
		New(6, satelliteCell[0], 20, 8, 9, true, 10, 11, 12.0),
		New(6, satelliteCell[0], 7, 20, 9, true, 10, 11, 12.0),
		New(6, satelliteCell[0], 7, 8, 20, true, 10, 11, 12.0),
		New(6, satelliteCell[0], 7, 8, 9, false, 10, 11, 12.0),
		New(6, satelliteCell[0], 7, 8, 9, true, 20, 11, 12.0),
		New(6, satelliteCell[0], 7, 8, 9, true, 10, 20, 12.0),
		New(6, satelliteCell[0], 7, 8, 9, true, 10, 11, 20.0),
	}

	if !cmp.Equal(testSatellite, satelliteCell[0]) {
		t.Errorf("signalCell[0] should be equal")
	}
	if !cmp.Equal(testSignal, signalCell[0]) {
		t.Errorf("cell %d should be equal", 0)
	}
	for i := 1; i < len(signalCell); i++ {
		if cmp.Equal(testSignal, signalCell[i]) {
			t.Errorf("cell %d should not be equal", i)
		}
	}
}
