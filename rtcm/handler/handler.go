package handler

import (
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/goblimey/go-ntrip/rtcm/pushback"
	"github.com/goblimey/go-ntrip/rtcm/type1005"
	msm4Message "github.com/goblimey/go-ntrip/rtcm/type_msm4/message"
	msm7Message "github.com/goblimey/go-ntrip/rtcm/type_msm7/message"
	"github.com/goblimey/go-ntrip/rtcm/utils"

	"github.com/goblimey/go-crc24q/crc24q"
)

// The rtcm package contains logic to read and decode and display RTCM3
// messages produced by GNSS devices.  See the README for this repository
// for a description of the RTCM version 3 protocol.
//
//     handler := handler.New(time.Now(), logger)
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

// glonassInvalidDay is the invalid value for the day part of the timestamp.
const glonassInvalidDay = 7

// maxTimestamp is the maximum timestamp value.  The timestamp is 30 bits
// giving milliseconds since the start of the day.
const maxTimestamp = 0x3fffffff // 0011 1111 1111 1111 1111 1111 1111 1111

// glonassDayBitMask is used to extract the Glonass day from the timestamp
// in an MSM7 message.  The 30 bit time value is a 3 bit day (0 is Sunday)
// followed by a 27 bit value giving milliseconds since the start of the
// day.
const glonassDayBitMask = 0x38000000 // 0011 1000 0000 0000 0000 0000 0000 0000

// gpsLeapSeconds is the duration that GPS time is ahead of UTC
// in seconds, correct from the start of 2017/01/01.  An extra leap
// second may be added every four years.  The start of 2021 was a
// candidate for adding another leap second but it was not necessary.
const gpsLeapSeconds = -18

// gpsTimeOffset is the offset to convert a GPS time to UTC.
var gpsTimeOffset time.Duration = time.Duration(gpsLeapSeconds) * time.Second

// glonassTimeOffset is the offset to convert Glonass time to UTC.
// Glonass is currently 3 hours ahead of UTC.
var glonassTimeOffset = time.Duration(-1*3) * time.Hour

// beidouTimeOffset is the offset to convert a BeiDou time value to
// UTC.  UTC is based on International Atomic Time (TAI):
// https://www.timeanddate.com/time/international-atomic-time.html.
//
// This reference from November 2013 compares Beidout Time (BDT) with
// TAI: https://www.unoosa.org/pdf/icg/2016/Beidou-Timescale2016.pdf.
//
// "BDT is a uniform scale and is 33 seconds behind TAI"
//
// HOWEVER the rtklib software (decode_msm_header) says that BDT is
// 14 seconds ahead of GPS time, which is 18 seconds behind UTC,
// meaning that BDT is 4 seconds behind UTC.  Analysis of real data
// confirms that.
var beidouLeapSeconds = -4
var beidouTimeOffset = time.Duration(beidouLeapSeconds) * time.Second

// Handler is the object used to fetch and analyse RTCM3 messages.
type Handler struct {

	// These dates are used to interpret the timestamps in RTCM3
	// messages.

	// startOfGPSWeek is the time in UTC of the start of
	// this GPS week.
	startOfGPSWeek time.Time

	// startOfGalileoWeek is the time in UTC of the start of
	// this Galileo week.
	startOfGalileoWeek time.Time

	// startOfThisGlonassWeek is the time in UTC of the start of
	// this Glonass week.
	startOfGlonassDay time.Time

	// startOfThisGPSWeek is the time in UTC of the start of
	// this GPS week.
	startOfBeidouWeek time.Time

	// These dates are used to detect the timestamp rolling over into the
	// next period.  (The strategy assumes that the time gap between
	// messages is short.)

	// timestampFromPreviousGPSMessage is the timestamp of the previous GPS
	// multiple signal message (MSM).
	timestampFromPreviousGPSMessage uint

	// timestampFromPreviousGalileoMessage is the timestamp of the previous Galileo
	// multiple signal message (MSM).
	timestampFromPreviousGalileoMessage uint

	// timestampFromPreviousBeidouMessage is the timestamp of the previous Beidou
	// multiple signal message (MSM).
	timestampFromPreviousBeidouMessage uint

	// glonassDayFromPreviousMessage is the day number from the previous Glonass
	// multiple signal message (MSM).
	glonassDayFromPreviousMessage uint
}

