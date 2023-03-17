package rtcm3

import (
	"encoding/hex"
	"fmt"
	"time"

	"github.com/goblimey/go-ntrip/rtcm/message1005"
	msm4Message "github.com/goblimey/go-ntrip/rtcm/msm4/message"
	msm7Message "github.com/goblimey/go-ntrip/rtcm/msm7/message"
	"github.com/goblimey/go-ntrip/rtcm/utils"
)

// Message contains an RTCM3 message, possibly broken out into readable form,
// or a stream of non-RTCM data.  Message type NonRTCMMessage indicates the
// second case.
type Message struct {
	// MessageType is the type of the RTCM message (the message number).
	// RTCM messages all have a positive message number.  Type NonRTCMMessage
	// is negative and indicates a stream of bytes that doesn't contain a
	// valid RTCM message, for example an NMEA message or a corrupt RTCM.
	MessageType int

	// Valid is true if the message is valid - complete (all bytes implied by the
	// message length are present), CRC check passes and there are no errors found
	// while decoding the message.
	Valid bool

	// Complete is true if the message is complete.  The last bytes in a
	// log of messages may not be complete.
	Complete bool

	// CRCValid is true if the Cyclic Redundancy Check bits are valid.
	CRCValid bool

	// ErrorMessage contains any error message encountered while fetching
	// the message.
	ErrorMessage string

	// RawData is the message frame in its original binary form
	//including the header and the CRC.
	RawData []byte

	// If the message is an MSM, UTCTime contains the time in UTC from the epochTime.
	// If the message is not an MSM, the value is nil.
	UTCTime *time.Time
	// Readable is a broken out version of the RTCM message.  It's accessed
	// via the Readable method and the message is only decoded on the
	// first call.  (Lazy evaluation.)
	Readable interface{}
}

// New creates a new message.  If the warning is an empty string, that field is left as nil.
func New(messageType int, warning string, bitStream []byte) *Message {

	if len(warning) > 0 {
		message := Message{
			MessageType:  messageType,
			RawData:      bitStream,
			ErrorMessage: warning,
		}
		return &message
	}

	message := Message{
		MessageType: messageType,
		RawData:     bitStream,
	}
	return &message
}

// NewNonRTCM creates a Non-RTCM message.
func NewNonRTCM(bitStream []byte) *Message {
	message := Message{
		MessageType: utils.NonRTCMMessage,
		RawData:     bitStream,
	}
	return &message
}

// Copy makes a copy of the message and its contents.
func (message *Message) Copy() Message {
	// Make a copy of the raw data.
	rawData := make([]byte, len(message.RawData))
	copy(rawData, message.RawData)
	// Create a new message.  Omit the readable part - it may not be needed
	// and if it is needed, it will be created automatically at that point.
	var newMessage = Message{
		MessageType:  message.MessageType,
		RawData:      rawData,
		Valid:        message.Valid,
		Complete:     message.Complete,
		CRCValid:     message.CRCValid,
		ErrorMessage: message.ErrorMessage,
	}
	return newMessage
}

// String takes the given Message object and returns it
// as a readable string.
//
func (message *Message) String() string {

	display := ""

	if message.UTCTime != nil {
		// If the time is set (which should only happen if the message is an MSM),
		// display it.
		display += message.UTCTime.Format(utils.DateLayout) + "\n"
	}

	display += fmt.Sprintf("message type %d, frame length %d %s\n",
		message.MessageType, len(message.RawData), message.Status())

	display += hex.Dump(message.RawData) + "\n"

	if len(message.ErrorMessage) > 0 {
		display += message.ErrorMessage + "\n"
	}

	if message.MessageType == utils.NonRTCMMessage {
		return display
	}

	if message.Readable != nil {

		// The message has a readable form.

		// In some cases the displayable is a simple string.
		m, ok := message.Readable.(string)
		if ok {
			display += m + "\n"
			return display
		}

		// prepareFoDisplay may have found an error and unset the valid flag.
		if !message.Valid {
			if len(message.ErrorMessage) > 0 {
				display += message.ErrorMessage + "\n"
			} else {
				display += "invalid message\n"
			}
			return display
		}

		// The message is a set of broken out fields.  Create a readable version.  If that reveals
		// an error, the Valid flag will be unset and a warning added to the message.
		switch {

		case message.MessageType == 1005:
			m, ok := message.Readable.(*message1005.Message)
			if !ok {
				// Internal error:  the message says the data are a type 1005 (base position)
				// message but when decoded they are not.
				display += "expected the readable message to be *Message1005\n"
				if len(message.ErrorMessage) > 0 {
					display += message.ErrorMessage + "\n"
				}
				break
			}
			display += m.String()

		case utils.MSM4(message.MessageType):
			m, ok := message.Readable.(*msm4Message.Message)
			if !ok {
				// Internal error:  the message says the data are an MSM4
				// message but when decoded they are not.
				display += "expected the readable message to be an MSM4\n"
				if len(message.ErrorMessage) > 0 {
					display += message.ErrorMessage + "\n"
				}
				break
			}
			display += m.String()

		case utils.MSM7(message.MessageType):
			m, ok := message.Readable.(*msm7Message.Message)
			if !ok {
				// Internal error:  the message says the data are an MSM4
				// message but when decoded they are not.
				display += "expected the readable message to be an MSM7\n"
				if len(message.ErrorMessage) > 0 {
					display += message.ErrorMessage + "\n"
				}
				break
			}
			display += m.String()

			// The default case can't be reached - PrepareForDisplay calls
			// Analyse which sets Readable field to an error message if it can't
			// display the message.  That case was taken care of earlier.
			//
			// default:
			// 	display += "the message is not displayable\n"
		}

	}

	return display
}

// displayable is true if the message type is one that we know how
// to display in a readable form.
func (message *Message) displayable() bool {
	// we currently can display messages of type 1005, MSM4 and MSM7.

	if message.MessageType == utils.NonRTCMMessage {
		return false
	}

	if utils.MSM(message.MessageType) || message.MessageType == 1005 {
		return true
	}

	return false
}

// Status returns the status of the message - valid, CRC checked failed and so on.
func (message *Message) Status() string {
	status := ""
	if message.Valid {
		status = "valid"
	} else {
		if message.Complete {
			status = "complete"
		} else {
			status = "incomplete"
		}
		if message.CRCValid {
			if len(status) > 0 {
				// We want "complete CRC check passed", not "completeCRC check passed"
				status += " "
			}
			status += "CRC check passed"
		} else {
			if len(status) > 0 {
				// We want "complete CRC check failed", not "completeCRC check failed"
				status += " "
			}
			status += "CRC check failed"
		}
	}

	return status
}
