package rtcm

import (
	"bufio"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"time"

	crc24q "github.com/goblimey/go-crc24q/crc24q"
)

// The rtcm package contains logic to read and decode and display RTCM3
// messages produced by GNSS devices.  See the README for this repository
// for a description of the RTCM version 3 protocol.
//
//     handler := rtcm.New(time.Now(), logger)
//
// creates an RTCM handler connected to a logger.  RTCM messages
// contain a timestamp that rolls over each week.  To make sense of the
// timestamp the handler needs a date within the week in which the data
// was collected.  If the handler is receiving live data, the current
// date and time can be used, as in the example.
//
// If the logger is non-nil the handler writes material such as error
// messages to it.
//
// Given a reader r yielding data, the handler returns the data as a
// series of rtcm.Message objects containing the raw data of the message
// and other values such as a flag to say if the data is a valid RTCM
// message and its message type.  RTCM message types are all greater than
// zero.  There is also a special type to indicate non-RTCM data such as
// NMEA messages.
//
//    message, err := handler.ReadNextMessage(r)
//
// The raw data in the returned message object is binary and tightly
// encoded.  The handler can decode some message types and add a
// much more verbose plain text readable version to the message:
//
//    handler.DisplayMessage(&message))
//
// DisplayMessage can decode RTCM message type 1005 (which gives the base
// station position) plus MSM7 and MSM4 messages for GPS, Galileo, GLONASS
// and Beidou (which carry the base station's observations of signals
// from satellites).  The structure of these messages is described in the
// RTCM standard, which is not open source.  However, the structure can be
// reverse-engineered by looking at existing software such as the RTKLIB
// library, which is written in the C programming language.
//
// I tested the software using UBlox equipment.  For accurate positioning
// a UBlox rover requires message type 1005 and the MSM messages.  It also
// requires type 1230 (GLONASS code/phase biases) and type 4072, which is
// in a proprietary unpublished UBlox format.  I cannot currently decipher
// either of these messages.
//
// For an example of usage, see the rtcmdisplay tool in this repository.
// The filter reads a stream of message data from a base station and
// emits a readable version of the messages.  That's useful when you are
// setting up a base station and need to know exactly what it's
// producing.
//
// It's worth saying that MSM4 messages are simply lower resolution
// versions of the equivalent MSM7 messages, so a base station only
// needs to issue MSM4 or MSM7 messages, not both.  I have two base
// stations, a Sparkfun RTK Express (based on the Ublox ZED-F9P chip)
// and an Emlid Reach RS2.  The Sparkfun device can be configured to
// produce MSM4 or MSM7 messages but the Reach only produces MSM4.  Both
// claim to support 2cm accuracy.  My guess is that MSM7 format is
// defined ready to support emerging equipment that's expected to give
// better accuracy in the future.

const leaderLengthBytes = 3
const leaderLengthInBits = leaderLengthBytes * 8

// HeaderLengthBits is the length of the RTCM header in bits.
const HeaderLengthBits = leaderLengthBytes * 8

// crcLengthBytes is the length of the Cyclic Redundancy check value in bytes.
const crcLengthBytes = 3

// StartOfMessageFrame is the value of the byte that starts an RTCM3 message frame.
const StartOfMessageFrame byte = 0xd3

// lenSatelliteMask is the length in bits of the satellite mask in an MSM header.
const lenSatelliteMask = 64

// lenSatelliteMask is the length in bits of the signal mask in an MSM header.
const lenSignalMask = 32

// NonRTCMMessage indicates a Message that does contain RTCM data.  Typically
// it will be a stream of data in other formats (NMEA, UBX etc).
// According to the spec, message numbers start at 1, but I've
// seen messages of type 0.
const NonRTCMMessage = -1

// invalidRange is the invalid value for the whole millis range in an MSM
// satellite cell.
const invalidRange = 0xff

// invalidRangeDeltaMSM4 is the invalid value for the delta in an MSM4
// signal cell. 15 bit two's complement 100 0000 0000 0000
const invalidRangeDeltaMSM4 = -16384

// invalidRangeDeltaMSM7 is the invalid value for the range delta in an MSM7
// signal cell. 20 bit two's complement 1000 0000 0000 0000 0000
const invalidRangeDeltaMSM7 = -524288

// invalidPhaseRangeDeltaMSM4 is the invalid value for the phase range delta
// in an MSM4 signal cell.  22 bit two's complement: 10 0000 0000 0000 0000 0000
const invalidPhaseRangeDeltaMSM4 = -2097152

// invalidPhaseRangeDeltaMSM7 is the invalid value for the phase range delta
// in an MSM4 signal cell.  24 bit two's complement: 1000 0000 0000 0000 0000 0000
const invalidPhaseRangeDeltaMSM7 = -8388608

// invalidPhaseRangeRate is the invalid value for the phase range rate in an
// MSM7 Satellite cell.  14 bit two's complement 10 0000 0000 0000
const invalidPhaseRangeRate = -8192

// invalidPhaseRangeRateDelta is the invalid value for the delta in an MSM7
// signal cell. 15 bit two's complement 100 0000 0000 0000
const invalidPhaseRangeRateDelta = -16384

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

// freq1 is the L1/E1 signal frequency in Hz.
const freq1 float64 = 1.57542e9

// freq2 is the L2 frequency in Hz.
const freq2 float64 = 1.22760e9

// freq5 is the L5/E5a frequency in Hz.
const freq5 float64 = 1.17645e9

// freq6 is the E6/LEX frequency (Hz).
const freq6 float64 = 1.27875e9

// freq7 is the E5b requency (Hz).
const freq7 float64 = 1.20714e9

// freq8 is the E5a+b  frequency (Hz).
const freq8 float64 = 1.191795e9

// freq1Glo is the GLONASS G1 base frequency (Hz).
const freq1Glo float64 = 1.60200e9

// dFreq1Glo is the GLONASS G1 bias frequency (Hz/n).
const dFreq1Glo float64 = 0.56250e6

// fReq2Glo is the GLONASS G2 base frequency (Hz).
const freq2Glo float64 = 1.24600e9

// dFreq2Glo is the GLONASS G2 bias frequency (Hz/n).
const dFreq2Glo float64 = 0.43750e6

// freq3Glo is the GLONASS G3 frequency (Hz).
const freq3Glo float64 = 1.202025e9

// freq1BD is the BeiDou B1 frequency (Hz).
const freq1BD float64 = 1.561098e9

// freq2BD is the BeiDou B2 frequency (Hz).
const freq2BD float64 = 1.20714e9

// freq3BD is the BeiDou B3 frequency (Hz).
const freq3BD float64 = 1.26852e9

// P2_5 is 2^-5.
const P2_5 = 0.03125

// P2_6 is 2^-6.
const P2_6 = 0.015625

// P2_7 is 2^-7.
const P2_7 = P2_6 / 2.0

// P2_8 is 2^-8.
const P2_8 = P2_6 / 4.0

// P2_9 is 2^-9.
const P2_9 = P2_6 / 8.0

// P2_10 is 2^-10.
const P2_10 = P2_6 / 16.0

// P2_11 is 2^-11.
const P2_11 = P2_6 / 32.0

// P2_24 is 2^-24
const P2_24 = 5.960464477539063e-08

// P2_29 is 2^-29.
const P2_29 = 1.862645149230957e-09

// P2_31 is 2^-31.
const P2_31 = 4.656612873077393e-10

// P2_33 is 2^-33.
const P2_33 = P2_31 / 4.0

// P2_34 is 2^-34.
const P2_34 = P2_31 / 8.0

// Clight is the speed of light in metres per second.
const cLight = 299792458.0

// oneLightMillisecond is the distance in metres traveled by light in one
// millisecond.  The value can be used to convert a range in milliseconds to a
// distance in metres.  The speed of light is 299792458.0 metres/second.
const oneLightMillisecond float64 = 299792.458

// dateLayout defines the layout of dates when they are displayed.  It
// produces "yyyy-mm-dd hh:mm:ss.ms timeshift timezone".
const dateLayout = "2006-01-02 15:04:05.000 -0700 MST"

// defaultWaitTimeOnEOF is the default value for RTCM.WaitTimeOnEOF.
const defaultWaitTimeOnEOF = 100 * time.Microsecond

// glonassDayBitMask is used to extract the Glonass day from the timestamp
// in an MSM7 message.  The 30 bit time value is a 3 bit day (0 is Sunday)
// followed by a 27 bit value giving milliseconds since the start of the
// day.
const glonassDayBitMask = 0x38000000 // 0011 1000 0000 0000

// gpsLeapSeconds is the duration that GPS time is ahead of UTC
// in seconds, correct from the start of 2017/01/01.  An extra leap
// second may be added every four years.  The start of 2021 was a
// candidate for adding another leap second but it was not necessary.
const gpsLeapSeconds = 18

// gpsTimeOffset is the offset to convert a GPS time to UTC.
var gpsTimeOffset time.Duration = time.Duration(-1*gpsLeapSeconds) * time.Second

// glonassTimeOffset is the offset to convert Glonass time to UTC.
// Glonass is currently 3 hours ahead of UTC.
var glonassTimeOffset = time.Duration(-1*3) * time.Hour

// beidouTimeOffset is the offset to convert a BeiDou time value to
// UTC.  Currently (Jan 2020) Beidou is 14 seconds behind UTC.
var beidouLeapSeconds = 14
var beidouTimeOffset = time.Duration(beidouLeapSeconds) * time.Second

var locationUTC *time.Location
var locationGMT *time.Location
var locationMoscow *time.Location

// RTCM is the object used to fetch and analyse RTCM3 messages.
type RTCM struct {

	// StopOnEOF indicates that the RTCM should stop reading data and
	// terminate if it encounters End Of File.  If the data stream is
	// a plain file which is not being written, this flag should be
	// set.  If the data stream is a serial USB connection, EOF just
	// means that you've read all the data that's arrived so far, so
	// the flag should not be set and the RTCM should continue reading.

	StopOnEOF bool

	// WaitTimeOnEOF is the time to wait for if we encounter EOF and
	// StopOnEOF is false.
	WaitTimeOnEOF time.Duration

	// logger is the logger (supplied via New).
	logger *log.Logger

	// displayWriter is used to write a verbose display.  Normally nil,
	// set by SetDisplayWriter.  Note that setting this will produce
	// *a lot* of data, so don't leave it set for too long.
	displayWriter io.Writer

	// These dates are used to interpret the timestamps in RTCM3
	// messages.

	// startOfThisGPSWeek is the time in UTC of the start of
	// this GPS week.
	startOfThisGPSWeek time.Time

	// startOfNextGPSWeek is the time in UTC of the start of
	// the next GPS week.  A time which is a few seconds
	// before the end of Saturday in UTC is in the next week.
	startOfNextGPSWeek time.Time

	// startOfThisGlonassWeek is the time in UTC of the start of
	// this Glonass week.
	startOfThisGlonassDay time.Time

	// startOfNextGlonassWeek is the time in UTC of the start of
	// the next Glonass week.  A time which is a few hours
	// before the end of Saturday in UTC is in the next week.
	startOfNextGlonassDay time.Time

	// startOfThisGPSWeek is the time in UTC of the start of
	// this GPS week.
	startOfThisBeidouWeek time.Time

	// startOfNextBeidouWeek is the time in UTC of the start of
	// the next Beidou week.  A time which is a few seconds
	// before the end of Saturday in UTC is in the next week.
	startOfNextBeidouWeek time.Time

	// previousGPSTimestamp is the timestamp of the previous GPS message.
	previousGPSTimestamp uint

	// previousBeidouTimestamp is the timestamp of the previous Beidou message.
	previousBeidouTimestamp uint

	// previousGlonassDay is the day number from the previous Glonass message.
	previousGlonassDay uint
}

// Message1005 contains a message of type 1005 - antenna position.
type Message1005 struct {
	// Some bits in the message are ignored by the RTKLIB decoder so
	// we're not sure what they are.  We just store them for display.

	// MessageType - uint12 - always 1005.
	MessageType uint `json:"message_type,omitempty"`

	// station ID - uint12.
	StationID uint `json:"station_id,omitempty"`

	// Reserved for ITRF Realisaton Year - uint6.
	ITRFRealisationYear uint `json:"itrf_realisation_year,omitempty"`

	// Ignored 1 represents the next four bits which are ignored.
	Ignored1 uint `json:"ignored1,omitempty"`

	// AntennaRefX is the antenna Reference Point coordinate X in ECEF - int38.
	// Scaled integer in 0.00001 m units (tenth mm).
	AntennaRefX int64 `json:"antenna_ref_x,omitempty"`

	// Ignored2 represents the next two bits which are ignored.
	Ignored2 uint `json:"ignored2,omitempty"`

	// AntennaRefY is the antenna Reference Point coordinate Y in ECEF - int38.
	AntennaRefY int64 `json:"antenna_ref_y,omitempty"`

	// Ignored3 represents the next two bits which are ignored.
	Ignored3 uint `json:"ignored3,omitempty"`

	// AntennaRefZ is the antenna Reference Point coordinate X in ECEF - int38.
	AntennaRefZ int64 `json:"antenna_ref_z,omitempty"`
}