// New creates a handler using the given year, month and day to
// identify which week the times in the messages refer to.
func New(startTime time.Time) *Handler {

	// GPS, Galileo and Beidou.  The week for each starts a few leap seconds
	// before midnight at the end of Saturday in UTC so most of Saturday UTC
	// is the end of one GPS week but the last few seconds are the beginning
	// of the next.

	// Get midnight at the start of last Sunday, Sunday next and the Sunday
	// before.  The start of each week is an offset from one of these.
	// Note:  the last few seconds of Saturday are, in effect, Sunday
	// according to GPS and Beidou.

	// Convert the start date to UTC.
	startTime = startTime.In(utils.LocationUTC)

	// Shift the start time forward by the number of leap seconds (so if it's
	// in the last few seconds of Saturday we get a time in Sunday).
	gpsShift := time.Duration(-1*gpsLeapSeconds) * time.Second
	gpsShiftedStartTime := startTime.Add(gpsShift)
	beidouShift := time.Duration(-1*beidouLeapSeconds) * time.Second
	beidouShiftedStartTime := startTime.Add(beidouShift)

	// Find last Sunday from the shifted start time (which may be the same day).
	gpsMidnightLastSunday := getStartOfLastSundayUTC(gpsShiftedStartTime)
	beidouMidnightLastSunday := getStartOfLastSundayUTC(beidouShiftedStartTime)

	// Crank back a few seconds to get the start of the GPS and Beidou weeks.
	startOfGPSWeek := gpsMidnightLastSunday.Add(gpsTimeOffset)
	// gpsMidnightNextSunday := gpsStartOfWeek.AddDate(0, 0, 7)
	startOfBeidouWeek := beidouMidnightLastSunday.Add(beidouTimeOffset)
	// beidouMidnightNextSunday := beidouMidnightLastSunday.AddDate(0, 0, 7)

	// midnightTheSundayBefore := gpsStartOfWeek.AddDate(0, 0, -7)

	// var startOfGalileoWeek time.Time
	// var startOfGPSWeek time.Time
	// var startOfBeidouWeek time.Time

	// if startTime.Weekday() == time.Saturday {
	// 	// This is Saturday so we may be in the old week or the new one.

	// 	if startTime.Before(gpsMidnightNextSunday) {
	// 		// It's Saturday just before the end of a GPS week.
	// 		startOfGPSWeek = gpsStartOfWeek
	// 	} else {
	// 		// It's the end of Saturday in the first few seconds of a new
	// 		// GPS week
	// 		startOfGPSWeek = gpsMidnightNextSunday
	// 	}

	// 	// The logic for Beidou is similar to that for GPS but with a different
	// 	// time offset.
	// 	if !startTime.Before(beidouMidnightNextSunday) {
	// 		// It's the end of Saturday in the first few seconds of a new
	// 		// Beidou week
	// 		startOfBeidouWeek = beidouMidnightNextSunday
	// 	} else {
	// 		// It's Saturday just before the end of the Beidou week.
	// 		startOfBeidouWeek = beidouMidnightLastSunday
	// 	}
	// } else {
	// 	// It's not Saturday, so the start of week times are offsets from the start
	// 	// of last week.
	// 	startOfGPSWeek = gpsStartOfWeek
	// 	startOfBeidouWeek = beidouMidnightLastSunday
	// }

	// Galileo keeps GPS time.
	startOfGalileoWeek := startOfGPSWeek

	// Set the stored timestamps to match the start time.
	timestampFromPreviousGPSMessage := (uint(startTime.Sub(startOfGPSWeek).Milliseconds()))
	timestampFromPreviousGalileoMessage := timestampFromPreviousGPSMessage
	timestampFromPreviousBeidouMessage := (uint(startTime.Sub(startOfBeidouWeek).Milliseconds()))

	handler := Handler{
		startOfGPSWeek:                      startOfGPSWeek,
		startOfGalileoWeek:                  startOfGalileoWeek,
		startOfBeidouWeek:                   startOfBeidouWeek,
		timestampFromPreviousGPSMessage:     timestampFromPreviousGPSMessage,
		timestampFromPreviousGalileoMessage: timestampFromPreviousGalileoMessage,
		timestampFromPreviousBeidouMessage:  timestampFromPreviousBeidouMessage,
	}

	// Glonass.  Set the Glonass day number and the start of this
	// Glonass day.  The day is 0: Sunday, 1: Monday and so on, but in
	// Moscow time which is three hours ahead of UTC, so the day value
	// rolls over at 21:00 UTC the day before.

	// Unlike GPS, we have a real timezone to work with - Moscow.
	startTimeMoscow := startTime.In(utils.LocationMoscow)
	startOfDayMoscow := time.Date(startTimeMoscow.Year(), startTimeMoscow.Month(),
		startTimeMoscow.Day(), 0, 0, 0, 0, utils.LocationMoscow)

	handler.startOfGlonassDay = startOfDayMoscow.In(utils.LocationUTC)

	// Set the Glonass day from the previous message to the day in Moscow
	// at the given start time - Sunday is 0, Monday is 1 and so on.  This
	// will be reset when the first Glonass Multiple Signal message (MSM)
	// arrives.
	handler.glonassDayFromPreviousMessage = uint(startOfDayMoscow.Weekday())

	return &handler
}

