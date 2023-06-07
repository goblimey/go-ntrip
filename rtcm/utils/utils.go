// the utils package contains general-purpose functions for the RTCM software.
package utils

import (
	"errors"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/goblimey/go-tools/dailylogger"
)

// StartOfMessageFrame is the value of the byte that starts an RTCM3 message frame.
const StartOfMessageFrame byte = 0xd3

// The message type is 12 bits unsigned.
const MaxMessageType = 4095

// NonRTCMMessage indicates a Message that does contain RTCM data.  Typically
// the incoming data stream will contains RTCM3 messages interspersed with data
// in other formats (NMEA, UBX etc).  Any non-RTCM messages in between two
// RTCM3 messages will be presented as a single non-RTCM message.
const NonRTCMMessage = -1

// MessageTypeStop indicates that a message handler should stop.  This should
// never happen in production.  It's a special facility to allow unit testing
// of processes that would normally run indefinitely.
const MessageTypeStop = -2

// RTCM3 Message types.
const MessageType1005 = 1005 // Base position.
const MessageType1006 = 1006 // Base position and height.
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

// These are used to identify MSM messages - values are filled in by init.
var MSM4MessageTypes map[int]interface{}
var MSM7MessageTypes map[int]interface{}

// Handling of timestamps and the equivalent times.

// Multiple Signal Messages contain a thirty-bit timestamp which is the time
// since the start of some period. For Glonass the timestamp is a 3-bit day
// day and a 27-bit value, milliseconds since the start of the day in
// Moscow time.  Day 0 is Sunday and day 7 is an illegal value.  For all
// the other constellations the timestamp is milliseconds since the start of
// the week, which starts a few leap seconds away from midnight at the start
// of Sunday UTC, a different number of leap seconds for each constellation.
// For example in 2023 the GPS week starts on Saturday 18 seconds before
// midnight UTC, so the last few seconds of Saturday UTC are, in the GPS
// "timezone", the start of Sunday.

// MaxTimestamp is the maximum timestamp value.  The timestamp is 30 bits
// giving milliseconds since the start of the week BUT it must be less than
// seven days worth of milliseconds.
// const MaxTimestamp = 0x3fffffff // 0011 1111 1111 1111 1111 1111 1111 1111
const MaxTimestamp = (7 * 24 * 3600 * 1000) - 1

// MaxTimestampGlonass is the maximum timestamp value for Glonass -
// a day value of 6 and a millisecond value of
// ((24 hours worth of milliseconds) - 1)
const MaxTimestampGlonass = (6 << 27) + ((24 * 3600 * 1000) - 1)

// GPSLeapSeconds is the duration that GPS time is ahead of UTC
// in seconds, correct from the start of 2017/01/01.  An extra leap
// second may be added every four years.  The start of 2021 was a
// candidate for adding another leap second but it was not necessary.
const GPSLeapSeconds = -18

// GPSTimeOffset is the offset to convert a GPS time to UTC.
var GPSTimeOffset time.Duration = time.Duration(GPSLeapSeconds) * time.Second

// GlonassTimeOffset is the offset to convert Glonass time to UTC.
// Glonass keeps Moscow time which is 3 hours ahead of UTC.
var GlonassTimeOffset = time.Duration(-1*3) * time.Hour

// GlonassInvalidDay is the invalid value for the day part of the timestamp.
const GlonassInvalidDay = 7

// GlonassDayBitMask is used to extract the Glonass day from the timestamp
// in an MSM7 message.  The 30 bit time value is a 3 bit day (0 is Sunday)
// followed by a 27 bit value giving milliseconds since the start of the
// day.
const GlonassDayBitMask = 0x38000000 // 0011 1000 0000 0000 0000 0000 0000 0000