// MSMHeader holds the header for MSM Messages.  Message types 1074,
// 1077, 1084, 1087 etc have an MSM header at the start.
type MSMHeader struct {

	// The C code in rtklib (rtcm3.c) gives the field names:
	//
	// typedef struct {                    /* multi-signal-message header type */
	// 	   unsigned char iod;              /* issue of data station */
	// 	   unsigned char time_s;           /* cumulative session transmitting time */
	// 	   unsigned char clk_str;          /* clock steering indicator */
	// 	   unsigned char clk_ext;          /* external clock indicator */
	// 	   unsigned char smooth;           /* divergence free smoothing indicator */
	// 	   unsigned char tint_s;           /* soothing interval */
	// 	   unsigned char nsat,nsig;        /* number of satellites/signals */
	// 	   unsigned char sats[64];         /* satellites */
	// 	   unsigned char sigs[32];         /* signals */
	// 	   unsigned char cellmask[64];     /* cell mask */
	// } msm_h_t;

	// MessageType - uint12 - one of 1074, 1077 etc.
	MessageType int

	// Constellation - one of "GPS, "BeiDou" etc.
	Constellation string

	// StationID - uint12.
	StationID uint

	// EpochTime - uint30.
	// The structure of the 30 bits varies with the constellation.  For GPS
	// and Galileo it's the number of milliseconds from the start of the
	// current GPS week, which starts at midnight GMT at the start of Sunday
	// (in GPS time, a few leap seconds before UTC).  For Beidou it's
	// similar but with a different number of leap seconds.  For GLONASS the
	// top three bits are the day of the week (0 is Sunday) and the rest are
	// milliseconds from the start of the day in the Moscow time zone.
	//
	EpochTime uint

	// The Epoch time translated to a time in the UTC timezone.
	UTCTime time.Time

	// MultipleMessage - bit(1) - true if more MSMs follow for this station
	// with the same epoch time.
	MultipleMessage bool

	// SequenceNumber - uint3. (Presumably for position of the message in a
	// multiple message sequence.)
	SequenceNumber uint

	// SessionTransmissionTime - uint7.
	SessionTransmissionTime uint

	// ClockSteeringIndicator - uint2.
	ClockSteeringIndicator uint

	// ExternalClockIndicator - uint2.
	ExternalClockIndicator uint

	// GNSSDivergenceFreeSmoothingIndicator - bit(1).
	GNSSDivergenceFreeSmoothingIndicator bool

	// GNSSSmoothingInterval - uint3.
	GNSSSmoothingInterval uint

	// SatelliteMaskBits is 64 bits, one bit per satellite.  Bit 63
	// is set if signals were observed from satellite 1, bit 62 for
	// satellite 2 and so on.  For example 101000..... means that
	// signals from satellites 1 and 3 were observed.
	SatelliteMaskBits uint64

	// SignalMaskBits is 32 bits, one per signal.  Bit 31 is set if
	// signal 1 was observed from any satellite, bit 30 is set if
	// signal 2 was observed from any satellite, and so on.  For
	// example if signal 1, 3 and 5 were observed from one satellite
	// and signals 1 and 2 from another, then bits 1, 2, 3 and 5
	// will be set in the signal mask.
	SignalMaskBits uint32

	// CellMaskBits is an array of bits nSat X nSig where nSat is the
	// number of observed satellites (the number of bits set in the
	// satellite mask) and nSig is the number of signals types observed
	// across all of the satellites (the number of bits set in the
	// signal mask).  It's guaranteed to be <= 64.
	//
	// For example, if signals 1, 3 and 5 were observed from satellite 1 and
	// signals 1 and 2 were observed from satellite 3, the satellite mask will
	// have bits 63 and 61 set.  The signal mask will have bits 31, 30, 29 and
	// 27 set. The cell mask will be a table 2 X 4 bits long.  In this example,
	// each element indicates whether signals 1, 2, 3 and/or 5 were observed for
	// that satellite so the array would be 1011, 1100.  (In practice all
	// constellations currently (in 2021) use only 2 signals).  Decoding the
	// cell mask requires counting through the bits in the other two masks.
	CellMaskBits uint64

	// Satellites is a slice made from the Satellite Mask bits.  If the satellite
	// mask has bits 63 and 61 set, signals were observed from satellites 1 and 3,
	// the slice will have two elements and will contain {1, 3}.
	Satellites []uint

	// Signals is a slice made from the signal mask.  If signals 1, 2, 3 and 5 were
	// observed from the satellites, the slice will have four elements and will
	// contain {1, 2, 3, 5}
	Signals []uint

	// CellMask is a slice of slices representing the cell mask.  For example, if
	// signals 1,2,3 and 5 were observed from satellites 1 and 3, the CellMask might
	// be arranged like so: {{t,f,t,t}, {t,t,f,f}}, meaning that signals 1, 3 and 5
	// were observed from satellite 1, no signals were observed from Satellite 2 and
	// signals 1 and 2 were observed from satellite 3.  (At present (2022) the
	// satellites are dual band, so actually they will send two signals and the
	// receiver might observe zero, one or both of them.)
	CellMask [][]bool

	// NumSignalCells is the total number of signal cells in the message.  Creating
	// this count avoids having to rummage through the masks when you need to know
	// its value.
	NumSignalCells int
}

// Display return a text version of the MSMHeader.
func (header *MSMHeader) Display(rtcm *RTCM) string {

	const titleMSM4 = "Full Pseudoranges and PhaseRanges plus CNR"
	const titleMSM7 = "Full Pseudoranges and PhaseRanges plus CNR (high resolution)"

	var title string

	switch header.MessageType {
	case 1074:
		title = titleMSM4
		header.Constellation = "GPS"
		header.UTCTime = rtcm.GetUTCFromGPSTime(header.EpochTime)
	case 1077:
		title = titleMSM7
		header.Constellation = "GPS"
		header.UTCTime = rtcm.GetUTCFromGPSTime(header.EpochTime)
	case 1084:
		title = titleMSM4
		header.Constellation = "GLONASS"
		header.UTCTime = rtcm.GetUTCFromGlonassTime(header.EpochTime)
	case 1087:
		title = titleMSM7
		header.Constellation = "GLONASS"
		header.UTCTime = rtcm.GetUTCFromGlonassTime(header.EpochTime)
	case 1094:
		title = titleMSM4
		header.Constellation = "Galileo"
		header.UTCTime = rtcm.GetUTCFromGalileoTime(header.EpochTime)
	case 1097:
		title = titleMSM7
		header.Constellation = "Galileo"
		header.UTCTime = rtcm.GetUTCFromGalileoTime(header.EpochTime)
	case 1124:
		title = titleMSM4
		header.Constellation = "BeiDou"
		header.UTCTime = rtcm.GetUTCFromBeidouTime(header.EpochTime)
	case 1127:
		title = titleMSM7
		header.Constellation = "BeiDou"
		header.UTCTime = rtcm.GetUTCFromBeidouTime(header.EpochTime)
	default:
		header.Constellation = "Unknown"
	}

	line := fmt.Sprintf("type %d %s %s\n",
		header.MessageType, header.Constellation, title)

	timeStr := header.UTCTime.Format(dateLayout)
	line += fmt.Sprintf("time %s (epoch time %d)\n", timeStr, header.EpochTime)
	mode := "single"
	if header.MultipleMessage {
		mode = "multiple"
	}
	line += fmt.Sprintf("stationID %d, %s message, sequence number %d, session transmit time %d\n",
		header.StationID, mode, header.SequenceNumber, header.SessionTransmissionTime)
	line += fmt.Sprintf("clock steering %d, external clock %d ",
		header.ClockSteeringIndicator, header.ExternalClockIndicator)
	line += fmt.Sprintf("divergence free smoothing %v, smoothing interval %d\n",
		header.GNSSDivergenceFreeSmoothingIndicator, header.GNSSSmoothingInterval)
	line += fmt.Sprintf("%d satellites, %d signals, %d signal cells\n",
		len(header.Satellites), len(header.Signals), header.NumSignalCells)

	return line
}

// MSMSatelliteCell holds the data for one satellite from an MSM message,
// type MSM4 (message type 1074, 1084 ...) or type MSM7 (1077, 1087 ...).
// An MSM4 cell has RangeWholeMillis and RangeFractionalMillis.  An MSM4
// cell has those plus ExtendedInfo and PhaseRangeRate.
//
type MSMSatelliteCell struct {
	// The field names, types and sizes and invalid values are shown in comments
	// in rtklib rtcm3.c - see the function decode_msm7().

	// MessageType is the type number from the message, which defines whether
	// it's an MSM4 or an MSM7.
	MessageType int

	// SatelliteID is the satellite ID, 1-64.
	SatelliteID uint

	// RangeWholeMillis - uint8 - the number of integer milliseconds
	// in the GNSS Satellite range (ie the transit time of the signals).  0xff
	// indicates an invalid value.  See also the fraction value here and the
	// delta value in the signal cell.
	RangeWholeMillis uint

	// ExtendedInfo - uint4.  Extended Satellite Information.  MSM7 only.
	ExtendedInfo uint

	// RangeFractionalMillis - unit10.  The fractional part of the range
	// in milliseconds.
	RangeFractionalMillis uint

	// PhaseRangeRate - int14.  The approximate phase range rate for all signals
	// that come later in this message.  MSM7 only.  Invalid if the top bit is
	// set and all the others are zero.  (The value is signed, so the invalid
	// value is a negative number.)  The true phase range rate for a signal is
	// derived by merging this (positive or negative) scaled value with the
	// signal's PhaseRangeRateDelta value.
	PhaseRangeRate int
}

func (cell *MSMSatelliteCell) Display() string {

	var rangeMillis string
	if cell.RangeWholeMillis == invalidRange {
		rangeMillis = "invalid"
	} else {
		rangeMillis = fmt.Sprintf("%d.%d",
			cell.RangeWholeMillis, cell.RangeFractionalMillis)
	}

	if MSM4(cell.MessageType) {
		return fmt.Sprintf("%2d {%s}\n", cell.SatelliteID, rangeMillis)
	} else {
		// An MSM7 has an extended info and a phase range rate field.
		var phaseRangeRate string
		if cell.PhaseRangeRate == invalidPhaseRangeRate {
			phaseRangeRate = "invalid"
		} else {
			phaseRangeRate = fmt.Sprintf("%d", cell.PhaseRangeRate)
		}
		return fmt.Sprintf("%2d {%s %d %s}\n",
			cell.SatelliteID, rangeMillis, cell.ExtendedInfo, phaseRangeRate)
	}
}

// MSMSignalCell holds the data from an MSM4 or MSM7 message for one signal
// from one satellite, plus values gathered from the satellite and signal data
// and merged together.
type MSMSignalCell struct {
	// Field names, sizes, invalid values etc are derived from rtklib rtcm3.c
	// (decode_msm7 function) plus other clues from the igs BNC application.

	// Header is the header of the Multiple Signal Message  cell
	Header *MSMHeader
	// Satellite is the Satellite cell for the satellite from which this signal
	// was observed.
	Satellite *MSMSatelliteCell

	// SignalID is the ID of the signal that was observed.
	SignalID uint

	// RangeDelta - in the original message, top bit 1 and all the others zero
	// indicate an invalid value.  (The value is a two's complement signed integer
	// so that's a negative number).  The original value is 15 bits including sign
	// in an MSM4 message or 20 bits including sign in an MSM7, but this software
	// scales the MSM4 value immediately, so in this object it always looks like a
	// positive or negative int set from a 20-bit signed value (ie between plus or
	// minus 2 to the power of 19.)
	//
	// The range is expressed as the transit time of the signal in milliseconds.  To
	// get that, merge this value with the approximate range value from the satellite
	// cell.  To get the range in metres, multiply the result by one light
	// millisecond, the distance light travels in a millisecond.
	RangeDelta int

	// PhaseRangeDelta - in an MSM4, int22, in an MSM7, int24.  Invalid if the top
	// bit is set and the others are all zero.  The true phase range for the signal
	// is derived by merging this with the approximate value in the satellite cell.
	PhaseRangeDelta int

	// LockTimeIndicator - uint4 in an MSM4, uint10 in an MSM7.
	LockTimeIndicator uint

	// HalfCycleAmbiguity flag - 1 bit.
	HalfCycleAmbiguity bool

	// CNR - uint6 in an MSM4, uint10 in an MSM7 - Carrier to Noise Ratio.
	CNR uint

	// PhaseRangeRateDelta - int15 - invalid if the top bit is set and the others are
	// all zero.  Only present in an MSM7.  This value is in ten thousands of a
	// millisecond. The true value of the signal's phase range rate is derived by
	// adding this (positive or negative) value to the approximate value in the
	// satellite cell.
	PhaseRangeRateDelta int

	// PhaseRangeCycles.  Derived by merging the PhaseRangeDelta with the satellite
	// range values and converting to cycles per second.
	PhaseRangeCycles float64

	// PhaseRangeRateMS.  Derived by merging the PhaseRangeRate values and converting
	// to meters per second.  (According to Geoffrey Blewitt's paper, this is the
	// velocity at which the satellite is approaching the base station, or if negative,
	// the velocity at which it's moving away from it.)
	PhaseRangeRateMS float64
}

// RangeInMetres combines the range values from an MSM satellite and
// signal cell and produces a range in metres.
func (sigCell *MSMSignalCell) RangeInMetres() (float64, error) {

	scaledRange := sigCell.GetAggregateRange()

	// scaleFactor is two to the power of 29:
	// 10 0000 0000 0000 0000 0000 0000 0000
	const scaleFactor = 0x20000000
	// Restore the scale to give the range in milliseconds.
	rangeInMillis := float64(scaledRange) / float64(scaleFactor)

	// Use the speed of light to convert that to the distance from the
	// satellite to the receiver.
	rangeInMetres := rangeInMillis * oneLightMillisecond

	return rangeInMetres, nil
}

// GetAggregateRange takes a header, satellite cell and signal cell, extracts the
// range values and returns the range as a 37-bit scaled unsigned integer, 8 bits
// whole part and 29 bits fractional part.  This is the transit time of the signal
// in milliseconds.  Use getRangeInMetres to convert it.  The values in the satellite
// and the signal cell can indicate that the measurement failed and the result is
// invalid.  If the approximate range values in the satellite cell are invalid, the
// result is 0.  If the delta in the signal cell is invalid, the result is the
// approximate range.
func (sigCell *MSMSignalCell) GetAggregateRange() uint64 {

	// The range delta invalid indicator depends on the MSM type.
	invalidRangeDelta := invalidRangeDeltaMSM7
	if sigCell.msm4() {
		invalidRangeDelta = invalidRangeDeltaMSM4
	}

	if sigCell.Satellite.RangeWholeMillis == invalidRange {
		return 0
	}

	if sigCell.RangeDelta == invalidRangeDelta {
		// The range is valid but the delta is not.
		return GetScaledRange(sigCell.Satellite.RangeWholeMillis,
			sigCell.Satellite.RangeFractionalMillis, 0)
	}

	// The delta value is valid.
	delta := sigCell.RangeDelta
	if sigCell.msm4() {
		// The range delta value in an MSM4 signal is 15 bits signed.  In an MSM7 signal
		// it's 20 bits signed, the bottom 5 bits being extra precision.  The calculation
		// assumes the MSM7 form, so for an MSM4, normalise the value to 20 bits before
		// using it.  The value may be negative, so multiply rather than shifting bits.
		delta = delta * 16
	}

	return GetScaledRange(sigCell.Satellite.RangeWholeMillis,
		sigCell.Satellite.RangeFractionalMillis, delta)
}

