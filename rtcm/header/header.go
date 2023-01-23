// The header package handles a Multiple Signal Message (MSM) header.
package header

import (
	"errors"
	"fmt"
	"time"

	"github.com/goblimey/go-ntrip/rtcm/utils"
)

// Field lengths in bits.
const lenMessageType = 12
const lenStationID = 12
const lenEpochTime = 30
const lenMultipleMessageFlag = 1
const lenIssueOfDataStation = 3
const lenSessionTransmissionTime = 7
const lenClockSteeringIndicator = 2
const lenExternalClockSteeringIndicator = 2
const lenExternalClockIndicator = 2
const lenGNSSDivergenceFreeSmoothingIndicator = 1
const lenGNSSSmoothingInterval = 3
const lenSatelliteMask = 64
const lenSignalMask = 32

// The maximum length of the cell mask.
const maxLengthOfCellMask = 64

// The minimum length of an MSM header
const minBitsInHeader = lenMessageType + lenStationID +
	lenEpochTime + lenMultipleMessageFlag + lenIssueOfDataStation +
	lenSessionTransmissionTime + lenClockSteeringIndicator +
	lenExternalClockIndicator + lenGNSSDivergenceFreeSmoothingIndicator +
	lenGNSSSmoothingInterval + lenSatelliteMask +
	lenSignalMask

// dateLayout defines the layout of dates when they are displayed.  It
// produces "yyyy-mm-dd hh:mm:ss.ms timeshift timezone".
const dateLayout = "2006-01-02 15:04:05.999 -0700 MST"

// Header holds the header for MSM Messages.  Message types 1074,
// 1077, 1084, 1087 etc have an MSM header at the start.
type Header struct {

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

	// MultipleMessage - bit(1) - true if more MSMs follow for this
	// constellation, station and epoch time.
	MultipleMessage bool

	// IssueOfDataStation - uint3. (Possibly the position of the message in a
	// multiple message sequence?)
	IssueOfDataStation uint

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

	// SatelliteMask is 64 bits, one bit per satellite.  Bit 63
	// is set if signals were observed from satellite 1, bit 62 for
	// satellite 2 and so on.  For example 101000..... means that
	// signals from satellites 1 and 3 were observed.
	SatelliteMask uint64

	// SignalMask is 32 bits, one per signal.  Bit 31 is set if
	// signal 1 was observed from any satellite, bit 30 is set if
	// signal 2 was observed from any satellite, and so on.  For
	// example if signal 1, 3 and 5 were observed from one satellite
	// and signals 1 and 2 from another, then bits 1, 2, 3 and 5
	// will be set in the signal mask.
	SignalMask uint32

	// CellMask is an array of bits nSat X nSig where nSat is the
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
	CellMask uint64

	// Satellites is a slice made from the Satellite Mask bits.  If the satellite
	// mask has bits 63 and 61 set, signals were observed from satellites 1 and 3,
	// the slice will have two elements and will contain {1, 3}.
	Satellites []uint

	// Signals is a slice made from the signal mask.  If signals 1, 2, 3 and 5 were
	// observed from the satellites, the slice will have four elements and will
	// contain {1, 2, 3, 5}
	Signals []uint

	// Cells is a slice of slices representing the cell mask.  For example, if
	// signals 1,2,3 and 5 were observed from satellites 1 and 3, the Cells might
	// be arranged like so: {{t,f,t,t}, {t,t,f,f}}, meaning that signals 1, 3 and 5
	// were observed from satellite 1, no signals were observed from Satellite 2 and
	// signals 1 and 2 were observed from satellite 3.  (At present (2022) the
	// satellites are dual band, so actually they will send two signals and the
	// receiver might observe zero, one or both of them.)
	Cells [][]bool

	// NumSignalCells is the total number of signal cells in the message.  Creating
	// this count avoids having to rummage through the masks when you need to know
	// its value.
	NumSignalCells int
}

