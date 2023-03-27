// the utils package contains general-purpose functions for the RTCM software.
package utils

import (
	"math"
	"time"
)

// dateLayout defines the layout of dates when they are displayed.  It
// produces "yyyy-mm-dd hh:mm:ss.ms timeshift timezone".
//
const DateLayout = "2006-01-02 15:04:05.999 -0700 MST"

// The message type is 12 bits unsigned.
const MaxMessageType = 4095

// NonRTCMMessage indicates a Message that does contain RTCM data.  Typically
// the incoming data stream will contains RTCM3 messages interspersed with data
// in other formats (NMEA, UBX etc).  Any non-RTCM messages in between two
// RTCM3 messages will be presented as a single non-RTCM message.
//
const NonRTCMMessage = -1

// Message types.
const MessageType1005 = 1005 // Base position.
const MessageTypeGCPB = 1230 // Glonass code/phase bias.
const MessageTypeMSM4GPS = 1074
const MessageTypeMSM7GPS = 1077
const MessageTypeMSM4Glonass = 1084
const MessageTypeMSM7Glonass = 1087
const MessageTypeMSM4Galileo = 1094
const MessageTypeMSM7Galileo = 1097
const MessageTypeMSM4SBAS = 1104
const MessageTypeMSM7SBAS = 1107
const MessageTypeMSM4QZSS = 1114
const MessageTypeMSM7QZSS = 1117
const MessageTypeMSM4Beidou = 1124
const MessageTypeMSM7Beidou = 1127
const MessageTypeMSM4NavicIrnss = 1134
const MessageTypeMSM7NavicIrnss = 1137

// These are used to idntify MSM messages - values are filled in by init.
var MSM4MessageTypes map[int]interface{}
var MSM7MessageTypes map[int]interface{}

// invalidRange is the invalid value for the whole millis range in an MSM4
// or MSM7 satellite cell.
const InvalidRange = 0xff

// InvalidRangeDelta is the invalid value for the range delta in an MSM4
// signal cell. 15 bit two's complement 100 0000 0000 0000
const InvalidRangeDelta = -16384

// InvalidPhaseRangeDelta is the invalid value for the phase range delta
// in an MSM4 signal cell.  22 bit two's complement signed:
// 10 0000 0000 0000 0000 0000
const InvalidPhaseRangeDelta = -2097152

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

// FreqL1Glonass is the GLONASS G1 base frequency (Hz).
const FreqL1Glonass float64 = 1.60200e9

// BiasFreq1Glo is the GLONASS G1 bias frequency (Hz/n).
const BiasFreq1Glo float64 = 0.56250e6

// fReq2Glo is the GLONASS G2 base frequency (Hz).
const FreqL2Glonass float64 = 1.24600e9

// BiasFreq2Glo is the GLONASS G2 bias frequency (Hz/n).
const BiasFreq2Glo float64 = 0.43750e6

// Freq3Glo is the GLONASS G3 frequency (Hz).
const Freq3Glo float64 = 1.202025e9

// FreqB1Beidou is the BeiDou B1 frequency (Hz).
const FreqB1Beidou float64 = 1.561098e9

// FreqB2Beidou is the BeiDou B2 frequency (Hz).
const FreqB2Beidou float64 = 1.17645e9

// FreqB3Beidou is the BeiDou B3 frequency (Hz).
const FreqB3Beidou float64 = 1.26852e9

// LeaderLengthBytes is the length of the message frame leader in bytes.
const LeaderLengthBytes = 3

// LeaderLengthBits is the length of the message frame leader in bits.
const LeaderLengthBits = LeaderLengthBytes * 8

// CRCLengthBytes is the length of the Cyclic Redundancy check value in bytes.
const CRCLengthBytes = 3

// CRCLengthBits is the length of the Cyclic Redundancy check value in bits.
const CRCLengthBits = CRCLengthBytes * 8

// Locations (timezones) - set up by the init function.
var LocationUTC *time.Location
var LocationGMT *time.Location
var LocationLondon *time.Location
var LocationMoscow *time.Location
var LocationParis *time.Location