// getMSMPhaseRange combines the range and the phase range from an MSM4 or MSM7
// message and returns the result in cycles. It returns zero if the input
// measurements are invalid and an error if the signal is not in use.
//
func (sigCell *MSMSignalCell) PhaseRange() (float64, error) {

	header := sigCell.Header

	// In the RTKLIB, the decode_msm7 function uses the range from the
	// satellite and the phase range from the signal cell to derive the
	// carrier phase:
	//
	// /* carrier-phase (cycle) */
	// if (r[i]!=0.0&&cp[j]>-1E12&&wl>0.0) {
	//    rtcm->obs.data[index].L[ind[k]]=(r[i]+cp[j])/wl;
	// }

	if !sigCell.msm4() && !sigCell.msm7() {
		em := fmt.Sprintf("message type %d is not a MSM and does not have a phase range value",
			header.MessageType)
		return 0.0, errors.New(em)
	}

	// This is similar to getMSMRange.  The phase range is in cycles
	// and derived from the range values from the satellite cell shifted up
	// 31 bits,  plus the signed phase range delta.  In an MSM4 and MSM5 message
	// the delta in the signal cell is 22 bits and in an MSM6 and MSM7 it's 24
	// bits.  The 24 bit value gives extra resolution, so we convert a 22 bit
	// value to a 24 bit value with two trailing zeroes.
	//
	//     ------ Range -------
	//     whole     fractional
	//     www wwww wfff ffff fff0 0000 0000 0000 0000 0000
	//     + or -             dddd dddd dddd dddd dddd dddd

	wavelength, err := sigCell.GetWavelength()
	if err != nil {
		return 0.0, err
	}

	aggregatePhaseRange := sigCell.GetAggregatePhaseRange()

	// Restore the scale of the aggregate value.
	phaseRangeMilliSeconds := getPhaseRangeMilliseconds(aggregatePhaseRange)

	// Convert to light milliseconds
	phaseRangeLMS := sigCell.GetPhaseRangeLightMilliseconds(phaseRangeMilliSeconds)

	// and divide by the wavelength to get cycles.
	phaseRangeCycles := phaseRangeLMS / wavelength

	return phaseRangeCycles, nil
}

// MSM7PhaseRangeRate combines the components of the phase range rate
// in an MSM7 message and returns the result in milliseconds.  If the rate
// value in the satellite cell is invalid, the result is zero.  If the delta
// in the signal cell is invalid, the result is based on the rate value in the
// satellite.
//
func (sigCell *MSMSignalCell) MSM7PhaseRangeRate() (float64, error) {
	header := sigCell.Header

	if !sigCell.msm7() {
		em := fmt.Sprintf("message type %d is not an MSM7", header.MessageType)
		return 0.0, errors.New(em)
	}

	aggregatePhaseRangeRate := sigCell.GetAggregatePhaseRangeRate()

	// The aggregate is milliseconds scaled up by 10,000.
	phaseRangeRateMillis := float64(aggregatePhaseRangeRate) / 10000

	// Blewitt's paper says that the phase range rate is the rate at which the
	// which the satellite is approaching or (if negative) receding from
	// the GPS device.

	return phaseRangeRateMillis, nil
}

// GetMSM7Doppler gets the doppler value in Hz from the phase
// range rate fields of a satellite and signal cell from an MSM7.
func (sigCell *MSMSignalCell) GetMSM7Doppler() (float64, error) {
	// RTKLIB save_msm_obs calculates the phase range, multiplies it by
	// the wavelength of the signal, reverses the sign of the result and
	// calls it the Doppler:
	//
	// /* doppler (hz) */
	// if (rr&&rrf&&rrf[j]>-1E12&&wl>0.0) {
	//     rtcm->obs.data[index].D[ind[k]]=(float)(-(rr[i]+rrf[j])/wl);
	// }
	//
	// When an MSM7 is converted to RINEX format, this value appears in one of
	// the fields, so we can test the handling using data collected from a real
	// device.

	phaseRangeRateMillis, err := sigCell.MSM7PhaseRangeRate()

	if err != nil {
		return 0.0, err
	}

	wavelength, err := sigCell.GetWavelength()
	if err != nil {
		return 0.0, err
	}

	return (phaseRangeRateMillis / wavelength) * -1, nil
}

// GetPhaseRangeLightMilliseconds gets the phase range of the signal in light milliseconds.
func (sigCell *MSMSignalCell) GetPhaseRangeLightMilliseconds(rangeMetres float64) float64 {
	return rangeMetres * oneLightMillisecond
}

// GetAggregatePhaseRange takes a header, satellite cell and signal cell, extracts
// the phase range values, aggregates them and returns them as a 41-bit scaled unsigned
// unsigned integer, 8 bits whole part and 33 bits fractional part.  Use
// getPhaseRangeCycles to convert this to the phase range in cycles.
func (sigCell *MSMSignalCell) GetAggregatePhaseRange() uint64 {

	// This is similar to getAggregateRange  but for the phase range.  The phase
	// range value in the signal cell is merged with the range values in the
	// satellite cell.

	satCell := sigCell.Satellite

	if satCell.RangeWholeMillis == invalidRange {
		return 0
	}

	var delta int

	if sigCell.msm4() {
		// The message is an MSM4.
		if sigCell.PhaseRangeDelta != invalidPhaseRangeDeltaMSM4 {
			// The phase range delta value is valid.
			// The value in an MSM4 signal cell is 22 bits signed.
			// In an MSM7 signal cell it's 24 bits signed, the bottom 2 bits being
			// extra precision.  The calculation assumes the MSM7 form, so for an MSM4,
			// normalise the value to 24 bits before using it.  The value may be
			// negative, so multiply it rather than shifting bits.
			delta = sigCell.PhaseRangeDelta * 4
		}
	} else {
		// The message is an MSM7.
		if sigCell.PhaseRangeDelta != invalidPhaseRangeDeltaMSM7 {
			delta = sigCell.PhaseRangeDelta
		}

	}

	scaledPhaseRange :=
		GetScaledPhaseRange(satCell.RangeWholeMillis, satCell.RangeFractionalMillis, delta)

	return scaledPhaseRange
}

// GetAggregatePhaseRangeRate returns the phase range rate as an int, scaled up
// by 10,000
func (sigCell *MSMSignalCell) GetAggregatePhaseRangeRate() int64 {

	// This is similar to getAggregateRange  but for the phase range rate.

	satCell := sigCell.Satellite

	if satCell.PhaseRangeRate == invalidPhaseRangeRate {
		return 0
	}

	var delta int

	if sigCell.PhaseRangeDelta != invalidPhaseRangeRateDelta {
		delta = sigCell.PhaseRangeRateDelta
	}

	return GetScaledPhaseRangeRate(satCell.PhaseRangeRate, delta)
}

// getWavelength returns the carrier wavelength for a signal ID.
// The result depends upon the constellation, each of which has its
// own list of signals and equivalent wavelengths.  Some of the possible
// signal IDs are not used and so have no associated wavelength, so the
// result may be an error.
//
func (sigCell *MSMSignalCell) GetWavelength() (float64, error) {

	var wavelength float64
	switch sigCell.Header.Constellation {
	case "GPS":
		var err error
		wavelength, err = sigCell.GetSigWaveLenGPS()
		if err != nil {
			return 0.0, err
		}
	case "Galileo":
		var err error
		wavelength, err = sigCell.GetSigWaveLenGalileo()
		if err != nil {
			return 0.0, err
		}
	case "GLONASS":
		var err error
		wavelength, err = sigCell.GetSigWaveLenGlo()
		if err != nil {
			return 0.0, err
		}
	case "BeiDou":
		var err error
		wavelength, err = sigCell.GetSigWaveLenBD()
		if err != nil {
			return 0.0, err
		}
	default:
		message := fmt.Sprintf("no such constellation as %s",
			sigCell.Header.Constellation)
		return 0.0, errors.New(message)
	}

	return wavelength, nil
}

// getSigWaveLenGPS returns the signal carrier wavelength for a GPS satellite
// if it's defined.
func (sigCell *MSMSignalCell) GetSigWaveLenGPS() (float64, error) {
	// Only some signal IDs are in use.
	var frequency float64
	switch sigCell.SignalID {
	case 2:
		frequency = freq1
	case 3:
		frequency = freq1
	case 4:
		frequency = freq1
	case 8:
		frequency = freq2
	case 9:
		frequency = freq2
	case 10:
		frequency = freq2
	case 15:
		frequency = freq2
	case 16:
		frequency = freq2
	case 17:
		frequency = freq2
	case 22:
		frequency = freq5
	case 23:
		frequency = freq5
	case 24:
		frequency = freq5
	case 30:
		frequency = freq1
	case 31:
		frequency = freq1
	case 32:
		frequency = freq1
	default:
		message := fmt.Sprintf("GPS signal ID %d not in use", sigCell.SignalID)
		return 0, errors.New(message)
	}
	return cLight / frequency, nil
}

// GetSigWaveLenGalileo returns the signal carrier wavelength for a Galileo satellite
// if it's defined.
func (sigCell *MSMSignalCell) GetSigWaveLenGalileo() (float64, error) {
	// Only some signal IDs are in use.
	var frequency float64
	switch sigCell.SignalID {
	case 2:
		frequency = freq1
	case 3:
		frequency = freq1
	case 4:
		frequency = freq1
	case 5:
		frequency = freq1
	case 6:
		frequency = freq1
	case 8:
		frequency = freq6
	case 9:
		frequency = freq6
	case 10:
		frequency = freq6
	case 11:
		frequency = freq6
	case 12:
		frequency = freq6
	case 14:
		frequency = freq7
	case 15:
		frequency = freq7
	case 16:
		frequency = freq7
	case 18:
		frequency = freq8
	case 19:
		frequency = freq8
	case 20:
		frequency = freq8
	case 22:
		frequency = freq5
	case 23:
		frequency = freq5
	case 24:
		frequency = freq5
	default:
		message := fmt.Sprintf("GPS signal ID %d not in use", sigCell.SignalID)
		return 0, errors.New(message)
	}
	return cLight / frequency, nil
}

// GetSigWaveLenGlo gets the signal carrier wavelength for a GLONASS satellite
// if it's defined.
//
func (sigCell *MSMSignalCell) GetSigWaveLenGlo() (float64, error) {
	// Only some signal IDs are in use.
	var frequency float64
	switch sigCell.SignalID {
	case 2:
		frequency = freq1Glo
	case 3:
		frequency = freq1Glo
	case 8:
		frequency = freq2Glo
	case 9:
		frequency = freq2Glo
	default:
		message := fmt.Sprintf("GLONASS signal ID %d not in use", sigCell.SignalID)
		return 0, errors.New(message)
	}
	return cLight / frequency, nil
}

// GetSigWaveLenBD returns the signal carrier wavelength for a Beidou satellite
// if it's defined.
//
func (sigCell *MSMSignalCell) GetSigWaveLenBD() (float64, error) {
	// Only some signal IDs are in use.
	var frequency float64
	switch sigCell.SignalID {
	case 2:
		frequency = freq1BD
	case 3:
		frequency = freq1BD
	case 4:
		frequency = freq1BD
	case 8:
		frequency = freq3BD
	case 9:
		frequency = freq3BD
	case 10:
		frequency = freq3BD
	case 14:
		frequency = freq2BD
	case 15:
		frequency = freq2BD
	case 16:
		frequency = freq2BD
	default:
		message := fmt.Sprintf("GPS signal ID %d not in use", sigCell.SignalID)
		return 0, errors.New(message)
	}
	return cLight / frequency, nil
}

// MSM1 returns true if the message is an MSM type 1.
func (sigCell *MSMSignalCell) msm1() bool {
	switch sigCell.Header.MessageType {
	// GPS
	case 1071: // GPS
		return true
	case 1081: // Glonass
		return true
	case 1091: // Galileo
		return true
	case 1101: // SBAS
		return true
	case 1111: // QZSS
		return true
	case 1121: // Beidou
		return true
	case 1131: // NavIC/IRNSS
		return true
	default:
		return false
	}
}

// MSM2 returns true if the message is an MSM type 2.
func (sigCell *MSMSignalCell) msm2() bool {
	switch sigCell.Header.MessageType {
	// GPS
	case 1072: // GPS
		return true
	case 1082: // Glonass
		return true
	case 1092: // Galileo
		return true
	case 1102: // SBAS
		return true
	case 1112: // QZSS
		return true
	case 1122: // Beidou
		return true
	case 1132: // NavIC/IRNSS
		return true
	default:
		return false
	}
}

// MSM3 returns true if the message is an MSM type 3.
func (sigCell *MSMSignalCell) msm3() bool {
	switch sigCell.Header.MessageType {
	// GPS
	case 1073: // GPS
		return true
	case 1083: // Glonass
		return true
	case 1093: // Galileo
		return true
	case 1103: // SBAS
		return true
	case 1113: // QZSS
		return true
	case 1123: // Beidou
		return true
	case 1133: // NavIC/IRNSS
		return true
	default:
		return false
	}
}

// MSM4 returns true if the message is an MSM type 4.
func (sigCell *MSMSignalCell) msm4() bool {
	switch sigCell.Header.MessageType {
	// GPS
	case 1074: // GPS
		return true
	case 1084: // Glonass
		return true
	case 1094: // Galileo
		return true
	case 1104: // SBAS
		return true
	case 1114: // QZSS
		return true
	case 1124: // Beidou
		return true
	case 1134: // NavIC/IRNSS
		return true
	default:
		return false
	}
}

// MSM5 returns true if the message is an MSM type 5.
func (sigCell *MSMSignalCell) msm5() bool {
	switch sigCell.Header.MessageType {
	case 1075:
		return true
	case 1085:
		return true
	case 1095:
		return true
	case 1105:
		return true
	case 1115:
		return true
	case 1125:
		return true
	case 1135:
		return true
	default:
		return false
	}
}

// MSM6 returns true if the message is an MSM type 6.
func (sigCell *MSMSignalCell) msm6() bool {
	switch sigCell.Header.MessageType {
	case 1076:
		return true
	case 1086:
		return true
	case 1096:
		return true
	case 1106:
		return true
	case 1116:
		return true
	case 1126:
		return true
	case 1136:
		return true
	default:
		return false
	}
}

// MSM7 returns true if the message is an MSM type 7.
func (sigCell *MSMSignalCell) msm7() bool {
	switch sigCell.Header.MessageType {
	case 1077:
		return true
	case 1087:
		return true
	case 1097:
		return true
	case 1107:
		return true
	case 1117:
		return true
	case 1127:
		return true
	case 1137:
		return true
	default:
		return false
	}
}

