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

// The rtcm package contains the logic to decode and display RTCM3
// messages produced by GNSS devices such as the U-Blox ZED-F9P.  The
// format of the messages is described by RTCM STANDARD 10403.3
// Differential GNSS (Global Navigation Satellite Systems) Services –
// Version 3 (RTCM3 10403.3).  This is not an open-source standard and
// it costs about $300 to buy a copy.  RTCM messages are in a very
// compact binary form and not readable by eye.
//
// There are tools available to convert an RTCM3 data stream into messages
// in RINEX format.  That's an open standard and the result is readable by
// humans.  There is a little bit of useful information scattered around
// various public web pages.  To figure out the format of the message I'm
// interested in, I read what I could find, took the RTCM3 messages
// that my device produced, converted them to RINEX format and examined
// the result.  However, most of the information i needed came from the
// open source RTKLIB software, which is written in C.  I've copied some of
// the more useful RTKLIB source files into this repository as a reference.
//
// Some of my unit tests take the results that my software produces and
// compares it with the results that RINEX tools produced.
//
// I claim that this Go software is the most readable public source of
// detailed information about the RTCM3 format, limited though it is.
// Onc you've read an understood this, you should be able to follow
// the more complete RTKLIB implementation to decode any messages not
// handled here.
//
// An RTCM3 message is binary and variable length.  Each message frame
// is composed of a three-byte header, an embedded message and 3 bytes of
// Cyclic Redundancy Check (CRC) data.  The header starts with 0xd3 and
// includes the length of the embedded message.  Each message starts with
// a 12-bit message number which defines the type.  Apart from that
// message number, each type of message is in a different format.
// Fortunately, I only have to worry about two major types.
//
// For example, this is a hex dump of a complete message frame and the
// start of another:
//
// d3 00 aa 44 90 00 33 f6  ea e2 00 00 0c 50 00 10
// 08 00 00 00 20 01 00 00  3f aa aa b2 42 8a ea 68
// 00 00 07 65 ce 68 1b b4  c8 83 7c e6 11 30 10 3f
// 05 ff 4f fc e0 4f 61 68  59 b6 86 b5 1b a1 31 b9
// d9 71 55 57 07 a0 00 d3  2e 0c 99 01 98 c4 fa 16
// 0e fa 6e ac 07 19 7a 07  3a a4 fc 53 c4 fb ff 97
// 00 4c 6f f8 65 da 4e 61  e4 75 2c 4b 01 e5 21 0d
// 4f c0 0b 02 b0 b0 2f 0c  02 70 94 23 0b c3 e9 e0
// 97 d1 70 63 00 45 8d e9  71 d7 e5 eb 5f f8 78 00
// 00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00
// 00 00 00 00 00 00 00 00  00 00 00 00 00 4d f5 5a
// d3 00 6d 46 40 00 33 f6  10 22 00 00 02 40 08 16
//
// The message starts at byte zero.  That byte has the value d3, which
// announces the start of the message frame.  The frame is composed of a
// 3-byte header, and embedded message and 3 bytes of Cyclic Rdundancy
// Check (CRC) data.
//
// Byte 0 of the frame is always d3.  The top six bits of byte 1 are
// always zero.  The lower two bits and the bits of byte 2 form the
// message length, in this case hex aa, decimal 176.  So the embedded
// message is 176 bytes long and therefore the whole message frame is 182
// bytes long.  As shown above, the embedded message may end with some
// padding bits which are always zero.
//
// The last three bytes of the frame (in this case 4d, f5 and 5a) are the
// CRC value.  To check the CRC, take the header and the embedded message,
// run the CRC calculation over those bytes and compare the result with
// the given CRC.  If they are different then the message is not RTCM3 or
// it's been corrupted in transit.
//
// The CRC check is calculated using an algorithm from Qualcomm.  See Mark
// Rafter's implementation at https://github.com/goblimey/go-crc24q.
//
// The first 12 bits of the embedded message give the message number, in
// this case hex 449, decimal 1097, which is a type 7 Multiple Signal Message
// (MSM7) containing high resolution observations of signals from Galileo
// satellites.
//
// The messages are binary and can contain a d3 byte.  Note the one on the
// fifth line of the hex dump above.  This is not the start of another
// message.  One clue is that it's not followed by six zero bits.  To extract
// a message frame from a stream of data and decode it, you need to read the
// header and the next two bytes, check the header, find the message length,
// read the whole message frame and check the CRC value.  This matters
// particularly when you start to receive a stream of data from a device.  You
// may come into the data stream part-way through and blunder into a d3 byte.
// You can't assume that it's the start of a message.
//
// The CRC data is there to check that the message has not been corrupted in
// transit.  If the CRC check fails, the mesage must be discarded.
//
// RTCM3 message frames in the data stream are contiguous with no separators or
// newlines.  In the example, the last line contains the start of the next
// message.  Other data in other formats may be interspersed between frames.
// This software discards anything that's not an RTCM3 message frame with a
// correct CRC value.
//
// There are many hundreds of RTCM3 message types, some of which are just
// different ways of representing the same information.  To get an accurate fix
// on its position, a rover only needs to know the position of the base station
// and a recent set of the base's observations of satellite signals, which is to
// say a type 1005 message and a set of MSM7 messages, one for each constellation
// of satellites (GPS, GLONASS or whatever).
//
// Message type 1005 gives the position of the base station (or more strictly,
// of a point in space a few centimetres above its antenna).
//
// Message types 1074, 1077, 1084, 1087 and so on are Multiple Signal messages
// (MSMs).  Each contains observations by the base station of signals from
// satellites in one constellation.  Type 1077 is in MSM7 format and contains
// high resolution signal data from GPS satellites.  Type 1074 is in MSM4 format
// which is simply a lower resolution version of the same data.  Similarly for the
// other constellations: 1087 messages contain high resolution observations of
// GLONASS satellites, 1087 is for Galileo and 1127 is for Bediou.
//
// There are other constellations which are only visible in certain parts of
// the world, and not in the UK where I live.  I don't decode those messages
// either.  If I tried, I wouldn't have any real data with which to check the
// results.
//
// Each MSM contains readings for satellites in one constellation.  Each
// satellite in a constellation is numbered.  An MSM allows 64 satellites
// numbered 1-64.  At any point on the Earth's surface only some satellites will
// be visible.  Signals from some of those may be too weak to register, so the
// message will contain readings of just some signals from just some satellites.
// My base station typically sees one or two signals from each of 6-8 satellites
// from each of the four visible constellations in each scan, and produces four
// MSMs containing those results.
//
// An MSM message starts with a header, represented here by an MSMHeader
// structure.  Following the header is a set of cells listing the satellites
// for which signals were observed.  Those data is represented by a
// []MSM7SatelliteCell.  The message ends with a set of signal readings,
// at least one per satellite cell and currently no more than two.  Those
// data are represented by a [][]MSM7SignalCell, one outer slice per
// satellite - if seven satellites were observed, there will be seven sets
// of signal cells with one or two entries in each set.
//
// The header includes a satellite mask, a signal mask and a cell mask.  These
// bit masks show how to relate the cells that come after the header, to satellite
// and signal numbers.  For example, for each of the satellites observed, a bit is
// set in the 64-bit satellite mask, bit 63 for satellite 1, bit 0 for satellite 64.
// If the satellite mask is:
//     0101100000000010000101000000010000000000000000000000000000
// seven bits are set in the mask so there will be seven satellite cells containing
// data for satellites, 2, 4, 5 and so on.
//
// If the 32-bit signal mask is:
//     1000000000001000000000000000000
// then the device observed signal types 1 and 13 from some of the satellites.
// The standard supports up to 32 types of signal.  Each signal can be on a different
// frequency, although some signals share the same frequency.  The meaning of each signal
// type and the frequency that it is broadcast on is defined for each constellation.
//
// The cell mask is variable length, nSignals * nSatellites bits, where nSignals
// is the number of signals (2 in the example) and nSatellites is the number of
// satellites (7 in the example).  The cell mask is an array of bits with nsatellite
// elements of nSignals each - in this example 7X2 = 14 bits long, showing which signals
// were observed for each satellite. For example:
//     01 11 11 10 10 10 10
// means that signal 1 from satellite 2 was not observed but signal 13 was not,
// both signals were observed from satellite 4, and so on.
//
// The header is followed by the satellites cell list.  It's m X 36 bits long where m is
// the number of satellite from which signals were observed.  However, that's NOT 36 bits
// for the first satellite followed by 36 bits for the next, and so on.  Instead, the bit
// stream is divided into fields, arranged as m millisecond readings, each 8 bits, m
// extended data values, each 4 bits, and so on.
//
// The signal list is laid out in the same way.  It's an array s X 80 bits where
// s is the total number of signals arranged by satellite.  For example, if
// signal 1 was observed from satellite 1, signals 1 and 3 from satellite 3 and
// signal 3 from satellite 5, there will be four signal cells, once again
// arranged as s fields of one type followed by s fields of another type and so on.
//
// The signal list is followed by any padding necessary to fill the last byte,
// possibly followed by a few zero bytes of more padding.  Finally comes the 3-byte
// CRC value.  The next message frame starts immediately after.
//
// My base station is driven by a UBlox ZED-F9P device, which operates in a fairly
// typical way.  It scans for signals from satellites and sends messages at intervals
// of a multiple of one second.  The useful life of an MSM message is short, so you
// might configure the device to scan and send a batch of observations once per second.
// For type 1005 messages, which give the position of the device, the situation is
// different.  When a rover connects to a base station and starts to receive messages,
// it needs a type 1005 (base station position) message to make sense of the MSM (signal
// observation) messages.  The base station doesn't move, so the rover only needs the
// first the 1005 message.  A good compromise is to configure the device to send one
// type 1005 message every ten seconds.  That reduces the traffic a little while ensuring
// that when a rover connects to the data stream it will start to produce position fixes
// reasonably quickly.