// beidouTimeOffset is the offset to convert a BeiDou time value to
// UTC.  UTC is based on International Atomic Time (TAI):
// https://www.timeanddate.com/time/international-atomic-time.html.
//
// This reference from November 2013 compares Beidout Time (BDT) with
// TAI: https://www.unoosa.org/pdf/icg/2016/Beidou-Timescale2016.pdf.
//
// "BDT is a uniform scale and is 33 seconds behind TAI"
//
// HOWEVER the rtklib software (decode_msm_header) says that BDT is
// 14 seconds ahead of GPS time, which is 18 seconds behind UTC,
// meaning that BDT is 4 seconds behind UTC.  Analysis of real data
// confirms that.
var BeidouLeapSeconds = -4
var BeidouTimeOffset = time.Duration(BeidouLeapSeconds) * time.Second

// DateLayout defines the layout of dates when they are displayed.  It
// produces "yyyy-mm-dd hh:mm:ss.ms timeshift timezone", for example
// "2023-05-12 00:00:05 +0000 UTC"
const DateLayout = "2006-01-02 15:04:05.999 -0700 MST"

// Locations (timezones) - set up by the init function.
var LocationUTC *time.Location
var LocationGMT *time.Location
var LocationLondon *time.Location
var LocationMoscow *time.Location
var LocationParis *time.Location

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

// MillisIn24Hours is 24 hours in milliseconds.
const MillisIn24Hours = 24 * 3600 * 1000

// MillisIn7Days7 is 7 days in milliseconds.
const MillisIn7Days = 7 * 24 * 3600 * 1000

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

// ParseTimestamp returns the number of days and the remaining
// number of milliseconds in a 30-bit timestamp.
func ParseTimestamp(constellation string, timestamp uint) (uint, uint, error) {

	const errorMessage = "timestamp out of range"
	const errorMessageMillis = "milliseconds in timestamp out of range"
	if constellation == "Glonass" {
		// A Glonass timestamp is a 3-bit number of days and a
		// 27-bit number of milliseconds from the start of the day.
		// Day zero is Sunday.  A days value of 7 is illegal.
		if timestamp > MaxTimestampGlonass {
			return 0, 0, errors.New(errorMessage)
		}
		days := timestamp >> 27
		millis := timestamp &^ GlonassDayBitMask
		if millis >= MillisIn24Hours {
			return 0, 0, errors.New(errorMessageMillis)
		}
		return days, millis, nil
	}

	// For all other constellation the timestamp is milliseconds
	// since the start of the week.
	if timestamp > MaxTimestamp {
		return 0, 0, errors.New(errorMessage)
	}
	days := timestamp / MillisIn24Hours
	millis := timestamp % MillisIn24Hours

	return days, millis, nil
}