// Display returns a readable version of a signal cell.
func (sigCell *MSMSignalCell) Display() string {
	var rangeM string
	r, rangeError := sigCell.RangeInMetres()
	if rangeError == nil {
		rangeM = fmt.Sprintf("%f", r)
	} else {
		rangeM = rangeError.Error()
	}

	var phaseRange string
	pr, prError := sigCell.PhaseRange()
	if prError == nil {
		phaseRange = fmt.Sprintf("%f", pr)
	} else {
		phaseRange = prError.Error()
	}

	var phaseRangeRate string
	if sigCell.msm7() {
		prr, prrError :=
			sigCell.MSM7PhaseRangeRate()
		if prrError == nil {
			phaseRangeRate = fmt.Sprintf("%f", prr)
		} else {
			phaseRangeRate = prrError.Error()
		}
	}

	satelliteID := sigCell.Satellite.SatelliteID

	if sigCell.msm7() {
		return fmt.Sprintf("%2d %2d {%s %s %d, %v, %d, %s}",
			satelliteID, sigCell.SignalID, rangeM, phaseRange,
			sigCell.LockTimeIndicator, sigCell.HalfCycleAmbiguity,
			sigCell.CNR, phaseRangeRate)
	} else {
		// An MSM4 does not have phase range rate fields
		return fmt.Sprintf("%2d %2d {%s %s %d, %v, %d}",
			satelliteID, sigCell.SignalID, rangeM, phaseRange,
			sigCell.LockTimeIndicator, sigCell.HalfCycleAmbiguity,
			sigCell.CNR)
	}
}

// MSMMessage is a broken-out version of an MSM4 or MSM7 message.
type MSMMessage struct {
	// Header is the MSM Header
	Header *MSMHeader

	// Satellites is a list of the satellites for which signals
	// were observed in an MSM7 message.
	Satellites []MSMSatelliteCell

	// Signals is a list of sublists, one sublist per satellite,
	// of signals at different frequencies observed by the base
	// station from the satellites in the Satellite list.
	Signals [][]MSMSignalCell
}

// Display return a text version of the MSMMessage.
func (message *MSMMessage) Display(rtcm *RTCM) string {
	result :=
		message.Header.Display(rtcm) +
			message.DisplaySatelliteCells() +
			message.DisplaySignalCells()

	return result
}

// DisplaySignalCells returns a text version of the signal data from an MSMMessage.
func (message *MSMMessage) DisplaySignalCells() string {

	if len(message.Signals) < 1 {
		return "No signals|n"
	}

	var heading string

	if MSM4(int(message.Header.MessageType)) {
		// the messages is an MSM4 and so doesn't have the phase range rate value.
		heading = fmt.Sprintf("%d Signals\nsat ID sig ID {range (delta), lock time ind, half cycle ambiguity,\n",
			len(message.Signals))
		heading += "        Carrier Noise Ratio}\n"
	} else {
		// the messages is an MSM7 and has the phase range rate value.
		heading = fmt.Sprintf("%d Signals\nsat ID sig ID {range (delta), phase range (delta), lock time ind, half cycle ambiguity,\n",
			len(message.Signals))
		heading += "        Carrier Noise Ratio,  phase range rate (delta)}\n"
	}

	body := ""

	for i := range message.Signals {
		for j := range message.Signals[i] {
			body += fmt.Sprintf("%s\n", message.Signals[i][j].Display())
		}
	}

	return heading + body
}

// DisplaySatelliteCells returns a text version of the satellite cells in the
// Multiple Signal Message (MSM).
func (message *MSMMessage) DisplaySatelliteCells() string {

	if len(message.Satellites) < 1 {
		return "No satellites\n"
	}

	heading := ""

	if MSM4(message.Header.MessageType) {
		heading = fmt.Sprintf("%d Satellites\nsatellite ID {range ms}\n",
			len(message.Satellites))
	} else {
		heading = fmt.Sprintf("%d Satellites\nsatellite ID {range ms, extended info, phase range rate m/s}\n",
			len(message.Satellites))
	}

	body := ""
	for i := range message.Satellites {
		body += message.Satellites[i].Display()
	}

	return heading + body
}

// Message contains an RTCM3 message, possibly broken out into readable form,
// or a stream of non-RTCM data.  Message type NonRTCMMessage indicates the
// second case.
type Message struct {
	// MessageType is the type of the RTCM message (the message number).
	// RTCM messages all have a positive message number.  Type NonRTCMMessage
	// is negative and indicates a stream of bytes that doesn't contain a
	// valid RTCM message, for example an NMEA message or a corrupt RTCM.
	MessageType int

	// Valid is true if the message is valid - complete and the CRC checks.
	Valid bool

	// Complete is true if the message is complete.  The last bytes in a
	// log of messages may not be complete.
	Complete bool

	// CRCValid is true if the Cyclic Redundancy Check bits are valid.
	CRCValid bool

	// Warning contains any error message encountered while fetching
	// the message.
	Warning string

	// RawData is the message frame in its original binary form
	//including the header and the CRC.
	RawData []byte

	// readable is a broken out version of the RTCM message.  It's accessed
	// via the Readable method and the message is only decoded on the
	// first call.  (Lazy evaluation.)
	readable interface{}
}

// Copy makes a copy of the message and its contents.
func (message *Message) Copy() Message {
	// Make a copy of the raw data.
	rawData := make([]byte, len(message.RawData))
	copy(rawData, message.RawData)
	// Create a new message.  Omit the readable part - it may not be needed
	// and if it is needed, it will be created automatically at that point.
	var newMessage = Message{
		MessageType: message.MessageType,
		RawData:     rawData,
		Valid:       message.Valid,
		Complete:    message.Complete,
		CRCValid:    message.CRCValid,
		Warning:     message.Warning,
	}
	return newMessage
}

// PrepareForDisplay returns a broken out version of the message - for example,
// if the message type is 1005, it's a Message1005.
func (m *Message) PrepareForDisplay(r *RTCM) interface{} {
	var err error
	if m.readable == nil {
		err = r.Analyse(m)
		if err != nil {
			// Message can't be analysed.  Log an error and mark the message
			// as not valid.
			log.Println(err.Error())
			m.Valid = false
		}
	}

	return m.readable
}

func (m *Message) SetReadable(r interface{}) {
	m.readable = r
}

func init() {
	locationUTC, _ = time.LoadLocation("UTC")
	locationGMT, _ = time.LoadLocation("GMT")
	locationMoscow, _ = time.LoadLocation("Europe/Moscow")
}

// New creates an RTCM object using the given year, month and day to
// identify which week the times in the messages refer to.
func New(startTime time.Time, logger *log.Logger) *RTCM {

	rtcm := RTCM{logger: logger, WaitTimeOnEOF: defaultWaitTimeOnEOF}

	// Convert the start date to UTC.
	startTime = startTime.In(locationUTC)

	// Get the start of last Sunday in UTC. (If today is Sunday, the start
	// of today.)

	startOfWeekUTC := GetStartOfLastSundayUTC(startTime)

	// GPS.  The GPS week starts gpsLeapSeconds before midnight at the
	// start of Sunday in UTC, ie on Saturday just before midnight.  So
	// most of Saturday UTC is the end of one GPS week but the last few
	// seconds are the beginning of the next.
	//
	if startTime.Weekday() == time.Saturday {
		// This is saturday, either in the old GPS week or the new one.
		// Get the time when the new GPS week starts (or started).
		sunday := startTime.AddDate(0, 0, 1)
		midnightNextSunday := GetStartOfLastSundayUTC(sunday)
		gpsWeekStart := midnightNextSunday.Add(gpsTimeOffset)
		if startTime.Equal(gpsWeekStart) || startTime.After(gpsWeekStart) {
			// It's Saturday in the first few seconds of a new GPS week
			rtcm.startOfThisGPSWeek = gpsWeekStart
		} else {
			// It's Saturday at the end of a GPS week.
			midnightLastSunday := GetStartOfLastSundayUTC(startTime)
			rtcm.startOfThisGPSWeek = midnightLastSunday.Add(gpsTimeOffset)
		}
	} else {
		// It's not Saturday.  The GPS week started just before midnight
		// at the end of last Saturday.
		midnightLastSunday := GetStartOfLastSundayUTC(startTime)
		rtcm.startOfThisGPSWeek = midnightLastSunday.Add(gpsTimeOffset)
	}

	rtcm.startOfNextGPSWeek =
		rtcm.startOfThisGPSWeek.AddDate(0, 0, 7)

	rtcm.previousGPSTimestamp = (uint(startTime.Sub(rtcm.startOfThisGPSWeek).Milliseconds()))

	// Beidou.
	// Get the start of this and the next Beidou week.  Despite
	// https://www.unoosa.org/pdf/icg/2016/Beidou-Timescale2016.pdf
	// the correct offset appears to be +14 seconds!!!

	rtcm.startOfThisBeidouWeek = startOfWeekUTC.Add(beidouTimeOffset)

	if startTime.Before(rtcm.startOfThisBeidouWeek) {
		// The given start date is in the previous Beidou week.  (This
		// happens if it's within the first few seconds of Sunday UTC.)
		rtcm.startOfThisBeidouWeek = rtcm.startOfThisBeidouWeek.AddDate(0, 0, -7)
	}

	rtcm.startOfNextBeidouWeek = rtcm.startOfThisBeidouWeek.AddDate(0, 0, 7)
	rtcm.previousBeidouTimestamp =
		(uint(startTime.Sub(rtcm.startOfThisBeidouWeek).Milliseconds()))

	// Glonass.
	// Get the Glonass day number and the start of this and the next
	// Glonass day.  The day is 0: Sunday, 1: Monday and so on, but in
	// Moscow time which is three hours ahead of UTC, so the day value
	// rolls over at 21:00 UTC the day before.

	// Unlike GPS, we have a real timezone to work with - Moscow.
	startTimeMoscow := startTime.In(locationMoscow)
	startOfDayMoscow := time.Date(startTimeMoscow.Year(), startTimeMoscow.Month(),
		startTimeMoscow.Day(), 0, 0, 0, 0, locationMoscow)

	rtcm.startOfThisGlonassDay = startOfDayMoscow.In(locationUTC)

	rtcm.startOfNextGlonassDay =
		rtcm.startOfThisGlonassDay.AddDate(0, 0, 1)

	// Set the previous Glonass day to the day in Moscow at the given
	// start time - Sunday is 0, Monday is 1 and so on.
	rtcm.previousGlonassDay = uint(startOfDayMoscow.Weekday())

	return &rtcm
}

func (r *RTCM) SetDisplayWriter(displayWriter io.Writer) {
	r.displayWriter = displayWriter

}

// GetMessage extracts an RTCM3 message from the given slice and returns it
// as a Message. If the data doesn't contain a valid message, it returns a
// message with type NonRTCMMessage.
//ae
func (r *RTCM) GetMessage(data []byte) (*Message, error) {

	if len(data) == 0 {
		return nil, errors.New("zero length message frame")
	}

	if data[0] != StartOfMessageFrame {
		// This is not an RTCM message.
		message := Message{
			MessageType: NonRTCMMessage,
			RawData:     data,
		}
		return &message, nil
	}

	messageLength, messageType, formatError := r.GetMessageLengthAndType(data)
	if formatError != nil {
		message := Message{
			MessageType: messageType,
			RawData:     data,
			Warning:     formatError.Error(),
		}
		return &message, formatError
	}

	if messageType == NonRTCMMessage {
		message := Message{MessageType: messageType, RawData: data}
		return &message, nil
	}

	// The message frame should contain a header, the variable-length message and
	// the CRC.  We now know the message length, so we can check that we have the
	// whole thing.

	frameLength := uint(len(data))
	expectedFrameLength := messageLength + leaderLengthBytes + crcLengthBytes
	// The message is analysed only when necessary (lazy evaluation).  For
	// now, just copy the byte stream into the Message.
	if expectedFrameLength > frameLength {
		// The message is incomplete, return what we have.
		// (This can happen if it's the last message in the input stream.)
		warning := "incomplete message frame"
		message := Message{
			MessageType: messageType,
			RawData:     data[:frameLength],
			Warning:     warning,
		}
		return &message, errors.New(warning)
	}

	// We have a complete message.

	message := Message{
		MessageType: messageType,
		RawData:     data[:expectedFrameLength],
		Complete:    true,
	}

	// Check the CRC.
	if !CheckCRC(data) {
		errorMessage := "CRC failed"
		message.Warning = errorMessage
		return &message, errors.New(errorMessage)
	}
	message.CRCValid = true

	// the message is valid if the CRC check passes and it (the message)
	// is complete.  Both are now true
	message.Valid = true

	return &message, nil
}

// Analyse decodes the raw byte stream and fills in the broken out message.
func (r *RTCM) Analyse(message *Message) error {
	var readable interface{}
	var err error = nil
	switch {
	case MSM(message.MessageType):
		readable, err = r.GetMSMMessage(message)
	case message.MessageType == 1005:
		readable, err = r.GetMessage1005(message.RawData)
	case message.MessageType == 1230:
		readable = "(Message type 1230 - GLONASS code-phase biases - don't know how to decode this.)"
	case message.MessageType == 4072:
		readable = "(Message type 4072 is in an unpublished format defined by U-Blox.)"
	default:
		readable = fmt.Sprintf("message type %d currently cannot be displayed",
			message.MessageType)
	}

	message.SetReadable(readable)

	return err
}

// GetMessageLengthAndType extracts the message length and the message type from an
// RTCMs message frame or returns an error, implying that this is not the start of a
// valid message.  The bit stream must be at least 5 bytes long.
func (rtcm *RTCM) GetMessageLengthAndType(bitStream []byte) (uint, int, error) {

	if len(bitStream) < leaderLengthBytes+2 {
		return 0, NonRTCMMessage, errors.New("the message is too short to get the header and the length")
	}

	// The message header is 24 bits.  The top byte is startOfMessage.
	if bitStream[0] != StartOfMessageFrame {
		message := fmt.Sprintf("message starts with 0x%0x not 0xd3", bitStream[0])
		return 0, NonRTCMMessage, errors.New(message)
	}

	// The next six bits must be zero.  If not, we've just come across
	// a 0xd3 byte in a stream of binary data.
	sanityCheck := GetBitsAsUint64(bitStream, 8, 6)
	if sanityCheck != 0 {
		errorMessage := fmt.Sprintf("bits 8 -13 of header are %d, must be 0", sanityCheck)
		return 0, NonRTCMMessage, errors.New(errorMessage)
	}

	// The bottom ten bits of the header is the message length.
	length := uint(GetBitsAsUint64(bitStream, 14, 10))

	// The 12-bit message type follows the header.
	messageType := int(GetBitsAsUint64(bitStream, 24, 12))

	// length must be > 0. (Defer this check until now, when we have the message type.)
	if length == 0 {
		errorMessage := fmt.Sprintf("zero length message type %d", messageType)
		return 0, messageType, errors.New(errorMessage)
	}

	return length, messageType, nil
}

