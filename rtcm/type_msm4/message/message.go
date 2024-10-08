package message

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/goblimey/go-ntrip/rtcm/header"
	"github.com/goblimey/go-ntrip/rtcm/type_msm4/satellite"
	"github.com/goblimey/go-ntrip/rtcm/type_msm4/signal"
	"github.com/goblimey/go-ntrip/rtcm/utils"
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

	// LogLevel controls the data output by String.
	LogLevel slog.Level
}

// New creates an MSM4 Message.
func New(
	header *header.Header,
	satellites []satellite.Cell,
	signals [][]signal.Cell,
	logLevel slog.Level) *Message {
	message := Message{
		Header:     header,
		Satellites: satellites,
		Signals:    signals,
		LogLevel:   logLevel,
	}

	return &message
}

// String return a text version of the MSM4Message.
func (message *Message) String() string {
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
		return "No Satellites\n"
	}

	heading := "Satellite ID {approx range - whole, frac, millis, metres}\n"

	body := ""
	for i := range message.Satellites {
		body += message.Satellites[i].String() + "\n"
	}

	return heading + body
}

// DisplaySignalCells returns a text version of the signal data from the signal
// cells in a type 4 multiple signal message.
func (message *Message) DisplaySignalCells() string {

	if len(message.Signals) < 1 {
		return "No Signals\n"
	}

	heading := "Signals:\nSat ID Sig ID {(range delta, delta m, range m), (phase range delta, cycles) lock time ind, half cycle ambiguity, Carrier Noise Ratio, wavelength}\n"

	body := ""

	for i := range message.Signals {
		for j := range message.Signals[i] {
			body += message.Signals[i][j].String() + "\n"
		}
	}

	return heading + body
}

// GetMessage presents an MSM4 (type 1074, 1084 etc) as  broken out fields.
func GetMessage(bitStream []byte, logLevel slog.Level) (*Message, error) {

	header, bitPosition, headerError := header.GetMSMHeader(
		bitStream, logLevel)

	if headerError != nil {
		return nil, headerError
	}

	// Sanity check.  The message type must be an MSM4.
	if !utils.MSM4(header.MessageType) {
		em := fmt.Sprintf("message type %d is not an MSM4", header.MessageType)
		return nil, errors.New(em)
	}

	satellites, fetchSatellitesError := satellite.GetSatelliteCells(
		bitStream, bitPosition, header.Satellites, logLevel,
	)

	if fetchSatellitesError != nil {
		return nil, fetchSatellitesError
	}

	bitPosition += uint(len(satellites) * satellite.CellLengthInBits)

	signals, fetchSignalsError := signal.GetSignalCells(
		bitStream, bitPosition, header, satellites, logLevel,
	)

	if fetchSignalsError != nil {
		return nil, fetchSignalsError
	}

	msm4Message := New(header, satellites, signals, logLevel)

	return msm4Message, nil
}