const u0 uint64 = 0            // 0 as a uint.
const minLengthMSMHeader = 170 // The minimum length of an MSM header
const satelliteMaskLength = 64 // The mask is 64 bits
const signalMaskLength = 32    // The mask is 32 bits.
const headerLengthBytes = 3
const crcLengthBytes = 3

// HeaderLengthBits is the length of the RTCM header in bits.
const HeaderLengthBits = headerLengthBytes * 8

// CRCLengthBits is the length of the Cyclic Redundancy Check value in bits.
const CRCLengthBits = crcLengthBytes * 8

// StartOfMessageFrame is the value of the byte that starts an RTCM3 message frame.
const StartOfMessageFrame byte = 0xd3

// NonRTCMMessage indicates a Message that does contain RTCM data.  Typically
// it will be a stream of data in other formats (NMEA, UBX etc).
// According to the spec, message numbers start at 1, but I've
// seen messages of type 0.
const NonRTCMMessage = -1

// rangeMillisWholeInvalidValue in the satellite's RangeWhole value
// indicates that the value is invalid.
const rangeMillisWholeInvalidValue = 0xff

// phaseRangeRateInvalidValue in the satellite phase range rate
// indicates that the value is invalid.
const phaseRangeRateInvalidValue = 0x2000

// msm4SignalRangeDeltaInvalidValue in the signal's pseudorange delta
// indicates that the value is invalid.
const msm4SignalRangeDeltaInvalidValue = 0x4000

// msm4PhaseRangeDeltaInvalidValue in the MSM4 signal fine pseudorange
// indicates that the value is invalid.
const msm4PhaseRangeDeltaInvalidValue = 0x200000

// msmSignalCMRInvalidValue in any MSM signal's CNR value indicates that
// the reading is invalid.
const msmSignalCMRInvalidValue = 0

// msm7SignalRangeDeltaInvalidValue in an MSM7 signal's range delta indicates
// that the value is invalid.
const msm7SignalRangeDeltaInvalidValue = 0x80000

// msm4SignalPhaseRangeDeltaInvalidValue in the MSM4 signal phase range
// delta indicates that the value is invalid.
const msm4SignalPhaseRangeDeltaInvalidValue = 0x200000

// msm7SignalPhaseRangeDeltaInvalidValue in the MSM7 signal phase range
// delta indicates that the value is invalid.
const msm7SignalPhaseRangeDeltaInvalidValue = 0x800000

// msm7PhaseRangeRateDeltaInvalidValue in the MSM7 signal's phase range
// rate delta value indicates that the reading is invalid.
const msm7PhaseRangeRateDeltaInvalidValue = 0x4000

const glonassCodeBiasInvalidValue = 0x8000

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

// CLight is the speed of light in metres per second
const CLight = 299792458.0

// oneLightMillisecond is the distance in metres traveled by light in one millisecond.
// It's used to convert a range in milliseconds to a distance in metres.
const oneLightMillisecond = (CLight * 0.001)

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

// dateLayout defines the layout of dates when they
// are displayed - "yyyy-mm-dd hh:mm:ss.ms timeshift timezone"
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

// pushBackData stores a character that has been pushed back.
// See the getChar and Pushback methods.
type pushedBackData struct {
	pushed bool // a byte has been pushed back
	b      byte // the byte that was pushed back
}

// RTCM is the object used to fetch and analyse RTCM3 messages.
type RTCM struct {

	// bufferedreader is connected to the source of the messages,
	// set up on the first call of getChar.
	bufferedReader *bufio.Reader

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
	// MessageType - uint12 - always 1005.
	MessageType uint

	// station ID - uint12.
	StationID uint

	// Reserved for ITRF Realisaton Year - uint6.
	ITRFRealisationYear uint

	// GPS bit - bit(1).  True if the device is configured to observe GPS satellites.
	GPS bool

	// Glonass bit - bit(1).  True if the device is configured to observe Glonass satellites.
	Glonass bool

	// Galileo bit - bit(1).  True if the device is configured to observe Galileo satellites.
	Galileo bool

	// Reference-Station Indicator - bit(1)
	ReferenceStation bool

	// Single Receiver Oscillator Indicator - 1 bit
	Oscilator bool

	// Quarter Cycle Indicator - 2 bits
	QuarterCycle uint

	// Reserved - 1 bit.
	ReservedBit bool

	// Antenna Reference Point coordinates in ECEF - int38.
	// Scaled integers in 0.00001 m units (tenth mm).
	AntennaRefX int64
	AntennaRefY int64
	AntennaRefZ int64
}

// Message1230 contains a message of type 1230 - GLONASS code-phase
// biases.  It's used to correct an associated GLONASS MSM message.
type Message1230 struct {
	// MessageType - uint12 - always 1230.
	MessageType uint
	// Station ID - uint12.
	StationID uint
	// Valid - Code-phase bias indicator - true if the values here
	// should be used.
	Aligned bool
	// SignalMask - 4-bit list of the signals included.
	Signalmask uint

	// L1_C_A_Bias_supplied - the L1 C/A Bias value is supplied.
	L1_C_A_Bias_supplied bool
	// L1_C_A_Bias_valid - the L1 C/A Bias value is valid.
	L1_C_A_Bias_valid bool
	// L1_C_A_Bias - L1 C/A Bias.
	L1_C_A_Bias int

	// L1_P_Bias_supplied - the L1 P Bias value is supplied.
	L1_P_Bias_supplied bool
	// L1_P_Bias_valid - the L1 P Bias value is valid.
	L1_P_Bias_valid bool
	// L1_P_Bias - L1 P Bias.
	L1_P_Bias int

	// L2_C_A_Bias_supplied - the L C/A Bias value is supplied.
	L2_C_A_Bias_supplied bool
	// L2_C_A_Bias_valid - the L2 C/A Bias value is valid.
	L2_C_A_Bias_valid bool
	// L2_C_A_Bias - L2 P Bias.
	L2_C_A_Bias int

	// L2_P_Bias_supplied - the L2 P Bias value is supplied.
	L2_P_Bias_supplied bool
	// L2_p_Bias_valid - the L2 P Bias value is valid.
	L2_P_Bias_valid bool
	// L2_P_Bias - L2 P Bias.
	L2_P_Bias int
}

// MSMHeader holds the header for MSM Messages.  Message types 1074,
// 1077, 1084, 1087 etc have an MSM header at the start.
type MSMHeader struct {

	// MessageType - uint12 - one of 1074, 1077 etc.
	MessageType uint

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

	// SequenceNumber - uint3.
	SequenceNumber uint

	// SessionTransmissionTime - uint7.
	SessionTransmissionTime uint

	// ClockSteeringIndicator - uint2.
	ClockSteeringIndicator uint

	// ExternalClockIndicator - uint2.
	ExternalClockIndicator uint

	// GNSSDivergenceFreeSmoothingIndicator - bit(1).
	GNSSDivergenceFreeSmoothingIndicator bool

	// GNSSSmoothingInterval - bit(3).
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
	// be arranged like so: {{1,0,1,1}, {1,1,0,0}}, meaning that signals 1, 3 and 5
	// were observed from satellite 1 and signals 1 and 2 were observed from
	// satellite 3.
	CellMask [][]bool

	// NumSignalCells is the total number of signal cells in the message.  Creating
	// this count avoids having to rummage through the masks when you need to know
	// its value.
	NumSignalCells int
}

// MSMSatelliteCell holds the MSM7 data for one satellite.
type MSMSatelliteCell struct {
	// SatelliteID is the satellite ID, 1-64.
	SatelliteID uint

	// RangeWholeMillis - uint8 - the number of integer milliseconds
	// in the GNSS Satellite range (ie the transit time of the signals).  0xff
	// indicates an invalid value.  See also the fraction value here and the
	// delta value in the signal cell.
	RangeWholeMillis uint

	// RangeValid is true when the RangeMillisWhole valid.
	RangeValid bool

	// ExtendedInfo - uint4.  Extended Satellite Information.
	ExtendedInfo uint

	// RangeFractionalMillis - unit10.  The fractional part of the range
	// in milliseconds.
	RangeFractionalMillis uint

	// PhaseRangeRate - int14 - GNSS Satellite phase range rate.  See also
	// the PhaseRangeRateDelta and PhaseRangeRateMS in the signal data.
	PhaseRangeRate int

	// PhaseRangeRateValid is true when the PhaseRangeRate value is valid.
	PhaseRangeRateValid bool
}

