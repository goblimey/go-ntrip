// the utils package contains general-purpose functions for the RTCM software.
package utils

import (
	"math"
)

// SpeedOfLightMS is the speed of light in metres per second.
const SpeedOfLightMS = 299792458.0

// OneLightMillisecond is the distance in metres traveled by light in one
// millisecond.  The value can be used to convert a range in milliseconds to a
// distance in metres.  The speed of light is 299792458.0 metres/second.
const OneLightMillisecond float64 = 299792.458

// GPS signal frequencies.  See https://en.wikipedia.org/wiki/GPS_signals for some
// clues, for example "In the case of the original GPS design, two frequencies are
// utilized; one at 1575.42 MHz (10.23 MHz × 154) called L1; and a second at
// 1227.60 MHz (10.23 MHz × 120), called L2.".  Later the same document describes
// band L2C and L5.
//
// Galileo uses the same frequencies but gives them different names - GPS L5 is
// Galileo E5a, and so on.
//
// The RTKLIB source code defines the bands and frequencies for all constellations.

// Freq1 is the L1/E1 signal frequency in Hz.
const Freq1 float64 = 1.57542e9

// Freq2 is the L2 frequency in Hz.
const Freq2 float64 = 1.22760e9

// Freq5 is the L5/E5a frequency in Hz.
const Freq5 float64 = 1.17645e9

// Freq6 is the E6/LEX frequency (Hz).
const Freq6 float64 = 1.27875e9

// Freq7 is the E5b requency (Hz).
const Freq7 float64 = 1.20714e9

// Freq8 is the E5a+b  frequency (Hz).
const Freq8 float64 = 1.191795e9

// Freq1Glo is the GLONASS G1 base frequency (Hz).
const Freq1Glo float64 = 1.60200e9

// BiasFreq1Glo is the GLONASS G1 bias frequency (Hz/n).
const BiasFreq1Glo float64 = 0.56250e6

// fReq2Glo is the GLONASS G2 base frequency (Hz).
const Freq2Glo float64 = 1.24600e9

// BiasFreq2Glo is the GLONASS G2 bias frequency (Hz/n).
const BiasFreq2Glo float64 = 0.43750e6

// Freq3Glo is the GLONASS G3 frequency (Hz).
const Freq3Glo float64 = 1.202025e9

// Freq1BD is the BeiDou B1 frequency (Hz).
const Freq1BD float64 = 1.561098e9

// Freq2BD is the BeiDou B2 frequency (Hz).
const Freq2BD float64 = 1.20714e9

// Freq3BD is the BeiDou B3 frequency (Hz).
const Freq3BD float64 = 1.26852e9

// LeaderLengthBytes is the length of the message frame leader in bytes.
const LeaderLengthBytes = 3

// LeaderLengthBits is the length of the message frame leader in bits.
const LeaderLengthBits = LeaderLengthBytes * 8

// CRCLengthBytes is the length of the Cyclic Redundancy check value in bytes.
const CRCLengthBytes = 3

// CRCLengthBits is the length of the Cyclic Redundancy check value in bits.
const CRCLengthBits = CRCLengthBytes * 8

// GetPhaseRangeLightMilliseconds gets the phase range of the signal in
// light milliseconds.
func GetPhaseRangeLightMilliseconds(rangeMetres float64) float64 {
	return rangeMetres * OneLightMillisecond
}