// HandleMessages reads bytes from ch_in, converts them to RTCM
// messages and writes the messages to ch_out.  The caller is responsible
// for creating and closing both channels.
func (rtcmHandler *Handler) HandleMessages(ch_in chan byte, ch_out chan Message) {

	// Turn the input channel into a pushback channel.
	pb := pushback.New(ch_in)

	// Fetch messages until there are no more.
	for {
		message, err := rtcmHandler.FetchNextMessageFrame(pb)
		if err != nil && err.Error() == "done" {
			// There is no more input.
			close(ch_out)
			return
		}

		// Send the message to the output channel
		ch_out <- *message
	}
}

// FetchNextMessageFrame gets the next message frame from the given byte
// stream, a channel of bytes.  The byte stream should contain RTCM3 message
// frames but they may be interspersed with messages in other formats such as
// NMEA, UBX etc.   The resulting slice contains either a single valid message
// or some non-RTCM text that precedes a message.

// the function encounters a fatal read error and it has not yet read any
// text, it returns the error.  If it has read some text, it just returns
// that (the assumption being that the next call will get no text and the
// same error).  Use GetMessage to extract the message from the result.
func (rtcmHandler *Handler) FetchNextMessageFrame(pc *pushback.ByteChannel) (*Message, error) {

	// A valid RTCM3 message frame is a leader containing the start of message
	// byte 0xd3 and two bytes containing a 10-bit message length, zero padded
	// to the left, for example 0xd3, 0x00, 0x8a.  The variable-length message
	// comes next and always starts with a 12-bit message type, zero padded to
	// the left.  The message may be padded with zero bytes at the end.  The
	// message frame then ends with a 3-byte Cyclic Redundancy Check value.
	//
	// So, to scan a complete message frame we need to scan the first five
	// bytes, the 3-byte leader and the first two bytes of the embedded message.
	// Then we can figure out the length of the embedded message, then scan the
	// remaining bytes of it and the 3-byte CRC.  While we are doing all this
	// we must watch for the input becoming exhausted, leaving us with part of
	// a message.  We also need to be aware that encountering a 0xd3 byte doesn't
	// guarantee the start of an RTCM message.  We may just have blundered across
	// one in the middle of an RTCM message or in some other binary data.  We
	// only know we have an RTCM message frame when we have scanned and checked
	// the CRC.
	//
	// If we scan some bytes and find that they are not a valid RTCM message
	// frame we return them as a Non-RTCM message (message type -1).

	// Create a buffer to hold the message frame.
	var frame = make([]byte, 0)

	// phase 1: eat bytes until we see the start of message frame byte.
	frame, eatError := eatUntilStartOfFrame(pc)

	if eatError != nil {
		// The channel is exhausted. If there's nothing in the buffer, return
		// an error.  If there is something in the buffer, continue and deal
		// with that - we should get an error and nothing in the buffer next
		// time.
		if len(frame) == 0 {
			// An error and no bytes.  We're done.
			return nil, eatError
		}
	}

	// eatUntilStartOfFrame has returned some text.  Figure out what it is.  It
	// could be just the start of message frame byte, some other text followed
	// by the start of message frame byte or just some other text. That last
	// case should only happen if we scanned some text and then hit the end of
	// the input.  In that case the next call of this will eat and immediately
	// get an error, but right now we want to return what we've read for
	// processing.)
	//
	if len(frame) > 1 {
		// We have some non-RTCM.
		if frame[len(frame)-1] == utils.StartOfMessageFrame {
			// The non-RTCM is followed by start of message byte.  Push the
			// start byte back so we see it next time.  Return the rest of the
			// buffer as a non-RTCM message.
			pc.PushBack(utils.StartOfMessageFrame)
			frameWithoutTrailingStartByte := frame[:len(frame)-1]
			return NewNonRTCM(frameWithoutTrailingStartByte), nil
		} else {
			// We just have some non-RTCM without a start byte.  (Probably
			// because we reached the end of the input).
			return NewNonRTCM(frame), nil
		}
	}

	// Phase 2:  if we get to here, the frame buffer contains one byte.  It's
	// the start of message frame byte which may (or may not) mark the beginning
	// of an RTCM message frame.  If so, the length of the frame is given by the
	// length of the embedded message plus leader and CRC.  We have to read
	// enough of the frame to find that length.

	const leaderAndMessageLength = utils.LeaderLengthBytes + 2

	for i := 1; i < leaderAndMessageLength; i++ {

		b, err := pc.GetNextByte()

		if err != nil {
			//Error - presumably end of input.  however, we've already read some
			// test so return that.  the end of input will be picked up on the
			// next call.
			return NewNonRTCM(frame), nil
		}

		frame = append(frame, b)
	}

	// Figure out the length of the frame. (This may detect that the message is
	// not RTCM.)
	messageLength, _, typeError := rtcmHandler.getMessageLengthAndType(frame)

	if typeError != nil {
		// We thought we'd found the start of an RTCM message but it's some
		// other data that just happens to contain the start of frame byte.
		// Return the collected data as a non-RTCM message.
		return NewNonRTCM(frame), nil
	}

	// Phase 3: get the rest of the message frame.

	messageFrameLength := messageLength + utils.LeaderLengthBytes + utils.CRCLengthBytes
	wantBytes := int(messageFrameLength) - len(frame)

	for i := 0; i < wantBytes; i++ {
		b, err := pc.GetNextByte()

		if err != nil {
			//Error - presumably end of input.  however, we've already read some
			// test so return that.  the end of input will be picked up on the
			// next call.
			return NewNonRTCM(frame), nil
		}

		frame = append(frame, b)
	}

	// Phase 4: create a message from the frame and return it.  (This also checks
	// the CRC.  If that fails the text is returned as a non-RTCM message.)

	return rtcmHandler.GetMessage(frame)
}

