package satellite

import (
	"fmt"
	"testing"

	"github.com/goblimey/go-ntrip/rtcm/utils"
)

// Tests for the handling of an MSM7 satellite cell.

// TestNew checks that New correctly creates an MSM7 satellite cell.
func TestNew(t *testing.T) {

	const rangeWhole uint = 1
	const rangeFractional uint = 2
	const extendedInfo uint = 3
	const phaseRangeRate = 4

	invalidPhaseRangeRateBytes := []byte{0x80, 00} // 14 bits plus filler: 1000 0000 0000 00 filler 00
	invalidPhaseRangeRate := int(utils.GetBitsAsInt64(invalidPhaseRangeRateBytes, 0, 14))

	var testData = []struct {
		Description      string
		ID               uint
		WholeMillis      uint
		FractionalMillis uint
		ExtendedInfo     uint
		PhaseRangeRate   int
		Want             Cell // expected result
	}{
		{"MSM7 all valid", 3, rangeWhole, rangeFractional, extendedInfo, phaseRangeRate,
			Cell{ID: 3,
				RangeWholeMillis: rangeWhole, RangeFractionalMillis: rangeFractional,
				ExtendedInfo: extendedInfo, PhaseRangeRate: phaseRangeRate}},
		{"MSM7 with invalid range", 4, utils.InvalidRange, rangeFractional, extendedInfo, phaseRangeRate,
			Cell{ID: 4,
				RangeWholeMillis: utils.InvalidRange, RangeFractionalMillis: rangeFractional,
				ExtendedInfo: extendedInfo, PhaseRangeRate: phaseRangeRate}},
		{"MSM7 with invalid range and phase range rate", 5, utils.InvalidRange, rangeFractional, extendedInfo, invalidPhaseRangeRate,
			Cell{ID: 5,
				RangeWholeMillis: utils.InvalidRange, RangeFractionalMillis: rangeFractional,
				ExtendedInfo: extendedInfo, PhaseRangeRate: invalidPhaseRangeRate}},
	}
	for _, td := range testData {
		got := *New(
			td.ID, td.WholeMillis, td.FractionalMillis, td.ExtendedInfo, td.PhaseRangeRate)
		if got != td.Want {
			t.Errorf("%s: want %v, got %v", td.Description, td.Want, got)
		}
	}
}

// TestGetSatelliteCells checks that GetSatelliteCells correctly interprets a
// bit stream from an MSM7 message containing two satellite cells.
func TestGetSatelliteCells(t *testing.T) {
	const satelliteID1 = 42
	const satelliteID2 = 43
	satellites := []uint{satelliteID1, satelliteID2}

	// The bit stream starts at byte 4 and contains two satellite cells - two 8-bit
	// whole millis followed by two 4-bit extended info fields, two 10-bit fractional
	// millis and two 14-bit signed phase range rate values set like so:
	// 1000 0001|  0000 0001|  1001|0110|  1000 0000  01|00 0000
	// 0001|1111   1111 1111   11|01 0000  0000 0001
	bitstream := []byte{
		0x00, 0xff, 0x00, 0xff,
		0x81, 0x01, 0x96, 0x80, 0x40,
		0x1f, 0xff, 0xd0, 0x01,
	}

	const startPosition = 32 // byte 4.

	want := []Cell{
		Cell{ID: satelliteID1,
			RangeWholeMillis: 0x81, RangeFractionalMillis: 0x201,
			ExtendedInfo: 9, PhaseRangeRate: -1},
		Cell{ID: satelliteID2,
			RangeWholeMillis: 1, RangeFractionalMillis: 1,
			ExtendedInfo: 6, PhaseRangeRate: 4097},
	}

	got, satError := GetSatelliteCells(bitstream, startPosition, satellites)

	if satError != nil {
		t.Error(satError)
		return
	}

	if len(got) != 2 {
		t.Errorf("got %d cells, expected 2", len(got))
	}

	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got %v expected %v", got[i], want[i])
		}
	}
}

// TestGetSatelliteCellsShortMessage checks that getMSM7SatelliteCells provides the correct
// error message if the bit stream is too short to hold two satellite cells.
func TestGetSatelliteCellsShortMessage(t *testing.T) {
	const satelliteID1 = 42
	const satelliteID2 = 43
	const wantError = "overrun - not enough data for 2 MSM7 satellite cells - need 72 bits, got 64"

	satellites := []uint{satelliteID1, satelliteID2}

	// The bit stream is as in the previous test, but we call
	// GetSatelliteCell with an offset, which makes the bit stream
	// too short and causes an error.
	bitstream := []byte{0x81, 0x01, 0x96, 0x80, 0x40,
		0x1f, 0xff, 0xd0, 0x01}

	got, gotError := GetSatelliteCells(bitstream, 8, satellites)

	if gotError == nil {
		t.Error("expected an overrun error")
		return
	}

	if gotError.Error() != wantError {
		em := fmt.Sprintf("\nwant %s\ngot  %s", wantError, gotError)
		t.Error(em)
		return
	}

	if len(got) != 0 {
		t.Errorf("got %d cells, expected none", len(got))
	}
}

// TestString checks the string method.
func TestString(t *testing.T) {
	const wholeMillis = 2
	// The fractional millis value is ten bits (1/1024).
	// This value (10 0000 0110) represents 0.5.
	const fracMillis = 0x200
	const extendedInfo = 3
	const phaseRangeRate = 4

	const satRange = 2.5 * utils.OneLightMillisecond

	const displayTemplate = " 1 {%.3f, 3, 4}"

	wantDisplay := fmt.Sprintf(displayTemplate, satRange)

	satellite := New(1, wholeMillis, fracMillis, extendedInfo, phaseRangeRate)

	display := satellite.String()

	if wantDisplay != display {
		t.Errorf("want \"%s\" got \"%s\"", wantDisplay, display)
	}
}

// TestStringWithInvalidRange checks the string method.
func TestStringWithInvalidRange(t *testing.T) {
	// When the whole millis value is marked as invalid,
	// both range values are ignored
	const invalidWhole = 0xff
	const fracMillis = 0x206 // 10 0000 0110
	const extendedInfo = 2
	const phaseRangeRate = 3

	const wantDisplay = " 1 {invalid, 2, 3}"

	satellite := New(1, invalidWhole, fracMillis, extendedInfo, phaseRangeRate)

	display := satellite.String()

	if wantDisplay != display {
		t.Errorf("want \"%s\" got \"%s\"", wantDisplay, display)
	}

}

// TestStringWithInvalidPhaseRangeRate checks that the string method displays
// the phase range rate correctly.
func TestStringWithInvalidPhaseRangeRate(t *testing.T) {
	// The approximate phase range rate value is marked as invalid.
	const rangeWhole = 2
	const rangeFrac = 0x001 // 00 0000 0001
	const extendedInfo = 3

	invalidRangeBits := []byte{0x20, 0, 0} // 0010 0000 0000 0000
	phaseRangeRate := int(utils.GetBitsAsInt64(invalidRangeBits, 2, 14))

	const satRange = (2 + (float64(rangeFrac) * 1.0 / 1024)) * utils.OneLightMillisecond

	const displayTemplate = " 1 {%.3f, 3, invalid}"

	wantDisplay := fmt.Sprintf(displayTemplate, satRange)

	satellite := New(1, rangeWhole, rangeFrac, extendedInfo, phaseRangeRate)

	display := satellite.String()

	if wantDisplay != display {
		t.Errorf("want \"%s\" got \"%s\"", wantDisplay, display)
	}

}