// New creates a Header.
func New(
	messageType int,
	stationID uint,
	epochTime uint,
	multipleMessage bool,
	issueOfDataStation uint,
	sessionTransmissionTime uint,
	clockSteeringIndicator uint,
	externalClockIndicator uint,
	gnssDivergenceFreeSmoothingIndicator bool,
	gnssSmoothingInterval uint,
	satelliteMask uint64,
	signalMask uint32,
	cellMask uint64,
) *Header {

	constellation := getConstellation(messageType)

	satellites := getSatellites(satelliteMask)

	signals := getSignals(signalMask)

	numberOfSignalCells := len(satellites) * len(signals)

	cells := getCells(cellMask, len(satellites), len(signals))

	header := Header{
		MessageType:                          messageType,
		Constellation:                        constellation,
		StationID:                            stationID,
		EpochTime:                            epochTime,
		MultipleMessage:                      multipleMessage,
		IssueOfDataStation:                   issueOfDataStation,
		SessionTransmissionTime:              sessionTransmissionTime,
		ClockSteeringIndicator:               clockSteeringIndicator,
		ExternalClockIndicator:               externalClockIndicator,
		GNSSDivergenceFreeSmoothingIndicator: gnssDivergenceFreeSmoothingIndicator,
		GNSSSmoothingInterval:                gnssSmoothingInterval,
		NumSignalCells:                       numberOfSignalCells,
		SatelliteMask:                        satelliteMask,
		SignalMask:                           signalMask,
		CellMask:                             cellMask,
		Satellites:                           satellites,
		Signals:                              signals,
		Cells:                                cells,
	}

	return &header
}

// NewWithMessageType creates an MSM header with the message type set (and the
// constellation, which is derived from the message type).
func NewWithMessageType(messageType int) *Header {

	constellation := getConstellation(messageType)

	// Create and return the header.
	header := Header{MessageType: messageType, Constellation: constellation}

	return &header
}

// String return a text version of the MSMHeader.
func (header *Header) String() string {

	title := header.getTitle()

	line := fmt.Sprintf("type %d %s %s\n",
		header.MessageType, header.Constellation, title)

	timeStr := header.UTCTime.Format(dateLayout)
	line += fmt.Sprintf("time %s (epoch time %d)\n", timeStr, header.EpochTime)
	mode := "single"
	if header.MultipleMessage {
		mode = "multiple"
	}
	line += fmt.Sprintf("stationID %d, %s message, sequence number %d, session transmit time %d\n",
		header.StationID, mode, header.IssueOfDataStation, header.SessionTransmissionTime)
	line += fmt.Sprintf("clock steering %d, external clock %d\n",
		header.ClockSteeringIndicator, header.ExternalClockIndicator)
	line += fmt.Sprintf("divergence free smoothing %v, smoothing interval %d\n",
		header.GNSSDivergenceFreeSmoothingIndicator, header.GNSSSmoothingInterval)
	line += fmt.Sprintf("%d satellites, %d signal types, %d signals\n",
		len(header.Satellites), len(header.Signals), header.NumSignalCells)

	return line
}

// getTitle is a helper for Display.  It gets the title for the display.
func (header *Header) getTitle() string {

	const titleMSM4 = "Full Pseudoranges and PhaseRanges plus CNR"
	const titleMSM7 = "Full Pseudoranges and PhaseRanges plus CNR (high resolution)"

	var title string

	switch header.MessageType {
	case 1074:
		title = titleMSM4
	case 1077:
		title = titleMSM7
	case 1084:
		title = titleMSM4
	case 1087:
		title = titleMSM7
	case 1094:
		title = titleMSM4
	case 1097:
		title = titleMSM7
	case 1104:
		title = titleMSM4
	case 1107:
		title = titleMSM7
	case 1114:
		title = titleMSM4
	case 1117:
		title = titleMSM7
	case 1124:
		title = titleMSM4
	case 1127:
		title = titleMSM7
	case 1134:
		title = titleMSM4
	case 1137:
		title = titleMSM7
	default:
		title = "Unknown MSM type"
	}

	return title
}