// ReadNextFrame gets the next message frame from a reader.  The incoming
// byte stream contains RTCM messages interspersed with messages in other
// formats such as NMEA, UBX etc.   The resulting slice contains either a
// single valid message or some non-RTCM text that precedes a message.  If
// the function encounters a fatal read error and it has not yet read any
// text, it returns the error.  If it has read some text, it just returns
// that (the assumption being that the next call will get no text and the
// same error).  Use GetMessage to extract the message from the result.
func (rtcm *RTCM) ReadNextFrame(reader *bufio.Reader) ([]byte, error) {

	// A valid RTCM message frame is a header containing the start of message
	// byte and two bytes containing a 10-bit message length, zero padded to
	// the left, for example 0xd3, 0x00, 0x8a.  The variable-length message
	// comes next and always starts with a two-byte message type.  It may be
	// padded with zero bytes at the end.  The message frame then ends with a
	// 3-byte Cyclic Redundancy Check value.

	// Call ReadBytes until we get some text or a fatal error.
	var frame = make([]byte, 0)
	var eatError error
	for {
		// Eat bytes until we see the start of message byte.
		frame, eatError = reader.ReadBytes(StartOfMessageFrame)
		if eatError != nil {
			// We only deal with an error if there's nothing in the buffer.
			// If there is any text, we deal with that and assume that we will see
			// any hard error again on the next call.
			if len(frame) == 0 {
				// An error and no bytes in the frame.  Deal with the error.
				if eatError == io.EOF {
					if rtcm.StopOnEOF {
						// EOF is fatal for the kind of input file we are reading.
						logEntry := "ReadNextFrame: hard EOF while eating"
						rtcm.makeLogEntry(logEntry)
						return nil, eatError
					} else {
						// For this kind of input, EOF just means that there is nothing
						// to read just yet, but there may be something later.  So we
						// just return, expecting the caller to call us again.
						logEntry := "ReadNextFrame: non-fatal EOF while eating"
						rtcm.makeLogEntry(logEntry)
						return nil, nil
					}
				} else {
					// Any error other than EOF is always fatal.  Return immediately.
					logEntry := fmt.Sprintf("ReadNextFrame: error at start of eating - %v", eatError)
					rtcm.makeLogEntry(logEntry)
					return nil, eatError
				}
			} else {
				logEntry := fmt.Sprintf("ReadNextFrame: continuing after error,  eaten %d bytes - %v",
					len(frame), eatError)
				rtcm.makeLogEntry(logEntry)
			}
		}

		if len(frame) == 0 {
			// We've got nothing.  Pause and try again.
			logEntry := "ReadNextFrame: frame is empty while eating, but no error"
			rtcm.makeLogEntry(logEntry)
			rtcm.pause()
			continue
		}

		// We've read some text.
		break
	}

	// Figure out what ReadBytes has returned.  Could be a start of message byte,
	// some other text followed by the start of message byte or just some other
	// text.
	if len(frame) > 1 {
		// We have some non-RTCM, possibly followed by a start of message
		// byte.
		logEntry := fmt.Sprintf("ReadNextFrame: read %d bytes", len(frame))
		rtcm.makeLogEntry(logEntry)
		if frame[len(frame)-1] == StartOfMessageFrame {
			// non-RTCM followed by start of message byte.  Push the start
			// byte back so we see it next time and return the rest of the
			// buffer as a non-RTCM message.
			logEntry1 := "ReadNextFrame: found d3 - unreading"
			rtcm.makeLogEntry(logEntry1)
			reader.UnreadByte()
			frameWithoutTrailingStartByte := frame[:len(frame)-1]
			logEntry2 := fmt.Sprintf("ReadNextFrame: returning %d bytes %s",
				len(frameWithoutTrailingStartByte),
				hex.Dump(frameWithoutTrailingStartByte))
			rtcm.makeLogEntry(logEntry2)
			return frameWithoutTrailingStartByte, nil
		} else {
			// Just some non-RTCM.
			logEntry := fmt.Sprintf("ReadNextFrame: got: %d bytes %s",
				len(frame),
				hex.Dump(frame))
			rtcm.makeLogEntry(logEntry)
			return frame, nil
		}
	}

	// The buffer contains just a start of message byte so
	// we may have the start of an RTCM message frame.
	// Get the rest of the message frame.
	logEntry := "ReadNextFrame: found d3 immediately"
	rtcm.makeLogEntry(logEntry)
	var n int = 1
	var expectedFrameLength uint = 0
	buf := make([]byte, 1)
	for {
		l, readErr := reader.Read(buf)
		// We've read some text, so log any read error, but ignore it.  If it's
		// a hard error it will be caught on the next call.
		if readErr != nil {
			if readErr != io.EOF {
				// Any error other than EOF is always fatal, but it will be caught
				logEntry := fmt.Sprintf("ReadNextFrame: ignoring error while reading message - %v", readErr)
				rtcm.makeLogEntry(logEntry)
				return frame, nil
			}

			if rtcm.StopOnEOF {
				// EOF is fatal for the kind of input file we are reading.
				logEntry := "ReadNextFrame: ignoring fatal EOF"
				rtcm.makeLogEntry(logEntry)
				return frame, nil
			} else {
				// For this kind of input, EOF just means that there is nothing
				// to read just yet, but there may be something later.  So we
				// just pause and try again.
				logEntry := "ReadNextFrame: ignoring non-fatal EOF"
				rtcm.makeLogEntry(logEntry)
				rtcm.pause()
				continue
			}
		}

		if l < 1 {
			// We expected to read exactly one byte, so there is currently
			// nothing to read.  Pause and try again.
			logEntry := "ReadNextFrame: no data.  Pausing"
			rtcm.makeLogEntry(logEntry)
			rtcm.pause()
			continue
		}

		frame = append(frame, buf[0])
		n++

		// What we do next depends upon how much of the message we have read.
		// On the first few trips around the loop we read the header bytes and
		// the 10-bit expected message length l.  Once we know l, we can work
		// out the total length of the frame (which is l+6) and we can then
		// read the remaining bytes of the frame.
		switch {
		case n < leaderLengthBytes+2:
			continue

		case n == leaderLengthBytes+2:
			// We have the first three bytes of the frame so we have enough data to find
			// the length and the type of the message (which we will need in a later trip
			// around this loop).
			messageLength, messageType, err := rtcm.GetMessageLengthAndType(frame)
			if err != nil {
				// We thought we'd found the start of a message, but it's something else
				// that happens to start with the start of frame byte.
				// Return the collected data.
				logEntry := fmt.Sprintf("ReadNextFrame: error getting length and type: %v", err)
				rtcm.makeLogEntry(logEntry)
				return frame, nil
			}

			logEntry1 := fmt.Sprintf("ReadNextFrame: found message type %d length %d", messageType, messageLength)
			rtcm.makeLogEntry(logEntry1)

			// The frame contains a 3-byte header, a variable-length message (for which
			// we now know the length) and a 3-byte CRC.  Now we just need to continue to
			// read bytes until we have the whole message.
			expectedFrameLength = messageLength + leaderLengthBytes + crcLengthBytes
			logEntry2 := fmt.Sprintf("ReadNextFrame: expecting a %d frame", expectedFrameLength)
			rtcm.makeLogEntry(logEntry2)

			// Now we read the rest of the message byte by byte, one byte every trip.
			// We know how many bytes we want, so we could just read that many using one
			// Read call, but if the input stream is a serial connection, we would
			// probably need several of those, so we might as well do it this way.
			continue

		case expectedFrameLength == 0:
			// We haven't figured out the message length yet.
			continue

		case n >= int(expectedFrameLength):
			// By this point the expected frame length has been decoded and set to a
			// non-zero value (otherwise the previous case would have triggered) and we have
			// read that many bytes.  So we are done.  Return the complete message frame.
			// The CRC will be checked later.
			//
			// (The case condition could use ==, but using >= guarantees that the loop will
			// terminate eventually even if my logic is faulty and the loop overruns!)
			//
			logEntry := fmt.Sprintf("ReadNextFrame: returning an RTCM message frame, %d bytes, expected %d", n, expectedFrameLength)
			rtcm.makeLogEntry(logEntry)
			return frame, nil

		default:
			// In most trips around the loop, we just read the next byte and build up the
			// message frame.
			continue
		}
	}
}

// ReadNextMessage gets the next message frame from a reader, extracts
// and returns the message.  It returns any read error that it encounters,
// such as EOF.
func (rtcm *RTCM) ReadNextMessage(reader *bufio.Reader) (*Message, error) {

	frame, err1 := rtcm.ReadNextFrame(reader)
	if err1 != nil {
		return nil, err1
	}

	if len(frame) == 0 {
		return nil, nil
	}

	// Return the chunk as a Message.
	message, messageFetchError := rtcm.GetMessage(frame)
	return message, messageFetchError
}

// GetMessage1005 returns a text version of a message type 1005
func (rtcm *RTCM) GetMessage1005(m []byte) (*Message1005, error) {
	var result Message1005
	// Pos is the position within the bitstream.
	var pos uint = HeaderLengthBits

	result.MessageType = uint(GetBitsAsUint64(m, pos, 12))
	pos += 12
	result.StationID = uint(GetBitsAsUint64(m, pos, 12))
	pos += 12
	result.ITRFRealisationYear = uint(GetBitsAsUint64(m, pos, 6))
	pos += 6
	result.Ignored1 = uint(GetBitsAsUint64(m, pos, 4))
	pos += 4
	result.AntennaRefX = GetbitsAsInt64(m, pos, 38)
	pos += 38
	result.Ignored2 = uint(GetBitsAsUint64(m, pos, 2))
	pos += 2
	result.AntennaRefY = GetbitsAsInt64(m, pos, 38)
	pos += 38
	result.Ignored3 = uint(GetBitsAsUint64(m, pos, 2))
	pos += 2
	result.AntennaRefZ = GetbitsAsInt64(m, pos, 38)
	pos += 38

	return &result, nil
}

// GetMSMHeader extracts the header from an MSM message (MSM4 or MSM7).
// It returns the header data and the bit position of the start of the
// satellite data (which comes next in the bit stream).  If the bit stream
// is not long enough, an error is returned.
//
func (rtcm *RTCM) GetMSMHeader(bitStream []byte) (*MSMHeader, uint, error) {

	// The MSMHeader contains:
	//    a 12-bit unsigned message type (1074, 1084 ... MSM4. 1077 ... MSM7.)
	//    a 12-bit unsigned station ID
	//    a 30-bit unsigned timestamp
	//    a boolean multiple message flag
	//    a 3-bit unsigned sequence number
	//    a 7-bit unsigned session transmission time value
	//    a 2-bit unsigned clock steering indicator
	//    a 2-bit unsigned external clock indicator
	//    a boolean GNSS Divergence Free Smoothing Indicator
	//    a 3-bit GNSS Smoothing Interval
	//    a 64-bit satellite mask (one bit sit per satellite observed)
	//    a 32-bit signal mask (one bit set per signal type observed)
	//    a cell mask (nSatellites X nSignals) bits long
	//
	// The function returns the broken out header and the bit position
	// of the start of the next part of the message, which follows
	// immediately after the cell mask.
	const lenMessageType = 12
	const lenStationID = 12
	const lenEpochTime = 30
	const lenMultipleMessageFlag = 1
	const lenSequenceNumber = 3
	const lenSessionTransmissionTime = 7
	const lenClockSteeringIndicator = 2
	const lenExternalClockSteeringIndicator = 2
	const lenSmoothingIndicator = 1
	const lenSmoothingInterval = 3

	// The minimum length of an MSM header
	const minLengthMSMHeader = lenMessageType + lenStationID + lenEpochTime + lenMultipleMessageFlag +
		lenSequenceNumber + lenSessionTransmissionTime + lenClockSteeringIndicator +
		lenExternalClockSteeringIndicator + lenSmoothingIndicator + lenSmoothingInterval +
		lenSatelliteMask + lenSignalMask

	const maxLengthOfCellMask = 64

	// lenBitStreamInBits is the length of the bitstream in bits, including
	// the 3-byte message leader.
	lenBitStreamInBits := len(bitStream) * 8

	// We don't know the length of the header yet, but we have a minimum.
	// Check that, ignoring the 3-byte message leader.
	if (lenBitStreamInBits - leaderLengthInBits) < minLengthMSMHeader {
		// Error - not enough data.
		em := fmt.Sprintf("bitstream is too short for an MSM header - got %d bits, expected at least %d",
			lenBitStreamInBits, minLengthMSMHeader)
		return nil, 0, errors.New(em)
	}

	// Get a header object with the type values filled in.  (Delegating
	// this to a subsidiary function simplifies the testing.)
	msmHeader, pos, headerError := getMSMType(bitStream, 0)

	if headerError != nil {
		return nil, 0, headerError
	}

	// Get the rest of the fixed-length values.
	msmHeader.StationID = uint(GetBitsAsUint64(bitStream, pos, lenStationID))
	pos += lenStationID
	msmHeader.EpochTime = uint(GetBitsAsUint64(bitStream, pos, lenEpochTime))
	pos += lenEpochTime
	msmHeader.MultipleMessage = (GetBitsAsUint64(bitStream, pos,
		lenMultipleMessageFlag) == 1)
	pos += lenMultipleMessageFlag
	msmHeader.SequenceNumber = uint(GetBitsAsUint64(bitStream, pos,
		lenSequenceNumber))
	pos += lenSequenceNumber
	msmHeader.SessionTransmissionTime = uint(GetBitsAsUint64(bitStream, pos,
		lenSessionTransmissionTime))
	pos += lenSessionTransmissionTime
	msmHeader.ClockSteeringIndicator = uint(GetBitsAsUint64(bitStream, pos,
		lenClockSteeringIndicator))
	pos += lenClockSteeringIndicator
	msmHeader.ExternalClockIndicator = uint(GetBitsAsUint64(bitStream, pos,
		lenExternalClockSteeringIndicator))
	pos += lenExternalClockSteeringIndicator
	msmHeader.GNSSDivergenceFreeSmoothingIndicator =
		(GetBitsAsUint64(bitStream, pos, lenSmoothingIndicator) == 1)
	pos += lenSmoothingIndicator
	msmHeader.GNSSSmoothingInterval = uint(GetBitsAsUint64(bitStream, pos,
		lenSmoothingInterval))
	pos += lenSmoothingInterval

	// Get the satellite mask, but don't advance the bit position.  That's
	// done next.
	msmHeader.SatelliteMaskBits = uint64(GetBitsAsUint64(bitStream, pos,
		lenSatelliteMask))

	// Create a slice of satellite IDs, advancing the bit position as we go.
	// Bit 63 of the mask is satellite number 1, bit 62 is 2, bit 0 is 64.
	// If signals were observed from satellites 3, 7 and 9, the slice will
	// contain {3, 7, 9}.  (Note, we then expect to see 3 signal cells later
	// in the message.)
	msmHeader.Satellites = make([]uint, 0)
	for satNum := 1; satNum <= lenSatelliteMask; satNum++ {
		if GetBitsAsUint64(bitStream, pos, 1) == 1 {
			msmHeader.Satellites = append(msmHeader.Satellites, uint(satNum))
		}
		pos++
	}

	// Get the signal mask, but don't advance the bit position.  That's
	// done next.
	msmHeader.SignalMaskBits = uint32(GetBitsAsUint64(bitStream, pos,
		lenSignalMask))

	// Create a slice of signal IDs, advancing the bit position as we go.
	// Bit 31 of the mask is signal number 1, bit 30 is 2, bit 0 is 32.
	// If we observed signals 62 and 64, the mask bits will be 0x0005
	// and the slice will contain {62, 64}.
	msmHeader.Signals = make([]uint, 0)
	for sigNum := 1; sigNum <= lenSignalMask; sigNum++ {
		if GetBitsAsUint64(bitStream, pos, 1) == 1 {
			msmHeader.Signals = append(msmHeader.Signals, uint(sigNum))
		}
		pos++
	}

	// The last component of the header is the cell mask.  This is variable
	// length - (number of signals) X (number of satellites) bits, no more
	// than maxLengthOfCellMask bits long.  Now that we know those values, we
	// can calculate the length and do some final sanity checks.
	//
	cellMaskLength := uint(len(msmHeader.Satellites) * len(msmHeader.Signals))

	if cellMaskLength > maxLengthOfCellMask {
		em := fmt.Sprintf("GetMSMHeader: cellMask is %d bits - expected <= %d",
			cellMaskLength, maxLengthOfCellMask)
		return nil, 0, errors.New(em)
	}

	// headerBits is the number of bits in the bitstream including the 24-bit
	// message leader.
	headerBits := uint(lenBitStreamInBits)

	// expectedHeaderLength is the length of the header including the cell mask
	// and the 24-bit message leader.
	expectedHeaderLength := minLengthMSMHeader + cellMaskLength + leaderLengthInBits

	// Check that the bitstream is long enough.
	if headerBits < expectedHeaderLength {
		// Error - not enough data.
		em := fmt.Sprintf("bitstream is too short for an MSM header with %d cell mask bits - got %d bits, expected at least %d",
			cellMaskLength, lenBitStreamInBits, expectedHeaderLength)
		return nil, 0, errors.New(em)

	}

	// Get the cell mask as a string of bits.  The bits form a two-dimensional
	// array of (number of signals) X (number of satellites) bits.  The length
	// is always <= 64.  If the receiver observed two signals types and it
	// observed both types from one satellite, just the first type from another
	// and just the second type from a third satellite, the mask will be 11 10 01.
	// Don't advance the bit position - that's done next.
	msmHeader.CellMaskBits = uint64(GetBitsAsUint64(bitStream, pos, cellMaskLength))

	// Create a slice of slices of bools, each bool representing a bit in the
	// cell mask.  If the receiver observed two signals types and it observed
	// them from 3 satellites, the cell mask might be:  {{t,t, f}, {t,f, t}}
	// meaning that the device received both types from the first satellite,
	// just the first signal type from the second satellite and just the
	// second type from the third satellite.  (Note: given that in that example
	// four items are set true, we also expect to see four signal cells later
	// in the message.
	//
	msmHeader.CellMask = make([][]bool, 0)
	for i := 0; i < len(msmHeader.Satellites); i++ {
		row := make([]bool, 0)
		for j := 0; j < len(msmHeader.Signals); j++ {
			value := (uint32(GetBitsAsUint64(bitStream, pos, 1)) == 1)
			pos++
			row = append(row, value)
			if value {
				msmHeader.NumSignalCells++
			}
		}
		msmHeader.CellMask = append(msmHeader.CellMask, row)
	}

	return msmHeader, pos, nil
}