// GetScaledRange combines the components of the range from an MSM message and
// returns the result as a 37-bit scaled integer, 8 bits whole, 29 bits fractional.
func GetScaledRange(wholeMillis, fractionalMillis uint, delta int) uint64 {
	// The raw range values are in milliseconds giving the transit time of each
	// signal from the satellite to the GPS receiver.  This can be converted to
	// the distance between the two using the speed of light.
	//
	// Since the signals were sent from the same satellite at the same time, the
	// transit times should always be the same.  In fact they can be different
	// because of factors such as interference by the ionosphere having a different
	// effect on signals of a different frequency.
	//
	// The transit time is provided in three parts: the unsigned  approximate range,
	// the unsigned 10-bit fractional range (in units of 1024 ms) and the 20-bit
	// signed delta. (In an MSM4 the delta is 15 bits with less resolution but
	// the caller is expected to scale have scaled it up by this point.)
	//
	// The approximate values are scaled and merged, and then the (positive or negative)
	// delta is added to give the resulting scaled integer, 18 bits whole and 19 bits
	// fractional:
	//
	//       8 bits  |     29 bits fractional
	//        whole
	//
	//     |--- approx Range ----|
	//     |whole    |fractional |
	//                           |------- delta --------|
	//     w wwww wwwf ffff ffff f000 0000 0000 0000 0000
	//     + pos or neg delta    dddd dddd dddd dddd dddd <- 20-bit signed delta
	//
	//
	// The two parts of the approximate value are uint and much bigger than the
	// delta when scaled, so the result is always positive - the transit time. The
	// calculation is done on scaled integers rather tham floating point values to
	// preserve accuracy for as long as possible.
	//
	// Hiving off this part of the calculation into a separate function eases unit
	// testing.

	return getScaledValue(wholeMillis, 29, fractionalMillis, 19, delta)
}

// GetPhaseRangeMilliseconds gets the phase range of the signal in milliseconds.
func GetPhaseRangeMilliseconds(scaledRange uint64) float64 {

	// The scaled range is made up from the range values in the satellite
	// cell and the phase range delta value in the signal cell.

	// The scaled range has 31 bits fractional part.
	// scaleFactor is two to the power of 31:
	// 1000 0000 0000 0000 0000 0000 0000 0000
	const scaleFactor = 0x80000000

	// Restore the scale of the aggregate value.
	return float64(scaledRange) / scaleFactor
}

// GetScaledPhaseRange combines the components of the phase range from an MSM message
// and returns the result as a 41-bit scaled integer, 8 bits whole, 33 bits
// fractional.
func GetScaledPhaseRange(wholeMillis, fractionalMillis uint, delta int) uint64 {
	// This is similar to getScaledRange, but the amounts shifted are different.
	// The incoming delta value is 24 bits signed and the delta and the fractional
	// part share 3 bits, producing a 39-bit result.
	//
	//     ------ Range -------
	//     whole     fractional
	//     876 5432 1098 7654 3210 9876 5432 1098 7654 3210
	//     www wwww wfff ffff fff0 0000 0000 0000 0000 0000
	//     + or -             dddd dddd dddd dddd dddd dddd <- phase range rate delta.

	return getScaledValue(wholeMillis, 31, fractionalMillis, 21, delta)
}

// GetScaledPhaseRangeRate takes the comonent values of the phase range rate
// and returns them as a scaled integer with a scale factor of 10,000.
func GetScaledPhaseRangeRate(wholeMillis, fractionalMillis int) int64 {

	// This is similar to getAggregateRange, but for the two-part phase
	// range rate.  The 14-bit signed phase range rate value in the
	// satellite cell is in milliseconds and the 15-bit signed delta in
	// the signal cell is in ten thousandths of a millisecond.  The result
	// is the rate in milliseconds scaled up by 10,000.
	aggregatePhaseRangeRate := int64(wholeMillis) * 10000
	aggregatePhaseRangeRate += int64(fractionalMillis)

	return aggregatePhaseRangeRate
}

// GetApproxRange takes an 8-bit whole value and a 10-bit fractional value (in 1/1024 units)
// merges them and produces the
// approximate range.
func GetApproxRange(wholeMillis, fractionalMillis uint) float64 {
	const twoToPowerTen = 0x400 // 100 0000 0000
	scaledRange := wholeMillis<<10 | fractionalMillis
	return float64(scaledRange) / twoToPowerTen
}

// GetWavelength returns the carrier wavelength for a signal ID.
// The result depends upon the constellation, each of which has its
// own list of signals and equivalent wavelengths.  Some of the possible
// signal IDs are not used and so have no associated wavelength, so the
// result may be an error.
//
func GetWavelength(constellation string, signalID uint) float64 {

	switch constellation {
	case "GPS":
		return getSigWaveLenGPS(signalID)
	case "Galileo":
		return getSigWaveLenGalileo(signalID)
	case "GLONASS":
		return getSigWaveLenGlo(signalID)
	case "BeiDou":
		return getSigWaveLenBD(signalID)
	default:
		return 0
	}

}

