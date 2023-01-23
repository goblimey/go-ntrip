package message

import (
	"fmt"

	"github.com/goblimey/go-ntrip/rtcm/header"
	"github.com/goblimey/go-ntrip/rtcm/msm4/satellite"
	"github.com/goblimey/go-ntrip/rtcm/msm4/signal"
)

// Message is a broken-out version of an MSM4 message.
type Message struct {
	// Header is the MSM Header
	Header *header.Header

	// Satellites is a list of the satellites for which signals
	// were observed in an MSM7 message.
	Satellites []satellite.Cell

	// Signals is a list of sublists, one sublist per satellite,
	// of signals at different frequencies observed by the base
	// station from the satellites in the Satellite list.
	Signals [][]signal.Cell
}

// New creates an MSM4 Message.
func New(header *header.Header, satellites []satellite.Cell, signals [][]signal.Cell) *Message {
	message := Message{Header: header, Satellites: satellites, Signals: signals}

	return &message
}

// Display return a text version of the MSM4Message.
func (message *Message) Display() string {
	result :=
		message.Header.String() +
			message.DisplaySatelliteCells() +
			message.DisplaySignalCells()

	return result
}

// DisplaySatelliteCells returns a text version of the satellite cells in the
// Multiple Signal Message (MSM).
func (message *Message) DisplaySatelliteCells() string {

	if len(message.Satellites) < 1 {
		return "No satellites\n"
	}

	heading := ""

	heading = fmt.Sprintf("%d Satellites\nsatellite ID {range ms}\n",
		len(message.Satellites))

	body := ""
	for i := range message.Satellites {
		body += message.Satellites[i].String()
	}

	return heading + body
}

// DisplaySignalCells returns a text version of the signal data from the signal
// cells in a type 4 multiple signal message.
func (message *Message) DisplaySignalCells() string {

	if len(message.Signals) < 1 {
		return "No signals|n"
	}

	var heading string

	heading = fmt.Sprintf("%d Signals\nsat ID sig ID {range (delta), lock time ind, half cycle ambiguity,\n",
		len(message.Signals))
	heading += "        Carrier Noise Ratio}\n"

	body := ""

	for i := range message.Signals {
		for j := range message.Signals[i] {
			body += fmt.Sprintf("%s\n", message.Signals[i][j].String())
		}
	}

	return heading + body
}

// GetMessage presents an MSM4 (type 1074, 1084 etc) as  broken out fields.
func GetMessage(bitStream []byte) (*Message, error) {

	header, bitPosition, headerError := header.GetMSMHeader(bitStream)

	if headerError != nil {
		return nil, headerError
	}

	satellites, fetchSatellitesError :=
		satellite.GetSatelliteCells(bitStream, bitPosition, header.Satellites)
	if fetchSatellitesError != nil {
		return nil, fetchSatellitesError
	}

	bitPosition += uint(len(satellites) * satellite.CellLengthInBits)

	signals, fetchSignalsError :=
		signal.GetSignalCells(bitStream, bitPosition, header, satellites)
	if fetchSignalsError != nil {
		return nil, fetchSignalsError
	}

	msm4Message := New(header, satellites, signals)

	return msm4Message, nil
}