// MSMSignalCell holds the data for one signal from one satellite, plus
// values gathered from the satellite and signal data and merged together.
type MSMSignalCell struct {
	// Satellite is the index into the satellite cell list of the satellite
	// that was observed.
	Satellite int

	// SignalID is the ID of the signal that was observed from the satellite.
	// Each signalID has an associated frequency.
	SignalID uint

	// RangeDelta - int20 in an MSM7 message (0x80000 indicates an invalid value),
	// uint15 in an MSM4 message (0x4000 indicates an invalid value).  Merged with the
	// range values from the satellite cell to give RangeMetres.
	RangeDelta int

	// RangeDeltaValid set false indicates that at least one of the range values is invalid.
	RangeDeltaValid bool

	// PhaseRangeDelta - int24 - to be merged with the range values and
	// converted to cycles to create PhaseRangeCycles.  0x800000 indicates an
	// invalid value.
	PhaseRangeDelta int

	// PhaseRangeDeltaValid is true if the PhaseRangeDelta is valid.
	PhaseRangeDeltaValid bool

	// LockTimeIndicator - uint10.
	LockTimeIndicator uint

	// HalfCycleAmbiguity flag - 1 bit.
	HalfCycleAmbiguity bool

	// CNR - uint10 - Carrier to Noise Ratio.  0 means invalid value.
	CNR uint

	// CNRValid indicates that the CNR value is valid.
	CNRValid bool

	// PhaseRangeRateDelta - int15 - the offset to the satellite's phase
	// range rate.  This value is in ten thousands of a millisecond.
	PhaseRangeRateDelta int

	// PhaseRangeRateDeltaValid is true if the phase range rate delta value is valid.
	PhaseRangeRateDeltaValid bool

	// RangeMetres is the range derived from the satellites range values and the
	// signal's range delta, expressed in metres.
	RangeMetres float64

	// PhaseRangeCycles is derived from the satellites range values and the
	// signal's delta value, expressed in cycles.
	PhaseRangeCycles float64

	// PhaseRangeRateMS is derived from the satellite's phase range rate and the
	// signal's delta value and expressed as metres per second.  (This is the
	// speed at which the satellite is approaching the base station, or if
	// negative, the speed that it's moving away from it.)
	PhaseRangeRateMS float64
}

// MSMMessage is a broken-out version of an MSM4 or MSM7 message.
type MSMMessage struct {
	// Header is the MSM Header
	Header *MSMHeader

	// Satellites is a list of the satellites for which signals
	// were observed.
	Satellites []MSMSatelliteCell

	// Signals is a list of sublists, one sublist per satellite,
	// of signals at different frequencies observed by the base
	// station from the satellites in the Satellite list. If
	// there are, say, eight items in the Satellites list, there
	// will be eight sublists here.  RTCM3 allows for up to 32
	// signals but currently (2021) each satellite only uses two
	// signal channels, so there should be either one or two
	// signals in each sublist.
	Signals [][]MSMSignalCell
}

