package satellite

import (
	"fmt"
	"testing"

	"github.com/goblimey/go-ntrip/rtcm/utils"
)

// Tests for the handling of an MSM4 satellite cell.

// TestNew checks that New correctly creates an MSM4 satellite cell.
func TestNew(t *testing.T) {

	const rangeWhole uint = 1
	const rangeFractional uint = 2

	var testData = []struct {
		Description      string
		ID               uint
		WholeMillis      uint
		FractionalMillis uint
		Want             Cell
	}{
		{"MSM4, all valid", 1, rangeWhole, rangeFractional,
			Cell{ID: 1,
				RangeWholeMillis: 1, RangeFractionalMillis: 2}},
		{"MSM4 with invalid range", 2, utils.InvalidRange, rangeFractional,
			Cell{ID: 2,
				RangeWholeMillis: utils.InvalidRange, RangeFractionalMillis: rangeFractional}},
	}
	for _, td := range testData {
		got := *New(td.ID, td.WholeMillis, td.FractionalMillis)
		if got != td.Want {
			t.Errorf("%s: want %v, got %v", td.Description, td.Want, got)
		}
	}
}

// TestGetSatelliteCells checks that GetSatelliteCells correctly interprets a
// bit stream from an MSM4 message containing two satellite cells.
func TestGetSatelliteCells(t *testing.T) {
	const satelliteID1 = 42
	const satelliteID2 = 43
	satellites := []uint{satelliteID1, satelliteID2}

	// The bit stream starts at bit 16 and contains two satellite cells - two
	// 8-bit whole millis followed by two 10-bit fractional millis set like so:
	// 1000 0001|  0000 0001|  0000 0000  00|11 1111  1111|0000
	bitstream := []byte{0xff, 0xff, 0x81, 0x01, 0x00, 0x3f, 0xf0,
		// CRC
		0, 0, 0}
	const startPosition = 16

	want := []Cell{
		Cell{ID: satelliteID1,
			RangeWholeMillis: 0x81, RangeFractionalMillis: 0},
		Cell{ID: satelliteID2,
			RangeWholeMillis: 1, RangeFractionalMillis: 1023},
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

// TestGetSatelliteCellsShortMessage checks that GetSatelliteCells provides the correct
// error message if the bit stream is too short to hold two satellite cells.
func TestGetSatelliteCellsShortMessage(t *testing.T) {
	const satelliteID1 = 42
	const satelliteID2 = 43
	const wantError = "overrun - not enough data for 2 MSM4 satellite cells - need 36 bits, got 32"

	satellites := []uint{satelliteID1, satelliteID2}

	// The bit stream is as in the previous test, but we call
	// GetSatelliteCell with an offset, which makes the bit stream
	// too short and causes an error.
	bitstream := []byte{0x81, 0x01, 0x00, 0x3f, 0xf0,
		// CRC
		0, 0, 0}

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
	// The fractional millis value is ten bits (1/1024).
	// This value (10 0000 0110) represents 0.5.
	const fracMillis = 0x200

	const satRange = 2.5 * utils.OneLightMillisecond

	const displayTemplate = " 1 {%.3f}"

	wantDisplay := fmt.Sprintf(displayTemplate, satRange)

	satellite := New(1, 2, fracMillis)

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

	const wantDisplay = " 1 {invalid}"

	satellite := New(1, invalidWhole, fracMillis)

	display := satellite.String()

	if wantDisplay != display {
		t.Errorf("want \"%s\" got \"%s\"", wantDisplay, display)
	}
}
