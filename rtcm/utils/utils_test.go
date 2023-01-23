package utils

import (
	"testing"
)

// TestGetMSMScaledRange checks getMSMScaledRange.
func TestGetMSMScaledRange(t *testing.T) {

	// getMSMScaledRange combine the 8-bit unsigned whole range, the 10-bit
	// unsigned fractional range and the 20-bit signed delta to get a
	// scaled integer with 18 bits whole and 19 bits fractional.  The whole
	// and the delta may both have values indicating that they are invalid
	// BUT the function under test assumes that the incoming values have
	// already been checked and it assumes that it will only receive valid
	// values.

	const maxWhole = 0xfe       // 1111 1110
	const maxFractional = 0x3ff // 11 1111 1111

	// The maximum delta value is (2 to the power 19) -1
	// (111 1111 1111 1111 1111 - 0x7ffff)
	const maxDelta = 0x7ffff

	// The maximum result is a 37-bit unsigned with all components at their
	// maximum value: 1 1111 1101 1111 1111 1111 1111 1111 1111 1111
	const maxScaledRange = 0x1fdfffffff

	// maxNoDelta is the result of combining the maximum whole and fractional
	// parts with a 0 or invalid delta:
	// 1 1111 1101 1111 1111 1000 0000 0000 0000 0000
	const maxNoDelta = 0x1fdff80000

	// wholeFracSmall is the result of combining a whole and fractional part,
	// both 1, with a zero or invalid delta:
	// 0 0000 0010 0000 0000 1000 0000 0000 0000 0000
	const wholeFracSmall = 0x20080000

	// allOne is the result of combining three values, all 1:
	// 0 0000 0010 0000 0000 1000 0000 0000 0000 0001
	const allOne = 0x20080001

	// deltaMinusOne is the result of combining 1, 1 and -1:
	// 0 0000 0010 0000 0000 0111 1111 1111 1111 1111
	const deltaMinusOne = 0x2007ffff

	var testData = []struct {
		WholeMillis      uint
		FractionalMillis uint
		Delta            int
		Want             uint64 // Expected result.
	}{
		{0, 0, 0, 0},
		{1, 1, 0, wholeFracSmall},
		{1, 1, 1, allOne},
		{1, 1, -1, deltaMinusOne},
		// Maximum approx range, zero delta.
		{maxWhole, maxFractional, 0, maxNoDelta},
		// All maximum values.
		{maxWhole, maxFractional, maxDelta, maxScaledRange},
	}

	for _, td := range testData {
		got := GetScaledRange(td.WholeMillis,
			td.FractionalMillis, td.Delta)
		if got != td.Want {
			if td.Delta < 0 {
				t.Errorf("(0x%x,0x%x,%d) want 0x%x, got 0x%x",
					td.WholeMillis, td.FractionalMillis, td.Delta, td.Want, got)
			} else {
				t.Errorf("(0x%x,0x%x,0x%x) want 0x%x, got 0x%x",
					td.WholeMillis, td.FractionalMillis, td.Delta, td.Want, got)
			}
		}
	}
}


// TestEqualWithin checks the EqualWithin test helper function.
func TestEqualWithin(t *testing.T) {

	var testData = []struct {
		N    uint
		F1   float64
		F2   float64
		Want bool
	}{
		{0, 100.1, 100.04, true},
		{1, 0.01, 0.04, true},
		{1, 0.01, 0.09, false}, // 0.09 will b rounded up to 0.1.
		{1, 0.5, 0.6, false},
		{1, 1, 2, false},
		{2, 1.111, 1.113, true},
		{2, 2.222, 2.232, false},
		{3, 9.9991, 9.9992, true},
	}

	for _, td := range testData {
		got := EqualWithin(td.N, td.F1, td.F2)

		if got != td.Want {
			t.Errorf("%d %f %f: want %v, got %v",
				td.N, td.F1, td.F2, td.Want, got)
		}
	}
}