// eatUntilStartOfFrame reads bytes from the channel until it encounters
// a byte signifying the start of a message frame or the channel is closed.
// It returns what it has eaten.  If there is an error (implying that the
// channel is closed) it returns what it read so far and the error.
func eatUntilStartOfFrame(pc *pushback.ByteChannel) ([]byte, error) {
	stuff := make([]byte, 0)
	for {
		b, err := pc.GetNextByte()
		if err != nil {
			return stuff, err
		}
		stuff = append(stuff, b)

		if b == utils.StartOfMessageFrame {
			return stuff, nil
		}
	}
}

// getMessageLengthAndType extracts the message length and the message type from an
// RTCMs message frame or returns an error, implying that this is not the start of a
// valid message.  The bit stream must be at least 5 bytes long.
func (rtcmHandler *Handler) getMessageLengthAndType(bitStream []byte) (uint, int, error) {

	if len(bitStream) < (utils.LeaderLengthBytes + 2) {
		return 0, utils.NonRTCMMessage, errors.New("the message is too short to get the header and the length")
	}

	// The message header is 24 bits.  The top byte is startOfMessage.
	if bitStream[0] != utils.StartOfMessageFrame {
		message := fmt.Sprintf("message starts with 0x%0x not 0xd3", bitStream[0])
		return 0, utils.NonRTCMMessage, errors.New(message)
	}

	// The next six bits must be zero.  If not, we've just come across
	// a 0xd3 byte in a stream of binary data.
	sanityCheck := utils.GetBitsAsUint64(bitStream, 8, 6)
	if sanityCheck != 0 {
		errorMessage := fmt.Sprintf("bits 8-13 of header are %d, must be 0", sanityCheck)
		return 0, utils.NonRTCMMessage, errors.New(errorMessage)
	}

	// The bottom ten bits of the leader give the message length.
	length := uint(utils.GetBitsAsUint64(bitStream, 14, 10))

	// The 12-bit message type follows the header.
	messageType := int(utils.GetBitsAsUint64(bitStream, 24, 12))

	// length must be > 0. (We deferred this check until now because we want
	// the message type before we exit.)
	if length == 0 {
		errorMessage := fmt.Sprintf("zero length message, type %d", messageType)
		return 0, messageType, errors.New(errorMessage)
	}

	return length, messageType, nil
}