// getMSMwithType extracts the message type from the bitstream, starting at
// the startBit position, and returns an MSMHeader object with the type
// fields filled in, the number of bits from the bitstream consumed and any
// error message.  The bitstream is presumed to contain an MSM4 or an MSM7
// Message and the MSM4 field in the header indicates which of those it is.
// An error is returned if the bitstream is too short or if the message
// is not an MSM4 or an MSM7.
//
func getMSMType(bitStream []byte, startBit uint) (*MSMHeader, uint, error) {

	// Number of bits in the message type value.
	const bitsInMessageType = 12
	// Number of bits in the 3-byte message leader.
	const bitsInMessageLeader = leaderLengthBytes * 8
	// For the bitstream to contain a message type, it must be at least this long.
	const minMessageLength = bitsInMessageLeader + bitsInMessageType

	// Check that the bitstream is long enough.
	messageBits := len(bitStream) * 8
	if messageBits < minMessageLength {
		// Error - not enough data.
		em := fmt.Sprintf("cannot extract the header from a bitstream %d bits long, expected at least %d bits",
			messageBits, minMessageLength)
		return nil, 0, errors.New(em)
	}

	// Get the message type.
	pos := startBit + leaderLengthInBits
	messageType := int(GetBitsAsUint64(bitStream, pos, 12))
	pos += 12
	constellation := ""

	// Figure out whether it's an MSM4, and MSM7 or some unexpected type.
	switch messageType {
	case 1074:
		constellation = "GPS"
	case 1084:
		constellation = "GLONASS"
	case 1094:
		constellation = "Galileo"
	case 1104:
		constellation = "SBAS"
	case 1114:
		constellation = "QZSS"
	case 1124:
		constellation = "BeiDou"
	case 1134:
		constellation = "NavIC/IRNSS"
	case 1077:
		constellation = "GPS"
	case 1087:
		constellation = "GLONASS"
	case 1097:
		constellation = "Galileo"
	case 1107:
		constellation = "SBAS"
	case 1117:
		constellation = "QZSS"
	case 1127:
		constellation = "BeiDou"
	case 1137:
		constellation = "NavIC/IRNSS"

	default:
		// Error - the message must be MSM4 or MSM7.
		em := fmt.Sprintf("message type %d is not an MSM4 or an MSM7", messageType)
		return nil, 0, errors.New(em)
	}

	// Create and return the header.
	msmHeader := MSMHeader{MessageType: messageType, Constellation: constellation}

	return &msmHeader, pos, nil
}

// getSatelliteCells extracts the satellite cell data from an MSM4 message.
// It returns a slice of cell data and the number of bits of the message
// bitstream consumed so far (which is the index of the start of the signal
// cells).  If the bitstream is not long enough to contain the message,
// an error is returned.
//
func (rtcm *RTCM) getSatelliteCells(bitStream []byte, header *MSMHeader, startOfSatelliteData uint) ([]MSMSatelliteCell, uint, error) {
	// The second part of the MSM message is the satellite data.  If signals were
	// observed from satellites 2, 3 and 15, there will be three sets of data fields.
	// The bit stream contains all of the rough range values, followed by the range
	// delta values.  The rough values and delta values are merged together later.

	// Define the lengths of each field and the invalid value if any.
	const lenWholeMillis = 8
	const lenExtendedInfo = 4 // MSM7 only.
	const lenFractionalMillis = 10
	const lenPhaseRangeRate = 14 // MSM7 only.

	bitsLeft := len(bitStream)*8 - int(startOfSatelliteData)
	minBitsPerMSM4 := len(header.Satellites) * (lenWholeMillis + lenFractionalMillis)
	minBitsPerMSM7 := minBitsPerMSM4 +
		(len(header.Satellites) * (lenPhaseRangeRate + lenExtendedInfo))

	if MSM4(header.MessageType) {
		if (len(bitStream) * 8) <= minBitsPerMSM4 {
			message := fmt.Sprintf("overrun - not enough data for %d MSM4 satellite cells - %d %d",
				header.Satellites, minBitsPerMSM4, bitsLeft)
			return nil, 0, errors.New(message)
		}

	} else {

		if (len(bitStream) * 8) < minBitsPerMSM7 {
			message := fmt.Sprintf("overrun - not enough data for %d MSM7 satellite cells - %d %d",
				len(header.Satellites), minBitsPerMSM7, bitsLeft)
			return nil, 0, errors.New(message)
		}
	}

	// Get the satellite ids.  If the satellite list in the header (h.Satellites)
	// contains {2, 3, 15} then we have observed satellites with IDs 2, 3 and 15.
	//
	id := make([]uint, 0)
	for i := range header.Satellites {
		id = append(id, uint(header.Satellites[i]))
	}

	// Set the bit position to the start of the satellite data in the message.
	pos := startOfSatelliteData

	// Get the rough range values (whole milliseconds).
	wholeMillis := make([]uint, 0)
	for range header.Satellites {
		millis := uint(GetBitsAsUint64(bitStream, pos, lenWholeMillis))
		pos += lenWholeMillis
		wholeMillis = append(wholeMillis, millis)
	}

	// An MSM7 has a phase range rate field, an MSM4 does not.
	extendedInfo := make([]uint, 0)
	if MSM7(header.MessageType) {
		for range header.Satellites {
			info := GetBitsAsUint64(bitStream, pos, lenExtendedInfo)
			pos += lenExtendedInfo
			extendedInfo = append(extendedInfo, uint(info))
		}
	}

	// Get the fractional millis values (fractions of a millisecond).
	fractionalMillis := make([]uint, 0)
	for range header.Satellites {
		fraction := GetBitsAsUint64(bitStream, pos, lenFractionalMillis)
		pos += lenFractionalMillis
		fractionalMillis = append(fractionalMillis, uint(fraction))
	}

	// An MSM7 has a phase range rate field, an MSM4 does not.
	phaseRangeRate := make([]int, 0)
	if MSM7(header.MessageType) {
		for range header.Satellites {
			rate := GetbitsAsInt64(bitStream, pos, lenPhaseRangeRate)
			pos += lenPhaseRangeRate
			phaseRangeRate = append(phaseRangeRate, int(rate))
		}
	}

	// Create a slice of satellite cells initialised from those data.
	satData := make([]MSMSatelliteCell, 0)
	for i := range header.Satellites {
		if MSM4(header.MessageType) {
			satCell := createSatelliteCell(header.MessageType, id[i], wholeMillis[i], fractionalMillis[i], 0, 0)
			satData = append(satData, satCell)
		} else {
			satCell := createSatelliteCell(header.MessageType, id[i], wholeMillis[i],
				fractionalMillis[i], extendedInfo[i], phaseRangeRate[i])
			satData = append(satData, satCell)
		}
	}

	// The bit position is now at the start of the signal data.
	startOfSignalData := pos

	return satData, startOfSignalData, nil
}

// createSatelliteCell creates a satellite cell from the given values.  If msm4 is
// true, it's an MSM4 cell, otherwise it's an MSM7 cell.
func createSatelliteCell(messageType int, id, wholeMillis, fractionalMillis, extendedInfo uint, phaseRangeRate int) MSMSatelliteCell {
	// Invalid values for various fields.  These are n-bit values with the top
	// bit set to 1 and the rest all zero.  For a two-s complement signed quantity,
	// the result will be a negative number.

	cell := MSMSatelliteCell{
		MessageType:           messageType,
		SatelliteID:           id,
		RangeWholeMillis:      wholeMillis,
		RangeFractionalMillis: fractionalMillis,
	}

	if MSM7(messageType) {
		// An MSM7 message has an extended info and a phase
		// range rate field.
		cell.ExtendedInfo = extendedInfo
		cell.PhaseRangeRate = phaseRangeRate
	}
	return cell
}