// GetNumberOfSignalCells gets the number of signal cells in the MSM message
// contained in a bitstream.  The startPosition is the point in the bit stream
// where the signal cells start.  The bitsPerCell gives the size of each cell.
//
func GetNumberOfSignalCells(bitStream []byte, startPosition, bitsPerCell uint) int {
	// This is used to interpret MSM4 and MSM7 messages.  The cell size is
	// different for both of those messages but apart from that, the logic is
	// same.
	//
	// The bit stream may have a series of zero byte padding at the end.
	// Instead of an array of cells, the bitstream contains an array of range
	// values followed by an array of phase range values ...., with the
	// padding at the end, so it's not simple to find the number of cells.
	// We have to work backwards removing any redundant bits at the end of the
	// bit stream.

	bitsLeft := (len(bitStream) * 8) - int(startPosition)
	cellsLeft := bitsLeft / int(bitsPerCell)
	cells := make([]uint64, 0)
	pos := startPosition
	for i := 0; i < cellsLeft; i++ {
		cell := GetBitsAsUint64(bitStream, pos, bitsPerCell)
		pos += bitsPerCell
		cells = append(cells, cell)
	}

	for {
		if len(cells) == 0 {
			// This shouldn't happen, but just in case ...
			break
		} else {
			lastCell := cells[len(cells)-1]
			if lastCell == 0 {
				// We have a stream of zero bits at the end of the signal data.
				// Remove it from the cell list.
				cells = cells[:len(cells)-1]
			} else {
				break
			}
		}
	}

	return len(cells)
}

// getSigWaveLenGPS returns the signal carrier wavelength for a GPS satellite
// if it's defined.
func getSigWaveLenGPS(signalID uint) float64 {
	// Only some signal IDs are in use.
	var frequency float64
	switch signalID {
	case 2:
		frequency = Freq1
	case 3:
		frequency = Freq1
	case 4:
		frequency = Freq1
	case 8:
		frequency = Freq2
	case 9:
		frequency = Freq2
	case 10:
		frequency = Freq2
	case 15:
		frequency = Freq2
	case 16:
		frequency = Freq2
	case 17:
		frequency = Freq2
	case 22:
		frequency = Freq5
	case 23:
		frequency = Freq5
	case 24:
		frequency = Freq5
	case 30:
		frequency = Freq1
	case 31:
		frequency = Freq1
	case 32:
		frequency = Freq1
	default:
		return 0 // No matching frequency.
	}
	return SpeedOfLightMS / frequency
}

// GetSigWaveLenGalileo returns the signal carrier wavelength for a Galileo satellite
// if it's defined.
func getSigWaveLenGalileo(signalID uint) float64 {
	// Only some signal IDs are in use.
	var frequency float64
	switch signalID {
	case 2:
		frequency = Freq1
	case 3:
		frequency = Freq1
	case 4:
		frequency = Freq1
	case 5:
		frequency = Freq1
	case 6:
		frequency = Freq1
	case 8:
		frequency = Freq6
	case 9:
		frequency = Freq6
	case 10:
		frequency = Freq6
	case 11:
		frequency = Freq6
	case 12:
		frequency = Freq6
	case 14:
		frequency = Freq7
	case 15:
		frequency = Freq7
	case 16:
		frequency = Freq7
	case 18:
		frequency = Freq8
	case 19:
		frequency = Freq8
	case 20:
		frequency = Freq8
	case 22:
		frequency = Freq5
	case 23:
		frequency = Freq5
	case 24:
		frequency = Freq5
	default:
		return 0 // No matching frequency.
	}
	return SpeedOfLightMS / frequency
}

// GetSigWaveLenGlo gets the signal carrier wavelength for a GLONASS satellite
// if it's defined.
//
func getSigWaveLenGlo(signalID uint) float64 {
	// Only some signal IDs are in use.
	var frequency float64
	switch signalID {
	case 2:
		frequency = Freq1Glo
	case 3:
		frequency = Freq1Glo
	case 8:
		frequency = Freq2Glo
	case 9:
		frequency = Freq2Glo
	default:
		return 0 // No matching frequency.
	}
	return SpeedOfLightMS / frequency
}