// GetMessage extracts an RTCM3 message from the given bit stream and returns it
// as an RTC3Message. If the bit stream is empty, it returns an error.  If the data
// doesn't contain a valid message, it returns a message with type NonRTCMMessage.
func (rtcmHandler *Handler) GetMessage(bitStream []byte) (*Message, error) {

	if len(bitStream) == 0 {
		return nil, errors.New("zero length message frame")
	}

	if bitStream[0] != utils.StartOfMessageFrame {
		// This is not an RTCM message.
		return NewNonRTCM(bitStream), nil
	}

	messageLength, messageType, formatError := rtcmHandler.getMessageLengthAndType(bitStream)
	if formatError != nil {
		return NewMessage(messageType, formatError.Error(), bitStream), formatError
	}

	// The message frame should contain a header, the variable-length message and
	// the CRC.  We now know the message length, so we can check that we have the
	// whole thing.

	frameLength := uint(len(bitStream))
	expectedFrameLength := messageLength + utils.LeaderLengthBytes + utils.CRCLengthBytes
	// The message is analysed only when necessary (lazy evaluation).  For
	// now, just copy the byte stream into the Message.
	if expectedFrameLength > frameLength {
		// The message is incomplete, return what we have as a
		// non-RTCM3 message.  (This can happen if it's the last message
		// in the input stream.)
		message := NewNonRTCM(bitStream)
		message.ErrorMessage = "incomplete message frame"
		return message, errors.New(message.ErrorMessage)
	}

	// We have a complete message.

	// Check the CRC.
	if !CheckCRC(bitStream) {
		message := NewNonRTCM(bitStream)
		message.ErrorMessage = "CRC is not valid"
		return message, errors.New(message.ErrorMessage)
	}

	// The message is complete and the CRC check passes, so it's valid.
	message := NewMessage(messageType, "", bitStream[:expectedFrameLength])

	// If the message is an MSM7, get the time (for the heading if displaying)
	// The message frame is: 3 bytes of leader, a 12-bit message type, a 12-bit
	// station ID followed by the 30-bit timestamp, followed by lots of other
	// stuff and finally a 3-byte CRC.  If we get to here then the leader and
	// CRC are present and the message contains at least a complete header.

	if utils.MSM(message.MessageType) {

		// The message is an MSM so get the timestamp and set the UTCTime.  The
		// message frame starts with 3 bytes of leader, a 12-bit message type, a
		// 12-bit station ID and the 30-bit timestamp.

		const firstBit = 48 // Leader plus 24 bits.
		const timestampLength = 30

		timestamp := uint(utils.GetBitsAsUint64(bitStream, firstBit, timestampLength))

		// Get the time from the timestamp.  This has to be done by the handler
		// because it depends on knowing which week we are in at the start and
		// then keeping track of time over many messages.  Only the handler lives
		// long enough to do that.
		utcTime, timeError :=
			rtcmHandler.getTimeFromTimeStamp(message.MessageType, timestamp)

		if timeError != nil {
			message.ErrorMessage = timeError.Error()
			return message, timeError
		}

		message.UTCTime = &utcTime

		return message, nil
	}

	return message, nil
}

// Analyse decodes the raw byte stream and fills in the broken out message.
func Analyse(message *Message) {
	var readable interface{}

	switch {

	case utils.MSM4(message.MessageType):
		analyseMSM4(message.RawData, message)

	case utils.MSM7(message.MessageType):
		analyseMSM7(message.RawData, message)

	case message.MessageType == 1005:
		analyse1005(message.RawData, message)

	case message.MessageType == 1230:
		readable = "(Message type 1230 - GLONASS code-phase biases - don't know how to decode this)"
		message.Readable = readable

	default:
		readable := fmt.Sprintf("message type %d currently cannot be displayed", message.MessageType)
		message.Readable = readable
	}
}

func analyseMSM4(messageBitStream []byte, message *Message) {
	msm4Message, msm4Error := msm4Message.GetMessage(messageBitStream)
	if msm4Error != nil {
		message.ErrorMessage = msm4Error.Error()
		return
	}

	message.Readable = msm4Message
}

func analyseMSM7(messageBitStream []byte, message *Message) {
	msm7Message, msm7Error := msm7Message.GetMessage(messageBitStream)
	if msm7Error != nil {
		message.ErrorMessage = msm7Error.Error()
		return
	}

	message.Readable = msm7Message
}

func analyse1005(messageBitStream []byte, message *Message) {
	message1005, message1005Error := type1005.GetMessage(messageBitStream)
	if message1005Error != nil {
		message.ErrorMessage = message1005Error.Error()
		return
	}

	message.Readable = message1005
}

// getTimeFromTimeStamp converts the 30-bit timestamp in the MSM header to a time value
// in the UTC timezone.  The message must be an MSM as others don't have a timestamp.
func (rtcmHandler *Handler) getTimeFromTimeStamp(messageType int, timestamp uint) (time.Time, error) {

	var zeroTimeValue time.Time

	// Convert the timestamp to UTC.  This requires keeping a notion of time
	// over many messages, potentially for many days, so it must be done by
	// this module.
	//
	// The Glonass timestamp has an invalid value, so the Glonass converter can
	// return an error.

	switch messageType {
	case utils.MessageTypeMSM4GPS:
		utcTime, err := rtcmHandler.getUTCFromGPSTime(timestamp)
		return utcTime, err
	case utils.MessageTypeMSM7GPS:
		utcTime, err := rtcmHandler.getUTCFromGPSTime(timestamp)
		return utcTime, err
	case utils.MessageTypeMSM4Glonass:
		utcTime, err := rtcmHandler.getUTCFromGlonassTime(timestamp)
		return utcTime, err
	case utils.MessageTypeMSM7Glonass:
		utcTime, err := rtcmHandler.getUTCFromGlonassTime(timestamp)
		return utcTime, err
	case utils.MessageTypeMSM4Galileo:
		utcTime, err := rtcmHandler.getUTCFromGalileoTime(timestamp)
		return utcTime, err
	case utils.MessageTypeMSM7Galileo:
		utcTime, err := rtcmHandler.getUTCFromGalileoTime(timestamp)
		return utcTime, err
	case utils.MessageTypeMSM4Beidou:
		utcTime, err := rtcmHandler.getUTCFromBeidouTime(timestamp)
		return utcTime, err
	case utils.MessageTypeMSM7Beidou:
		utcTime, err := rtcmHandler.getUTCFromBeidouTime(timestamp)
		return utcTime, err
	default:
		// This MSM is one that we don't know how to decode.
		return zeroTimeValue, errors.New("unknown message type")
	}
}

