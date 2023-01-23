package message1005

import (
	"errors"
	"fmt"

	"github.com/goblimey/go-ntrip/rtcm/utils"
)

// Message contains a message of type 1005 - antenna position.
type Message struct {
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

// display1005 returns a text version of a message type 1005
func (message *Message) Display() string {

	l1 := fmt.Sprintln("message type 1005 - Base Station Information")

	l2 := fmt.Sprintf("stationID %d, ITRF realisation year %d, ignored 0x%4x,\n",
		message.StationID, message.ITRFRealisationYear, message.Ignored1)
	l2 += fmt.Sprintf("x %d ignored 0x%2x, y %d, ignored 0x%2x z %d,\n",
		message.AntennaRefX, message.Ignored2, message.AntennaRefY,
		message.Ignored3, message.AntennaRefZ)

	// The Antenna Reference coordinates are in units of 1/10,000 of a metre.
	const scaleFactor = 0.0001
	x := float64(message.AntennaRefX) * scaleFactor
	y := float64(message.AntennaRefY) * scaleFactor
	z := float64(message.AntennaRefZ) * scaleFactor
	l2 += fmt.Sprintf("ECEF coords in metres (%8.4f,%8.4f,%8.4f)\n",
		x, y, z)
	return l1 + l2
}

// GetMessage1005 returns a text version of a message type 1005
func GetMessage(bitStream []byte) (*Message, error) {

	const lenMessageType = 12
	const lenStationID = 12
	const lenITRFRealisationYear = 6
	const lenIgnoredBits1 = 4
	const lenAntennaRefX = 38
	const lenIgnoredBits2 = 2
	const lenAntennaRefY = 38
	const lenIgnoredBits3 = 2
	const lenAntennaRefZ = 38

	const bitsExpected = lenMessageType + lenStationID + lenITRFRealisationYear +
		lenIgnoredBits1 + lenAntennaRefX + lenIgnoredBits2 + lenAntennaRefY +
		lenIgnoredBits3 + lenAntennaRefZ

	gotBits := len(bitStream) * 8

	// Check that the bit stream is long enough.
	if gotBits < bitsExpected {
		errorMessage := fmt.Sprintf("overrun - expected %d bits in a message type 1005, got %d",
			bitsExpected, gotBits)
		return nil, errors.New(errorMessage)
	}

	// Pos is the position within the bitstream.
	var pos uint = 0

	var result Message

	result.MessageType = uint(utils.GetBitsAsUint64(bitStream, pos, lenMessageType))
	pos += lenMessageType
	result.StationID = uint(utils.GetBitsAsUint64(bitStream, pos, lenStationID))
	pos += lenStationID
	result.ITRFRealisationYear = uint(utils.GetBitsAsUint64(bitStream, pos, lenITRFRealisationYear))
	pos += lenITRFRealisationYear
	result.Ignored1 = uint(utils.GetBitsAsUint64(bitStream, pos, lenIgnoredBits1))
	pos += lenIgnoredBits1
	result.AntennaRefX = utils.GetBitsAsInt64(bitStream, pos, lenAntennaRefX)
	pos += lenAntennaRefX
	result.Ignored2 = uint(utils.GetBitsAsUint64(bitStream, pos, lenIgnoredBits2))
	pos += lenIgnoredBits2
	result.AntennaRefY = utils.GetBitsAsInt64(bitStream, pos, lenAntennaRefY)
	pos += lenAntennaRefY
	result.Ignored3 = uint(utils.GetBitsAsUint64(bitStream, pos, lenIgnoredBits3))
	pos += lenIgnoredBits3
	result.AntennaRefZ = utils.GetBitsAsInt64(bitStream, pos, lenAntennaRefZ)
	pos += lenAntennaRefZ

	return &result, nil
}