// GetSigWaveLenBD returns the signal carrier wavelength for a Beidou satellite
// if it's defined.
//
func getSigWaveLenBD(signalID uint) float64 {
	// Only some signal IDs are in use.
	var frequency float64
	switch signalID {
	case 2:
		frequency = Freq1BD
	case 3:
		frequency = Freq1BD
	case 4:
		frequency = Freq1BD
	case 8:
		frequency = Freq3BD
	case 9:
		frequency = Freq3BD
	case 10:
		frequency = Freq3BD
	case 14:
		frequency = Freq2BD
	case 15:
		frequency = Freq2BD
	case 16:
		frequency = Freq2BD
	default:
		return 0 // No matching frequency.
	}
	return SpeedOfLightMS / frequency
}

// GetBitsAsUint64 extracts len bits from a slice of  bytes, starting
// at bit position pos and returns them as a uint.  See RTKLIB's getbitu.
func GetBitsAsUint64(buff []byte, pos uint, len uint) uint64 {
	// The C version in RTKLIB is:
	//
	// extern unsigned int getbitu(const unsigned char *buff, int pos, int len)
	// {
	//     unsigned int bits=0;
	//     int i;
	//     for (i=pos;i<pos+len;i++) bits=(bits<<1)+((buff[i/8]>>(7-i%8))&1u);
	//     return bits;
	// }
	//
	const u64One uint64 = 1
	var result uint64 = 0
	for i := pos; i < pos+len; i++ {
		byteNumber := i / 8
		// Work on a 64-bit copy of the byte contents.
		var byteContents uint64 = uint64(buff[byteNumber])
		var shiftBy uint = 7 - i%8
		// Shift the contents down to put the desired bit at the bottom.
		b := byteContents >> shiftBy
		// Extract the bottom bit.
		bit := b & u64One
		// Shift the result up one bit and glue in the extracted bit.
		result = (result << 1) | uint64(bit)
	}
	return result
}

// GetBitsAsInt64 extracts len bits from a slice of bytes, starting at bit
// position pos, interprets the bits as a twos-complement integer and returns
// the resulting as a 64-bit signed int.  Se RTKLIB's getbits() function.
func GetBitsAsInt64(buff []byte, pos uint, len uint) int64 {
	// This algorithm is a version of the Python code in
	//  https://en.wikipedia.org/wiki/Two%27s_complement,
	//
	// def twos_complement(input_value: int, num_bits: int) -> int:
	//     """Calculates a two's complement integer from the given input value's bits."""
	//     mask = 2 ** (num_bits - 1)
	//     return -(input_value & mask) + (input_value & ~mask)

	// If the first bit is a 1, the result is negative.
	negative := GetBitsAsUint64(buff, pos, 1) == 1
	// Get the whole bit string
	uval := GetBitsAsUint64(buff, pos, len)
	// If it's not negative, we're done.
	if negative {
		// It's negative.  Use the algorithm from the Wiki page.
		var mask uint64 = 2 << (len - 2)
		weightOfTopBit := int64(uval & mask)
		weightOfLowerBits := int64(uval & ^mask)
		return (-1 * weightOfTopBit) + weightOfLowerBits
	}

	return int64(uval)
}

// getScaledValue is a helper for functions such as getScaledRange.
func getScaledValue(v1, shift1, v2, shift2 uint, delta int) uint64 {
	scaledApprox := (uint64(v1) << shift1) | (uint64(v2) << shift2)
	// Add the signed delta.
	scaledResult := uint64(int64(scaledApprox) + int64(delta))
	return scaledResult
}

// SlicesEqual returns true if uint slices a and b contain the same
// elements.  A nil argument is equivalent to an empty slice.
// https://yourbasic.org/golang/compare-slices/
func SlicesEqual(a, b []uint) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

// EqualWithin return true if the given float64 values are equal
// within (precision) decimal places after rounding.  (This can fail if
// either of the numbers or the difference between them are too large.)
func EqualWithin(precision uint, f1, f2 float64) bool {

	// see http://docs.oracle.com/cd/E19957-01/806-3568/ncg_goldberg.html

	var scaleFactor float64 = math.Pow(10, float64(precision))

	f1 = math.Round(f1 * scaleFactor)
	f2 = math.Round(f2 * scaleFactor)

	return math.Abs(f1-f2) <= 0.1
}