func init() {
	LocationUTC, _ = time.LoadLocation("UTC")
	LocationGMT, _ = time.LoadLocation("GMT")
	LocationLondon, _ = time.LoadLocation("Europe/London")
	LocationMoscow, _ = time.LoadLocation("Europe/Moscow")
	LocationParis, _ = time.LoadLocation("Europe/Paris")

	MSM4MessageTypes = make(map[int]interface{})
	MSM4MessageTypes[MessageTypeMSM4GPS] = nil
	MSM4MessageTypes[MessageTypeMSM4Glonass] = nil
	MSM4MessageTypes[MessageTypeMSM4Galileo] = nil
	MSM4MessageTypes[MessageTypeMSM4SBAS] = nil
	MSM4MessageTypes[MessageTypeMSM4QZSS] = nil
	MSM4MessageTypes[MessageTypeMSM4Beidou] = nil
	MSM4MessageTypes[MessageTypeMSM4NavicIrnss] = nil

	MSM7MessageTypes = make(map[int]interface{})
	MSM7MessageTypes[MessageTypeMSM7GPS] = nil
	MSM7MessageTypes[MessageTypeMSM7Glonass] = nil
	MSM7MessageTypes[MessageTypeMSM7Galileo] = nil
	MSM7MessageTypes[MessageTypeMSM7SBAS] = nil
	MSM7MessageTypes[MessageTypeMSM7GPS] = nil
	MSM7MessageTypes[MessageTypeMSM7QZSS] = nil
	MSM7MessageTypes[MessageTypeMSM7Beidou] = nil
	MSM7MessageTypes[MessageTypeMSM7NavicIrnss] = nil
}

// GetPhaseRangeLightMilliseconds gets the phase range of the signal in
// light milliseconds.
func GetPhaseRangeLightMilliseconds(rangeMilliseconds float64) float64 {
	return rangeMilliseconds * OneLightMillisecond
}

func MSM4(messageType int) bool {
	_, prs := MSM4MessageTypes[messageType]
	return prs
}

func MSM7(messageType int) bool {
	_, prs := MSM7MessageTypes[messageType]
	return prs
}

func MSM(messageType int) bool {
	return MSM4(messageType) || MSM7(messageType)
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
	//     |--- Approx Range ----|
	//     | whole  | fractional |
	//                           |------- delta --------|
	//     w wwww wwwf ffff ffff f000 0000 0000 0000 0000
	//     + pos or neg delta    sddd dddd dddd dddd dddd <- 20-bit signed delta
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
	// This is similar to getScaledRange, but the amounts shifted are different. The
	// phase range is made up from the range values (8 bits whole milliseconds,
	// 10 bit fractional milliseconds) plus a 24-bit signed delta which is in units
	// of (2 to the power -31) so the delta value can be
	// plus or minus (2 to the power -8) to (2 to the power -31).  The result is a
	// 39-bit scaled integer, 8 bits whole, 31 bits fractional.
	//
	// The fractional range and the delta overlap by two bits, ignoring the sign,
	// so if both are at their maximum values, adding them together would cause the
	// fractional part of the result to overflow.  However, the maximum whole
	// millisecond value is only 254, as 255 indicates that the reading is invalid.
	// So, the result will always fit into 39 bits.
	//
	//     |--- Approx Range ----|
	//     | whole  | fractional |
	//                       |---------- delta -----------|
	//     876 5432 1098 7654 3210 9876 5432 1098 7654 3210
	//     www wwww wfff ffff fff0 0000 0000 0000 0000 0000
	//     + or -             sddd dddd dddd dddd dddd dddd <- phase range rate delta.

	result := getScaledValue(wholeMillis, 31, fractionalMillis, 21, delta)
	return result
}

// GetScaledPhaseRangeRate takes the component values of the phase range rate
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