// GetUTCFromGPSTime converts a GPS time to UTC, using the start time
// to find the time of the start of the week.
func (rtcmHandler *Handler) getUTCFromGPSTime(timestamp uint) (time.Time, error) {
	// The GPS week starts at midnight at the start of Sunday
	// but GPS time is ahead of UTC by a few leap seconds, so in
	// UTC terms the week starts on Saturday a few seconds before
	// Saturday/Sunday midnight.
	//
	// Note: we have to be careful when the start time is Saturday
	// and close to midnight, because that is within the new GPS
	// week.  If create a handler around then, we have to specify
	// the start time carefully.

	timeFromTimestamp, newStartOfWeek, err := getUTCFromTimestamp(
		timestamp, rtcmHandler.timestampFromPreviousGPSMessage,
		rtcmHandler.startOfGPSWeek)

	if err != nil {
		return timeFromTimestamp, err
	}

	// We may have moved into the next week.
	rtcmHandler.startOfGPSWeek = newStartOfWeek

	// Get ready for the next call.
	rtcmHandler.timestampFromPreviousGPSMessage = timestamp

	return timeFromTimestamp, nil
}

// GetUTCFromGlonassTimestamp converts a Glonass timestamp to UTC using
// the start time to give the correct Glonass week.
func (rtcmHandler *Handler) getUTCFromGlonassTime(timestamp uint) (time.Time, error) {
	// The Glonass timestamp contains two bit fields.  Bits 0-26 give
	// milliseconds since the start of the day.  Bits 27-29 give the
	// day, 0: Sunday, 1: Monday ... 6: Saturday and 7: invalid.  The
	// Glonass day starts at midnight in the Moscow timezone, so three
	// hours ahead of UTC.
	//
	// day = 1, glonassTime = 1 is 1 millisecond into Russian Monday,
	// which in UTC is Sunday 21:00:00 plus one millisecond.
	//
	// Day = 1, glonassTime = (4*3600*1000) is 4 am on Russian Monday,
	// which in UTC is 1 am on Monday.
	//
	// The rollover mechanism assumes that the GetUTCFromGlonassTimestamp is called fairly
	// regularly, at least once each day, so the day in one call should
	// be the either the same as the day in the last call or one day more.
	// If there is a gap between the days, we can't know how big that
	// gap is - three days?  Three months?  (In real life, a base station
	// will be producing RTCM3 messages something like once per second, so
	// this assumption is safe.)

	day, millis, err := ParseGlonassTimeStamp(timestamp)

	if err != nil {
		var zeroTimeValue time.Time // 0001-01-01 00:00:00 +0000 UTC.
		return zeroTimeValue, errors.New("out of range")
	}

	if day == glonassInvalidDay {
		// The day value indicates an invalid time stamp.
		var zeroTimeValue time.Time // 0001-01-01 00:00:00 +0000 UTC.
		return zeroTimeValue, errors.New("invalid day")
	}

	// The timestamp is valid.  We have day (1, 2 ... or 6) and milliseconds
	// since the start of day.

	// Set the start of day if different.
	if day != rtcmHandler.glonassDayFromPreviousMessage {
		// The day has rolled over.
		rtcmHandler.startOfGlonassDay =
			rtcmHandler.startOfGlonassDay.AddDate(0, 0, 1)
	}

	// Add the millisecond offset from the timestamp
	offset := time.Duration(millis) * time.Millisecond
	timeFromTimestamp := rtcmHandler.startOfGlonassDay.Add(offset)

	// Set the day ready for next time.
	rtcmHandler.glonassDayFromPreviousMessage = uint(timeFromTimestamp.Weekday())

	return timeFromTimestamp, nil

}