// GetMSMHeader extracts the header from an MSM message (MSM4 or MSM7).
// It returns the header data and the bit position of the start of the
// satellite data (which comes next in the bit stream).  If the bit stream
// is not long enough to hold the header, an error is returned.
//
func GetMSMHeader(bitStream []byte) (*Header, uint, error) {

	// The MSMHeader contains:
	//    a 12-bit unsigned message type (1074, 1084 ... MSM4. 1077 ... MSM7.)
	//    a 12-bit unsigned station ID
	//    a 30-bit unsigned timestamp
	//    a boolean multiple message flag
	//    a 3-bit unsigned Issue Of Data Station
	//    a 7-bit unsigned session transmission time value
	//    a 2-bit unsigned clock steering indicator
	//    a 2-bit unsigned external clock indicator
	//    a boolean GNSS Divergence Free Smoothing Indicator
	//    a 3-bit GNSS Smoothing Interval
	//    a 64-bit satellite mask (one bit set per satellite observed)
	//    a 32-bit signal mask (one bit set per signal type observed)
	//    a cell mask (nSatellites X nSignals) bits long, 64 bits or less
	//
	// The message contains satellite and signal data from one scan of one
	// constellation of satellites.  The message has a limited length.  If the
	// scan produces more data than can fit in one message, it's broken into
	// several messages.  the multiple message flags are set and the Issue of Data
	// Station field provides the sequence number.
	//
	// The satellite, signal and cell masks together show which signals were received
	// ("observed") from which satellites.  The uppermost bit of the satellite mask
	// (bit 63) is set if signals were observed from satellite 1 of the constellation,
	// bit 62 is set if signals were received from satellite 2, and so on.  The signal
	// mask works in a similar way but represents all of the signal types received - if
	// signal 3 was received from satellite 7, signals 3 and 5 were received from
	// satellite 9 and signal 5 was received from satellite 15 then signals of type 3
	// and 5 were received in that scan.  The bits for satellites 7, 9 and 15 would be
	// set in the satellite mask and the bits for signals 3 and 5 would be set in the
	// signal mask.  The cell mask would be an array 3X2 bits long "10 11 01", showing
	// that only the first signal was received from the first satellite, both signals
	// were received from the second, and only the second signal from the third
	// satellite.  The message would contain three satellite cells and four signal cells.
	//
	// The cell mask is variable length but no more than 64 bits.  If that's not enough
	// (for example, 6 signals types received from 12 satellites would need 72 bits)
	// then the data for the scan would be sent in a series of messages.
	//
	// The function returns the broken out header and the number of bits consumed, which
	// is the position in the bit stream of the start of the next part of the message,
	// the first satellite cell.

	// lenBitStreamInBits is the length of the bitstream in bits, including
	// the 3-byte message leader.
	lenBitStreamInBits := len(bitStream) * 8

	// We don't know the length of the header yet, but we have a minimum.
	// Check that.
	if lenBitStreamInBits < minBitsInHeader {
		// Error - not enough data.
		em := fmt.Sprintf("bitstream is too short for an MSM header - got %d bits, expected at least %d",
			lenBitStreamInBits, minBitsInHeader)
		return nil, 0, errors.New(em)
	}

	// Get a header object with the type value filled in.  (Delegating
	// this to a subsidiary function simplifies the testing.)
	messageType, pos, headerError := getMSMType(bitStream)

	if headerError != nil {
		return nil, 0, headerError
	}

	// Get the rest of the fixed-length values.
	stationID := uint(utils.GetBitsAsUint64(bitStream, pos, lenStationID))
	pos += lenStationID

	epochTime := uint(utils.GetBitsAsUint64(bitStream, pos, lenEpochTime))
	pos += lenEpochTime

	mm := utils.GetBitsAsUint64(bitStream, pos, lenMultipleMessageFlag)
	multipleMessage := (mm == 1)
	pos += lenMultipleMessageFlag

	issueOfDataStation := uint(utils.GetBitsAsUint64(bitStream, pos,
		lenIssueOfDataStation))
	pos += lenIssueOfDataStation

	sessionTransmissionTime := uint(utils.GetBitsAsUint64(bitStream, pos,
		lenSessionTransmissionTime))
	pos += lenSessionTransmissionTime

	clockSteeringIndicator := uint(utils.GetBitsAsUint64(bitStream, pos,
		lenClockSteeringIndicator))
	pos += lenClockSteeringIndicator

	externalClockIndicator := uint(utils.GetBitsAsUint64(bitStream, pos,
		lenExternalClockSteeringIndicator))
	pos += lenExternalClockSteeringIndicator

	giFlag := utils.GetBitsAsUint64(bitStream, pos, lenGNSSDivergenceFreeSmoothingIndicator)
	gnssDivergenceFreeSmoothingIndicator := (giFlag == 1)
	pos += lenGNSSDivergenceFreeSmoothingIndicator

	gnssSmoothingInterval := uint(utils.GetBitsAsUint64(bitStream, pos,
		lenGNSSSmoothingInterval))
	pos += lenGNSSSmoothingInterval

	satelliteMask :=
		uint64(utils.GetBitsAsUint64(bitStream, pos, lenSatelliteMask))
	pos += lenSatelliteMask
	// Create a slice of satellite IDs, advancing the bit position as we go.
	// Bit 63 of the mask is satellite number 1, bit 62 is 2, bit 0 is 64.
	// If signals were observed from satellites 3, 7 and 9, the slice will
	// contain {3, 7, 9}.  (Note, we then expect to see 3 signal cells later
	// in the message.)
	// header.Satellites = make([]uint, 0)
	// for satNum := 1; satNum <= lenSatelliteMaskBits; satNum++ {
	// 	if utils.GetBitsAsUint64(bitStream, pos, 1) == 1 {
	// 		header.Satellites = append(header.Satellites, uint(satNum))
	// 	}
	// 	pos++
	// }

	satellites := getSatellites(satelliteMask)

	signalMask := uint32(utils.GetBitsAsUint64(bitStream, pos, lenSignalMask))

	pos += lenSignalMask
	signals := getSignals(signalMask)

	// The last component of the header is the cell mask.  This is variable
	// length - (number of signals) X (number of satellites) bits, no more
	// than maxLengthOfCellMask bits long.  Now that we know those values, we
	// can calculate the expected message length and do some final sanity
	// checks.
	//
	lenCellMaskBits := uint(len(satellites) * len(signals))

	if lenCellMaskBits > maxLengthOfCellMask {
		em := fmt.Sprintf("GetMSMHeader: cellMask is %d bits - expected <= %d",
			lenCellMaskBits, maxLengthOfCellMask)
		return nil, 0, errors.New(em)
	}

	// We now know the length of the (variable-length) header.  Check that the bit stream is long
	// enough to contain it.
	//
	// bitStreamLength is the number of bits in the bitstream.
	bitStreamLength := uint(len(bitStream) * 8)

	// lengthRequired is the required minimum length of the bitstream.
	lengthRequired := minBitsInHeader + lenCellMaskBits

	// Check that the bitstream is long enough.
	if bitStreamLength < lengthRequired {
		// Error - not enough data in the bit stream.
		em := fmt.Sprintf("bitstream is too short for an MSM header with %d cell mask bits - got %d bits, expected at least %d",
			lenCellMaskBits, lenBitStreamInBits, lengthRequired)
		return nil, 0, errors.New(em)

	}

	// The bitstream is long enough.  Get the cell list.

	cellMask := utils.GetBitsAsUint64(bitStream, pos, lenCellMaskBits)

	header := New(messageType, stationID, epochTime, multipleMessage, issueOfDataStation,
		sessionTransmissionTime, clockSteeringIndicator, externalClockIndicator,
		gnssDivergenceFreeSmoothingIndicator, gnssSmoothingInterval,
		satelliteMask, signalMask, cellMask)
	header.Satellites = satellites
	header.Signals = signals
	header.NumSignalCells = len(satellites) * len(signals)
	return header, pos, nil
}