// Message contains an RTCM3 message, possibly broken out into readable form,
// or a stream of non-RTCM data.  Message type NonRTCMMessage indicates the
// second case.
type Message struct {
	// MessageType is the type of the RTCM message (the message number).
	// Type NonRTCMMessage contains a stream of bytes that doesn't contain an
	// RTCM message.
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

// Readable returns a broken out version of the message - if the
// message type is 1005, it's a Message1005, if it's an MSM7 message,
// it's an MSM7Message.
func (m *Message) Readable(r *RTCM) interface{} {
	if m.readable == nil {
		r.Analyse(m)
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
	expectedFrameLength := messageLength + headerLengthBytes + crcLengthBytes
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
func (r *RTCM) Analyse(message *Message) {
	var readable interface{}

	if MSM(message.MessageType) {
		readable = r.GetMSMMessage(message)
	} else {
		switch message.MessageType {
		case 1005:
			readable, _ = r.GetMessage1005(message.RawData)
		case 1230:
			readable, _ = r.GetMessage1230(message.RawData)
		case 4072:
			readable = "(Message type 4072 is in an unpublished format defined by U-Blox.)"
		default:
			readable = fmt.Sprintf("message type %d currently cannot be displayed",
				message.MessageType)
		}
	}

	message.SetReadable(readable)
}

// GetMessageLengthAndType extracts the message length and the message type from an
// RTCMs message frame or returns an error, implying that this is not the start of a
// valid message.  The data must be at least 5 bytes long.
func (rtcm *RTCM) GetMessageLengthAndType(data []byte) (uint, int, error) {

	if len(data) < headerLengthBytes+2 {
		return 0, NonRTCMMessage, errors.New("the message is too short to get the header and the length")
	}

	// The message header is 24 bits.  The top byte is startOfMessage.
	if data[0] != StartOfMessageFrame {
		message := fmt.Sprintf("message starts with 0x%0x not 0xd3", data[0])
		return 0, NonRTCMMessage, errors.New(message)
	}

	// The next six bits must be zero.  If not, we've just come across
	// a 0xd3 byte in a stream of binary data.
	sanityCheck := Getbitu(data, 8, 6)
	if sanityCheck != 0 {
		errorMessage := fmt.Sprintf("bits 8 -13 of header are %d, must be 0", sanityCheck)
		return 0, NonRTCMMessage, errors.New(errorMessage)
	}

	// The bottom ten bits of the header is the message length.
	length := uint(Getbitu(data, 14, 10))

	// The 12-bit message type follows the header.
	messageType := int(Getbitu(data, 24, 12))

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
		case n < headerLengthBytes+2:
			continue

		case n == headerLengthBytes+2:
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
			expectedFrameLength = messageLength + headerLengthBytes + crcLengthBytes
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
func (rtcm *RTCM) GetMessage1005(m []byte) (*Message1005, uint) {
	var result Message1005
	// Pos is the position within the bitstream.
	var pos uint = HeaderLengthBits

	result.MessageType = uint(Getbitu(m, pos, 12))
	pos += 12
	result.StationID = uint(Getbitu(m, pos, 12))
	pos += 12
	result.ITRFRealisationYear = uint(Getbitu(m, pos, 6))
	pos += 6
	result.GPS = (Getbitu(m, pos, 1) == 1)
	pos++
	result.Glonass = (Getbitu(m, pos, 1) == 1)
	pos++
	result.Galileo = (Getbitu(m, pos, 1) == 1)
	pos++
	result.ReferenceStation = (Getbitu(m, pos, 1) == 1)
	pos++
	result.AntennaRefX = Getbits(m, pos, 38)
	pos += 38
	result.Oscilator = (Getbitu(m, pos, 1) == 1)
	pos++
	result.ReservedBit = (Getbitu(m, pos, 1) == 1)
	pos++
	result.AntennaRefY = Getbits(m, pos, 38)
	pos += 38
	result.QuarterCycle = uint(Getbitu(m, pos, 2))
	pos += 2
	result.AntennaRefZ = Getbits(m, pos, 38)
	pos += 38

	return &result, pos
}

// GetMessage1230 returns a text version of a message type 1230
// GLONASS code-phase biases
func (rtcm *RTCM) GetMessage1230(m []byte) (*Message1230, uint) {
	const codeBiasValueLength = 16
	var result Message1230
	// Pos is the position within the bitstream.  It starts just beyond
	// the 24-bit frame header.
	var pos uint = HeaderLengthBits

	// Pos should never get beyond the end of the message within the frame.
	messageEnd := uint((len(m) * 8) - CRCLengthBits)

	result.MessageType = uint(Getbitu(m, pos, 12))
	pos += 12
	result.StationID = uint(Getbitu(m, pos, 12))
	pos += 12
	result.Aligned = (Getbitu(m, pos, 1) == 1)
	pos++

	pos += 3 // 3 bits reserved

	// The signal mask is 4 bits.  The bits are:
	// MSB 0 the L1 C/A code-phase bias is supplied
	//     1 the L1 P code-phase bias is supplied
	//     2 the L2 C/W code-phase bias is supplied
	// LSB 3 the L2 P code-phase bias is supplied

	// Get the whole mask.  Don't advance the position.
	result.Signalmask = uint(Getbitu(m, pos, 4))

	// Get the bits from the mask, advancing the position
	// and counting the number of set bits.
	var bitsSetInMask uint64 = 0
	var maskBit [4]uint64
	for i, _ := range maskBit {
		maskBit[i] = Getbitu(m, pos, 1)
		pos++
		bitsSetInMask += maskBit[i]
	}

	result.L1_C_A_Bias_supplied = (maskBit[0] == 1)
	result.L1_P_Bias_supplied = (maskBit[1] == 1)
	result.L2_C_A_Bias_supplied = (maskBit[2] == 1)
	result.L2_P_Bias_supplied = (maskBit[3] == 1)

	// If there are any bias values they come next, one 16-bit number
	// for each bit set in the mask.  The number may be the special
	// invalid value.  If a bias value is not given it's assumed to be
	// valid and zero.

	var biasValue [4]int
	var biasValueValid [4]bool

	// The default setting for the valid field is true.
	for i, _ := range biasValueValid {
		biasValueValid[i] = true
	}

	if bitsSetInMask > 0 {

		// There are some bias values.

		// sanity check
		if pos+codeBiasValueLength >= messageEnd {
			logEntry := fmt.Sprintf("GetMessage1230: warning - overrun - pos %d messageEnd %d", pos, messageEnd)
			rtcm.makeLogEntry(logEntry)
		}

		// Get the bias values, but avoid overrunning the message.
		for i := range biasValue {
			if maskBit[i] == 1 &&
				(pos+codeBiasValueLength) <= messageEnd {

				bits := Getbitu(m, pos, 16)
				if bits == glonassCodeBiasInvalidValue {
					biasValueValid[i] = false
					biasValue[i] = 0
				} else {
					biasValueValid[i] = true
					biasValue[i] = int(Getbits(m, pos, 16))
				}
				pos += codeBiasValueLength
			}
		}
	}

	result.L1_C_A_Bias_valid = biasValueValid[0]
	result.L1_C_A_Bias = biasValue[0]

	result.L1_P_Bias_valid = biasValueValid[1]
	result.L1_P_Bias = biasValue[1]

	result.L2_C_A_Bias_valid = biasValueValid[2]
	result.L2_C_A_Bias = biasValue[2]

	result.L2_P_Bias_valid = biasValueValid[3]
	result.L2_P_Bias = biasValue[3]

	return &result, pos
}

// GetMSMHeader extracts the header from an MSM message.  It returns
// the MSMHeader and the bit position of the start of the satellite
// data (which comes next in the bit stream).
//
func (rtcm *RTCM) GetMSMHeader(m []byte) (header *MSMHeader, startOfSatelliteData uint) {
	var h MSMHeader
	// Pos is the position within the bitstream.
	// Skip over the header.
	var pos uint = HeaderLengthBits
	h.MessageType = uint(Getbitu(m, pos, 12))
	pos += 12
	switch h.MessageType {
	case 1077:
		h.Constellation = "GPS"
	case 1087:
		h.Constellation = "GLONASS"
	case 1097:
		h.Constellation = "Galileo"
	case 1127:
		h.Constellation = "BeiDou"
	}
	h.StationID = uint(Getbitu(m, pos, 12))
	pos += 12
	h.EpochTime = uint(Getbitu(m, pos, 30))
	pos += 30
	// The epoch time in the message is milliseconds since the start of the
	// constellation's epoch.  UTCTime is the same time in UTC.
	switch h.MessageType {
	case 1077:
		//GPS.
		h.UTCTime = rtcm.GetUTCFromGPSTime(h.EpochTime)
		h.Constellation = "GPS"

	case 1087:
		// Glonass.
		h.UTCTime = rtcm.GetUTCFromGlonassTime(h.EpochTime)
		h.Constellation = "GLONASS"

	case 1097:
		// Galileo (which actually uses GPS time).
		h.UTCTime = rtcm.GetUTCFromGalileoTime(h.EpochTime)
		h.Constellation = "Galileo"

	case 1127:
		// Beidou.
		h.UTCTime = rtcm.GetUTCFromBeidouTime(h.EpochTime)
		h.Constellation = "BeiDou"
	default:
		h.Constellation = "Unknown"
	}

	h.MultipleMessage = (Getbitu(m, pos, 1) == 1)
	pos++
	h.SequenceNumber = uint(Getbitu(m, pos, 3))
	pos += 3
	h.SessionTransmissionTime = uint(Getbitu(m, pos, 7))
	pos += 7
	h.ClockSteeringIndicator = uint(Getbitu(m, pos, 2))
	pos += 2
	h.ExternalClockIndicator = uint(Getbitu(m, pos, 2))
	pos += 2
	h.GNSSDivergenceFreeSmoothingIndicator =
		(Getbitu(m, pos, 1) == 1)
	pos++
	h.GNSSSmoothingInterval = uint(Getbitu(m, pos, 3))
	pos += 3

	h.SatelliteMaskBits = uint64(Getbitu(m, pos, satelliteMaskLength))
	// Bit 63 of the mask is satellite number 1, bit 62 is 2,
	// bit 0 is 64.
	h.Satellites = make([]uint, 0)
	for satNum := 1; satNum <= satelliteMaskLength; satNum++ {
		if Getbitu(m, pos, 1) == 1 {
			h.Satellites = append(h.Satellites, uint(satNum))
		}
		pos++
	}

	h.SignalMaskBits = uint32(Getbitu(m, pos, signalMaskLength))

	// Bit 31 of the mask is signal number 1, bit 30 is 2,
	// bit 0 is 32.
	h.Signals = make([]uint, 0)
	for sigNum := 1; sigNum <= signalMaskLength; sigNum++ {
		if Getbitu(m, pos, 1) == 1 {
			h.Signals = append(h.Signals, uint(sigNum))
		}
		pos++
	}

	// The cell mask is variable length but <= 64.
	cellMaskLength := uint(len(h.Satellites) * len(h.Signals))
	if cellMaskLength > 64 {
		logEntry := fmt.Sprintf("GetMSMHeader: cellMask is %d bits - expected <= 64",
			cellMaskLength)
		rtcm.makeLogEntry(logEntry)
		return nil, 0
	}
	h.CellMaskBits = uint64(Getbitu(m, pos, cellMaskLength))

	h.CellMask = make([][]bool, len(h.Satellites), len(h.Satellites))
	for i := 0; i < len(h.Satellites); i++ {
		h.CellMask[i] = make([]bool, len(h.Signals), len(h.Signals))
		for j := 0; j < len(h.Signals); j++ {
			h.CellMask[i][j] = (uint32(Getbitu(m, pos, 1)) == 1)
			pos++
			if h.CellMask[i][j] {
				h.NumSignalCells++
			}
		}
	}

	return &h, pos
}

// GetMSM4SatelliteCells extracts the satellite cell data from an MSM4 message.
// It returns a slice of cell data and the number of bits of the message
// bitstream consumed so far (which is the index of the start of the signal
// cells).
//
func (rtcm *RTCM) GetMSM4SatelliteCells(m []byte, h *MSMHeader, startOfSatelliteData uint) ([]MSMSatelliteCell, uint) {
	// The second part of the MSM message is the satellite data.  If signals were
	// observed from satellites 2, 3 and 15, there will be three sets of data fields.
	// The bit stream contains all of the rough range values, followed by the range
	// delta values.  The rough values and delta values are merged together later.
	// It's more convenient to present these as an array of fields, one for each
	// satellite from which signals were observed.
	satData := make([]MSMSatelliteCell, len(h.Satellites))

	// Define the lengths of each field.
	const lenRangeMillisWhole = 8
	const lenRangeMillisFractional = 10

	// Set the satellite ids.  If the satellite list in the header (h.Satellites)
	// contains {2, 3, 15} then we have observed satellites with IDs 2, 3 and 15.
	//
	for i := range satData {
		satData[i].SatelliteID = h.Satellites[i]
	}

	// Set all of the valid values - they may be unset later.
	for i := range satData {
		satData[i].RangeValid = true
		satData[i].PhaseRangeRateValid = true
	}

	// Get the rough range values (whole milliseconds).  Watch out for the invalid value.
	pos := startOfSatelliteData
	for i := 0; i < int(len(h.Satellites)); i++ {
		v := uint(Getbitu(m, pos, lenRangeMillisWhole))
		if v == rangeMillisWholeInvalidValue {
			satData[i].RangeValid = false
		} else {
			satData[i].RangeWholeMillis = v
		}

		pos += lenRangeMillisWhole
	}

	// Get the range delta values (fractions of a millisecond).
	for i := 0; i < int(len(h.Satellites)); i++ {
		if satData[i].RangeValid {
			satData[i].RangeFractionalMillis =
				uint(Getbitu(m, pos, lenRangeMillisFractional))
		}
		pos += lenRangeMillisFractional
	}

	startOfSignalData := pos

	return satData, startOfSignalData
}

// GetMSM7SatelliteCells extracts the satellite cell data from an MSM4 message.
// It returns a slice of cell data and the number of bits of the message
// bitstream consumed so far (which is the index of the start of the signal
// cells).
//
func (rtcm *RTCM) GetMSM7SatelliteCells(m []byte, h *MSMHeader, startOfSatelliteData uint) ([]MSMSatelliteCell, uint) {
	// The second part of the MSM message is the satellite data.  If signals were
	// observed from satellites 2, 3 and 15, there will be three sets of data fields.
	// Compared with the satellite data in an MSM4 message, there are some extra fields.
	// The bit stream contains:  all of the rough range values, followed by the extended
	// satellite data values, the range delta values, then the rough phase range rate
	// values (the deltas for which are in the signal data).  The rough values and the
	// delta values are merged together later.
	//
	// It's more convenient to present the result as a slice of MSMSatelliteCell objects,
	// one for each observed satellite.
	//
	satData := make([]MSMSatelliteCell, len(h.Satellites))

	// Define the lengths of each field.
	const lenRangeMillisWhole = 8
	const lenExtendedInfo = 4
	const lenRangeMillisFractional = 10
	const lenPhaseRangeRate = 14

	// Set the satellite ids.  If the satellite list in the header (h.Satellites)
	// contains {2, 3, 15} then we have observed satellites with IDs 2, 3 and 15.
	//
	for i := range satData {
		satData[i].SatelliteID = h.Satellites[i]
	}

	// Set all of the valid values - they may be unset later.
	for i := range satData {
		satData[i].RangeValid = true
		satData[i].PhaseRangeRateValid = true
	}

	// Get the rough range values (whole milliseconds).  Watch out for the invalid value.
	pos := startOfSatelliteData
	for i := 0; i < int(len(h.Satellites)); i++ {
		v := uint(Getbitu(m, pos, lenRangeMillisWhole))
		if v == rangeMillisWholeInvalidValue {
			satData[i].RangeValid = false
		} else {
			satData[i].RangeWholeMillis = v
		}

		pos += lenRangeMillisWhole
	}

	// Get the extended satellite information.
	for i := 0; i < int(len(h.Satellites)); i++ {
		satData[i].ExtendedInfo = uint(Getbitu(m, pos, lenExtendedInfo))
		pos += lenExtendedInfo
	}

	// Get the range delta values (fractions of a millisecond)
	for i := 0; i < int(len(h.Satellites)); i++ {
		satData[i].RangeFractionalMillis = uint(Getbitu(m, pos, lenRangeMillisFractional))
		pos += lenRangeMillisFractional
	}

	// Get the rough phase range rate values.  Watch out for the invalid value.
	for i := 0; i < int(len(h.Satellites)); i++ {
		if uint(Getbits(m, pos, lenPhaseRangeRate)) == phaseRangeRateInvalidValue {
			satData[i].PhaseRangeRateValid = false
		} else {
			satData[i].PhaseRangeRate =
				int(Getbits(m, pos, lenPhaseRangeRate))
		}
		pos += lenPhaseRangeRate
	}

	startOfSignalData := pos

	return satData, startOfSignalData
}

// GetMSM4SignalCells gets the raw signal data.  The returned object contains the raw data
// for each signal collected together.
func (rtcm *RTCM) GetMSM4SignalCells(m []byte, h *MSMHeader, satData []MSMSatelliteCell, startOfSignaldata uint) [][]MSMSignalCell {
	// The third part of the message bit stream is the signal data.  Each satellite can
	// send many signals, each on a different frequency.  For example, if we observe one
	// signal from satellite 2, two from satellite 3 and 2 from satellite 15, there will
	// be five sets of signal data.  It's composed of the pseudo range delta values for
	// all five signals, followed by all of the phase range delta values, the phase range
	// lock time indicator values, the half-cycle ambiguity indicator values, then
	// the GNSS signal CNR values.  It's more convenient to present these as a slice
	// of slices of fields, one outer slice for each satellite and one inner slice for
	// each observed signal.  The raw data in the fields are merged together later.

	var signalCell [][]MSMSignalCell
	sigData := make([]MSMSignalCell, h.NumSignalCells)
	lengthInBits := len(m) * 8

	// Define the lengths of the fields
	const lenRangeDelta = 15
	const lenPhaseRangeDelta = 22
	const lenLockTimeIndicator = 4
	const lenHalfCycleAmbiguity = 1
	const lenCNR = 6
	const lenPhaseRangeRateDelta = 15

	// Pos is the position within the bitstream.
	pos := startOfSignaldata

	// Check for overrun.
	lastPosition := int(pos) + h.NumSignalCells*
		int(lenRangeDelta+lenPhaseRangeDelta+lenLockTimeIndicator+lenHalfCycleAmbiguity+lenCNR)

	if lastPosition >= lengthInBits {
		logEntry := fmt.Sprintf("GetMSM4SignalCells: overrun %d %d", lastPosition, lengthInBits)
		rtcm.makeLogEntry(logEntry)
		return signalCell
	}

	// Set all of the valid values - they may be unset later.
	for i := range sigData {
		sigData[i].CNRValid = true
		sigData[i].PhaseRangeDeltaValid = true
		sigData[i].PhaseRangeRateDeltaValid = true
		sigData[i].RangeDeltaValid = true
	}

	// Get the pseudorange delta values.  Watch out for the invalid value.
	for i := 0; i < h.NumSignalCells; i++ {
		if uint(Getbits(m, pos, lenRangeDelta)) == msm4SignalRangeDeltaInvalidValue {
			sigData[i].RangeDeltaValid = false
		} else {
			sigData[i].RangeDelta = int(Getbits(m, pos, lenRangeDelta))
		}
		pos += lenRangeDelta
	}

	// Get the phase range delta values.  Watch out for the invalid value.
	for i := 0; i < h.NumSignalCells; i++ {
		if uint(Getbits(m, pos, lenPhaseRangeDelta)) == msm4SignalPhaseRangeDeltaInvalidValue {
			sigData[i].PhaseRangeDeltaValid = false
		} else {
			sigData[i].PhaseRangeDelta = int(Getbits(m, pos, lenPhaseRangeDelta))
		}
		pos += lenPhaseRangeDelta
	}

	// Get the phase range lock time indicator.
	for i := 0; i < h.NumSignalCells; i++ {
		sigData[i].LockTimeIndicator = uint(Getbitu(m, pos, lenLockTimeIndicator))
		pos += lenLockTimeIndicator
	}

	// Get the half-cycle ambiguity indicator bits.
	for i := 0; i < h.NumSignalCells; i++ {
		sigData[i].HalfCycleAmbiguity = (Getbitu(m, pos, lenHalfCycleAmbiguity) == 1)
		pos += lenHalfCycleAmbiguity
	}

	// Get the CNR values.  Watch out for the invalid value.
	for i := 0; i < h.NumSignalCells; i++ {
		v := uint(Getbitu(m, pos, lenCNR))
		if v == msmSignalCMRInvalidValue {
			sigData[i].CNR = v
		} else {
			sigData[i].CNRValid = false
		}
		pos += lenCNR
	}

	// Get the phase range rate delta values.  Watch out for the invalid value.
	for i := 0; i < h.NumSignalCells; i++ {
		v := uint(Getbits(m, pos, lenPhaseRangeRateDelta))
		if v == msm7PhaseRangeRateDeltaInvalidValue {
			sigData[i].PhaseRangeRateDeltaValid = false
		} else {
			sigData[i].PhaseRangeRateDelta = int(Getbits(m, pos, lenPhaseRangeRateDelta))
		}
		pos += lenPhaseRangeRateDelta
	}

	// Set the satellite and signal numbers by cranking through the cell mask.

	cell := 0
	for i := range h.CellMask {
		var cellSlice []MSMSignalCell
		for j := range h.CellMask[i] {
			if h.CellMask[i][j] {
				sigData[cell].Satellite = i
				sigData[cell].SignalID = h.Signals[j]
				cellSlice = append(cellSlice, sigData[cell])
				cell++
			}
		}
		signalCell = append(signalCell, cellSlice)
	}
	// Set the combined values.  For example the range in metres is derived from
	// the range values in the satellite cell and the range delta value in the
	// signal cell.
	for i := range signalCell {
		for j := range signalCell[i] {
			satelliteID := signalCell[i][j].Satellite
			satellite := &satData[satelliteID]
			signal := &signalCell[i][j]
			signal.RangeMetres = getMSMRangeInMetres(h, satellite, signal)
			var err error
			signal.PhaseRangeCycles, err = getMSMPhaseRange(h, satellite, signal)
			if err != nil {
				logEntry := fmt.Sprintf("error getting phase range - %s\n", err.Error())
				rtcm.makeLogEntry(logEntry)
			}
		}
	}

	return signalCell
}

// GetMSM7SignalCells extracts the signal cells from an MSM message
// starting at bit (startOfSignaldata).  It returns the signal cells
// as a slice of slices of MSM7SignalCell objects, one outer slice
// element for each satellite and one inner slice element for each
// signal from that satellite.
//
// For example, If signal 5 from satellite 1 was observed and signals
// 5 and 13 from satellite 6 were observed, element 0 of the outer
// slice will contain a slice with one element (for satellite 1,
// signal 5) and element 1 will contain a slice with two elements,
// representing the signals from satellite 6.  To make this setup
// easier to use, each MSM7SignalCell contains a satellite ID and a
// signal ID along with the data from the cell.
//
func (rtcm *RTCM) GetMSM7SignalCells(m []byte, h *MSMHeader, satData []MSMSatelliteCell, startOfSignaldata uint) [][]MSMSignalCell {
	// The number of cells is variable, NumSignalCells in the
	// header.  As explained in GetMSM7SatelliteCell, the bit
	// stream is laid out with all pseudo range values followed
	// by all phase range values, and so on.

	var signalCell [][]MSMSignalCell

	// Get the cell data into a more manageable form - a slice of
	// MSM7SignalCell objects, one for each expected signal cell.
	sigData := make([]MSMSignalCell, h.NumSignalCells)
	lengthInBits := len(m) * 8

	// Define the lengths of the fields
	const lenRangeDelta = 15
	const lenPhaseRangeDelta = 22
	const lenLockTimeIndicator = 4
	const lenHalfCycleAmbiguity = 1
	const lenCNR = 6

	// Pos is the position within the bitstream.
	pos := startOfSignaldata

	// Check for overrun.
	lastPosition := int(pos) + h.NumSignalCells*
		int(lenRangeDelta+lenPhaseRangeDelta+lenLockTimeIndicator+lenHalfCycleAmbiguity+lenCNR)

	if lastPosition >= lengthInBits {
		logEntry := fmt.Sprintf("GetMSM4SignalCells: overrun %d %d", lastPosition, lengthInBits)
		rtcm.makeLogEntry(logEntry)
		return signalCell
	}

	// Set all of the valid values - they may be unset later.
	for i := range sigData {
		sigData[i].CNRValid = true
		sigData[i].PhaseRangeDeltaValid = true
		sigData[i].PhaseRangeRateDeltaValid = true
		sigData[i].RangeDeltaValid = true
	}

	for i := 0; i < h.NumSignalCells; i++ {
		if pos+20 < uint(lengthInBits) {
			rangeDelta := int(Getbits(m, pos, 20))
			pos += 20
			sigData[i].RangeDelta = rangeDelta
		} else {
			logEntry := fmt.Sprintf("GetMSM7SignalCells: overrun %d %d", pos, lengthInBits)
			rtcm.makeLogEntry(logEntry)
			return signalCell
		}
	}
	for i := 0; i < h.NumSignalCells; i++ {
		if pos+24 < uint(lengthInBits) {
			sigData[i].PhaseRangeDelta = int(Getbits(m, pos, 24))
			pos += 24
		} else {
			logEntry := fmt.Sprintf("GetMSM7SignalCells: overrun %d %d", pos, lengthInBits)
			rtcm.makeLogEntry(logEntry)
			return signalCell
		}
	}
	for i := 0; i < h.NumSignalCells; i++ {
		if pos+10 < uint(lengthInBits) {
			sigData[i].LockTimeIndicator = uint(Getbitu(m, pos, 10))
			pos += 10
		} else {
			logEntry := fmt.Sprintf("GetMSM7SignalCells: overrun %d %d", pos, lengthInBits)
			rtcm.makeLogEntry(logEntry)
			return signalCell
		}
	}
	for i := 0; i < h.NumSignalCells; i++ {
		if pos < uint(lengthInBits) {
			sigData[i].HalfCycleAmbiguity = (Getbitu(m, pos, 1) == 1)
			pos++
		} else {
			logEntry := fmt.Sprintf("GetMSM7SignalCells: overrun %d %d", pos, lengthInBits)
			rtcm.makeLogEntry(logEntry)
			return signalCell
		}
	}
	for i := 0; i < h.NumSignalCells; i++ {
		if pos+10 < uint(lengthInBits) {
			sigData[i].CNR = uint(Getbitu(m, pos, 10))
			pos += 10
		} else {
			logEntry := fmt.Sprintf("GetMSM7SignalCells: overrun %d %d", pos, lengthInBits)
			rtcm.makeLogEntry(logEntry)
			return signalCell
		}
	}
	for i := 0; i < h.NumSignalCells; i++ {
		if pos+15 < uint(lengthInBits) {
			sigData[i].PhaseRangeRateDelta = int(Getbits(m, pos, 15))
			pos += 15
		} else {
			logEntry := fmt.Sprintf("GetMSM7SignalCells: overrun %d %d", pos, lengthInBits)
			rtcm.makeLogEntry(logEntry)
			return signalCell
		}
	}

	// Set the satellite and signal numbers by cranking through the cell mask.

	cell := 0
	for i := range h.CellMask {
		var cellSlice []MSMSignalCell
		for j := range h.CellMask[i] {
			if h.CellMask[i][j] {
				sigData[cell].Satellite = i
				sigData[cell].SignalID = h.Signals[j]
				cellSlice = append(cellSlice, sigData[cell])
				cell++
			}
		}
		signalCell = append(signalCell, cellSlice)
	}
	// Set the combined values.  For example the range in metres is derived from
	// the range values in the satellite cell and the range delta value in the
	// signal cell.
	for i := range signalCell {
		for j := range signalCell[i] {
			satelliteID := signalCell[i][j].Satellite
			satellite := &satData[satelliteID]
			signal := &signalCell[i][j]
			signal.RangeMetres = getMSMRangeInMetres(h, satellite, signal)
			var err error
			signal.PhaseRangeCycles, err = getMSMPhaseRange(h, satellite, signal)
			if err != nil {
				logEntry := fmt.Sprintf("error getting phase range - %s\n", err.Error())
				rtcm.makeLogEntry(logEntry)
			}
			signal.PhaseRangeRateMS, err = getMSMPhaseRangeRate(h, satellite, signal)
			if err != nil {
				logEntry := fmt.Sprintf("error getting phase range rate - %s\n", err.Error())
				rtcm.makeLogEntry(logEntry)
			}
		}
	}

	return signalCell
}

// GetSatellites gets the list of satellites from the 64-bit
// satellite mask as a slice of satellite numbers each >= 1,
// <= 64.  The mask starts at bit pos in the message.
func GetSatellites(message []byte, pos uint) []uint {
	// Bit 63 of the mask is satellite number 1, bit 62 is 2,
	// bit 0 is 64.
	satellites := make([]uint, 0)
	for satNum := 1; satNum <= satelliteMaskLength; satNum++ {
		if Getbitu(message, pos, 1) == 1 {
			satellites = append(satellites, uint(satNum))
		}
		pos++
	}
	return satellites
}

// GetSignals gets the list of signals from the 32-bit signal
// mask as a slice of signal numbers each >= 1, <= 32.  The
// mask starts at bit pos in the message.
func GetSignals(message []byte, pos uint) []uint {

	// Bit 31 of the mask is signal number 1, bit 30 is 2,
	// bit 0 is 32.
	signals := make([]uint, 0)
	for sigNum := 1; sigNum <= signalMaskLength; sigNum++ {
		if Getbitu(message, pos, 1) == 1 {
			signals = append(signals, uint(sigNum))
		}
		pos++
	}
	return signals
}

// GetMSMMessage presents an Multiple Signal Message as broken out fields.
// If it's type 4 or 7, it produces readable versions of the data.
//
func (rtcm *RTCM) GetMSMMessage(message *Message) *MSMMessage {

	if !MSM(message.MessageType) {
		return nil
	}

	header, startOfSatelliteData := rtcm.GetMSMHeader(message.RawData)

	var satellites []MSMSatelliteCell
	var signals [][]MSMSignalCell
	var startOfSignals uint

	switch {
	case MSM4(message.MessageType):
		satellites, startOfSignals =
			rtcm.GetMSM4SatelliteCells(message.RawData, header, startOfSatelliteData)

		signals = rtcm.GetMSM4SignalCells(message.RawData, header, satellites, startOfSignals)

	case MSM7(message.MessageType):
		satellites, startOfSignals =
			rtcm.GetMSM7SatelliteCells(message.RawData, header, startOfSatelliteData)

		signals = rtcm.GetMSM7SignalCells(message.RawData, header, satellites, startOfSignals)
	}

	var msmMessage = MSMMessage{Header: header, Satellites: satellites, Signals: signals}

	return &msmMessage
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
func (r *RTCM) DisplayMessage(message *Message) string {
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

	if message.Valid {

		if MSM(message.MessageType) {
		} else {
			m, ok := message.Readable(r).(*MSMMessage)
			if !ok {
				return ("expected the readable message to be *MSM7Message\n")
			}
			return leader + r.displayMSM(m)
		}

		switch message.MessageType {
		case NonRTCMMessage:
			display := fmt.Sprintf("not RTCM, %d bytes, %s\n%s\n",
				len(message.RawData), message.Warning, hex.Dump(message.RawData))
			return display

		case 1005:
			m, ok := message.Readable(r).(*Message1005)
			if !ok {
				return ("expected the readable message to be *Message1005\n")
			}
			return leader + r.display1005(m)

		case 1230:
			m, ok := message.Readable(r).(*Message1230)
			if !ok {
				return ("expected the readable message to be *Message1230\n")
			}
			return leader + r.display1230(m)

		case 4072:
			m, ok := message.Readable(r).(string)
			if !ok {
				return ("expected the readable message to be a string\n")
			}
			return leader + m

		default:
			return leader + "\n"
		}
	} else {
		return leader
	}
}

// display1005 returns a text version of a message type 1005
func (rtcm *RTCM) display1005(message *Message1005) string {

	l1 := fmt.Sprintln("message type 1005 - Base Station Information")

	l2 := fmt.Sprintf("stationID %d, ITRF realisation year %d, GPS %v, GLONASS %v Galileo %v,\n",
		message.StationID, message.ITRFRealisationYear,
		message.GPS, message.Glonass, message.Galileo)
	l2 += fmt.Sprintf("reference station %v, oscilator %v, quarter cycle %d,\n",
		message.ReferenceStation, message.Oscilator, message.QuarterCycle)

	x := Scaled5ToFloat(message.AntennaRefX)
	y := Scaled5ToFloat(message.AntennaRefY)
	z := Scaled5ToFloat(message.AntennaRefZ)
	l2 += fmt.Sprintf("ECEF coords in metres (%8.4f,%8.4f,%8.4f)\n",
		x, y, z)
	return l1 + l2
}

// display1230 returns a text version of a message type 1230
func (rtcm *RTCM) display1230(message *Message1230) string {

	l1 := fmt.Sprintln("message type 1230 - GLONASS code-phase biases")

	var l2, l3, alignment string
	if message.Aligned {
		alignment = "aligned"
	} else {
		alignment = "not aligned"
	}

	l2 = fmt.Sprintf("stationID %d, %s, mask %4b\n",
		message.StationID, alignment, message.Signalmask)

	if message.L1_C_A_Bias_supplied {
		if message.L1_C_A_Bias_valid {
			l3 = fmt.Sprintf("L1 C/A bias %d, ", message.L1_C_A_Bias)
		}
	}

	if message.L1_P_Bias_supplied {
		if message.L1_P_Bias_valid {
			l3 += fmt.Sprintf("L1 P bias %d, ", message.L1_P_Bias)
		}
	}

	if message.L2_C_A_Bias_supplied {
		if message.L1_C_A_Bias_valid {
			l3 += fmt.Sprintf("L2 C/A bias %d, ", message.L2_C_A_Bias)
		}
	}

	if message.L2_P_Bias_supplied {
		if message.L2_P_Bias_valid {
			l3 += fmt.Sprintf("L1 P bias %d", message.L2_P_Bias)
		}
	}
	l3 += fmt.Sprint("\n")

	return l1 + l2 + l3
}

// displayMSM returns a text version of an MSM7 message.
func (rtcm *RTCM) displayMSM(message *MSMMessage) string {
	messageType := int(message.Header.MessageType)
	if !MSM(messageType) {
		return fmt.Sprintf("type %d is not MSM", message.Header.MessageType)
	}
	headerStr := rtcm.displayMSMHeader(message.Header)
	headerStr += "\n"

	satelliteDisplay := ""
	signalDisplay := ""

	if MSM4(messageType) || MSM7(messageType) {
		satelliteDisplay = displayMSMSatelliteCells(message.Satellites)
		signalDisplay = displayMSM7SignalCells(message)
	}

	return headerStr + satelliteDisplay + "\n" + signalDisplay + "\n"
}

// displayMSMHeader return a text version of an MSMHeader.
func (rtcm *RTCM) displayMSMHeader(h *MSMHeader) string {
	var timeUTC time.Time
	line := fmt.Sprintf("type %d %s Full Pseudoranges and PhaseRanges plus CNR (high resolution)\n",
		h.MessageType, h.Constellation)
	switch h.Constellation {
	case "GPS":
		timeUTC = rtcm.GetUTCFromGPSTime(h.EpochTime)
	case "GLONASS":
		timeUTC = rtcm.GetUTCFromGlonassTime(h.EpochTime)
	case "BeiDou":
		timeUTC = rtcm.GetUTCFromBeidouTime(h.EpochTime)
	default:
		timeUTC = rtcm.GetUTCFromGalileoTime(h.EpochTime)
	}
	timeStr := timeUTC.Format(dateLayout)
	line += fmt.Sprintf("time %s (%d millisecs from epoch)\n", timeStr, h.EpochTime)
	mode := "single"
	if h.MultipleMessage {
		mode = "multiple"
	}
	line += fmt.Sprintf("stationID %d, %s message, sequence number %d, session transmit time %d\n",
		h.StationID, mode, h.SequenceNumber, h.SessionTransmissionTime)
	line += fmt.Sprintf("clock steering %d, external clock %d ",
		h.ClockSteeringIndicator, h.ExternalClockIndicator)
	line += fmt.Sprintf("divergence free smoothing %v, smoothing interval %d\n",
		h.GNSSDivergenceFreeSmoothingIndicator, h.GNSSSmoothingInterval)
	line += fmt.Sprintf("%d satellites, %d signals, %d signal cells\n",
		len(h.Satellites), len(h.Signals), h.NumSignalCells)

	return line
}

// displayMSMSatelliteCells returns a text version of a slice of MSM7 satellite data
func displayMSMSatelliteCells(satellites []MSMSatelliteCell) string {

	l1 := fmt.Sprintf("%d Satellites\nsatellite ID {range ms, extended info, phase range rate m/s}\n",
		len(satellites))
	l2 := ""
	for i := range satellites {
		// Mark the range and phase values as valid or invalid.
		rangeMillis := "invalid"
		if satellites[i].RangeValid {
			rangeMillis = fmt.Sprintf("%d.%d", satellites[i].RangeWholeMillis,
				satellites[i].RangeFractionalMillis)
		}

		phaseRangeRate := "invalid"
		if satellites[i].PhaseRangeRateValid {
			phaseRangeRate = fmt.Sprintf("%d", satellites[i].PhaseRangeRate)
		}
		l2 += fmt.Sprintf("%2d {%s %d %s}\n", satellites[i].SatelliteID,
			rangeMillis, satellites[i].ExtendedInfo, phaseRangeRate)
	}

	return l1 + l2
}

// displayMSM7SignalCells returns a text version of the MSM7 signal data from an MSM7Message.
func displayMSM7SignalCells(message *MSMMessage) string {

	if !MSM(int(message.Header.MessageType)) {
		return fmt.Sprintf("message type %d does not contain signal cells",
			message.Header.MessageType)
	}

	l1 := fmt.Sprintf("%d Signals\nsat ID sig ID {range m (delta) phase range cycles (delta), lock time ind, half cycle ambiguity,\n",
		len(message.Signals))
	l1 += "        Carrier Noise Ratio,  phase range rate (delta)}\n"
	l2 := ""

	for _, list := range message.Signals {
		for _, signal := range list {
			// Mark the range and phase values as valid or invalid.
			r := "invalid"
			if signal.RangeDeltaValid {
				r = fmt.Sprintf("%f (%d)", signal.RangeMetres, signal.RangeDelta)
			}
			phaseRange := "invalid"
			if signal.PhaseRangeDeltaValid {
				phaseRange = fmt.Sprintf("%f (%d)", signal.PhaseRangeCycles, signal.PhaseRangeRateDelta)
			}
			phaseRangeRate := "invalid"
			if signal.PhaseRangeRateDeltaValid {
				phaseRangeRate = fmt.Sprintf("%f (%d)", signal.PhaseRangeRateMS, signal.PhaseRangeRateDelta)
			}

			cnr := "invalid"
			if signal.CNRValid {
				cnr = fmt.Sprintf("%d", signal.CNR)
			}
			satelliteID := message.Header.Satellites[signal.Satellite]
			l2 += fmt.Sprintf("%2d %2d {%s %s %d, %v, %s, %s}\n",
				satelliteID, signal.SignalID, r, phaseRange, signal.LockTimeIndicator,
				signal.HalfCycleAmbiguity, cnr, phaseRangeRate)
		}
	}
	return l1 + l2
}

// getScaled5 takes a scaled integer a.b where b has 5 decimal places and
// returns the float value.
//
func Scaled5ToFloat(scaled5 int64) float64 {
	return float64(scaled5) * 0.0001
}

// CheckCRC checks the CRC of a message frame.
func CheckCRC(frame []byte) bool {
	if len(frame) < headerLengthBytes+crcLengthBytes {
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

// Getbitu extracts len bits from a slice of  bytes,
// starting at bit position pos and returns them as a uint.
//
// This is RTKLIB's getbitu, translated from C to Go.
//
// extern unsigned int getbitu(const unsigned char *buff, int pos, int len)
// {
//     unsigned int bits=0;
//     int i;
//     for (i=pos;i<pos+len;i++) bits=(bits<<1)+((buff[i/8]>>(7-i%8))&1u);
//     return bits;
// }
//
func Getbitu(buff []byte, pos uint, len uint) uint64 {
	var u1 uint64 = 1
	var bits uint64 = 0
	for i := pos; i < pos+len; i++ {
		bits = (bits << 1) + (uint64((buff[i/8])>>(7-i%8)) & u1)
	}
	return bits
}

// Getbits extracts len bits from a slice of bytes, starting at bit
// position pos, interprets the bits as a twos-complement inte and
// returns the resulting as a signed int.
//
// This does the same as RTKLIB's getbits() function, but the
// algorithm is from https://en.wikipedia.org/wiki/Two%27s_complement,
// which includes this Python code:
//
// def twos_complement(input_value: int, num_bits: int) -> int:
//     """Calculates a two's complement integer from the given input value's bits."""
//     mask = 2 ** (num_bits - 1)
//     return -(input_value & mask) + (input_value & ~mask)
//
// The two's complement version of a positive number is, of course,
// the number.  So if the sign bit is not set, we just return that,
// otherwise we do the same calculation as the Python code.
//
func Getbits(buff []byte, pos uint, len uint) int64 {
	negative := Getbitu(buff, pos, 1) == 1
	uval := Getbitu(buff, pos, len)
	if negative {
		var mask uint64 = uint64(2) << (len - 2)
		weightOfTopBit := int(uval & mask)
		weightOfLowerBits := int(uval & ^mask)
		return int64((-1 * weightOfTopBit) + weightOfLowerBits)
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
		for i, _ := range channels {
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

// getMSMRangeInMetres combines the components of the range from an MSM
// message and returns the result in metres.  It returns zero if the range
// in the satellite cell is invalid.
func getMSMRangeInMetres(header *MSMHeader, satellite *MSMSatelliteCell, signal *MSMSignalCell) float64 {

	if !satellite.RangeValid {
		// There is no valid range reading.
		return 0.0
	}

	// This only works for MSM4, MSM5, MSM6 and MSM7 messages.
	// Return 0.0 for any other message type.
	switch {
	case MSM4(int(header.MessageType)):
		break
	case MSM5(int(header.MessageType)):
		break
	case MSM6(int(header.MessageType)):
		break
	case MSM7(int(header.MessageType)):
		break
	default:
		return 0.0
	}

	// The raw range values are in milliseconds giving the transit time of each
	// signal from the satellite to the GPS receiver.  This can be converted to
	// the distance between the two using the speed of light.
	//
	// Since the signals were sent from the same satellite at the same time, the
	// transit times should always be the same.  In fact they can be different
	// because of factors such as interference by the ionosphere having a different
	// effect on signals of a different frequency.  The resulting differences in
	// the values provide useful information about the interference.
	//
	// The transit time is provided in three parts, the satellite cell contains
	// an approximate range for all the signals, which is always positive.  The
	// unsigned 8-bit RangeWhole is whole milliseconds and the unsigned 10-bit
	// RangeFractional is the fractional part.  The range value can be marked as
	// invalid, in which case there is no sensible range value and we return
	// zero.  (That was done earlier.)
	//
	// Each signal cell contains a signed delta.  In an MSM4 and an MSM5 message
	// it's 15 bits long.  In an MSM6 and an MSM7, it's 20 bits long, giving an
	// extra 5 bits of resolution.  The positive or negative delta is added to
	// the satellite range to correct it.  The delta is very small, so the result
	// is always still positive.
	//
	// The delta also has an invalid value.  If the satellite range is valid but
	// the delta is invalid, then we return the uncorrected approximate range
	// from the values in the satellite cell.
	//
	// To preserve accuracy during the calculation, the values are kept as scaled
	// integers and converted to a float at the end.
	//
	//     ------ Range -------
	//     whole     fractional
	//     w wwww wwwf ffff ffff f000 0000 0000 0000 0000
	//     + or - range delta    dddd dddd dddd dddd dddd <- 20-bit delta

	//     ------ Range -------
	//     whole     fractional
	//     w wwww wwwf ffff ffff f000 0000 0000 0000 0000
	//     + or - range delta    dddd dddd dddd ddd0 0000 <- 15-bit delta

	// Merge the satellite values to give the approximate range shifted up 29 bits.
	aggregateRange := int64(satellite.RangeWholeMillis<<29 |
		satellite.RangeFractionalMillis<<19)

	// If the range delta is valid, add it to the aggregate.
	if signal.RangeDeltaValid {

		// If the message is MSM4 or MSM5, convert the 15-bit delta to a 20-bit
		// delta with trailing zeroes.
		delta := int64(signal.RangeDelta)
		if MSM4(int(header.MessageType)) || MSM5(int(header.MessageType)) {
			delta = delta << 5
		}

		aggregateRange += delta
	}

	// Restore the scale to give the range in milliseconds.
	rangeInMillis := float64(aggregateRange) * P2_29

	// Use the speed of light to convert that to the distance from the
	// satellite to the receiver.
	rangeInMetres := rangeInMillis * oneLightMillisecond

	return rangeInMetres
}

// getMSMPhaseRange combines the range and the phase range from an MSM4, MSM5,
// MSM6 or MSM7 message and returns the result in cycles. It returns zero if
// the input measurements are invalid and an error if the signal is not in use.
//
func getMSMPhaseRange(header *MSMHeader, satellite *MSMSatelliteCell, signal *MSMSignalCell) (float64, error) {

	if !satellite.RangeValid {
		return 0.0, nil
	}

	if !signal.PhaseRangeDeltaValid {
		return 0.0, nil
	}

	// This only works for MSM4, MSM5, MSM6 and MSM7 messages.
	// Return 0.0 for any other message type.
	switch {
	case MSM4(int(header.MessageType)):
		break
	case MSM5(int(header.MessageType)):
		break
	case MSM6(int(header.MessageType)):
		break
	case MSM7(int(header.MessageType)):
		break
	default:
		return 0.0, nil
	}

	// This is similar to getMSMRangeInMetres.  The phase range is in cycles
	// and derived from the range values from the satellite cell shifted up
	// 31 bits plus the signed phase range delta.  In an MSM4 and MSM5 message
	// the delta in the signal cell is 22 bits and in an MSM6 and MSM7 it's 24
	// bits.  The 24 bit value gives extra resolution, so we convert a 22 bit
	// value to a 24 bit value with two trailing spaces.
	//
	// The values are kept as integers during the calculation to preserve
	// accuracy.
	//
	//     ------ Range -------
	//     whole     fractional
	//     www wwww wfff ffff fff0 0000 0000 0000 0000 0000
	//     + or -             dddd dddd dddd dddd dddd dddd

	wavelength, err := getWavelength(header.Constellation, signal.SignalID)
	if err != nil {
		return 0.0, err
	}

	var aggregatePhaseRange int64 = 0
	millis := int64(satellite.RangeWholeMillis<<31 |
		satellite.RangeFractionalMillis<<21)

	// If the message is MSM4 or MSM5, convert the 22-bit delta to 24 bits
	// with trailing zeroes.  Since it's signed, do that by multiplying it
	// by 4.
	delta := int64(signal.PhaseRangeDelta)
	if MSM4(int(header.MessageType)) || MSM5(int(header.MessageType)) {
		delta = delta * 4
	}

	aggregatePhaseRange = millis + delta

	// Restore the scale of the aggregate value - it's shifted up 31 bits.
	phaseRangeMilliSeconds := float64(aggregatePhaseRange) * P2_31

	// Convert to light milliseconds
	phaseRangeLMS := phaseRangeMilliSeconds * oneLightMillisecond

	// and divide by the wavelength to get cycles.
	phaseRangeCycles := phaseRangeLMS / wavelength

	return phaseRangeCycles, nil
}

// getMSMPhaseRangeRate combines the components of the phase range rate
// in an MSM5 or MSM7 message and returns the result in milliseconds.  If
// either the component values are invalid, it returns float zero.
//
func getMSMPhaseRangeRate(header *MSMHeader, satellite *MSMSatelliteCell, signal *MSMSignalCell) (float64, error) {

	if !satellite.PhaseRangeRateValid {
		return 0.0, nil
	}

	if !signal.PhaseRangeRateDeltaValid {
		return 0.0, nil
	}

	// This only works for MSM5 and MSM7 messages.
	// Return 0.0 for any other message type.
	switch {
	case MSM5(int(header.MessageType)):
		break
	case MSM7(int(header.MessageType)):
		break
	default:
		return 0.0, nil
	}

	// The 14-bit signed phase range rate is in milliseconds and the
	// 15-bit signed delta is in ten thousandths of a millisecond.
	aggregatePhaseRangeRate := int64(satellite.PhaseRangeRate) * 10000
	aggregatePhaseRangeRate += int64(signal.PhaseRangeRateDelta)
	wavelength, err := getWavelength(header.Constellation, signal.SignalID)
	if err != nil {
		return 0.0, err
	}
	return (1.0 - float64(aggregatePhaseRangeRate)/10000.0/wavelength), nil
}

// getWavelength returns the carrier wavelength for a signal ID.
// The result depends upon the constellation, each of which has its
// own list of signals and equivalent wavelengths.  Some of the possible
// signal IDs are not used and so have no associated wavelength, so the
// result may be an error.
//
func getWavelength(constellation string, signalID uint) (float64, error) {
	var wavelength float64
	switch constellation {
	case "GPS":
		var err error
		wavelength, err = getSigWaveLenGPS(signalID)
		if err != nil {
			return 0.0, err
		}
	case "Galileo":
		var err error
		wavelength, err = getSigWaveLenGalileo(signalID)
		if err != nil {
			return 0.0, err
		}
	case "GLONASS":
		var err error
		wavelength, err = getSigWaveLenGlo(signalID)
		if err != nil {
			return 0.0, err
		}
	case "BeiDou":
		var err error
		wavelength, err = getSigWaveLenBD(signalID)
		if err != nil {
			return 0.0, err
		}
	default:
		message := fmt.Sprintf("no such constellation as %s", constellation)
		return 0.0, errors.New(message)
	}

	return wavelength, nil
}

// getSigWaveLenGPS returns the signal carrier wavelength for a GPS satellite
// if it's defined.
func getSigWaveLenGPS(signalID uint) (float64, error) {
	// Only some signal IDs are in use.
	var frequency float64
	switch signalID {
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
		message := fmt.Sprintf("GPS signal ID %d not in use", signalID)
		return 0, errors.New(message)
	}
	return CLight / frequency, nil
}

// getSigWaveLenGalileo returns the signal carrier wavelength for a Galileo satellite
// if it's defined.
func getSigWaveLenGalileo(signalID uint) (float64, error) {
	// Only some signal IDs are in use.
	var frequency float64
	switch signalID {
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
		message := fmt.Sprintf("GPS signal ID %d not in use", signalID)
		return 0, errors.New(message)
	}
	return CLight / frequency, nil
}

// getSigWaveLenGlo gets the signal carrier wavelength for a GLONASS satellite
// if it's defined.
//
func getSigWaveLenGlo(signalID uint) (float64, error) {
	// Only some signal IDs are in use.
	var frequency float64
	switch signalID {
	case 2:
		frequency = freq1Glo
	case 3:
		frequency = freq1Glo
	case 8:
		frequency = freq2Glo
	case 9:
		frequency = freq2Glo
	default:
		message := fmt.Sprintf("GLONASS signal ID %d not in use", signalID)
		return 0, errors.New(message)
	}
	return CLight / frequency, nil
}

// getSigWaveLenBD returns the signal carrier wavelength for a Beidou satellite
// if it's defined.
//
func getSigWaveLenBD(signalID uint) (float64, error) {
	// Only some signal IDs are in use.
	var frequency float64
	switch signalID {
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
		message := fmt.Sprintf("GPS signal ID %d not in use", signalID)
		return 0, errors.New(message)
	}
	return CLight / frequency, nil
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
	return MSM1(messageType) || MSM2(messageType) || MSM3(messageType) ||
		MSM4(messageType) || MSM5(messageType) || MSM6(messageType) || MSM7(messageType)
}