// GetUTCFromGalileoTime converts a Galileo time to UTC, using the
// start time to find the start time of the current week.
func (rtcmHandler *Handler) getUTCFromGalileoTime(timestamp uint) (time.Time, error) {
	// Galileo follows GPS time, but we keep separate state variables.
	//
	// Note: we have to be careful when the start time is Saturday
	// and close to midnight, because that is within the new GPS
	// week.  If create a handler around then, we have to specify
	// the start time carefully.

	timeFromTimestamp, newStartOfWeek, err := getUTCFromTimestamp(
		timestamp,
		rtcmHandler.timestampFromPreviousGalileoMessage,
		rtcmHandler.startOfGPSWeek)

	if err != nil {
		return timeFromTimestamp, err
	}

	// We may have moved into the next week.
	rtcmHandler.startOfGalileoWeek = newStartOfWeek

	// Get ready for the next call.
	rtcmHandler.timestampFromPreviousGalileoMessage = timestamp

	return timeFromTimestamp, nil
}

// GetUTCFromBeidouTime converts a Baidou time to UTC, using the
// start time to find the time of the start of the current week.
func (rtcmHandler *Handler) getUTCFromBeidouTime(timestamp uint) (time.Time, error) {

	// The Beidou week starts at midnight at the start of Sunday
	// but Beidou time is ahead of UTC by a few seconds, so in UTC
	// terms the week starts a few seconds before midnight at the
	// end of Saturday.
	//
	// Note: we have to be careful when the start time is Saturday
	// and just before midnight, because that is within the new Beidou
	// week.  If create a handler around then, we have to specify
	// the start time carefully.

	timeFromTimestamp, newStartOfWeek, err := getUTCFromTimestamp(
		timestamp, rtcmHandler.timestampFromPreviousBeidouMessage,
		rtcmHandler.startOfBeidouWeek)

	if err != nil {
		return timeFromTimestamp, err
	}

	// We may have moved into the next week.
	rtcmHandler.startOfBeidouWeek = newStartOfWeek

	// Get ready for the next call.
	rtcmHandler.timestampFromPreviousBeidouMessage = timestamp

	return timeFromTimestamp, nil
}

// getStartOfLastSundayUTC gets midnight at the start of the
// last Sunday (which may be today) in UTC.
func getStartOfLastSundayUTC(now time.Time) time.Time {
	// Convert the time to UTC, which may change the day.
	now = now.In(utils.LocationUTC)

	// Crank the day back to Sunday.  (It may already be there.)
	for {
		if now.Weekday() == time.Sunday {
			break
		}
		now = now.AddDate(0, 0, -1)
	}

	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, utils.LocationUTC)
}

// DisplayMessage takes the given Message object and returns it
// as a readable string.
func (rtcmHandler *Handler) DisplayMessage(message *Message) string {

	display := fmt.Sprintf("message type %d, frame length %d\n",
		message.MessageType, len(message.RawData))

	display += hex.Dump(message.RawData) + "\n"

	if len(message.ErrorMessage) > 0 {
		display += message.ErrorMessage + "\n"
		return display
	}

	if message.MessageType == utils.NonRTCMMessage {
		return display
	}

	readable := PrepareForDisplay(message)

	if len(message.ErrorMessage) > 0 {
		display += message.ErrorMessage + "\n"
	}

	// The message could be one of type 1005 or an MSM, which means that
	// it's a set of broken out fields and can be displayed as a string,
	// or it could have a Readable which is already a string.  If any of
	// those, add the string to the display.
	s, isString := readable.(string)
	m1005, is1005 := readable.(*type1005.Message)
	msm4, isMSM4 := readable.(*msm4Message.Message)
	msm7, isMSM7 := readable.(*msm7Message.Message)

	switch {
	case isString:
		display += s + "\n"
	case is1005:
		// The message is type 1005
		display += m1005.String()
		return display
	case isMSM4:
		display += msm4.String()

	case isMSM7:
		display += msm7.String()
	}

	return display
}

// Message contains an RTCM3 message, possibly broken out into readable form,
// or a stream of non-RTCM data.  Message type NonRTCMMessage indicates the
// second case.
type Message struct {
	// MessageType is the type of the RTCM message (the message number).
	// RTCM messages all have a positive message number.  Type NonRTCMMessage
	// is negative and indicates a stream of bytes that doesn't contain a
	// valid RTCM message, for example an NMEA message, an incomplete RTCM or
	// a corrupt RTCM.
	MessageType int

	// ErrorMessage contains any error message encountered while fetching
	// the message.
	ErrorMessage string

	// RawData is the message frame in its original binary form
	//including the header and the CRC.
	RawData []byte

	// If the message is an MSM, UTCTime points to a Time value containing
	// the time in UTC from the message timestamp.
	// If the message is not an MSM, the value is nil.
	UTCTime *time.Time

	// Readable is a broken out version of the RTCM message.  It's accessed
	// via the Readable method.
	Readable interface{}
}

