// The header package handles a Multiple Signal Message (MSM) header.
package header

import (
	"errors"
	"fmt"

	"github.com/goblimey/go-ntrip/rtcm/utils"
)

// Field lengths in bits.  The visible values are used by other packages.
const LenMessageType = 12
const LenStationID = 12
const LenTimeStamp = 30

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
const minBitsInHeader = LenMessageType + LenStationID +
	LenTimeStamp + lenMultipleMessageFlag + lenIssueOfDataStation +
	lenSessionTransmissionTime + lenClockSteeringIndicator +
	lenExternalClockIndicator + lenGNSSDivergenceFreeSmoothingIndicator +
	lenGNSSSmoothingInterval + lenSatelliteMask +
	lenSignalMask

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

	// Constellation - one of "GPS, "Beidou" etc.
	Constellation string

	// StationID - uint12.
	StationID uint

	// Timestamp - uint30.
	//
	// The structure of the 30 bits varies with the constellation.  For GPS,
	// Galileo and Beidou it's a single uint, the number of milliseconds from
	// the start of the current week.  For GPS and Galileo that starts on
	// Saturday UTC at a few leap seconds before midnight (in 2013, 18 seconds
	// before.  For Beidou it's similar but, in 2013, 4 seconds before midnight
	// UTC.  The maximum value is ((milliseconds in 7 days) - 1)
	// (utils.MaxTimestamp).
	//
	// For GLONASS the top three bits are the day of the week (0 is Sunday) and
	// the lower 27 bits are milliseconds from the start of the day in the
	// Moscow time zone, which is 3 hours ahead of UTC.  The maximum value is
	// day == 6, milliseconds == ((milliseconds in 24 hours) - 1),
	// (utils.MaxTimestampGlonass).
	Timestamp uint

	// MultipleMessage - bit(1) - true if more MSMs follow for this
	// constellation, station and timestamp.
	MultipleMessage bool

	// IssueOfDataStation - uint3. (Possibly the sequence number of the
	// message in a multiple message sequence?)
	IssueOfDataStation uint

	// SessionTransmissionTime - uint7.
	SessionTransmissionTime uint

	// ClockSteeringIndicator - uint2.
	ClockSteeringIndicator uint

	// ExternalClockSteeringIndicator - uint2.
	ExternalClockSteeringIndicator uint

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
	timestamp uint,
	multipleMessage bool,
	issueOfDataStation uint,
	sessionTransmissionTime uint,
	clockSteeringIndicator uint,
	externalClockSteeringIndicator uint,
	gnssDivergenceFreeSmoothingIndicator bool,
	gnssSmoothingInterval uint,
	satelliteMask uint64,
	signalMask uint32,
	cellMask uint64,
) *Header {

	constellation := utils.GetConstellation(messageType)

	satellites := getSatellites(satelliteMask)

	signals := getSignals(signalMask)

	cells := getCells(cellMask, len(satellites), len(signals))

	header := Header{
		MessageType:                          messageType,
		Constellation:                        constellation,
		StationID:                            stationID,
		Timestamp:                            timestamp,
		MultipleMessage:                      multipleMessage,
		IssueOfDataStation:                   issueOfDataStation,
		SessionTransmissionTime:              sessionTransmissionTime,
		ClockSteeringIndicator:               clockSteeringIndicator,
		ExternalClockSteeringIndicator:       externalClockSteeringIndicator,
		GNSSDivergenceFreeSmoothingIndicator: gnssDivergenceFreeSmoothingIndicator,
		GNSSSmoothingInterval:                gnssSmoothingInterval,
		SatelliteMask:                        satelliteMask,
		SignalMask:                           signalMask,
		CellMask:                             cellMask,
		Satellites:                           satellites,
		Signals:                              signals,
		Cells:                                cells,
	}

	// Set the number of signal cells.
	for i := range cells {
		for j := range cells[i] {
			if cells[i][j] {
				header.NumSignalCells++
			}
		}
	}

	return &header
}