// GetMSMType is a helper function for GetMSHeader.  It extracts the message type from the bit stream
// and returns it, plus the number of bits consumed.  An error is returned if the message is too short
// or not an MSM4 or an MSM7.
//
func getMSMType(bitStream []byte) (int, uint, error) {

	lenBitStream := len(bitStream) * 8
	if lenBitStream < lenMessageType {
		em := fmt.Sprintf("bit stream is %d bits long, too short for a message type", lenBitStream)
		return 0, 0, errors.New(em)
	}

	var pos uint = 0
	messageType := int(utils.GetBitsAsUint64(bitStream, pos, lenMessageType))
	pos += lenMessageType

	// Check that the message type is an MSM.
	switch messageType {
	case 1074: // GPS
		break
	case 1077:
		break
	case 1084: // Glonass
		break
	case 1087:
		break
	case 1094:
		break
	case 1097:
		break
	case 1104: // SBAS
		break
	case 1107:
		break
	case 1114: // QZSS
		break
	case 1117:
		break
	case 1124: // Beidou
		break
	case 1127:
		break
	case 1134: // NavIC/IRNSS
		break
	case 1137:
		break
	default:
		em := fmt.Sprintf("message type %d is not an MSM4 or an MSM7", messageType)
		return 0, 0, errors.New(em)
	}

	return messageType, pos, nil
}