// ParseMilliseconds breaks a millisecond timestamp down into hours, minutes etc.
func ParseMilliseconds(timestamp uint) (hours, minutes, seconds, milliseconds uint) {
	milliseconds = timestamp % 1000
	// Get the number of seconds.
	totalSeconds := timestamp / 1000
	seconds = totalSeconds % 60
	// Get the number of minutes.
	totalMinutes := totalSeconds / 60
	minutes = totalMinutes % 60
	// Get the number of hours.
	totalHours := totalMinutes / 60
	hours = totalHours % 24

	return // hours, minutes, seconds, milliseconds
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
func GetSignalWavelength(constellation string, signalID uint) float64 {

	switch constellation {
	case "GPS":
		return getSignalWavelengthGPS(signalID)
	case "Galileo":
		return getSignalWavelengthGalileo(signalID)
	case "Glonass":
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

// TitleAndComment is used to derive a title and comment from a message type.
// See GetTitleAndComment.  The data are taken mostly from
// https://www.use-snip.com/kb/knowledge-base/rtcm-3-message-list/?gclid=Cj0KCQjwpPKiBhDvARIsACn-gzC4jCabJSzgB6WgHJv3QF2a26alPfUjrSqSHMQPsUHU6sMIS_3SJP4aAoPVEALw_wcB
type TitleAndComment struct {
	// Title is the title of the message.
	Title string
	// Comment is a comment about the message type.
	Comment string
}

func GetTitleAndComment(messageType int) *TitleAndComment {

	titleComment := map[int]TitleAndComment{
		NonRTCMMessage: {
			Title:   "Non-RTCM data",
			Comment: "Data which is not in RTCM3 format, for example NMEA messages.",
		},
		1001: {"L1-Only GPS RTK Observables",
			"This GPS message type is not generally used or supported; type 1004 is to be preferred."},
		1002: {"Extended L1-Only GPS RTK Observables",
			"This GPS message type is used when only L1 data is present and bandwidth is very tight, often 1004 is used in such cases (even when no L2 data is present)."},
		1003: {"L1&L2 GPS RTK Observables",
			"This GPS message type is not generally used or supported; type 1004 is to be preferred."},
		1004: {"Extended L1&L2 GPS RTK Observables",
			"This GPS message type is the most common observational message type, with L1/L2/SNR content. This is the most common legacy message found."},
		1005: {"Stationary RTK Reference Station Antenna Reference Point (ARP)",
			"Commonly called the Station Description this message includes the ECEF location of the ARP of the antenna (not the phase center) and also the quarter phase alignment details.  The datum field is not used/defined, which often leads to confusion if a local datum is used. See message types 1006 and 1032. The 1006 message also adds a height about the ARP value."},
		1006: {"Stationary RTK Reference Station ARP with Antenna Height",
			"Commonly called the Station Description this message includes the ECEF location of the antenna (the antenna reference point (ARP) not the phase center) and also the quarter phase alignment details.  The height about the ARP value is also provided. The datum field is not used/defined, which often leads to confusion if a local datum is used. See message types 1005 and 1032. The 1005 message does not convey the height about the ARP value."},
		1007: {"Antenna Descriptor",
			"A textual description of the antenna “descriptor” which is used as a model number. Also has station ID (a number). The descriptor can be used to look up model specific details of that antenna.   See 1008 as well.  Search for ADVNULLANTENNA for additional articles on controlling this setting."},
		1008: {"Antenna Descriptor and Serial Number",
			"A textual description of the antenna “descriptor” which is used as a model number, and a (presumed unique) antenna serial number (text). Also has station ID (a number). The descriptor can be used to look up model specific details of that antenna.   See 1007 as well. Search for ADVNULLANTENNA for additional articles on controlling this setting."},
		1009: {"L1-Only GLONASS RTK Observables",
			"This GLONASS message type is not generally used or supported; type 1012 is to be preferred."},
		1010: {"Extended L1-Only GLONASS RTK Observables",
			"This GLONASS message type is used when only L1 data is present and bandwidth is very tight, often 1012 is used in such cases."},
		1011: {"L1&L2 GLONASS RTK Observables",
			"This GLONASS message type is not generally used or supported; type 1012 is to be preferred."},
		1012: {"Extended L1&L2 GLONASS RTK Observables",
			"This GLONASS message type is the most common observational message type, with L1/L2/SNR content.  This is one of the most common legacy messages found."},
		1013: {"System Parameters",
			"This message provides a table of what message types are sent at what rates.  This is the same information you find in the Caster Table (this message predates NTRIP).  SNIP infers this information by observing the data stream, and creates Caster Table entries when required.  This message is also notable in that it contains the number of leap seconds then in effect.  Not many NTRIP devices send this message."},
		1014: {"Network Auxiliary Station Data",
			"Contains a summary of the number of stations that are part of a Network RTK system, along with the relative location of the auxiliary reference stations from the master station."},
		1015: {"GPS Ionospheric Correction Differences",
			"Contains a short message with ionospheric carrier phase correction information for a single auxiliary reference station for the GPS GNSS type.  See also message 1017."},
		1016: {"GPS Geometric Correction Differences",
			"Contains a short message with geometric carrier phase correction information for a single auxiliary reference station for the GPS GNSS type.  See also message 1017."},
		1017: {"GPS Combined Geometric and Ionospheric Correction Differences",
			"Contains a short message with both ionospheric and geometric carrier phase correction information for a single auxiliary reference station for the GPS GNSS type.  See also messages 1015 and 1016."},
		1018: {"RESERVED for Alternative Ionospheric Correction Difference Message",
			"This message has not been developed or released by SC-104 at this time."},
		1019: {"GPS Ephemerides",
			"Sets of these messages (one per SV) are used to send the broadcast orbits for GPS in a Kepler format."},
		1020: {"GLONASS Ephemerides",
			"Sets of these messages (one per SV) are used to send the broadcast orbits for GLONASS in a XYZ dot product format."},
		1021: {"Helmert / Abridged Molodenski Transformation Parameters",
			"A classical Helmert 7-parameter coordinate transformation message.  Not often found in actual use."},
		1022: {"Molodenski-Badekas Transformation Parameters",
			"A coordinate transformation message using the Molodenski-Badekas method (translates through an arbitrary point rather than the origin)   Not often found in actual use."},
		1023: {"Residuals, Ellipsoidal Grid Representation",
			"A coordinate transformation message.  Not often found in actual use."},
		1024: {"Residuals, Plane Grid Representation",
			"A coordinate transformation message.  Not often found in actual use."},
		1025: {"Projection Parameters, Projection Types other than Lambert Conic Conformal",
			"A coordinate projection message.  Not often found in actual use."},
		1026: {"Projection Parameters, Projection Type LCC2SP (Lambert Conic Conformal",
			"A coordinate projection message.  Not often found in actual use."},
		1027: {"Projection Parameters, Projection Type OM (Oblique Mercator)",
			"A coordinate projection message.  Not often found in actual use."},
		1028: {"Reserved for Global to Plate-Fixed Transformation",
			"This message has not been developed or released by SC-104 at this time."},
		1029: {"Unicode Text String",
			"A message which provides a simple way to send short textual strings within the RTCM message set. About ~128 UTF-8 encoded characters are allowed."},
		1030: {"GPS Network RTK Residual Message",
			"This message provides per-SV non-dispersive interpolation residual data for the SVs used in a GPS network RTK system.  Not often found in actual use."},
		1031: {"GLONASS Network RTK Residual",
			"This message provides per-SV non-dispersive interpolation residual data for the SVs used in a GLONASS network RTK system.  Not often found in actual use."},
		1032: {"Physical Reference Station Position",
			"This message provides the ECEF location of the physical antenna used.  See message types 1005 and 1006.  Depending on the deployment needs, 1005, 1006, and 1032 are all commonly found."},
		1033: {"Receiver and Antenna Descriptors",
			"A message which provides short textual strings about the GNSS device and the Antenna device.  These strings can be used to obtain additional phase bias calibration information. This message is often sent along with either MT1007 or MT1008."},
		1034: {"GPS Network FKP Gradient",
			"A message which provides Network RTK Area Correction Parameters using a method of localized horizontal gradients for the GPS GNSS system."},
		1035: {"GLONASS Network FKP Gradient",
			"A message which provides Network RTK Area Correction Parameters using a method of localized horizontal gradients for the GLONASS GNSS system."},
		1036: {"Not defined at this time",
			"This message has not been developed or released by SC-104 at this time."},
		1037: {"GLONASS Ionospheric Correction Differences",
			"Contains a short message with ionospheric carrier phase correction information for a single auxiliary reference station for the GLONASS GNSS type.  See also message 1039."},
		1038: {"GLONASS Geometric Correction Differences",
			"Contains a short message with geometric carrier phase correction information for a single auxiliary reference station for the GLONASS GNSS type.  See also message 1039."},
		1039: {"GLONASS Combined Geometric and Ionospheric Correction Differences",
			"Contains a short message with both ionospheric and geometric carrier phase correction information for a single auxiliary reference station for the GLONASS GNSS type.  See also messages 1037 and 1037."},
		1042: {"BDS Satellite Ephemeris Data",
			"Sets of these messages (one per SV) are used to send the broadcast orbits for the BeiDou (Compass) system."},
		1043: {"Not defined at this time",
			"This message has not been developed or released by SC-104 at this time."},
		1044: {"QZSS Ephemerides",
			"Sets of these messages (one per SV) are used to send the broadcast orbits for QZSS in a Kepler format."},
		1045: {"Galileo F/NAV Satellite Ephemeris Data",
			"Sets of these messages (one per SV) are used to send the Galileo F/NAV orbital data."},
		1046: {"Galileo I/NAV Satellite Ephemeris Data",
			"Sets of these messages (one per SV) are used to send the Galileo I/NAV orbital data."},
		1057: {"SSR GPS Orbit Correction",
			"A state space representation message which provides per-SV data.  It contains orbital error / deviation from the current broadcast information for GPS GNSS types."},
		1058: {"SSR GPS Clock Correction",
			"A state space representation message which provides per-SV data.  It contains SV clock error / deviation from the current broadcast information for GPS GNSS types."},
		1059: {"SSR GPS Code Bias",
			"A state space representation message which provides per-SV data.  It contains code bias errors for GPS GNSS types."},
		1060: {"SSR GPS Combined Orbit and Clock Correction",
			"A state space representation message which provides per-SV data.  It contains both the orbital errors and the clock errors from the current broadcast information for GPS GNSS types. Note these are given as offsets from the current broadcast data."},
		1061: {"SSR GPS URA",
			"A state space representation message which provides per-SV data.  It contains User Range Accuracy (URA) for GPS GNSS types."},
		1062: {"SSR GPS High Rate Clock Correction",
			"A state space representation message which provides a higher update rate than message 1058.  It provides more precise data on the per-SV clock error / deviation from the current broadcast information for GPS GNSS types."},
		1063: {"SSR GLONASS Orbit Correction",
			"A state space representation message which provides per-SV data.  It contains orbital error / deviation from the current broadcast information for GLONASS GNSS types."},
		1064: {"SSR GLONASS Clock Correction",
			"A state space representation message which provides per-SV data.  It contains SV clock error / deviation from the current broadcast information for GLONASS GNSS types."},
		1065: {"SSR GLONASS Code Bias",
			"A state space representation message which provides per-SV data.  It contains code bias errors for GLONASS GNSS types."},
		1066: {"SSR GLONASS Combined Orbit and Clock Corrections",
			"A state space representation message which provides per-SV data.  It contains both the orbital errors and the clock errors from the current broadcast information for GLONASS GNSS types."},
		1067: {"SSR GLONASS URA",
			"A state space representation message which provides per-SV data.  It contains User Range Accuracy (URA) data for GLONASS GNSS types."},
		1068: {"SSR GLONASS High Rate Clock Correction",
			"A state space representation message which provides a higher update rate than message 1064.  It provides more precise data on the per-SV clock error / deviation from the current broadcast information for GLONASS GNSS types."},
		1070: {"Reserved for MSM",
			"This Multiple Signal Message type has not yet been assigned for use."},
		1071: {"GPS MSM1",
			"The type 1 Multiple Signal Message format for the USA’s GPS system."},
		1072: {"GPS MSM2",
			"The type 2 Multiple Signal Message format for the USA’s GPS system."},
		1073: {"GPS MSM3",
			"The type 3 Multiple Signal Message format for the USA’s GPS system."},
		1074: {"GPS Full Pseudoranges and PhaseRanges plus Carrier to Noise Ratio",
			"The type 4 Multiple Signal Message format for the American GPS system."},
		1075: {"GPS MSM5",
			"The type 5 Multiple Signal Message format for the USA’s GPS system."},
		1076: {"GPS MSM6",
			"The type 6 Multiple Signal Message format for the USA’s GPS system."},
		1077: {"GPS Full Pseudoranges and PhaseRanges plus Carrier to Noise Ratio (high resolution)",
			"The type 7 Multiple Signal Message format for the USA’s GPS system."},
		1078: {"Reserved MSM",
			"This Multiple Signal Message type has not yet been assigned for use."},
		1079: {"Reserved MSM",
			"This Multiple Signal Message type has not yet been assigned for use."},
		1080: {"Reserved MSM",
			"This Multiple Signal Message type has not yet been assigned for use."},
		1081: {"GLONASS MSM1",
			"The type 1 Multiple Signal Message format for the Russian GLONASS system."},
		1082: {"GLONASS MSM2",
			"The type 2 Multiple Signal Message format for the Russian GLONASS system."},
		1083: {"GLONASS MSM3",
			"The type 3 Multiple Signal Message format for the Russian GLONASS system."},
		1084: {"GLONASS Full Pseudoranges and PhaseRanges plus Carrier to Noise Ratio",
			"The type 4 Multiple Signal Message format for the Russian GLONASS system."},
		1085: {"GLONASS MSM5",
			"The type 5 Multiple Signal Message format for the Russian GLONASS system."},
		1086: {"GLONASS MSM6",
			"The type 6 Multiple Signal Message format for the Russian GLONASS system."},
		1087: {"GLONASS Full Pseudoranges and PhaseRanges plus Carrier to Noise Ratio (high resolution)",
			"The type 7 Multiple Signal Message format for the Russian GLONASS system."},
		1088: {"Reserved MSM",
			"This Multiple Signal Message type has not yet been assigned for use."},
		1089: {"Reserved MSM",
			"This Multiple Signal Message type has not yet been assigned for use."},
		1090: {"Reserved MSM",
			"This Multiple Signal Message type has not yet been assigned for use."},
		1091: {"Galileo MSM1",
			"The type 1 Multiple Signal Message format for Europe’s Galileo system."},
		1092: {"Galileo MSM2",
			"The type 2 Multiple Signal Message format for Europe’s Galileo system."},
		1093: {"Galileo MSM3",
			"The type 3 Multiple Signal Message format for Europe’s Galileo system."},
		1094: {"Galileo Full Pseudoranges and PhaseRanges plus Carrier to Noise Ratio",
			"The type 4 Multiple Signal Message format for Europe’s Galileo system."},
		1095: {"Galileo MSM5",
			"The type 5 Multiple Signal Message format for Europe’s Galileo system."},
		1096: {"Galileo MSM6",
			"The type 6 Multiple Signal Message format for Europe’s Galileo system."},
		1097: {"Galileo Full Pseudoranges and PhaseRanges plus Carrier to Noise Ratio (high resolution)",
			"The type 7 Multiple Signal Message format for Europe’s Galileo system."},
		1098: {"Reserved MSM",
			"This Multiple Signal Message type has not yet been assigned for use."},
		1099: {"Reserved MSM",
			"This Multiple Signal Message type has not yet been assigned for use."},
		1100: {"Reserved MSM",
			"This Multiple Signal Message type has not yet been assigned for use."},
		1101: {"SBAS MSM1",
			"The type 1 Multiple Signal Message format for SBAS/WAAS systems."},
		1102: {"SBAS MSM2",
			"The type 2 Multiple Signal Message format for SBAS/WAAS systems."},
		1103: {"SBAS MSM3",
			"The type 3 Multiple Signal Message format for SBAS/WAAS systems."},
		1104: {"SBAS Full Pseudoranges and PhaseRanges plus Carrier to Noise Ratio",
			"The type 4 Multiple Signal Message format for SBAS/WAAS systems."},
		1105: {"SBAS MSM5",
			"The type 5 Multiple Signal Message format for SBAS/WAAS systems."},
		1106: {"SBAS MSM6",
			"The type 6 Multiple Signal Message format for SBAS/WAAS systems."},
		1107: {"SBAS Full Pseudoranges and PhaseRanges plus Carrier to Noise Ratio (high resolution)",
			"The type 7 Multiple Signal Message format for SBAS/WAAS systems."},
		1108: {"Reserved MSM",
			"This Multiple Signal Message type has not yet been assigned for use."},
		1109: {"Reserved MSM",
			"This Multiple Signal Message type has not yet been assigned for use."},
		1110: {"Reserved MSM",
			"This Multiple Signal Message type has not yet been assigned for use."},
		1111: {"QZSS MSM1",
			"The type 1 Multiple Signal Message format for Japan’s QZSS system."},
		1112: {"QZSS MSM2",
			"The type 2 Multiple Signal Message format for Japan’s QZSS system."},
		1113: {"QZSS MSM3",
			"The type 3 Multiple Signal Message format for Japan’s QZSS system."},
		1114: {"QZSS Full Pseudoranges and PhaseRanges plus Carrier to Noise Ratio",
			"The type 4 Multiple Signal Message format for Japan’s QZSS system."},
		1115: {"QZSS MSM5",
			"The type 5 Multiple Signal Message format for Japan’s QZSS system."},
		1116: {"QZSS MSM6",
			"The type 6 Multiple Signal Message format for Japan’s QZSS system."},
		1117: {"QZSS Full Pseudoranges and PhaseRanges plus Carrier to Noise Ratio (high resolution)",
			"The type 7 Multiple Signal Message format for Japan’s QZSS system."},
		1118: {"Reserved MSM",
			"This Multiple Signal Message type has not yet been assigned for use."},
		1119: {"Reserved MSM",
			"This Multiple Signal Message type has not yet been assigned for use."},
		1120: {"Reserved MSM",
			"This Multiple Signal Message type has not yet been assigned for use."},
		1121: {"BeiDou MSM1",
			"The type 1 Multiple Signal Message format for China’s BeiDou system."},
		1122: {"BeiDou MSM2",
			"The type 2 Multiple Signal Message format for China’s BeiDou system."},
		1123: {"BeiDou MSM3",
			"The type 3 Multiple Signal Message format for China’s BeiDou system."},
		1124: {"BeiDou Full Pseudoranges and PhaseRanges plus Carrier to Noise Ratio",
			"The type 4 Multiple Signal Message format for China’s BeiDou system."},
		1125: {"BeiDou MSM5",
			"The type 5 Multiple Signal Message format for China’s BeiDou system."},
		1126: {"BeiDou MSM6",
			"The type 6 Multiple Signal Message format for China’s BeiDou system."},
		1127: {"BeiDou Full Pseudoranges and PhaseRanges plus Carrier to Noise Ratio (high resolution)",
			"The type 7 Multiple Signal Message format for China’s BeiDou system."},
		1128: {"Reserved MSM",
			"This Multiple Signal Message type has not yet been assigned for use."},
		1134: {"NavIC/IRNSS Full Pseudoranges and PhaseRanges plus Carrier to Noise Ratio",
			"The type 4 Multiple Signal Message format for the NavIC/IRNSS systems."},
		1137: {
			"NavIC/IRNSS Full Pseudoranges and PhaseRanges plus Carrier to Noise Ratio (high resolution)",
			"The type 7 Multiple Signal Message format for the NavIC/IRNSS systems."},
		1229: {"Reserved MSM",
			"This Multiple Signal Message type has not yet been assigned for use."},
		1230: {"GLONASS L1 and L2 Code-Phase Biases",
			"This message provides corrections for the inter-frequency bias caused by the different FDMA frequencies (k, from -7 to 6) used."},
		4095: {"Assigned to: Ashtech",
			"The content and format of this message is defined by its owner."},
		4094: {"Assigned to: Trimble Navigation Ltd.",
			"The content and format of this message is defined by its owner."},
		4093: {"Assigned to: NovAtel Inc.",
			"The content and format of this message is defined by its owner."},
		4092: {"Assigned to: Leica Geosystems",
			"The content and format of this message is defined by its owner."},
		4091: {"Assigned to: Topcon Positioning Systems",
			"The content and format of this message is defined by its owner."},
		4090: {"Assigned to: Geo++",
			"The content and format of this message is defined by its owner."},
		4089: {"Assigned to: Septentrio Satellite Navigation",
			"The content and format of this message is defined by its owner."},
		4088: {"Assigned to: IfEN GmbH",
			"The content and format of this message is defined by its owner."},
		4087: {"Assigned to:  Fugro",
			"The content and format of this message is defined by its owner."},
		4086: {"Assigned to: inPosition GmbH",
			"The content and format of this message is defined by its owner."},
		4085: {"Assigned to: European GNSS Supervisory Authority",
			"The content and format of this message is defined by its owner."},
		4084: {"Assigned to: Geodetics, Inc.",
			"The content and format of this message is defined by its owner."},
		4083: {"Assigned to: German Aerospace Center, (DLR)",
			"The content and format of this message is defined by its owner."},
		4082: {"Assigned to: Cooperative Research Centre for Spatial Information",
			"The content and format of this message is defined by its owner."},
		4081: {"Assigned to: Seoul National University GNSS Lab",
			"The content and format of this message is defined by its owner."},
		4080: {"Assigned to: NavCom Technology, Inc.",
			"The content and format of this message is defined by its owner."},
		4079: {"Assigned to: SubCarrier Systems Corp. (SCSC) The makers of SNIP",
			"The content and format of this message is defined by its owner."},
		4078: {"Assigned to: ComNav Technology Ltd.",
			"The content and format of this message is defined by its owner."},
		4077: {"Assigned to: Hemisphere GNSS Inc.",
			"The content and format of this message is defined by its owner."},
		4076: {"Assigned to: International GNSS Service (IGS)",
			"The content and format of this message is defined by its owner."},
		4075: {"Assigned to: Alberding GmbH",
			"The content and format of this message is defined by its owner."},
		4074: {"Assigned to: Unicore Communications Inc.",
			"The content and format of this message is defined by its owner."},
		4073: {"Assigned to: Mitsubishi Electric Corp.",
			"The content and format of this message is defined by its owner."},
		4072: {"Assigned to: u-blox AG",
			"The content and format of this message is defined by its owner."},
		4071: {"Assigned to: Wuhan Navigation and LBS",
			"The content and format of this message is defined by its owner."},
		4070: {"Assigned to: Wuhan MengXin Technology",
			"The content and format of this message is defined by its owner."},
		4069: {"Assigned to: VERIPOS Ltd",
			"The content and format of this message is defined by its owner."},
		4068: {"Assigned to: Qianxun Location Networks Co. Ltd",
			"The content and format of this message is defined by its owner."},
		4067: {"Assigned to: China Transport telecommunications & Information Center",
			"The content and format of this message is defined by its owner."},
		4066: {"Assigned to: Lantmateriet",
			"The content and format of this message is defined by its owner."},
		4065: {"Assigned to: Allystar Technology (Shenzhen) Co. Ltd.",
			"The content and format of this message is defined by its owner."},
		4064: {"Assigned to: NTLab",
			"The content and format of this message is defined by its owner."},
		4063: {"Assigned to: CHC Navigation (CHCNAV)",
			"The content and format of this message is defined by its owner."},
		4062: {"Assigned to: SwiftNav Inc.",
			"The content and format of this message is defined by its owner."},
		4061: {"Assigned to: Geely",
			"The content and format of this message is defined by its owner."},
	}

	tc := titleComment[messageType]
	if len(tc.Title) == 0 {
		title := fmt.Sprintf("message type %d is not known", messageType)
		result := TitleAndComment{title, ""}
		return &result
	}

	return &tc
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

// GetDailyLogger gets a daily log file which can be written to as a logger
// (each line decorated with filename, date, time, etc).
func GetDailyLogger() *log.Logger {
	dailyLog := dailylogger.New("logs", "rtcmfilter.", ".log")
	logFlags := log.LstdFlags | log.Lshortfile | log.Lmicroseconds
	return log.New(dailyLog, "rtcmfilter", logFlags)
}