// getSignalCells gets the raw signal data.  The returned object contains the raw data
// for each signal collected together.
func (rtcm *RTCM) getSignalCells(bitStream []byte, header *MSMHeader, satData []MSMSatelliteCell, startOfSignaldata uint) ([][]MSMSignalCell, error) {
	// The third part of the message bit stream is the signal data.  Each satellite can
	// send many signals, each on a different frequency.  For example, if we observe one
	// signal from satellite 2, two from satellite 3 and 2 from satellite 15, there will
	// be five sets of signal data.  Irritatingly they are not laid out in a convenient
	// way.  First, we get the pseudo range delta values for each of the five signals,
	// followed by all of the phase range delta values, and so on.  Some of the fields
	// are different length depending on the message type, MSM4 or MSM7.
	//
	// It's more convenient to present these as a slice of slices of fields, one outer
	// slice for each satellite and one inner slice for each observed signal.
	//
	// Some of the values in the MSMSignalCell objects are derived from the data in the
	// message.  For example, the range in metres is derived by aggregating the approximate
	// value from the MSMSatelliteCell and the delta in the MSMS, which produces a
	// transit time, and then applying a conversion.

	// Define the lengths of the fields
	const lenRangeDeltaMSM4 uint = 15
	const lenRangeDeltaMSM7 uint = 20
	const lenPhaseRangeDeltaMSM4 uint = 22
	const lenPhaseRangeDeltaMSM7 uint = 24
	const lenLockTimeIndicatorMSM4 uint = 4
	const lenLockTimeIndicatorMSM7 uint = 10
	const lenHalfCycleAmbiguity uint = 1
	const lenCNRMSM4 uint = 6
	const lenCNRMSM7 uint = 10
	const lenPhaseRangeRateDelta uint = 15

	const bitsPerMSM4 = lenRangeDeltaMSM4 + lenPhaseRangeDeltaMSM4 +
		lenLockTimeIndicatorMSM4 + lenHalfCycleAmbiguity + lenCNRMSM4

	const bitsPerMSM7 = lenRangeDeltaMSM7 + lenPhaseRangeDeltaMSM7 +
		lenLockTimeIndicatorMSM7 + lenHalfCycleAmbiguity + lenCNRMSM7 +
		lenPhaseRangeRateDelta

	// Pos is the position within the bitstream.
	pos := startOfSignaldata

	// If type4 is true, the message is an MSM4, otherwise it's an MSM7
	type4 := MSM4(int(header.MessageType))

	// Check that there are enough bits in the message for the expected
	// number of signals.
	bitsLeft := uint(len(bitStream)*8) - pos
	if type4 {
		bitsExpected := uint(header.NumSignalCells) * bitsPerMSM4
		if bitsLeft < bitsExpected {
			message := fmt.Sprintf("overrun - not enough data for %d MSM4 signals - %d %d",
				header.NumSignalCells, bitsExpected, bitsLeft)
			return nil, errors.New(message)
		}
	} else {
		bitsExpected := uint(header.NumSignalCells) * bitsPerMSM7
		if bitsLeft < bitsExpected {
			message := fmt.Sprintf("overrun - not enough data for %d MSM7 signals - %d %d",
				header.NumSignalCells, bitsExpected, bitsLeft)
			return nil, errors.New(message)
		}
	}

	// Get the range deltas.
	rangeDelta := make([]int, 0)
	{
		// The length of this field in an MSM4 is different from the length in an MSM7.
		length := lenRangeDeltaMSM7
		if type4 {
			length = lenRangeDeltaMSM4
		}
		for i := 0; i < header.NumSignalCells; i++ {
			rd := int(GetbitsAsInt64(bitStream, pos, length))
			pos += length
			rangeDelta = append(rangeDelta, rd)
		}
	}

	// Get the phase range deltas.
	phaseRangeDelta := make([]int, 0)
	{
		// The length of this field in an MSM4 is different from the length in an MSM7.
		length := lenPhaseRangeDeltaMSM7
		if type4 {
			length = lenPhaseRangeDeltaMSM4
		}
		for i := 0; i < header.NumSignalCells; i++ {
			prd := int(GetbitsAsInt64(bitStream, pos, length))
			pos += length
			phaseRangeDelta = append(phaseRangeDelta, prd)
		}
	}

	// Get the lock time indicators.
	lockTimeIndicator := make([]uint, 0)
	{
		// The length of this field in an MSM4 is different from the length in an MSM7.
		length := lenLockTimeIndicatorMSM7
		if type4 {
			length = lenLockTimeIndicatorMSM4
		}
		for i := 0; i < header.NumSignalCells; i++ {
			lti := uint(GetBitsAsUint64(bitStream, pos, length))
			pos += length
			lockTimeIndicator = append(lockTimeIndicator, lti)
		}
	}

	// Get the half-cycle ambiguity indicator bits.
	halfCycleAmbiguity := make([]bool, 0)
	for i := 0; i < header.NumSignalCells; i++ {
		hca := (GetBitsAsUint64(bitStream, pos, lenHalfCycleAmbiguity) == 1)
		pos += lenHalfCycleAmbiguity
		halfCycleAmbiguity = append(halfCycleAmbiguity, hca)
	}

	// Get the CNRs.
	cnr := make([]uint, 0)
	{
		// The length of this field in an MSM4 is different from the length in an MSM7.
		length := lenCNRMSM7
		if type4 {
			length = lenCNRMSM4
		}
		for i := 0; i < header.NumSignalCells; i++ {
			c := uint(GetBitsAsUint64(bitStream, pos, length))
			pos += length
			cnr = append(cnr, c)
		}
	}

	// Get the phase range rate deltas (MSM7 only)
	phaseRangeRateDelta := make([]int, 0)
	if !type4 {
		length := lenPhaseRangeRateDelta
		for i := 0; i < header.NumSignalCells; i++ {
			delta := int(GetbitsAsInt64(bitStream, pos, length))
			pos += length
			phaseRangeRateDelta = append(phaseRangeRateDelta, delta)
		}
	}

	// Create the signal cells in a slice.
	cellSlice := make([]MSMSignalCell, 0)
	for i := 0; i < header.NumSignalCells; i++ {
		if type4 {
			// Create an MSM4 cell.
			cell := createSignalCell(true, 0, rangeDelta[i], phaseRangeDelta[i],
				lockTimeIndicator[i], halfCycleAmbiguity[i], cnr[i], 0)
			cellSlice = append(cellSlice, cell)
		} else {
			// Create an MSM7 cell.
			cell := createSignalCell(false, 0, rangeDelta[i], phaseRangeDelta[i],
				lockTimeIndicator[i], halfCycleAmbiguity[i], cnr[i], phaseRangeRateDelta[i])
			cellSlice = append(cellSlice, cell)
		}
	}

	// Arrange the cells into a slice of slices, and return.
	sigData := arrangeSignalCells(header, satData, cellSlice)

	return sigData, nil
}

// createSignalCell creates an MSM Signal Cell.
func createSignalCell(type4 bool, signalID uint, rangeDelta, phaseRangeDelta int, lockTimeIndicator uint, halfCycleAmbiguity bool, cnr uint, phaseRangeRateDelta int) MSMSignalCell {

	// Create a cell with the values that are always valid.
	cell := MSMSignalCell{SignalID: signalID,
		RangeDelta: rangeDelta, PhaseRangeDelta: phaseRangeDelta,
		LockTimeIndicator: lockTimeIndicator, HalfCycleAmbiguity: halfCycleAmbiguity,
		CNR: cnr}

	if !type4 {
		// An MSM4 message doesn't have the phase range rate field.
		cell.PhaseRangeRateDelta = phaseRangeRateDelta
	}

	return cell
}

// arrangeSignalCells arranges the signal cells into a slice of slices, one outer
// slice per satellite containing a slice of signals for that satellite.  It also
// adds pointers to the satellite cell for the satellite that received the signal
// and the header of the message.
func arrangeSignalCells(header *MSMHeader, satCell []MSMSatelliteCell, signalCell []MSMSignalCell) [][]MSMSignalCell {
	// For example if the satellite mask in the header contains {3, 5, 8} and
	// cell mask contains {{1,5},{1},{5}} then we received signals 1 and 5 from
	// satellite 3, signal 1 from satellite 5 and signal 5 from satellite 8.
	// We would return this slice of slices:
	//     0: a slice of two signal cells with satellite ID 3, signal IDs 1 and 5
	//     1: a slice of one signal cell with satellite ID 5, signal ID 1
	//     2: a slice of one signal cell with satellite ID 8, signal ID 5
	// and so on ...
	//
	// Figuring this out is a bit messy, because the necessary information
	// is distributed over the satellite, signal and cell masks in a form
	// that's compact but difficult to unpick.
	//
	signalCells := make([][]MSMSignalCell, len(header.Satellites))
	for i := range signalCells {
		signalCells[i] = make([]MSMSignalCell, 0)
	}

	// signalCellSlice is a slice of cells, c is the index of the next entry.
	c := 0

	// We want a slice of slices of signal cells, one outer slice per satellite,
	// one inner slice for the signals that the satellite received.  The logic is
	// tricky.  If signals 1 and 3 were observed, the signal mask will contain
	// {1,3}.  If satellite 4 received signals 1 and 3 and satellite 6 received
	// signal 1, h.Satellites will contain {4, 6} and h.CellMask will contain
	// {{t,t},{t,f}} - three true values, implying three signal cells in total.
	// The result is a slice of two slices, the first for satellite 4 and
	// containing two signal cells and the second for satellite 6 containing
	// just one signal cell.
	//
	// We need the same logic to figure out which satellite received each
	// signal, so we do that here too.
	//
	for i := range header.CellMask {
		for j := range header.CellMask[i] {
			if header.CellMask[i][j] {
				signalCell[c].Header = header
				// Set the reference to the satellite that received this signal.
				signalCell[c].Satellite = &(satCell[i])
				// Get the signal Id from the signal mask.
				signalCell[c].SignalID = header.Signals[j]
				// Put the signal cell into the correct slice.
				signalCells[i] = append(signalCells[i], signalCell[c])
				// Prepare to process the next signal cell.
				c++
			}
		}
	}

	return signalCells
}

// getSatellites gets the list of satellites from the 64-bit
// satellite mask as a slice of satellite numbers each >= 1,
// <= 64.  The mask starts at bit pos in the message.
func getSatellites(message []byte, pos uint) []uint {
	// Bit 63 of the mask is satellite number 1, bit 62 is 2,
	// bit 0 is 64.
	satellites := make([]uint, 0)
	for satNum := 1; satNum <= lenSatelliteMask; satNum++ {
		if GetBitsAsUint64(message, pos, 1) == 1 {
			satellites = append(satellites, uint(satNum))
		}
		pos++
	}
	return satellites
}

// getSignals gets the list of signals from the 32-bit signal
// mask as a slice of signal numbers each >= 1, <= 32.  The
// mask starts at bit pos in the message.
func getSignals(message []byte, pos uint) []uint {

	// Bit 31 of the mask is signal number 1, bit 30 is 2,
	// bit 0 is 32.
	signals := make([]uint, 0)
	for sigNum := 1; sigNum <= lenSignalMask; sigNum++ {
		if GetBitsAsUint64(message, pos, 1) == 1 {
			signals = append(signals, uint(sigNum))
		}
		pos++
	}
	return signals
}

// GetMSMMessage presents an MSM (type 1074, 1084 etc) or an MSM7 Message
// (type 1077, 1087 etc) as plain text showing the broken out fields.
func (rtcm *RTCM) GetMSMMessage(message *Message) (*MSMMessage, error) {

	if !MSM(message.MessageType) {
		message := fmt.Sprintf("message type %d is not an MSM", message.MessageType)
		return nil, errors.New(message)
	}

	header, startOfSatelliteData, headerError :=
		rtcm.GetMSMHeader(message.RawData)

	if headerError != nil {
		return nil, headerError
	}

	satellites, startOfSignals, fetchSatellitesError :=
		rtcm.getSatelliteCells(message.RawData, header, startOfSatelliteData)
	if fetchSatellitesError != nil {
		return nil, fetchSatellitesError
	}

	signals, fetchSignalsError :=
		rtcm.getSignalCells(message.RawData, header, satellites, startOfSignals)
	if fetchSignalsError != nil {
		return nil, fetchSignalsError
	}

	var msmMessage = MSMMessage{Header: header, Satellites: satellites, Signals: signals}

	return &msmMessage, nil
}

// GetUTCFromGPSTime converts a GPS time to UTC, using the start time
// to find the correct epoch.
//
func (rtcm *RTCM) GetUTCFromGPSTime(gpsTime uint) time.Time {
	// The GPS week starts at midnight at the start of Sunday
	// but GPS time is ahead of UTC by a few leap seconds, so in
	// UTC terms the week starts on Saturday a few seconds before
	// Saturday/Sunday midnight.
	//
	// We have to be careful when the start time is Saturday
	// and close to midnight, because that is within the new GPS
	// week.  We also have to keep track of the last GPS timestamp
	// and watch for it rolling over into the next week.

	if rtcm.previousGPSTimestamp > gpsTime {
		// The GPS Week has rolled over
		rtcm.startOfThisGPSWeek = rtcm.startOfNextGPSWeek
		rtcm.startOfNextGPSWeek = rtcm.startOfNextGPSWeek.AddDate(0, 0, 7)
	}
	rtcm.previousGPSTimestamp = gpsTime

	durationSinceStart := time.Duration(gpsTime) * time.Millisecond
	return rtcm.startOfThisGPSWeek.Add(durationSinceStart)
}

// GetUTCFromGlonassTime converts a Glonass epoch time to UTC using
// the start time to give the correct Glonass epoch.
func (rtcm *RTCM) GetUTCFromGlonassTime(epochTime uint) time.Time {
	// The Glonass epoch time is two bit fields giving the day and
	// milliseconds since the start of the day.  The day is 0: Sunday,
	// 1: Monday and so on, but three hours ahead of UTC.  The Glonass
	// day starts at midnight.
	//
	// day = 1, glonassTime = 1 is 1 millisecond into Russian Monday,
	// which in UTC is Sunday 21:00:00 plus one millisecond.
	//
	// Day = 1, glonassTime = (4*3600*1000) is 4 am on Russian Monday,
	// which in UTC is 1 am on Monday.
	//
	// The rollover mechanism assumes that the method is called fairly
	// regularly, at least once each day, so the day in one call should
	// be the either the same as the day in the last call or one day more.
	// If there is a gap between the days, we can't know how big that
	// gap is - three days?  Three months?  (In real life, a base station
	// will be producing RTCM3 messages something like once per second, so
	// this assumption is safe.)

	day, millis := ParseGlonassEpochTime(epochTime)

	if day != rtcm.previousGlonassDay {
		// The day has rolled over.
		rtcm.startOfThisGlonassDay =
			rtcm.startOfThisGlonassDay.AddDate(0, 0, 1)
		rtcm.startOfNextGlonassDay =
			rtcm.startOfThisGlonassDay.AddDate(0, 0, 1)
		rtcm.previousGlonassDay = uint(rtcm.startOfThisGlonassDay.Weekday())
	}

	offset := time.Duration(millis) * time.Millisecond
	return rtcm.startOfThisGlonassDay.Add(offset)
}

// GetUTCFromGalileoTime converts a Galileo time to UTC, using the same epoch
// as the start time.
//
func (rtcm *RTCM) GetUTCFromGalileoTime(galileoTime uint) time.Time {
	// Galileo time is currently (Jan 2020) the same as GPS time.
	return rtcm.GetUTCFromGPSTime(galileoTime)
}

// GetUTCFromBeidouTime converts a Baidou time to UTC, using the Beidou
// epoch given by the start time.
//
func (rtcm *RTCM) GetUTCFromBeidouTime(epochTime uint) time.Time {
	// BeiDou - the first few seconds of UTC Sunday are in one week,
	// then the epoch time rolls over and all subsequent times are
	// in the next week.
	if epochTime < rtcm.previousBeidouTimestamp {
		rtcm.startOfThisBeidouWeek = rtcm.startOfNextBeidouWeek
		rtcm.startOfNextBeidouWeek =
			rtcm.startOfNextBeidouWeek.AddDate(0, 0, 1)
	}
	rtcm.previousBeidouTimestamp = epochTime

	durationSinceStart := time.Duration(epochTime) * time.Millisecond
	return rtcm.startOfThisBeidouWeek.Add(durationSinceStart)
}

// GetStartOfLastSundayUTC gets midnight at the start of the
// last Sunday (which may be today) in UTC.
//
func GetStartOfLastSundayUTC(now time.Time) time.Time {
	// Convert the time to UTC, which may change the day.
	now = now.In(locationUTC)

	// Crank the day back to Sunday.  (It may already be there.)
	for {
		if now.Weekday() == time.Sunday {
			break
		}
		now = now.AddDate(0, 0, -1)
	}

	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, locationUTC)
}