// NewMessage creates a new message.
func NewMessage(messageType int, errorMessage string, bitStream []byte) *Message {

	message := Message{
		MessageType:  messageType,
		RawData:      bitStream,
		ErrorMessage: errorMessage,
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
		ErrorMessage: message.ErrorMessage,
	}
	return newMessage
}

// String takes the given Message object and returns it
// as a readable string.
func (message *Message) String() string {

	if message.Readable == nil {
		// Expand the message.  Only do this if readable is nil.
		// (This is partly to make the testing easier.  Some tests
		// set the readable part to sensible values and set a junk
		// version of the raw data.  Calling this on one of those
		// objects would trash the Readable values.)
		PrepareForDisplay(message)
	}

	display := ""

	if message.UTCTime != nil {
		// If the time is set (which should only happen if the message is an MSM),
		// display it.
		display += message.UTCTime.Format(utils.DateLayout) + "\n"
	}

	display += fmt.Sprintf("message type %d, frame length %d\n",
		message.MessageType, len(message.RawData))

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

		// If the message is a set of broken out fields, create a readable version
		// and add it to the display.

		m1005, is1005 := message.Readable.(*type1005.Message)
		msm4, isMSM4 := message.Readable.(*msm4Message.Message)
		msm7, isMSM7 := message.Readable.(*msm7Message.Message)

		switch {
		case is1005:
			display += m1005.String()

		case isMSM4:
			display += msm4.String()

		case isMSM7:
			display += msm7.String()
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

// PrepareForDisplay creates and returns the readable component of the message
// ready for String to display it.
func PrepareForDisplay(message *Message) interface{} {
	// Do this at most once for each message.
	if message.Readable == nil {
		Analyse(message)
	}
	return message.Readable
}

// CheckCRC checks the CRC of a message frame.
func CheckCRC(frame []byte) bool {
	if len(frame) < (utils.LeaderLengthBytes + utils.CRCLengthBytes) {
		return false
	}
	// The CRC is the last three bytes of the message frame.
	// The rest of the frame should produce the same CRC.
	startOfCRC := len(frame) - utils.CRCLengthBytes
	crcHiByte := frame[startOfCRC]
	crcMiByte := frame[startOfCRC+1]
	crcLoByte := frame[startOfCRC+2]

	headerAndMessage := frame[:startOfCRC]
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

// ParseGlonassTimeStamp separates out the two parts of a Glonass
// timestamp -3/27 day/milliseconds from start of day.
func ParseGlonassTimeStamp(timestamp uint) (uint, uint, error) {

	// The timestamp must fit into 30 bits.
	if timestamp > maxTimestamp {
		return 0, 0, errors.New("out of range")
	}

	day := timestamp >> 27
	millis := timestamp &^ glonassDayBitMask
	return day, millis, nil
}

// getUTCFromTimestamp converts a GPS, Galileo or Beidou timestamp to UTC
// using the given start time to find the correct week.  If the timestamp
// has rolled over, The returned start time is the start of the next week.
func getUTCFromTimestamp(timestamp, timestampFromPreviousMessage uint, startOfWeek time.Time) (timeFromTimestamp, newStartOfWeek time.Time, rangeError error) {
	// GPS, Galileo and Beidou each measure time within a week starting on a
	// Sunday, but at different times.  The timestamp in a multiple signal
	// message (MSM) is milliseconds since the start of week.  If the timestamp
	// is smaller than the one in the previous message, it's rolled over and
	// we move to the next week.  The timestamp is 30 bits maximum.

	if timestamp > maxTimestamp {
		var zeroTimeValue time.Time // 0001-01-01 00:00:00 +0000 UTC.
		rangeError = errors.New("out of range")
		return zeroTimeValue, startOfWeek, rangeError
	}

	// watch for the timestamp rolling over.
	if timestampFromPreviousMessage > timestamp {
		// The timestamp has rolled over
		newStartOfWeek = startOfWeek.AddDate(0, 0, 7) // Move to the next week.
	} else {
		newStartOfWeek = startOfWeek // Stay in the same week.
	}

	durationSinceStart := time.Duration(timestamp) * time.Millisecond

	timeFromTimestamp = newStartOfWeek.Add(durationSinceStart)

	return timeFromTimestamp, newStartOfWeek, nil
}

// // makeLogEntry writes a string to the logger.  If the logger is nil
// // it writes to the default system log.
// func (rtcmHandler *Handler) makeLogEntry(s string) {
// 	if rtcmHandler.logger == nil {
// 		log.Print(s)
// 	} else {
// 		rtcmHandler.logger.Print(s)
// 	}
// }