// getConstellation is a helper function for New and NewWithMessageType.  Given
// a message type it returns the constellation.
func getConstellation(messageType int) string {

	var constellation string

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
		constellation = "unknown"
	}

	return constellation
}

// getSatellites gets a satellite list from the given bit mask.
func getSatellites(satelliteMask uint64) []uint {

	// Bit 63 of the mask is satellite number 1, bit 62 is 2, bit 0 is 64.
	// If signals were observed from satellites 3, 7 and 9, the slice will
	// contain {3, 7, 9}.  (Note, we then expect to see 3 satellite cells
	// in the message.)
	satellites := make([]uint, 0)
	for satNum := 1; satNum <= lenSatelliteMask; satNum++ {
		// Satellite 1 is bit 63, satellite 2 is bit 62 ...
		bitPosition := lenSatelliteMask - satNum
		// Shift down to remove the lower bits.
		shiftedDown := satelliteMask >> bitPosition
		// Mask out any higher bits to isolate the bit we want.
		bit := shiftedDown & 1
		if bit == 1 {
			satellites = append(satellites, uint(satNum))
		}
	}

	return satellites
}

// getSignals gets a signal list from the given bit mask.
func getSignals(signalMask uint32) []uint {
	// Bit 31 of the mask is signal number 1, bit 30 is 2, bit 0 is 32.
	// If we observed signals 62 and 64, the mask bits will be 0x0005
	// and the slice will contain {62, 64}.
	signals := make([]uint, 0)
	for sigNum := 1; sigNum <= lenSignalMask; sigNum++ {
		// Satellite 1 is bit 63, satellite 2 is bit 62 ...
		bitPosition := lenSignalMask - sigNum
		// Shift down to remove the lower bits.
		shiftedDown := signalMask >> bitPosition
		// Mask out any higher bits to isolate the bit we want.
		bit := shiftedDown & 1
		if bit == 1 {
			signals = append(signals, uint(sigNum))
		}
	}

	return signals
}

// getCells gets the cell list
func getCells(cellMask uint64, numberOfSatellites, numberOfSignalTypes int) [][]bool {
	// Get the cell mask as a string of bits.  The bits form a two-dimensional
	// array of (number of signals) X (number of satellites) bits.  The length
	// is always <= 64.  If the receiver observed two signals types and it
	// observed both types from one satellite, just the first type from another
	// and just the second type from a third satellite, the mask will be 11 10 01.
	//
	// Create a slice of slices of bools, each bool representing a bit in the
	// cell mask.  If the receiver observed two signals types and it observed
	// them from 3 satellites, the cell mask might be:  {{t,t, f}, {t,f, t}}
	// meaning that the device received both signal types from the first satellite,
	// just the first signal type from the second satellite and just the second
	// signal type from the third satellite.  (Note: given that in that example
	// four items are set true, we also expect to see four signal cells later.
	// However, if there are a lot of signal cells, they may be spread over
	// several messages.

	numberOfCells := numberOfSatellites * numberOfSignalTypes
	cellNumber := 0
	cells := make([][]bool, 0)
	for i := 0; i < numberOfSatellites; i++ {
		row := make([]bool, 0)
		for j := 0; j < numberOfSignalTypes; j++ {
			cellNumber++
			bitPosition := numberOfCells - cellNumber
			// Shift down to remove the lower bits.
			shiftedDown := cellMask >> bitPosition
			// Mask out any higher bits to isolate the bit we want.
			bit := shiftedDown & 1
			val := bit == 1
			row = append(row, val)
		}
		cells = append(cells, row)
	}
	return cells
}