// DisplayMessage takes the given Message object and returns it
// as a readable string.
//
func (handler *RTCM) DisplayMessage(message *Message) string {

	if message.MessageType == NonRTCMMessage {
		return fmt.Sprintf("not RTCM, %d bytes, %s\n%s\n",
			len(message.RawData), message.Warning, hex.Dump(message.RawData))
	}

	status := ""
	if message.Valid {
		status = "valid"
	} else {
		if message.Complete {
			status = "complete "
		} else {
			status = "incomplete "
		}
		if message.CRCValid {
			status += "CRC check passed"
		} else {
			status += "CRC check failed"
		}
	}

	leader := fmt.Sprintf("message type %d, frame length %d %s %s\n",
		message.MessageType, len(message.RawData), status, message.Warning)
	leader += fmt.Sprintf("%s\n", hex.Dump(message.RawData))

	if !message.Valid {
		return leader
	}

	if !displayable(message.MessageType) {
		return leader
	}

	// The message is displayable.  Prepare it for display.
	_, ok := message.PrepareForDisplay(handler).(*MSMMessage)
	if !ok {
		return ("expected the readable message to be *MSMMessage\n")
	}

	switch {

	case message.MessageType == 1005:
		m, ok := message.PrepareForDisplay(handler).(*Message1005)
		if !ok {
			return ("expected the readable message to be *Message1005\n")
		}
		return leader + handler.display1005(m)

	case MSM4(message.MessageType):
		m, ok := message.PrepareForDisplay(handler).(*MSMMessage)
		if !ok {
			return ("expected the readable message to be an MSM4\n")
		}
		return leader + m.Display(handler)

	case MSM7(message.MessageType):
		m, ok := message.PrepareForDisplay(handler).(*MSMMessage)
		if !ok {
			return ("expected the readable message to be an MSM7\n")
		}
		return leader + m.Display(handler)

	default:
		return leader + "\n"
	}
}

// Displayable is true if the message type is one that we know how
// to display in a readable form.
func displayable(messageType int) bool {
	// we currently can display messages of type 1005, MSM4 and MSM7.
	// A non-RTCM message has type -1, so messageType must be an int.

	if messageType == NonRTCMMessage {
		return false
	}

	if MSM4(messageType) || MSM7(messageType) || messageType == 1005 {
		return true
	}

	return false
}

// displayableMSM is true if the message is MSM and we know how to
// display it.
func displayableMSM(messageType int) bool {

	if MSM4(messageType) {
		return true
	}

	if MSM7(messageType) {
		return true
	}

	return false
}

// display1005 returns a text version of a message type 1005
func (rtcm *RTCM) display1005(message *Message1005) string {

	l1 := fmt.Sprintln("message type 1005 - Base Station Information")

	l2 := fmt.Sprintf("stationID %d, ITRF realisation year %d, ignored 0x%4x,\n",
		message.StationID, message.ITRFRealisationYear, message.Ignored1)
	l2 += fmt.Sprintf("x %d ignored 0x%2x, y %d, ignored 0x%2x z %d,\n",
		message.AntennaRefX, message.Ignored2, message.AntennaRefY,
		message.Ignored3, message.AntennaRefZ)

	x := scaled5ToFloat(message.AntennaRefX)
	y := scaled5ToFloat(message.AntennaRefY)
	z := scaled5ToFloat(message.AntennaRefZ)
	l2 += fmt.Sprintf("ECEF coords in metres (%8.4f,%8.4f,%8.4f)\n",
		x, y, z)
	return l1 + l2
}

// CheckCRC checks the CRC of a message frame.
func CheckCRC(frame []byte) bool {
	if len(frame) < leaderLengthBytes+crcLengthBytes {
		return false
	}
	// The CRC is the last three bytes of the message frame.
	// The rest of the frame should produce the same CRC.
	crcHiByte := frame[len(frame)-3]
	crcMiByte := frame[len(frame)-2]
	crcLoByte := frame[len(frame)-1]

	l := len(frame) - crcLengthBytes
	headerAndMessage := frame[:l]
	newCRC := crc24q.Hash(headerAndMessage)

	if crc24q.HiByte(newCRC) != crcHiByte ||
		crc24q.MiByte(newCRC) != crcMiByte ||
		crc24q.LoByte(newCRC) != crcLoByte {

		// The calculated CRC does not match the one at the end of the message frame.
		return false
	}

	// We have a valid frame.
	return true
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

// GetbitsAsInt64 extracts len bits from a slice of bytes, starting at bit
// position pos, interprets the bits as a twos-complement integer and returns
// the resulting as a 64-bit signed int.  Se RTKLIB's getbits() function.
func GetbitsAsInt64(buff []byte, pos uint, len uint) int64 {
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

// ParseGlonassEpochTime separates out the two parts of a Glonass
// epoch time value -3/27 day/milliseconds from start of day.
//
func ParseGlonassEpochTime(epochTime uint) (uint, uint) {
	// fmt.Printf("ParseGlonassEpochTime %x\n", epochTime)
	day := (epochTime & glonassDayBitMask) >> 27
	millis := epochTime &^ glonassDayBitMask
	return day, millis
}

// HandleMessages reads from the input stream until it's exhausted, extracting any
// valid RTCM messages and copying them to those output channels which are not nil.
//
func (rtcm *RTCM) HandleMessages(reader io.Reader, channels []chan Message) {
	// HandleMessages is the core of a number of applications including the NTRIP
	// server.  It reads from the input stream until it's exhausted, extracting any
	// valid RTCM messages and copying them to those output channels which are not nil.
	//
	// In production this function is called from an application such as the rtcmlogger,
	// with one of the channel consumers sending messages to stdout and another consumer
	// writing to a log file.  Another example us the rtcmfilter, which works in a similar
	// way but with just one output channel connected to a consumer that writes what it
	// receives to stdout.
	//
	// The function can also be called from a separate program for integration testing.  For
	// example, the test program could supply a reader which is connected to a file of RTCM
	// messages and one channel, with a consumer writing to a file.  In that case the
	// function will extract the RTCM Messages, send them to the output channel and terminate.
	// The channel consumer could simply write the messages to a file, creating a copy program.
	//
	// If displayWriter is set, we write a readable version of the message to it.

	bufferedReader := bufio.NewReaderSize(reader, 64*1024)

	for {
		// Get the next message from the reader, discarding any intervening
		// non-RTCM3 material.  Return on any error.  (The channels should also
		// be closed to avoid a leak.  The caller created them so it's assumed
		// that it will close them.)
		message, messageFetchError := rtcm.ReadNextMessage(bufferedReader)
		if messageFetchError != nil {
			if message == nil {
				return
			} else {
				logEntry := fmt.Sprintf("HandleMessages ignoring error %v", messageFetchError)
				rtcm.makeLogEntry(logEntry)
			}
		}

		if message == nil {
			// There is no message yet.  Pause and try again.
			rtcm.makeLogEntry("HandleMessages: nil message - pausing")
			rtcm.pause()
			continue
		}

		// Send a copy of the message to each of the non-nil channels.
		for i := range channels {
			if channels[i] != nil {
				messageCopy := message.Copy()
				channels[i] <- messageCopy
			}
		}
	}
}

// getMidnightOn(time) takes a date and time in any timezone, converts it to UTC and returns
// midnight at the start of that date.  For example, 2021-01-01 01:00:00 in Paris produces
// 2020-12-30 00:00:00 UTC.
func getMidnightOn(date time.Time) time.Time {
	date = date.In(locationUTC)
	return (time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, locationUTC))
}

// getChar gets the next byte from the reader, or returns an error.
func (rtcm *RTCM) getChar(reader bufio.Reader) (byte, error) {

	// The read could return:
	//     a byte and no error
	//     a byte and an error
	//     no byte and no error
	//     no byte and an error
	//
	// For some types of input (for example a plain file which is not
	// being written by another process) EOF is a fatal error and we stop
	// processing.  For others (for example, a serial USB connection with
	// a device at the other end sending messages), EOF just means that
	// there is no input to read just now but if we try again later there
	// may be some more.  In that case, we just pause and try again.  (If
	// the device is disconnected, we expect to get an error, but not EOF.)
	//
	// If we get nothing (no error and no byte) we also assume that we may
	// get something if we try again later.
	//
	// If we get a fatal error AND a byte, then we want to process that
	// byte, so we ignore the error, assuming that we will get it again
	// on the next call, this time with no byte.

	buffer := make([]byte, 1)
	for {
		n, readError := reader.Read(buffer)
		if readError != nil {
			logEntry := "getChar - error - " + readError.Error()
			rtcm.makeLogEntry(logEntry)
			if n > 0 {
				break // We got some data.
			}
			// We got an error and no data.
			if readError == io.EOF {
				if rtcm.StopOnEOF {
					// EOF is fatal. Return a nul byte and the error.
					return 0, readError
				} else {
					// EOF is not fatal.  Pause and try again.
					rtcm.pause()
					continue
				}
			} else {
				// The error is not EOF, so always fatal.  Return
				// a null byte and the error.
				return 0, readError
			}
		}

		// We didn't get an error.
		if n == 0 {
			logEntry := "readChar - no error but no data"
			rtcm.makeLogEntry(logEntry)
			rtcm.pause()
			continue
		}

		// We didn't get an error and we have some data.
		break
	}

	// We have some data to process.
	return buffer[0], nil
}

// getScaledValue is a helper for functions such as getMSMScaledRange.
func getScaledValue(v1, shift1, v2, shift2 uint, delta int) uint64 {
	return uint64(int64((uint64(v1)<<shift1)|(uint64(v2)<<shift2)) + int64(delta))
}

// getPhaseRangeMilliseconds gets the phase range of the signal in metres
func getPhaseRangeMilliseconds(scaledRange uint64) float64 {

	// The scaled range is made up from the range values in the satellite
	// cell and the phase range delta value in the signal cell.

	// The scaled range has 31 bits fractional part.
	// scaleFactor is two to the power of 31:
	// 1000 0000 0000 0000 0000 0000 0000 0000
	const scaleFactor = 0x80000000

	// Restore the scale of the aggregate value.
	return float64(scaledRange) / scaleFactor
}

// getPhaseRangeLightMilliseconds gets the phase range of the signal in light milliseconds.
func getPhaseRangeLightMilliseconds(rangeMetres float64) float64 {
	return rangeMetres * oneLightMillisecond
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
	// signed delta. These are scaled up and the (positive or negative) delta is
	// added to give the resulting scaled integer, 18 bits whole and 19 bits
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
	// The two parts of the approximate value are uint and much bigger than the
	// delta when scaled, so the result is always positive - the transit time. The
	// calculation is done on scaled integers rather tham floating point values to
	// preserve accuracy for as long as possible.
	//
	// Hiving off this part of the calculation into a separate function eases unit
	// testing.

	return getScaledValue(wholeMillis, 29, fractionalMillis, 19, delta)
}

// GetScaledPhaseRange combines the components of the phase range from an MSM message
// and returns the result as a 41-bit scaled integer, 8 bits whole, 33 bits
// fractional.
func GetScaledPhaseRange(wholeMillis, fractionalMillis uint, delta int) uint64 {
	// This is similar to getAggregateRange, but the amounts shifted are different.
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

// pause sleeps for the time defined in the RTCM.
func (rtcm *RTCM) pause() {
	// if int(rtcm.WaitTimeOnEOF) == 0 {
	// 	// Looks like this rtcm wasn't created with New.
	// 	logEntry := fmt.Sprintf("pause: %d", defaultWaitTimeOnEOF)
	// 	rtcm.makeLogEntry(logEntry)
	// 	time.Sleep(defaultWaitTimeOnEOF)
	// } else {
	// 	logEntry := fmt.Sprintf("pause: %d", rtcm.WaitTimeOnEOF)
	// 	rtcm.makeLogEntry(logEntry)
	// 	time.Sleep(rtcm.WaitTimeOnEOF)
	// }
}

// makeLogEntry writes a string to the logger.  If the logger is nil
// it writes to the default system log.
func (rtcm *RTCM) makeLogEntry(s string) {
	if rtcm.logger == nil {
		log.Print(s)
	} else {
		rtcm.logger.Print(s)
	}

}

// MSM1 returns true if the message is an MSM type 1.
func MSM1(messageType int) bool {
	switch messageType {
	// GPS
	case 1071: // GPS
		return true
	case 1081: // Glonass
		return true
	case 1091: // Galileo
		return true
	case 1101: // SBAS
		return true
	case 1111: // QZSS
		return true
	case 1121: // Beidou
		return true
	case 1131: // NavIC/IRNSS
		return true
	default:
		return false
	}
}

// MSM2 returns true if the message is an MSM type 2.
func MSM2(messageType int) bool {
	switch messageType {
	// GPS
	case 1072: // GPS
		return true
	case 1082: // Glonass
		return true
	case 1092: // Galileo
		return true
	case 1102: // SBAS
		return true
	case 1112: // QZSS
		return true
	case 1122: // Beidou
		return true
	case 1132: // NavIC/IRNSS
		return true
	default:
		return false
	}
}

// MSM3 returns true if the message is an MSM type 3.
func MSM3(messageType int) bool {
	switch messageType {
	// GPS
	case 1073: // GPS
		return true
	case 1083: // Glonass
		return true
	case 1093: // Galileo
		return true
	case 1103: // SBAS
		return true
	case 1113: // QZSS
		return true
	case 1123: // Beidou
		return true
	case 1133: // NavIC/IRNSS
		return true
	default:
		return false
	}
}

// MSM4 returns true if the message is an MSM type 4.
func MSM4(messageType int) bool {
	switch messageType {
	// GPS
	case 1074: // GPS
		return true
	case 1084: // Glonass
		return true
	case 1094: // Galileo
		return true
	case 1104: // SBAS
		return true
	case 1114: // QZSS
		return true
	case 1124: // Beidou
		return true
	case 1134: // NavIC/IRNSS
		return true
	default:
		return false
	}
}

// MSM5 returns true if the message is an MSM type 5.
func MSM5(messageType int) bool {
	switch messageType {
	case 1075:
		return true
	case 1085:
		return true
	case 1095:
		return true
	case 1105:
		return true
	case 1115:
		return true
	case 1125:
		return true
	case 1135:
		return true
	default:
		return false
	}
}

// MSM6 returns true if the message is an MSM type 6.
func MSM6(messageType int) bool {
	switch messageType {
	case 1076:
		return true
	case 1086:
		return true
	case 1096:
		return true
	case 1106:
		return true
	case 1116:
		return true
	case 1126:
		return true
	case 1136:
		return true
	default:
		return false
	}
}

// MSM7 returns true if the message is an MSM type 7.
func MSM7(messageType int) bool {
	switch messageType {
	case 1077:
		return true
	case 1087:
		return true
	case 1097:
		return true
	case 1107:
		return true
	case 1117:
		return true
	case 1127:
		return true
	case 1137:
		return true
	default:
		return false
	}
}

// MSM returns true if the message type in the header is an Multiple Signal
// Message of any type.
func MSM(messageType int) bool {
	if messageType == NonRTCMMessage {
		return false
	}

	return MSM1(messageType) || MSM2(messageType) || MSM3(messageType) ||
		MSM4(messageType) || MSM5(messageType) || MSM6(messageType) || MSM7(messageType)
}

// getScaled5 takes a scaled integer a.b where b has 5 decimal places and
// returns the float value.
//
func scaled5ToFloat(scaled5 int64) float64 {
	return float64(scaled5) * 0.0001
}