// String return a text version of the MSMHeader.
func (header *Header) String() string {

	mode := "single message"
	if header.MultipleMessage {
		mode = "multiple message"
	}
	display := fmt.Sprintf("stationID %d, %s, issue of data station %d\n",
		header.StationID, mode, header.IssueOfDataStation)
	display += fmt.Sprintf("session transmit time %d, clock steering %d, external clock %d\n",
		header.SessionTransmissionTime, header.ClockSteeringIndicator,
		header.ExternalClockSteeringIndicator)
	display += fmt.Sprintf("divergence free smoothing %v, smoothing interval %d\n",
		header.GNSSDivergenceFreeSmoothingIndicator, header.GNSSSmoothingInterval)

	// Display the 64-bit satellite mask in 4 bit chunks.
	display += "Satellite mask:\n"
	for s := 60; s >= 0; s -= 4 {
		display += fmt.Sprintf("%04b", (header.SatelliteMask>>s)&0xf)
		if s > 0 {
			display += " "
			if s%16 == 0 {
				display += " " // An extra space every 16 bits.
			}
		}
	}
	display += "\n"

	// Display the 32-bit signal mask in 4 bit chunks.
	display += "Signal mask: "
	for s := 28; s >= 0; s -= 4 {
		display += fmt.Sprintf("%04b", (header.SignalMask>>s)&0xf)
		if s > 0 {
			display += " "
			if s%16 == 0 {
				display += " " // An extra space every 16 bits.
			}
		}
	}
	display += "\n"

	// Display the cell mask which is a slice of slices of bools.
	display += "cell mask:"
	for i, _ := range header.Cells {
		display += " "
		for j, _ := range header.Cells[i] {
			if header.Cells[i][j] {
				display += "t"
			} else {
				display += "f"
			}
		}

	}
	display += "\n"

	display += fmt.Sprintf("%d satellites, %d signal types, %d signals\n",
		len(header.Satellites), len(header.Signals), header.NumSignalCells)

	return display
}

// getTitle gets the title of the MSM.
func (header *Header) GetTitle() string {

	tc := utils.GetTitleAndComment(header.MessageType)
	return tc.Title
}

// GetMSMHeader extracts the header from an MSM message (MSM4 or MSM7).
// It returns the header data and the bit position of the start of the
// satellite data (which comes next in the bit stream).  If the bit stream
// is not long enough to hold the header, an error is returned.
func GetMSMHeader(bitStream []byte) (*Header, uint, error) {

	// The bit stream contains a 3-byte leader, a Multiple Signal Message and a
	// 3-byte CRC.  The MSM starts with an MSMHeader.
	//
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

	lenMessageInBits := (len(bitStream) - utils.LeaderLengthBytes - utils.CRCLengthBytes) * 8

	// We don't know the length of the header yet, but we have a minimum.
	// Check that.
	if lenMessageInBits < minBitsInHeader {
		// Error - not enough data.
		em := fmt.Sprintf("bitstream is too short for an MSM header - got %d bits, expected at least %d",
			lenMessageInBits, minBitsInHeader)
		return nil, 0, errors.New(em)
	}

	// Get a header object with the type value filled in.  (Delegating
	// this to a subsidiary function simplifies the testing.)
	messageType, pos, headerError := getMSMType(bitStream)

	if headerError != nil {
		return nil, 0, headerError
	}

	// Get the rest of the fixed-length values.
	stationID := uint(utils.GetBitsAsUint64(bitStream, pos, LenStationID))
	pos += LenStationID

	timestamp := uint(utils.GetBitsAsUint64(bitStream, pos, LenTimeStamp))
	pos += LenTimeStamp

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
	lengthRequired := utils.LeaderLengthBits + utils.CRCLengthBits +
		minBitsInHeader + lenCellMaskBits

	// Check that the bitstream is long enough.
	if bitStreamLength < lengthRequired {
		// Error - not enough data in the bit stream.
		em := fmt.Sprintf("bitstream is too short for an MSM header with %d cell mask bits - got %d bits, expected at least %d",
			lenCellMaskBits, lenBitStreamInBits, lengthRequired)
		return nil, 0, errors.New(em)

	}

	// The bitstream is long enough.  Get the cell list.

	cellMask := utils.GetBitsAsUint64(bitStream, pos, lenCellMaskBits)

	pos += lenCellMaskBits

	header := New(messageType, stationID, timestamp, multipleMessage, issueOfDataStation,
		sessionTransmissionTime, clockSteeringIndicator, externalClockIndicator,
		gnssDivergenceFreeSmoothingIndicator, gnssSmoothingInterval,
		satelliteMask, signalMask, cellMask)

	header.Satellites = satellites
	header.Signals = signals
	return header, pos, nil
}

