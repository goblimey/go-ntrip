package utils

import (
	"testing"
)

// Signal frequencies.
const l1 = 1.57542e9
const l2 = 1.22760e9
const l5 = 1.17645e9
const e1 = 1.57542e9
const e6 = 1.27875e9
const e5a = 1.17645e+09
const e5b = 1.20714e9
const e5aPlusb = 1.191795e+09
const g1 = 1.6020e9
const g2 = 1.246e9
const b1 = 1.561098e9
const b2a = 1.17645e9
const b3i = 1.26852e9

// TestGetScaledRange checks GetScaledRange.
func TestGetScaledRange(t *testing.T) {

	// GetcaledRange combine the 8-bit unsigned whole range, the 10-bit
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

// TestGetScaledPhaseRange checks GetScaledPhaseRange.
func TestGetScaledPhaseRange(t *testing.T) {

	//     ------ Range -------
	//     whole     fractional
	//     876 5432 1098 7654 3210 9876 5432 1098 7654 3210
	//     www wwww wfff ffff fff0 0000 0000 0000 0000 0000
	//     + or -             sddd dddd dddd dddd dddd dddd <- phase range rate delta.
	//              1000 0000
	// Sanity check - the result must fit into 39 bits unsigned.
	const maxAllowedResult = 0x7fffffffff // 111 1111 1111 1111 1111 1111 1111 1111 1111 1111

	const maxWhole = 254        // 1111 1110
	const maxFractional = 0x3ff // 11 1111 1111
	// 111 1111 0111 1111 1110 0000 0000 0000 0000 0000 max whole and max fractional
	// 000 0000 0000 0000 0111 1111 1111 1111 1111 1111 plus max delta
	// 111 1111 1000 0000 0101 1111 1111 1111 1111 1111 equals this

	const maxResult = 0x7f805fffff

	const maxDelta = 0x7fffff // 24 bits signed 0111 1111 1111 1111 1111 1111
	const minDelta = -1 * 0x800000

	// maxRangeNoDelta is the result of combining the maximum whole and fractional
	// parts with a 0 delta:
	// 111 1111 0111 1111 1110 0000 0000 0000 0000 0000
	const maxRangeNoDelta = 0x7f7fe00000

	// maxRangeDeltaOne is the result of combining the maximum whole and fractional
	// parts with a delta of 1:
	// 000 0000 1000 0000 0010 0000 0000 0000 0000 0001
	const maxRangeDeltaOne = 0x7f7fe00001

	// maxDeltaOne is the result of combining the maximum whole and fractional
	// parts with a delta of -1:
	// 111 1111 0111 1111 1101 1111 1111 1111 1111 1111
	const maxDeltaMinusOne = 0x7f7fdfffff

	// maxRangeMinDelta is the result of combining the maximum whole and fractional
	// parts with the minimum delta: 0x7f7fe00000 - 0x800000.
	const maxRangeMinDelta = 0x7f7f600000

	// allOne is the result of combining three values, all 1:
	// 0 0000 0010 0000 0000 1000 0000 0000 00ffff00 0001
	const allOne = 0x80200001

	// 80200000 1000 0000 0010 00000 00000 00000 0000

	var testData = []struct {
		whole      uint
		fractional uint
		delta      int
		want       uint64
	}{

		{maxWhole, maxFractional, 0, maxRangeNoDelta},
		{maxWhole, maxFractional, 1, maxRangeDeltaOne},
		{maxWhole, maxFractional, maxDelta, maxResult},
		{1, 1, 1, allOne},
		{maxWhole, maxFractional, -1, maxDeltaMinusOne},
		{maxWhole, maxFractional, minDelta, maxRangeMinDelta},
	}

	for _, td := range testData {

		got := GetScaledPhaseRange(td.whole, td.fractional, td.delta)

		// Sanity check - with valid inputs the result must fit into 39 bits.
		if got > maxAllowedResult {
			t.Errorf("result %d is bigger than the max %d", got, maxAllowedResult)
		}

		// Check that the result is as expected.
		if got != td.want {
			if td.delta < 0 {
				t.Errorf("(0x%x,0x%x,%d) want 0x%x, got 0x%x",
					td.whole, td.fractional, td.delta, td.want, got)
			} else {
				t.Errorf("(0x%x,0x%x,0x%x) want 0x%x, got 0x%x",
					td.whole, td.fractional, td.delta, td.want, got)
			}
		}
	}
}

// TestGetPhaseRangeMilliseconds checks GetPhaseRangeMilliseconds.
func TestGetPhaseRangeMilliseconds(t *testing.T) {
	// This value is taken from RTKLIB - P2_31 in rtklib.h, used by decode_msm7 in rtcm3.c.
	// The phase range value is scaled up by shifting it up 31 bits.  Multiplying by this
	// value reverses that scaling.
	const twoToMinus31 = 4.656612873077393e-10

	// These input value are taken from TestGetScaledRange.

	// maxDeltaOne is the result of combining the maximum whole and fractional
	// parts with a delta of -1:
	// 111 1111 0111 1111 1101 1111 1111 1111 1111 1111
	const maxRangeDeltaMinusOne = 0x7f7fdfffff

	// maxRangeMinDelta is the result of combining the maximum whole and fractional
	// parts with the minimum delta: 0x7f7fe00000 - 0x800000.
	const maxRangeMinDelta = 0x7f7f600000

	var testData = []struct {
		input uint64
		want  float64
	}{
		{0, 0.0},
		{maxRangeDeltaMinusOne, maxRangeDeltaMinusOne * twoToMinus31},
		{maxRangeMinDelta, maxRangeMinDelta * twoToMinus31},
	}

	for _, td := range testData {
		got := GetPhaseRangeMilliseconds(td.input)
		if !EqualWithin(6, td.want, got) {
			t.Errorf("%d want %f got %f", td.input, td.want, got)
		}
	}
}

// TestGetPhaseRangeLightMilliseconds checks GetPhaseRangeLightMilliseconds,
func TestGetPhaseRangeLightMilliseconds(t *testing.T) {
	const twoToPower31 = 0x80000000 // 1000 0000 0000 0000 0000 0000 0000 0000
	const twoToPowerMinus31 = 1 / float64(twoToPower31)
	const rangeMilliseconds = 128.5 + float64(twoToPowerMinus31)
	const want = rangeMilliseconds * OneLightMillisecond

	got := GetPhaseRangeLightMilliseconds(rangeMilliseconds)

	if !EqualWithin(6, want, got) {
		t.Errorf("for input %f want %f got %f", rangeMilliseconds, want, got)
	}
}

// TestGetScaledPhaseRangeRate checks GetScaledPhaseRangeRate.
func TestGetScaledPhaseRangeRate(t *testing.T) {
	const maxWhole = 8191    // 14 bit signed
	const maxFraction = 9999 // signed, one in 10000
	const maxResult = maxWhole*10000 + maxFraction
	const minWhole = -8190
	const minFraction = -9998
	const minResult = minWhole*10000 + minFraction

	var testData = []struct {
		whole    int
		fraction int
		want     int64
	}{
		{0, 0, 0},
		{0, 1, 1},
		{1, 0, 10000},
		{1, 1, 10001},
		{-1, 1, -9999},
		{-1, 0, -10000},
		{-1, -1, -10001},
		{1, -1, 9999},
		{maxWhole, maxFraction, maxResult},
		{minWhole, minFraction, minResult},
	}

	for _, td := range testData {
		got := GetScaledPhaseRangeRate(td.whole, td.fraction)
		if td.want != got {
			t.Errorf("%d %d want %d got %d",
				td.whole, td.fraction, td.want, got)
		}
	}
}

// TestGetApproxRange checks GetApproxRange
func TestGetApproxRange(t *testing.T) {

	var testData = []struct {
		whole    uint
		fraction uint
		want     float64
	}{
		{1, 0, 1.0},
		{1, 512, 1.5},
		{255, 0, 255.0},
		{255, 1, 255.0 + 1.0/1024.0},
	}

	for _, td := range testData {
		got := GetApproxRange(td.whole, td.fraction)
		if !EqualWithin(6, td.want, got) {
			t.Errorf("%d %d want %f got %f",
				td.whole, td.fraction, td.want, got)
		}
	}
}

// TestGetSignalFrequencyGPS checks getSignalFrequencyGPS
func TestGetSignalFrequencyGPS(t *testing.T) {

	var testData = []struct {
		signalID uint
		want     float64
	}{
		{1, 0},
		{2, l1},
		{3, l1},
		{4, l1},
		{5, 0},
		{6, 0},
		{7, 0},
		{8, l2},
		{9, l2},
		{10, l2},
		{11, 0},
		{12, 0},
		{13, 0},
		{14, 0},
		{15, l2},
		{16, l2},
		{17, l2},
		{18, 0},
		{19, 0},
		{20, 0},
		{21, 0},
		{22, l5},
		{23, l5},
		{24, l5},
		{25, 0},
		{26, 0},
		{27, 0},
		{28, 0},
		{29, 0},
		{30, l1},
		{31, l1},
		{32, l1},
	}

	for _, td := range testData {
		got := getSignalFrequencyGPS(td.signalID)
		if td.want != got {
			t.Errorf("%d want %f got %f",
				td.signalID, td.want, got)
		}
	}
}

// TestGetSignalFrequencyGalileo checks getSignalFrequencyGPS
func TestGetSignalFrequencyGalileo(t *testing.T) {

	var testData = []struct {
		signalID uint
		want     float64
	}{
		{1, 0},
		{2, e1},
		{3, e1},
		{4, e1},
		{5, e1},
		{6, e1},
		{7, 0},
		{8, e6},
		{9, e6},
		{10, e6},
		{11, e6},
		{12, e6},
		{13, 0},
		{14, e5b},
		{15, e5b},
		{16, e5b},
		{17, 0},
		{18, e5aPlusb},
		{19, e5aPlusb},
		{20, e5aPlusb},
		{21, 0},
		{22, e5a},
		{23, e5a},
		{24, e5a},
		{25, 0},
		{26, 0},
		{27, 0},
		{28, 0},
		{29, 0},
		{30, 0},
		{31, 0},
		{32, 0},
	}

	for _, td := range testData {
		got := getSignalFrequencyGalileo(td.signalID)
		if td.want != got {
			t.Errorf("%d want %f got %f",
				td.signalID, td.want, got)
		}
	}
}

// TestGetSignalFrequencyGlonass checks getSignalFrequencyGlonass
func TestGetSignalFrequencyGlonass(t *testing.T) {

	var testData = []struct {
		signalID uint
		want     float64
	}{
		{1, 0},
		{2, g1},
		{3, g1},
		{4, 0},
		{5, 0},
		{6, 0},
		{7, 0},
		{8, g2},
		{9, g2},
		{10, 0},
		{11, 0},
		{12, 0},
		{13, 0},
		{14, 0},
		{15, 0},
		{16, 0},
		{17, 0},
		{18, 0},
		{19, 0},
		{20, 0},
		{21, 0},
		{22, 0},
		{23, 0},
		{24, 0},
		{25, 0},
		{26, 0},
		{27, 0},
		{28, 0},
		{29, 0},
		{30, 0},
		{31, 0},
		{32, 0},
	}

	for _, td := range testData {
		got := getSignalFrequencyGlonass(td.signalID)
		if td.want != got {
			t.Errorf("%d want %f got %f",
				td.signalID, td.want, got)
		}
	}
}

// TestGetSignalFrequencyBeidou checks getSignalFrequencyBeidou
func TestGetSignalFrequencyBeidou(t *testing.T) {

	var testData = []struct {
		signalID uint
		want     float64
	}{
		{1, 0},
		{2, b1},
		{3, b1},
		{4, b1},
		{5, 0},
		{6, 0},
		{7, 0},
		{8, b3i},
		{9, b3i},
		{10, b3i},
		{11, 0},
		{12, 0},
		{13, 0},
		{14, b2a},
		{15, b2a},
		{16, b2a},
		{17, 0},
		{18, 0},
		{19, 0},
		{20, 0},
		{21, 0},
		{22, 0},
		{23, 0},
		{24, 0},
		{25, 0},
		{26, 0},
		{27, 0},
		{28, 0},
		{29, 0},
		{30, 0},
		{31, 0},
		{32, 0},
	}

	for _, td := range testData {
		got := getSignalFrequencyBeidou(td.signalID)
		if td.want != got {
			t.Errorf("%d want %f got %f",
				td.signalID, td.want, got)
		}
	}
}

// TestGetSignalWavelengthGPS checks getSignalWavelengthGPS
func TestGetSignalWavelengthGPS(t *testing.T) {

	var testData = []struct {
		signalID uint
		want     float64
	}{
		{1, 0},
		{2, SpeedOfLightMS / l1},
		{33, 0},
	}

	for _, td := range testData {
		got := getSignalWavelengthGPS(td.signalID)
		if !EqualWithin(6, td.want, got) {
			t.Errorf("%d want %f got %f",
				td.signalID, td.want, got)
		}
	}
}

// TestGetSignalWavelengthGalilio checks getSignalWavelengthGalileo
func TestGetSignalWavelengthGalileo(t *testing.T) {

	var testData = []struct {
		signalID uint
		want     float64
	}{
		{1, 0},
		{8, SpeedOfLightMS / e6},
		{33, 0},
	}

	for _, td := range testData {
		got := getSignalWavelengthGalileo(td.signalID)
		if !EqualWithin(6, td.want, got) {
			t.Errorf("%d want %f got %f",
				td.signalID, td.want, got)
		}
	}
}

// TestGetSignalWavelengthGalilio checks getSignalWavelengthGalileo
func TestGetSignalWavelengthGlonass(t *testing.T) {

	var testData = []struct {
		signalID uint
		want     float64
	}{
		{1, 0},
		{3, SpeedOfLightMS / g1},
		{33, 0},
	}

	for _, td := range testData {
		got := getSignalWavelengthGlonass(td.signalID)
		if !EqualWithin(6, td.want, got) {
			t.Errorf("%d want %f got %f",
				td.signalID, td.want, got)
		}
	}
}

// TestGetSignalWavelengthBeidou checks getSignalWavelengthBeidou
func TestGetSignalWavelengthBeidou(t *testing.T) {

	var testData = []struct {
		signalID uint
		want     float64
	}{
		{1, 0},
		{16, SpeedOfLightMS / b2a},
		{33, 0},
	}

	for _, td := range testData {
		got := getSignalWavelengthBeidou(td.signalID)
		if !EqualWithin(6, td.want, got) {
			t.Errorf("%d want %f got %f",
				td.signalID, td.want, got)
		}
	}
}

// TestGetSignalWavelength checks GetSignalWavelength
func TestGetSignalWavelength(t *testing.T) {

	var testData = []struct {
		constellation string
		signalID      uint
		want          float64
	}{
		{"GPS", 1, 0},
		{"GPS", 15, SpeedOfLightMS / l2},
		{"GPS", 33, 0},
		{"Galileo", 1, 0},
		{"Galileo", 22, SpeedOfLightMS / e5a},
		{"Galileo", 33, 0},
		{"GLONASS", 1, 0},
		{"GLONASS", 8, SpeedOfLightMS / g2},
		{"GLONASS", 33, 0},
		{"Beidou", 1, 0},
		{"Beidou", 16, SpeedOfLightMS / b2a},
		{"Beidou", 33, 0},
		{"junk", 2, 0},
	}

	for _, td := range testData {
		got := GetSignalWavelength(td.constellation, td.signalID)
		if !EqualWithin(6, td.want, got) {
			t.Errorf("%s %d want %f got %f",
				td.constellation, td.signalID, td.want, got)
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

// TestGetBitsAsUint64 checks GetBitsAsUint64.
func TestGetBitsAsUint64(t *testing.T) {

	var bitStream = []byte{
		/* bits 000-063 */ 0x00, 0xaa, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		/* bits 064-127 */ 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		/* bits 128-195 */ 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
	}
	var testData = []struct {
		position uint
		length   uint
		want     uint64
	}{
		// Large numbers worked out using
		// https://www.rapidtables.com/convert/number/hex-to-decimal.html/
		{0, 1, 0},
		{0, 2, 0},
		{0, 8, 0},
		{0, 11, 5},
		{15, 16, 32767},
		{16, 16, 65535},
		{16, 2, 3},
		{16, 8, 255},
		{64, 32, 0},
		{64, 64, 0},
		{95, 32, 0},
		{95, 64, 2147483647},
		{127, 64, 9223372036854775807},
		{96, 32, 0},
		{96, 64, 4294967295},
		{128, 32, 4294967295},
		{128, 64, 18446744073709551615},
	}

	for _, td := range testData {
		got := GetBitsAsUint64(bitStream, td.position, td.length)
		if td.want != got {
			t.Errorf("%d %d want %d got %d",
				td.position, td.length, td.want, got)
		}
	}
}

// TstGetBitsAsInt64 checks GetBitsAsInt64.
func TestGetBitsAsInt64(t *testing.T) {
	var bitStream = []byte{
		0x00, 0xaa, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
	}
	var testData = []struct {
		position uint
		length   uint
		want     int64
	}{
		// Two's complement negative numbers worked out using
		// https://www.rapidtables.com/convert/number/hex-to-decimal.html/
		{0, 1, 0},
		{0, 2, 0},
		{0, 8, 0},
		{0, 11, 5},
		{8, 1, 1}, // two's complement of a single bit set to 1 is 1.
		{8, 8, -86},
		{8, 16, -21761},
		{15, 2, 1},
		{15, 16, 32767},
		{16, 16, -1},
		{56, 16, -256},
		{64, 32, 0},
		{64, 64, 0},
		{127, 32, 2147483647},
		{127, 64, 9223372036854775807},
		{128, 32, -1},
		{128, 64, -1},
	}

	for _, td := range testData {
		got := GetBitsAsInt64(bitStream, td.position, td.length)
		if td.want != got {
			t.Errorf("%d %d want %d got %d",
				td.position, td.length, td.want, got)
		}
	}
}

func TestGetNumberOfSignalCells(t *testing.T) {
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
		// Padding
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}

	const bitsPerMSM7SignalCell = 80

	const startPosition = 48 // Byte 6.

	const want = 3

	var testData = []struct {
		bitStream     []byte
		startPosition uint
		want          int
	}{
		{bitStream, startPosition, want},
		// Not enough data for one cell.
		{bitStream[60:], 0, 0},
	}

	for _, td := range testData {
		got := GetNumberOfSignalCells(td.bitStream, td.startPosition, bitsPerMSM7SignalCell)

		if td.want != got {
			t.Errorf("want %d got %d", td.want, got)
		}
	}
}

func TestSlicesEqual(t *testing.T) {
	empty1 := make([]uint, 0)
	empty2 := make([]uint, 0)
	a := []uint{1, 2, 3}
	b := []uint{1, 2, 3}
	c := []uint{1, 2}
	d := []uint{1, 2, 4}

	var testData = []struct {
		description string
		s1          []uint
		s2          []uint
		want        bool
	}{
		// Two's complement negative numbers worked out using
		// https://www.rapidtables.com/convert/number/hex-to-decimal.html/
		{"nil, nil", nil, nil, true},
		{"nil, empty", nil, empty1, true},
		{"empty, nil", empty1, nil, true},
		{"empty, empty", empty1, empty2, true},
		{"a, b", a, b, true},
		{"a, c", a, c, false},
		{"b, d", b, d, false},
	}

	for _, td := range testData {
		got := SlicesEqual(td.s1, td.s2)
		if td.want != got {
			t.Errorf("%s want %v got %v",
				td.description, td.want, got)
		}
	}
}