// GetApproxRangeMilliseconds takes an 8-bit whole value and a 10-bit fractional value (in 1/1024 units)
// merges them and produces the approximate range in milliseconds.
func GetApproxRangeMilliseconds(wholeMillis, fractionalMillis uint) float64 {
	const twoToPowerTen = 0x400 // 100 0000 0000
	scaledRange := wholeMillis<<10 | fractionalMillis
	return float64(scaledRange) / twoToPowerTen
}

// GetApproxRangeMetres takes an 8-bit whole value and a 10-bit fractional value (in 1/1024 units)
func GetApproxRangeMetres(wholeMillis, fractionalMillis uint) float64 {
	rangeMillis := GetApproxRangeMilliseconds(wholeMillis, fractionalMillis)
	return rangeMillis * OneLightMillisecond
}

// GetSignalWavelength returns the carrier wavelength for a signal ID.
// The result depends upon the constellation, each of which has its
// own list of signals and equivalent wavelengths.  Some of the possible
// signal IDs are not used and so have no associated wavelength, so the
// result may be an error.
//
func GetSignalWavelength(constellation string, signalID uint) float64 {

	switch constellation {
	case "GPS":
		return getSignalWavelengthGPS(signalID)
	case "Galileo":
		return getSignalWavelengthGalileo(signalID)
	case "GLONASS":
		return getSignalWavelengthGlonass(signalID)
	case "Beidou":
		return getSignalWavelengthBeidou(signalID)
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

// GetConstellation returns the constellation given a message type.
func GetConstellation(messageType int) string {

	var constellation string

	switch messageType {
	case MessageTypeMSM4GPS:
		constellation = "GPS"
	case MessageTypeMSM4Glonass:
		constellation = "Glonass"
	case MessageTypeMSM4Galileo:
		constellation = "Galileo"
	case MessageTypeMSM4SBAS:
		constellation = "SBAS"
	case MessageTypeMSM4QZSS:
		constellation = "QZSS"
	case MessageTypeMSM4Beidou:
		constellation = "Beidou"
	case MessageTypeMSM4NavicIrnss:
		constellation = "NavIC/IRNSS"
	case MessageTypeMSM7GPS:
		constellation = "GPS"
	case MessageTypeMSM7Glonass:
		constellation = "Glonass"
	case MessageTypeMSM7Galileo:
		constellation = "Galileo"
	case MessageTypeMSM7SBAS:
		constellation = "SBAS"
	case MessageTypeMSM7QZSS:
		constellation = "QZSS"
	case MessageTypeMSM7Beidou:
		constellation = "Beidou"
	case MessageTypeMSM7NavicIrnss:
		constellation = "NavIC/IRNSS"

	default:
		constellation = "unknown constellation"
	}

	return constellation
}

// getSignalFrequencyGPS returns the frequency of each GPS signal, 0 if
// the ID is out of range or not in use.
func getSignalFrequencyGPS(signalID uint) float64 {
	// Each of the 32 signals is transmitted on a defined frequency  These are
	// named L1 (1575.42 MHz) and L2 (1227.60 MHz). In the future, there will be a
	// third frequency L5 (1176.45 MHz).
	// See https://www.sciencedirect.com/topics/mathematics/l1-frequency
	// and the RTKLIB source code.

	switch signalID {
	case 2:
		return Freq1
	case 3:
		return Freq1
	case 4:
		return Freq1
	case 8:
		return Freq2
	case 9:
		return Freq2
	case 10:
		return Freq2
	case 15:
		return Freq2
	case 16:
		return Freq2
	case 17:
		return Freq2
	case 22:
		return Freq5
	case 23:
		return Freq5
	case 24:
		return Freq5
	case 30:
		return Freq1
	case 31:
		return Freq1
	case 32:
		return Freq1
	default:
		return 0 // No matching frequency.
	}
}

// getSignalWavelengthGPS returns the signal carrier wavelength for a GPS satellite
// if it's defined.
func getSignalWavelengthGPS(signalID uint) float64 {
	frequency := getSignalFrequencyGPS(signalID)
	if frequency == 0 {
		// Avoid division by zero.
		return 0
	}
	return SpeedOfLightMS / frequency
}

// getSignalFrequencyGalileo returns the frequency of each Galileo signal, 0 if
// the ID is out of range.
func getSignalFrequencyGalileo(signalID uint) float64 {
	// Each of the 32 signals is transmitted on a defined frequency  These are
	// named E1 (1575.42 MHz, same as GPS L1), E5a (1176.45, same as GPS L5),
	// E5b (1207.14 MHz), E5a+b (1191.8) and E6 (1278.75 MHz).
	// See https://www.esa.int/Applications/Navigation/Galileo/Galileo_navigation_signals_and_frequencies
	// and the RTKLIB source code.

	switch signalID {
	case 2:
		return Freq1 // L1
	case 3:
		return Freq1
	case 4:
		return Freq1
	case 5:
		return Freq1
	case 6:
		return Freq1
	case 8:
		return Freq6 // E6
	case 9:
		return Freq6
	case 10:
		return Freq6
	case 11:
		return Freq6
	case 12:
		return Freq6
	case 14:
		return Freq7
	case 15:
		return Freq7 // E5b
	case 16:
		return Freq7
	case 18:
		return Freq8
	case 19:
		return Freq8
	case 20:
		return Freq8 // E5a+b
	case 22:
		return Freq5
	case 23:
		return Freq5
	case 24:
		return Freq5
	default:
		return 0 // No matching frequency.
	}
}

// getSignalWavelengthGalileo returns the signal carrier wavelength for a Galileo satellite
// if it's defined.
func getSignalWavelengthGalileo(signalID uint) float64 {
	frequency := getSignalFrequencyGalileo(signalID)
	if frequency == 0 {
		// Avoid division by zero.
		return 0
	}
	return SpeedOfLightMS / frequency
}

// getSignalFrequencyGlonass returns the frequency of each Glonass signal, 0 if
// the ID is out of range.
func getSignalFrequencyGlonass(signalID uint) float64 {

	// See https://gssc.esa.int/navipedia/index.php/GLONASS_Signal_Plan

	switch signalID {
	case 2:
		return FreqL1Glonass
	case 3:
		return FreqL1Glonass
	case 8:
		return FreqL2Glonass
	case 9:
		return FreqL2Glonass
	default:
		return 0 // No matching frequency.
	}
}

// getSignalWavelengthGlonass gets the signal carrier wavelength for a GLONASS satellite
// if it's defined.
//
func getSignalWavelengthGlonass(signalID uint) float64 {
	frequency := getSignalFrequencyGlonass(signalID)
	if frequency == 0 {
		// Avoid division by zero.
		return 0
	}
	return SpeedOfLightMS / frequency
}

// getSignalFrequencyBeidou returns the frequency of each Beidou signal, 0 if
// the ID is out of range.
func getSignalFrequencyBeidou(signalID uint) float64 {
	// Each of the 32 signals is broadcast on a defined frequency.  These are
	// B1: 1561.098 MHz
	// B2a: 1176.45 MHz
	// B3I: 1268.52 MHz
	// See https://gssc.esa.int/navipedia/index.php/BeiDou_Signal_Plan#BeiDou_B1I_Band

	switch signalID {
	case 2:
		return FreqB1Beidou
	case 3:
		return FreqB1Beidou
	case 4:
		return FreqB1Beidou
	case 8:
		return FreqB3Beidou
	case 9:
		return FreqB3Beidou
	case 10:
		return FreqB3Beidou
	case 14:
		return FreqB2Beidou
	case 15:
		return FreqB2Beidou
	case 16:
		return FreqB2Beidou
	default:
		return 0 // No matching frequency.
	}
}

// GetSigWaveLenBD returns the signal carrier wavelength for a Beidou satellite
// if it's defined.
//
func getSignalWavelengthBeidou(signalID uint) float64 {
	frequency := getSignalFrequencyBeidou(signalID)
	if frequency == 0 {
		// Avoid division by zero.
		return 0
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

// SlicesEqual returns true if uint slices a and b are the same length and
// contain the same elements.  A nil argument is equivalent to an empty slice.
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