// GetMSMType is a helper function for GetMSHeader.  It extracts the message type from the bit stream
// and returns it, plus the number of bits consumed.  An error is returned if the message is too short
// or not an MSM4 or an MSM7.
func getMSMType(bitStream []byte) (int, uint, error) {

	// The bit stream contains a 3-byte leader, an embedded message and a 3-byte CRC.
	// Here we are only concerned with the embedded message.
	lenBitStream := (len(bitStream) - utils.LeaderLengthBytes - utils.CRCLengthBytes) * 8
	if lenBitStream < LenMessageType {
		em := fmt.Sprintf("bit stream is %d bits long, too short for a message type", lenBitStream)
		return 0, 0, errors.New(em)
	}

	var pos uint = utils.LeaderLengthBits // Jump over the leader.
	messageType := int(utils.GetBitsAsUint64(bitStream, pos, LenMessageType))
	pos += LenMessageType

	// Check that the message type is an MSM.
	switch messageType {
	case utils.MessageTypeMSM4GPS: // GPS
		break
	case utils.MessageTypeMSM7GPS:
		break
	case utils.MessageTypeMSM4Glonass:
		break
	case utils.MessageTypeMSM7Glonass:
		break
	case utils.MessageTypeMSM4Galileo:
		break
	case utils.MessageTypeMSM7Galileo:
		break
	case utils.MessageTypeMSM4SBAS:
		break
	case utils.MessageTypeMSM7SBAS:
		break
	case utils.MessageTypeMSM4QZSS:
		break
	case utils.MessageTypeMSM7QZSS:
		break
	case utils.MessageTypeMSM4Beidou:
		break
	case utils.MessageTypeMSM7Beidou:
		break
	case utils.MessageTypeMSM4NavicIrnss:
		break
	case utils.MessageTypeMSM7NavicIrnss:
		break
	default:
		em := fmt.Sprintf("message type %d is not an MSM4 or an MSM7", messageType)
		return 0, 0, errors.New(em)
	}

	return messageType, pos, nil
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
	// array of (number of signals) X (number of satellites) bits.  The length is
	// always <= 64 and they are at the bottom end of cellMask .  If the receiver
	// observed two signals types and it observed both types from one satellite,
	/// just the first type from another and just the second type from a third
	// satellite, the mask will be:
	// bit 63              0
	//      000000....111001.
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

	shift := (numberOfSatellites * numberOfSignalTypes) - 1
	cellNumber := 0
	cells := make([][]bool, 0)
	for i := 0; i < numberOfSatellites; i++ {
		row := make([]bool, 0)
		for j := 0; j < numberOfSignalTypes; j++ {
			cellNumber++
			// bit 63 is first bit, bit 62 is second bit ...
			// shift := uint(bitsInCellMask - cellNumber)
			// Shift down to remove the lower bits.
			shiftedDown := cellMask >> shift
			// Isolate the bit we want.
			bit := shiftedDown & 1
			val := bit == 1
			row = append(row, val)
			shift--
		}
		cells = append(cells, row)
	}
	return cells
}
