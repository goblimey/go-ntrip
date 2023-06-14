// type1006 handles messages of type 1006 - Stationary RTK Reference Station ARP
// with Antenna Height (base position and height).
package type1006

import (
	"errors"
	"fmt"

	"github.com/goblimey/go-ntrip/rtcm/utils"
)

const expectedMessageType = 1006

// Lengths of the fields in the bit stream.
const lenMessageType = 12
const lenStationID = 12
const lenITRFRealisationYear = 6
const lenIgnoredBits1 = 4
const lenAntennaRefX = 38
const lenIgnoredBits2 = 2
const lenAntennaRefY = 38
const lenIgnoredBits3 = 2
const lenAntennaRefZ = 38
const lenAntennaHeight = 16

const lengthOfMessageInBits = lenMessageType + lenStationID +
	lenITRFRealisationYear + lenIgnoredBits1 +
	lenAntennaRefX + lenIgnoredBits2 + lenAntennaRefY +
	lenIgnoredBits3 + lenAntennaRefZ + lenAntennaHeight

// Message contains a message of type 1006 - antenna position and height.
type Message struct {
	// Some bits in the message are ignored by the RTKLIB decoder so
	// we're not sure what they are.  We just store them for display.

	// MessageType - uint12 - always 1006.
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

	// AntennaHeight is the height of the antenna above some base height
	// (for example the height above ground level).
	AntennaHeight uint `json:"antenna_height,omitempty"`
}

func New(stationID, itrfRealisationYear, ignored1 uint,
	antennaRefX int64, ignored2 uint, antennaRefY int64, ignored3 uint, antennaRefZ int64,
	antennaHeight uint) *Message {

	message := Message{
		MessageType:         utils.MessageType1006,
		StationID:           stationID,
		ITRFRealisationYear: itrfRealisationYear,
		Ignored1:            ignored1,
		AntennaRefX:         antennaRefX,
		Ignored2:            ignored2,
		AntennaRefY:         antennaRefY,
		Ignored3:            ignored3,
		AntennaRefZ:         antennaRefZ,
		AntennaHeight:       antennaHeight,
	}

	return &message
}

// String returns a text version of a message type 1006
func (message *Message) String() string {

	display := fmt.Sprintf("stationID %d, ITRF realisation year %d, unknown bits %04b,\n",
		message.StationID, message.ITRFRealisationYear, message.Ignored1)
	display += fmt.Sprintf("x %d, unknown bits %02b, y %d, unknown bits %02b, z %d,\n",
		message.AntennaRefX, message.Ignored2, message.AntennaRefY,
		message.Ignored3, message.AntennaRefZ)

	// The Antenna Reference coordinates and the height are in units of 1/10,000 of a metre.
	const scaleFactor = 0.0001
	x := float64(message.AntennaRefX) * scaleFactor
	y := float64(message.AntennaRefY) * scaleFactor
	z := float64(message.AntennaRefZ) * scaleFactor
	height := float64(message.AntennaHeight) * 0.0001

	display += fmt.Sprintf("ECEF coords in metres (%.4f, %.4f, %.4f)\n", x, y, z)
	display += fmt.Sprintf("Antenna height %.4f\n", height)
	return display
}

// GetMessage returns a text version of a message type 1006
func GetMessage(bitStream []byte) (*Message, error) {

	// The bit stream contains a 3-byte leader, an embedded message and a 3-byte CRC.
	// Here we are only concerned with the embedded message.
	lenBitStream := len(bitStream) * 8
	lenMessageInBits := lenBitStream - utils.LeaderLengthBits - utils.CRCLengthBits

	// Check that the bit stream is long enough.
	if lenMessageInBits < lengthOfMessageInBits {
		errorMessage := fmt.Sprintf("overrun - expected %d bits in a message type 1006, got %d",
			lengthOfMessageInBits, lenMessageInBits)
		return nil, errors.New(errorMessage)
	}

	// Pos is the position within the bitstream.
	// Jump over the leader.
	var pos uint = utils.LeaderLengthBits

	messageType := uint(utils.GetBitsAsUint64(bitStream, pos, lenMessageType))
	pos += lenMessageType

	// Sanity check.
	if messageType != expectedMessageType {
		em := fmt.Sprintf("expected message type %d got %d",
			expectedMessageType, messageType)
		return nil, errors.New(em)
	}

	stationID := uint(utils.GetBitsAsUint64(bitStream, pos, lenStationID))
	pos += lenStationID
	itrfRealisationYear := uint(utils.GetBitsAsUint64(bitStream, pos, lenITRFRealisationYear))
	pos += lenITRFRealisationYear
	ignored1 := uint(utils.GetBitsAsUint64(bitStream, pos, lenIgnoredBits1))
	pos += lenIgnoredBits1
	antennaRefX := utils.GetBitsAsInt64(bitStream, pos, lenAntennaRefX)
	pos += lenAntennaRefX
	ignored2 := uint(utils.GetBitsAsUint64(bitStream, pos, lenIgnoredBits2))
	pos += lenIgnoredBits2
	antennaRefY := utils.GetBitsAsInt64(bitStream, pos, lenAntennaRefY)
	pos += lenAntennaRefY
	ignored3 := uint(utils.GetBitsAsUint64(bitStream, pos, lenIgnoredBits3))
	pos += lenIgnoredBits3
	antennaRefZ := utils.GetBitsAsInt64(bitStream, pos, lenAntennaRefZ)
	pos += lenAntennaRefZ
	antennaHeight := uint(utils.GetBitsAsUint64(bitStream, pos, lenAntennaHeight))
	pos += lenAntennaHeight

	message := New(stationID, itrfRealisationYear, ignored1,
		antennaRefX, ignored2, antennaRefY, ignored3, antennaRefZ, antennaHeight)
	return message, nil
}
