package rtcm

import (
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
// various web pages.  To figure out the format of the message I'm
// interested in, I read what I could find, took the RTCM3 messages
// that my device produced, converted them to RINEX format and examined
// the result, and I spent a lot of time reading the open source RTKLIB
// software, which is written in C and gives lots of useful clues as to
// the detail.
//
// Some of my unit tests take the results that this software produces and
// compares it with the results that RINEX tools produced.
//
// An RTCM3 message is binary and variable length.  Each message frame
// is composed of a three-byte header, an embedded message and 3 bytes of
// Cyclic Redundancy Check (CRC) data.  The header starts with 0xd3 and
// includes the length of the embedded message.  Each message starts with
// a 12-bit message number which defines the type.  Apart from that
// message number, each type of message is in a different format.
// (Fortunately, I only have to worry about two major types)
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
// announces the start of the message frame.  The top six bits of byte 1
// are always zero.  The lower two bits and the bits of byte 2 form the
// message length, in this case hex aa, decimal 176.  So the embedded
// message is 176 bytes long and the whole message frame is 182 bytes long.
// As shown above, the embedded message may end with some padding bits,
// always zero.
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
// message.  One clue is that it's not followed by six zero bits.  To
// decode a stream of message frames you need to check the header, extract the
// message length, read the whole message frame and check the CRC bytes.  This
// matters particularly when you switch on and start to receive a stream of
// data from a device.  You will come into the data stream part-way through.
// You can't assume that a d3 byte is the start of a message.
//
// Message frames are contiguous with no separators or newlines.  In the
// example, the last line contains the start of the next message.  My device
// intersperses other messages in other formats, which this software ignores.
//
// There are many hundreds of RTCM3 message types, some of which are just
// different ways of representing the same information.  To get an accurate fix
// on its position, a rover only needs to know the position of the base station
// and a recent set of the base's observations of satellite signals, which is to
// say a tpe 1005 message and a set of MSM7 messages, one for each constellation
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
// either.  If I tried, I wouldn't be able to test the results.
//
// Each MSM contains readings for satellites in one constellation.  Each
// satellite in a constellation is numbered.  An MSM allows 64 satellites
// numbered 1-64.  At any point on the Earth's surface only some satellites will
// be visible.  Signals from some of those may be too weak to register, so the
// message will contain readings of just some signals from just some satellites.
// My base station typically sees one or two signals from each of 6-8 satellites
// in a scan.
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
const StartOfMessageFrame = 0xd3

// rangeMillisWholeInvalidValue in the satellite's RangeWhole value
// indicates that the reading is invalid.
const rangeMillisWholeInvalidValue = 0xff

// phaseRangeRateInvalidValue in the satellite phase range rate
// indicates that the reading is invalid.
const phaseRangeRateInvalidValue = -8192 // 0x2000

// phaseRangeInvalidValue
const phaseRangeDeltaInvalidValue = -8388608 // 0x800000

// rangeDeltaInvalidValue in the signal's range delta indicates
// that the reading is invalid.
const rangeDeltaInvalidValue = -16384 // 0x4000

// phaseRangeRateDeltaInvalidValue in the signal's phase range
// rate delta value indicates that the reading is invalid.
const phaseRangeRateDeltaInvalidValue = -16384 // 0x4000

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

// freqGlo is lookup table for converting a frequency id to a GLONASS frequency.
var freqGlo = []float64{freq1Glo, freq2Glo, freq3Glo}

// dfreqGlo is a lookup table for converting a GLONASS bias frequency ID to a freqency.
var dfreqGlo = []float64{dFreq1Glo, dFreq2Glo, 0.0}

// freqGPS is a lookup table for coverting a GPS frequency ID to a frequency.
var freqGPS = []float64{freq1, freq2, freq5, freq6, freq7, freq8}

var freqBD = []float64{freq1BD, freq2BD, freq3BD}

// dateLayout defines the layout of dates when they
// are displayed - "yyyy-mm-dd hh:mm:ss.ms timeshift timezone"
const dateLayout = "2006-01-02 15:04:05.000 -0700 MST"

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

// RTCM is the object used to analyse an RTCM3 message.
type RTCM struct {

	// These dates are used to intrepret the timestamps in RTCM3
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

// MSMHeader holds the header for MSM Messages.  Message types 1074,
// 1077, 1084, 1087 etc have an MSM header at the start.
type MSMHeader struct {

	// MessageType - uint12 - one of 1005, 1077, 1087 etc.
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

// MSM7SatelliteCell holds the MSM7 data for one satellite.
type MSM7SatelliteCell struct {
	// SatelliteID is the satellite ID, 1-64.
	SatelliteID uint

	// RangeMillisWhole - uint8 - the number of integer milliseconds
	// in the GNSS Satellite range (ie the transit time of the signals).  0xff
	// indicates an invalid value.  See also the fraction value here and the
	// delta value in the signal cell.
	RangeMillisWhole uint

	// ExtendedInfo - uint4.  Extended Satellite Information.
	ExtendedInfo uint

	// RangeMillisFractional - unit10.  The fractional part of the range
	// in milliseconds.
	RangeMillisFractional uint

	// PhaseRangeRate - int14 - GNSS Satellite phase range rate.  See also
	// the PhaseRangeRateDelta and PhaseRangeRateMS in the signal data.
	PhaseRangeRate int
}

// MSM7SignalCell holds the data for one signal from one satellite, plus
// values gathered from the satellite and signal data and merged together.
type MSM7SignalCell struct {
	// Satellite is the index into the satellite cell list of the satellite
	// that was observed.
	Satellite int

	// SignalID is the ID of the signal that was observed from the satellite.
	// Each signalID has an associated frequency.
	SignalID uint

	// RangeDelta - int20 - to be merged with the range values from the satellite
	// to give RangeMetres. 0x80000 indicates an invalid value.
	RangeDelta int

	// PhaseRangeDelta - int24 - to be merged with the range values and
	// converted to cycles to create PhaseRangeCycles.  0x800000 indicates an
	// invalid value.
	PhaseRangeDelta int

	// LockTimeIndicator - uint10.
	LockTimeIndicator uint

	// HalfCycleAmbiguity flag - 1 bit.
	HalfCycleAmbiguity bool

	// CNR - uint10 - Carrier to Noise Ratio.  0 means invalid value.
	CNR uint

	// PhaseRangeRateDelta - int15 - the offset to the satellite's phase
	// range rate.  This value is in ten thousands of a millisecond.
	PhaseRangeRateDelta int

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

// MSM7Message is a broken-out version of an MSM7 message.
type MSM7Message struct {
	// Header is the MSM Header
	Header *MSMHeader

	// Satellites is a list of the satellites for which signals
	// were observed.
	Satellites []MSM7SatelliteCell

	// Signals is a list of sublists, one sublist per satellite,
	// of signals at different frequencies observed by the base
	// station from the satellites in the Satellite list. If
	// there are, say, eight items in the Satellites list, there
	// will be eight sublists here.  RTCM3 allows for up to 32
	// signals but currently (2021) each satellite only uses two
	// signal channels, so there should be either one or two
	// signals in each sublist.
	Signals [][]MSM7SignalCell
}

// Message contains an RTCM3 message broken out into readable form.
type Message struct {
	// MessageType is the type of the message (the message number).
	MessageType uint

	// RawData is the message frame in its original binary form
	//including the header and the CRC.
	RawData []byte

	// Readable is a broken out version of the message - if the message type
	// is 1005 it's a Message1005, if it's an MSM7 message, it's an
	// MSM7Message.
	Readable interface{}
}

func init() {

	locationUTC, _ = time.LoadLocation("UTC")
	locationGMT, _ = time.LoadLocation("GMT")
	locationMoscow, _ = time.LoadLocation("Europe/Moscow")
}

// New creates an RTCM object using the given year, month and day to
// identify which week the times in the messages refer to.
func New(startTime time.Time) *RTCM {

	var rtcm RTCM

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

// GetMessages takes the given stream of bytes, exracts all the complete
// RTCM3 messages and returns it as a slice of Message objects.
func (rtcm *RTCM) GetMessages(data []byte) []*Message {
	var result []*Message
	pos := 0
	for {
		if pos >= len(data) {
			break
		}
		message, err := rtcm.GetMessage(data[pos:])
		if err != nil {
			continue
		}
		if message == nil {
			break
		}
		result = append(result, message)
		pos += len(message.RawData)
	}
	return result
}

// GetMessage extract an RTCM3 message from the given data stream and
// returns it as a Message. If the data doesn't contain a single valid
// message, it returns an error.
func (rtcm *RTCM) GetMessage(data []byte) (*Message, error) {

	messageLength, messageType, err := GetMessageLengthAndType(data)
	if err != nil {
		return nil, err
	}

	// The message follows the header and is followed by three bytes of CRC.
	frameLength := messageLength + headerLengthBytes + crcLengthBytes
	if frameLength > uint(len(data)) {
		// Ignore the last message if it's incomplete.
		return nil, nil
	}
	var message Message
	message.RawData = data[:frameLength]
	message.MessageType = messageType
	switch message.MessageType {
	case 1005:
		message.Readable, _ = rtcm.GetMessage1005(message.RawData)
	case 1077:
		message.Readable = rtcm.GetMSM7Message(message.RawData)
	case 1087:
		message.Readable = rtcm.GetMSM7Message(message.RawData)
	case 1097:
		message.Readable = rtcm.GetMSM7Message(message.RawData)
	case 1127:
		message.Readable = rtcm.GetMSM7Message(message.RawData)
	}

	return &message, nil
}

// GetMessageLengthAndType extracts the message length and the message type from an
// RTCMs message frame or returns an error, implying that this is not the start of a
// valid message.  The data must be at least 5 bytes long.
func GetMessageLengthAndType(data []byte) (uint, uint, error) {

	if len(data) < headerLengthBytes+2 {
		return 0, 0, errors.New("the message is too short to get the header and the length")
	}

	// The message header is 24 bits.  The top byte is startOfMessage.
	if data[0] != StartOfMessageFrame {
		message := fmt.Sprintf("message starts with 0x%0x not 0xd3", data[0])
		return 0, 0, errors.New(message)
	}

	// The next six bits must be zero.  If not, we've just come across
	// a 0xd3 byte in a stream of binary data.
	sanityCheck := Getbitu(data, 8, 6)
	if sanityCheck != 0 {
		message := fmt.Sprintf("bits 8 -13 of header are %d, must be 0", sanityCheck)
		return 0, 0, errors.New(message)
	}

	// The bottom ten bits of the header is the message length.
	length := uint(Getbitu(data, 14, 10))

	// The message follows the header and is followed by three
	// bytes of CRC.
	frameLength := length + headerLengthBytes + crcLengthBytes
	if frameLength > uint(len(data)) {
		message := fmt.Sprintf("incomplete message - frame length %d, data length %d",
			frameLength, len(data))
		return 0, 0, errors.New(message)
	}

	// The message type follows the header.
	messageType := uint(Getbitu(data, 24, 12))

	return length, messageType, nil
}

// ReadNextMessageFrame gets the next message frame from a reader.
// A message frame is a header, containing the message length, the
// message and a CRC.  It returns any read error that it encounters,
// such as EOF.
func (rtcm *RTCM) ReadNextMessageFrame(reader io.Reader) ([]byte, error) {

	var frame []byte

	for {
		// Eat data until we get the start of a message frame.
		err := eat(reader)
		if err != nil {
			var emptyFrame []byte
			return emptyFrame, err
		}

		// Start the new message frame.
		frame = append(frame, StartOfMessageFrame)

		// Get the rest of the message frame.
		var n uint = 1
		for {
			var frameLength uint = 0
			c, err := getChar(reader)
			if err != nil {
				// EOF while reading the final (incomplete) message frame. Abandon it.
				var emptyFrame []byte
				return emptyFrame, err
			}
			n++
			frame = append(frame, c)
			if n == headerLengthBytes+2 {
				// We have enough data to find the length and the type (which we don't need).
				// Get the expected frame length.
				messagelength, _, err := GetMessageLengthAndType(frame)
				if err != nil {
					// We thought we'd found the start of a message, but we haven't.
					// Abandon the collected data and continue scanning.
					break
				}
				frameLength = messagelength + headerLengthBytes + crcLengthBytes

			}

			if n == frameLength {

				// We have a complete message frame.  Check the CRC.
				if !CheckCRC(frame) {
					// This is not a valid message frame.  Abandon the
					// data collected so far and carry on scanning.
					break
				}

				// We have a complete and valid message frame.
				return frame, nil
			}
		}
	}
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
		log.Printf("GetMSMHeader: cellMask is %d bits - expected <= 64",
			cellMaskLength)
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

// GetMSM7SatelliteCells extracts the satellite cells from an MSM message.
// It returns the cells and the number of bits of the message bitstream
// consumed so far (which is the index of the start of the signal cells).
//
func (rtcm *RTCM) GetMSM7SatelliteCells(m []byte, h *MSMHeader, startOfSatelliteData uint) ([]MSM7SatelliteCell, uint) {
	satData := make([]MSM7SatelliteCell, len(h.Satellites))
	// If we have observed satellites 2, 3 and 15, h.Satellites will
	// contain {2, 3, 15}.  satData[0].SatelliteID will be 2, and so on.
	for i := range satData {
		satData[i].SatelliteID = h.Satellites[i]
	}
	// The satellite data bit stream is composed of a set of millisecond values
	// followed by a set of extended information values, and so on.
	pos := startOfSatelliteData
	for i := 0; i < int(len(h.Satellites)); i++ {
		satData[i].RangeMillisWhole = uint(Getbitu(m, pos, 8))
		pos += 8
	}
	for i := 0; i < int(len(h.Satellites)); i++ {
		satData[i].ExtendedInfo = uint(Getbitu(m, pos, 4))
		pos += 4
	}
	for i := 0; i < int(len(h.Satellites)); i++ {
		satData[i].RangeMillisFractional = uint(Getbitu(m, pos, 10))
		pos += 10
	}
	for i := 0; i < int(len(h.Satellites)); i++ {
		satData[i].PhaseRangeRate = int(Getbits(m, pos, 14))
		pos += 14
	}

	startOfSignalData := pos

	return satData, startOfSignalData
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
func (rtcm *RTCM) GetMSM7SignalCells(m []byte, h *MSMHeader, satData []MSM7SatelliteCell, startOfSignaldata uint) [][]MSM7SignalCell {
	// The number of cells is variable, NumSignalCells in the
	// header.  As explained in GetMSM7SatelliteCell, the bit
	// stream is laid out with all pseudo range values followed
	// by all phase range values, and so on.

	// Get the cell data into a more manageable form - a slice of
	// MSM7SignalCell objects, one for each expected signal cell.
	sigData := make([]MSM7SignalCell, h.NumSignalCells)
	pos := startOfSignaldata // Pos is the position within the bitstream.

	for i := 0; i < h.NumSignalCells; i++ {
		rangeDelta := int(Getbits(m, pos, 20))
		pos += 20
		sigData[i].RangeDelta = rangeDelta
	}
	for i := 0; i < h.NumSignalCells; i++ {
		sigData[i].PhaseRangeDelta = int(Getbits(m, pos, 24))
		pos += 24
	}
	for i := 0; i < h.NumSignalCells; i++ {
		sigData[i].LockTimeIndicator = uint(Getbitu(m, pos, 10))
		pos += 10
	}
	for i := 0; i < h.NumSignalCells; i++ {
		sigData[i].HalfCycleAmbiguity = (Getbitu(m, pos, 1) == 1)
		pos++
	}
	for i := 0; i < h.NumSignalCells; i++ {
		sigData[i].CNR = uint(Getbitu(m, pos, 10))
		pos += 10
	}
	for i := 0; i < h.NumSignalCells; i++ {
		sigData[i].PhaseRangeRateDelta = int(Getbits(m, pos, 15))
		pos += 15
	}

	// Set the satellite and signal numbers by cranking through the cell mask.
	var signalCell [][]MSM7SignalCell
	cell := 0
	for i := range h.CellMask {
		var cellSlice []MSM7SignalCell
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
			signal.RangeMetres = getRangeInMetres(satellite, signal)
			var err error
			signal.PhaseRangeCycles, err = getPhaseRangeCycles(h, satellite, signal)
			if err != nil {
				log.Printf("error getting phase range - %s\n", err.Error())
			}
			signal.PhaseRangeRateMS, err = getPhaseRangeRate(h, satellite, signal)
			if err != nil {
				log.Printf("error getting phase range rate - %s\n", err.Error())
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

// GetMSM7Message extracts the data from an MSM7 messages and presents it as
// broken out fields.
//
func (rtcm *RTCM) GetMSM7Message(frame []byte) *MSM7Message {

	header, startOfSatelliteData := rtcm.GetMSMHeader(frame)

	satellites, startOfSignals :=
		rtcm.GetMSM7SatelliteCells(frame, header, startOfSatelliteData)

	signals := rtcm.GetMSM7SignalCells(frame, header, satellites, startOfSignals)

	var message = MSM7Message{Header: header, Satellites: satellites, Signals: signals}

	return &message
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
func (rtcm *RTCM) DisplayMessage(message *Message) string {

	leader := fmt.Sprintf("message type %d\n", message.MessageType)
	leader += fmt.Sprintf("%s\n", hex.Dump(message.RawData))

	switch message.MessageType {
	case 1005:
		m, ok := message.Readable.(*Message1005)
		if !ok {
			return ("expected the readable message to be *Message1005\n")
		}
		return leader + rtcm.Display1005(m)
	case 1077:
		m, ok := message.Readable.(*MSM7Message)
		if !ok {
			return ("expected the readable message to be *MSM7Message\n")
		}
		return rtcm.DisplayMSM7(m)
	case 1087:
		m, ok := message.Readable.(*MSM7Message)
		if !ok {
			return ("expected the readable message to be *MSM7Message\n")
		}
		return rtcm.DisplayMSM7(m)
	case 1097:
		m, ok := message.Readable.(*MSM7Message)
		if !ok {
			return ("expected the readable message to be *MSM7Message\n")
		}
		return rtcm.DisplayMSM7(m)
	case 1127:
		m, ok := message.Readable.(*MSM7Message)
		if !ok {
			return ("expected the readble message to be *MSM7Message\n")
		}
		return rtcm.DisplayMSM7(m)
	}

	return fmt.Sprintf("%s message type %d\n", leader, message.MessageType)
}

// Display1005 returns a text version of a message type 1005
func (rtcm *RTCM) Display1005(message *Message1005) string {

	l1 := fmt.Sprintln("message type 1005 - Base Staion Information")

	l2 := fmt.Sprintf("stationID %d ITRF realisation year %d GPS %v GLONASS %v Galileo %v reference station %v ",
		message.StationID, message.ITRFRealisationYear,
		message.GPS, message.Glonass, message.Galileo,
		message.ReferenceStation)

	x, partx := GetScaled5(message.AntennaRefX)
	y, party := GetScaled5(message.AntennaRefY)
	z, partz := GetScaled5(message.AntennaRefX)
	l2 += fmt.Sprintf("oscilator %v quarter cycle %d ECEF coords (%d.%d,%d.%d,%d.%d)\n",
		message.Oscilator, message.QuarterCycle, x, partx, y, party, z, partz)
	return l1 + l2
}

// DisplayMSM7 returns a text version of an MSM7 message.
func (rtcm *RTCM) DisplayMSM7(message *MSM7Message) string {

	headerStr := rtcm.DisplayMSMHeader(message.Header)
	headerStr += "\n"

	satelliteStr := DisplayMSM7SatelliteCells(message.Satellites)
	satelliteStr += "\n"

	signalStr := DisplayMSM7SignalCells(message)
	signalStr += "\n"

	return headerStr + satelliteStr + signalStr
}

// DisplayMSMHeader return a text version of an MSMHeader.
func (rtcm *RTCM) DisplayMSMHeader(h *MSMHeader) string {
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

// DisplayMSM7SatelliteCells returns a text version of a slice of MSM7 satellite data
func DisplayMSM7SatelliteCells(satellites []MSM7SatelliteCell) string {
	l1 := "Satellites - satellite ID {range in millisecs (validity) extended info, phase range rate in metres/sec (validity)}\n"
	l2 := ""
	for i := range satellites {
		// Mark the range and phase values as valid or invalid.
		rangeValidity := "invalid"
		if satellites[i].RangeMillisWhole != rangeMillisWholeInvalidValue {
			rangeValidity = "valid"
		}
		phaseValidity := "invalid"
		if satellites[i].PhaseRangeRate != phaseRangeRateInvalidValue {
			phaseValidity = "valid"
		}
		l2 += fmt.Sprintf("%2d {%d.%d (%s) %d %d (%s)}\n", satellites[i],
			satellites[i].RangeMillisWhole, satellites[i].RangeMillisFractional, rangeValidity,
			satellites[i].ExtendedInfo,
			satellites[i].PhaseRangeRate, phaseValidity)
	}

	return l1 + l2
}

// DisplayMSM7SignalCells returns a text version of the MSM7 signal data from an MSM7Message.
func DisplayMSM7SignalCells(message *MSM7Message) string {
	l1 := "Signals - sat ID sig ID {range in metres (delta & validity) phase range in cycles (delta & validity)\n"
	l1 += "    lock time ind, half cycle ambiguity, Carrier Noise Ratio,  phase range rate (delta & validity)}\n"
	l2 := ""

	for _, list := range message.Signals {
		for _, signal := range list {
			// Mark the range and phase values as valid or invalid.
			rangeValidity := "invalid"
			if signal.RangeDelta != rangeDeltaInvalidValue {
				rangeValidity = "valid"
			}
			phaseRangeValidity := "invalid"
			if signal.PhaseRangeDelta != phaseRangeDeltaInvalidValue {
				phaseRangeValidity = "valid"
			}
			phaseRangeRateValidity := "invalid"
			if signal.PhaseRangeRateDelta != phaseRangeRateDeltaInvalidValue {
				phaseRangeRateValidity = "valid"
			}
			satelliteID := message.Header.Satellites[signal.Satellite]
			l2 += fmt.Sprintf("%2d %2d {%f (%d %s) %f (%d %s) %d, %v, %d, %f (%d %s)}\n",
				satelliteID, signal.SignalID,
				signal.RangeMetres, signal.RangeDelta, rangeValidity,
				signal.PhaseRangeCycles, signal.PhaseRangeRateDelta, phaseRangeValidity,
				signal.LockTimeIndicator, signal.HalfCycleAmbiguity, signal.CNR,
				signal.PhaseRangeRateMS, signal.PhaseRangeRateDelta, phaseRangeRateValidity)
		}
	}
	return l1 + l2
}

// getScaled5 takes a scaled integer a.b where b has 5 decimal places and returns
// the whole component a and the part component b, both as ints.
//
func GetScaled5(scaled5 int64) (int, int) {
	whole := int64(scaled5 / 100000)
	part := int64(scaled5) - (whole * 100000)
	if scaled5 < 0 {
		// Always return a positive part even when the input is negative.
		part = 0 - part
	}
	return int(whole), int(part)
}

// CheckCRC checks the CRC of a message frame.
func CheckCRC(frame []byte) bool {
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

// getMidnightOn(time) takes a date and time in any timezone, converts it to UTC and returns
// midnight at the start of that date.  For example, 2021-01-01 01:00:00 in Paris produces
// 2020-12-30 00:00:00 UTC.
func getMidnightOn(date time.Time) time.Time {
	date = date.In(locationUTC)
	return (time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, locationUTC))
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

// getChar gets the next byte from a reader, or returns an error.
func getChar(reader io.Reader) (byte, error) {
	buffer := make([]byte, 1)
	for {
		n, err := reader.Read(buffer)

		if err != nil {
			return 0, err
		}

		if n != 1 {
			log.Printf("no error but read %d bytes - expected 1 byte\n", n)
		}

		return buffer[0], nil
	}
}

// eat reads, eating bytes, looking for the start of a message frame or EOF.
// If it finds the start of a frame it return nil.  If it gets a read error
// (probably EOF) it returns the error.
func eat(reader io.Reader) error {
	for {
		c, err := getChar(reader)
		if err != nil {
			// Probably EOF
			return err
		}

		if c == StartOfMessageFrame {
			// Success!
			return nil
		}
	}
}

// getRangeInMetres combines the components of the range and returns the
// result in metres.  It returns zero if any values are invalid.
func getRangeInMetres(satellite *MSM7SatelliteCell, signal *MSM7SignalCell) float64 {

	if satellite.RangeMillisWhole == rangeMillisWholeInvalidValue {
		return 0.0
	}
	if signal.RangeDelta == rangeDeltaInvalidValue {
		return 0.0
	}
	// The range is in milliseconds (ie the signal transit time) and
	// provided in three parts, the 8-bit unsigned whole, the 10-bit
	// unsigned fraction and the 20-bit signed delta.  These are
	// scaled and combined into a 37-bit value.  The delta is
	// added or subtracted, according to its sign.  It's very small,
	// so the result is always still positive.  to preserve accuracy
	// the values are kept as scaled integers for as long as possible.
	//
	//     ------ Range -------
	//     whole     fractional
	//     w wwww wwwf ffff ffff f000 0000 0000 0000 0000
	//     + or - range delta    dddd dddd dddd dddd dddd

	var aggregateRange int64 = 0
	millis := int64(satellite.RangeMillisWhole<<29 |
		satellite.RangeMillisFractional<<19)
	aggregateRange = millis + int64(signal.RangeDelta)

	// The aggregate value is shifted up 29 bits.  Restore the scale to give the
	// range in milliseconds.
	rangeInMillis := float64(aggregateRange) * P2_29

	rangeInMetres := rangeInMillis * oneLightMillisecond

	return rangeInMetres
}

// getPhaseRangeCyclic combines the range and the phase range and
// returns the result in cycles.  It returns zero if any of the input
// measurements are invalid and an error if the signal is not in use.
//
func getPhaseRangeCycles(h *MSMHeader, satellite *MSM7SatelliteCell, signal *MSM7SignalCell) (float64, error) {

	if satellite.RangeMillisWhole == rangeMillisWholeInvalidValue {
		return 0.0, nil
	}

	if signal.PhaseRangeDelta == phaseRangeDeltaInvalidValue {
		return 0.0, nil
	}

	// The phase range is in cycles and derived from the range values
	// shifted up 31 bits plus the signed phase range delta.  This is
	// similar to getRangeInMetres but shifting by a different number
	// of bits.  Again, the values are kept as integers for as long as
	// possible to preserve accuracy.
	//
	//     ------ Range -------
	//     whole     fractional
	//     www wwww wfff ffff fff0 0000 0000 0000 0000 0000
	//     + or -             dddd dddd dddd dddd dddd dddd

	wavelength, err := getWavelength(h.Constellation, signal.SignalID)
	if err != nil {
		return 0.0, err
	}

	var aggregatePhaseRange int64 = 0
	millis := int64(satellite.RangeMillisWhole<<31 |
		satellite.RangeMillisFractional<<21)
	aggregatePhaseRange = millis + int64(signal.PhaseRangeDelta)

	// Restore the scale of the aggregate value - it's shifted up 31 bits.
	phaseRangeMilliSeconds := float64(aggregatePhaseRange) * P2_31

	// Convert to light milliseconds
	phaseRangeLMS := phaseRangeMilliSeconds * oneLightMillisecond

	// and divide by the wavelength to get cycles.
	phaseRangeCycles := phaseRangeLMS / wavelength

	return phaseRangeCycles, nil
}

// getPhaseRangeRate combines the components of the phase range rate
// and returns the result in milliseconds.  If either of the component
// values are invalid, it returns float zero.
//
func getPhaseRangeRate(h *MSMHeader, satellite *MSM7SatelliteCell, signal *MSM7SignalCell) (float64, error) {

	if satellite.PhaseRangeRate == phaseRangeRateInvalidValue {
		return 0.0, nil
	}
	if signal.PhaseRangeRateDelta == phaseRangeRateDeltaInvalidValue {
		return 0.0, nil
	}
	// The 14-bit signed phase range rate is in milliseconds and the
	// 15-bit signed delta is in ten thousandths of a millisecond.
	aggregatePhaseRangeRate := int64(satellite.PhaseRangeRate) * 10000
	aggregatePhaseRangeRate += int64(signal.PhaseRangeRateDelta)
	wavelength, err := getWavelength(h.Constellation, signal.SignalID)
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
