package handler

import (
	"bufio"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/goblimey/go-ntrip/rtcm/message1005"
	msm4Message "github.com/goblimey/go-ntrip/rtcm/msm4/message"
	msm7Message "github.com/goblimey/go-ntrip/rtcm/msm7/message"
	"github.com/goblimey/go-ntrip/rtcm/rtcm3"
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

// defaultWaitTimeOnEOF is the default value for RTCM.WaitTimeOnEOF.
const defaultWaitTimeOnEOF = 100 * time.Microsecond

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

// startOfMessageFrame is the value of the byte that starts an RTCM3 message frame.
const startOfMessageFrame byte = 0xd3

// RTCM is the object used to fetch and analyse RTCM3 messages.
type RTCM struct {

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

// HandleMessages reads from the input stream until it's exhausted, extracting any
// valid RTCM messages and copying them to those output channels which are not nil.
//
func (handler *RTCM) HandleMessages(reader io.Reader, channels []chan rtcm3.Message) {
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
		message, messageFetchError := handler.ReadNextRTCM3Message(bufferedReader)
		if messageFetchError != nil {
			if message == nil {
				return
			} else {
				logEntry := fmt.Sprintf("HandleMessages ignoring error %v", messageFetchError)
				handler.makeLogEntry(logEntry)
			}
		}

		if message == nil {
			// There is no message yet.  Pause and try again.
			handler.makeLogEntry("HandleMessages: nil message - pausing")
			handler.pause()
			continue
		}

		// Send a copy of the message to each of the non-nil channels.
		for i := range channels {
			if channels[i] != nil {
				messageCopy := message.Copy()
				channels[i] <- messageCopy
			}
		}
	}
}

// getMessageLengthAndType extracts the message length and the message type from an
// RTCMs message frame or returns an error, implying that this is not the start of a
// valid message.  The bit stream must be at least 5 bytes long.
func (handler *RTCM) getMessageLengthAndType(bitStream []byte) (uint, int, error) {

	if len(bitStream) < (utils.LeaderLengthBytes + 2) {
		return 0, utils.NonRTCMMessage, errors.New("the message is too short to get the header and the length")
	}

	// The message header is 24 bits.  The top byte is startOfMessage.
	if bitStream[0] != startOfMessageFrame {
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

	// The bottom ten bits of the leader is the message length.
	length := uint(utils.GetBitsAsUint64(bitStream, 14, 10))

	// The 12-bit message type follows the header.
	messageType := int(utils.GetBitsAsUint64(bitStream, 24, 12))

	// length must be > 0. (Defer this check until now, when we have the message type.)
	if length == 0 {
		errorMessage := fmt.Sprintf("zero length message type %d", messageType)
		return 0, messageType, errors.New(errorMessage)
	}

	return length, messageType, nil
}

// ReadNextRTCM3MessageFrame gets the next message frame from a reader.  The incoming
// byte stream contains RTCM messages interspersed with messages in other
// formats such as NMEA, UBX etc.   The resulting slice contains either a
// single valid message or some non-RTCM text that precedes a message.  If
// the function encounters a fatal read error and it has not yet read any
// text, it returns the error.  If it has read some text, it just returns
// that (the assumption being that the next call will get no text and the
// same error).  Use GetMessage to extract the message from the result.
func (handler *RTCM) ReadNextRTCM3MessageFrame(reader *bufio.Reader) ([]byte, error) {

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
		frame, eatError = reader.ReadBytes(startOfMessageFrame)
		if eatError != nil {
			// We only deal with an error if there's nothing in the buffer.
			// If there is any text, we deal with that and assume that we will see
			// any hard error again on the next call.
			if len(frame) == 0 {
				// An error and no bytes in the frame.  Deal with the error.
				if eatError == io.EOF {
					if handler.StopOnEOF {
						// EOF is fatal for the kind of input file we are reading.
						logEntry := "ReadNextRTCM3MessageFrame: hard EOF while eating"
						handler.makeLogEntry(logEntry)
						return nil, eatError
					} else {
						// For this kind of input, EOF just means that there is nothing
						// to read just yet, but there may be something later.  So we
						// just return, expecting the caller to call us again.
						logEntry := "ReadNextRTCM3MessageFrame: non-fatal EOF while eating"
						handler.makeLogEntry(logEntry)
						return nil, nil
					}
				} else {
					// Any error other than EOF is always fatal.  Return immediately.
					logEntry := fmt.Sprintf("ReadNextRTCM3MessageFrame: error at start of eating - %v", eatError)
					handler.makeLogEntry(logEntry)
					return nil, eatError
				}
			} else {
				logEntry := fmt.Sprintf("ReadNextRTCM3MessageFrame: continuing after error,  eaten %d bytes - %v",
					len(frame), eatError)
				handler.makeLogEntry(logEntry)
			}
		}

		if len(frame) == 0 {
			// We've got nothing.  Pause and try again.
			logEntry := "ReadNextRTCM3MessageFrame: frame is empty while eating, but no error"
			handler.makeLogEntry(logEntry)
			handler.pause()
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
		logEntry := fmt.Sprintf("ReadNextRTCM3MessageFrame: read %d bytes", len(frame))
		handler.makeLogEntry(logEntry)
		if frame[len(frame)-1] == startOfMessageFrame {
			// non-RTCM followed by start of message byte.  Push the start
			// byte back so we see it next time and return the rest of the
			// buffer as a non-RTCM message.
			logEntry1 := "ReadNextRTCM3MessageFrame: found d3 - unreading"
			handler.makeLogEntry(logEntry1)
			reader.UnreadByte()
			frameWithoutTrailingStartByte := frame[:len(frame)-1]
			logEntry2 := fmt.Sprintf("ReadNextRTCM3MessageFrame: returning %d bytes %s",
				len(frameWithoutTrailingStartByte),
				hex.Dump(frameWithoutTrailingStartByte))
			handler.makeLogEntry(logEntry2)
			return frameWithoutTrailingStartByte, nil
		} else {
			// Just some non-RTCM.
			logEntry := fmt.Sprintf("ReadNextRTCM3MessageFrame: got: %d bytes %s",
				len(frame),
				hex.Dump(frame))
			handler.makeLogEntry(logEntry)
			return frame, nil
		}
	}

	// The buffer contains just a start of message byte so
	// we may have the start of an RTCM message frame.
	// Get the rest of the message frame.
	logEntry := "ReadNextRTCM3MessageFrame: found d3 immediately"
	handler.makeLogEntry(logEntry)
	var n int = 1
	var expectedFrameLength uint = 0
	for {
		// Read and handle the next byte.
		buf := make([]byte, 1)
		l, readErr := reader.Read(buf)
		// We've read some text, so log any read error, but ignore it.  If it's
		// a hard error it will be caught on the next call.
		if readErr != nil {
			if readErr != io.EOF {
				// Any error other than EOF is always fatal, but it will be caught
				logEntry := fmt.Sprintf("ReadNextRTCM3MessageFrame: ignoring error while reading message - %v", readErr)
				handler.makeLogEntry(logEntry)
				return frame, nil
			}

			if handler.StopOnEOF {
				// EOF is fatal for the kind of input file we are reading.
				logEntry := "ReadNextRTCM3MessageFrame: ignoring fatal EOF"
				handler.makeLogEntry(logEntry)
				return frame, nil
			} else {
				// For this kind of input, EOF just means that there is nothing
				// to read just yet, but there may be something later.  So we
				// just pause and try again.
				logEntry := "ReadNextRTCM3MessageFrame: ignoring non-fatal EOF"
				handler.makeLogEntry(logEntry)
				handler.pause()
				continue
			}
		}

		if l < 1 {
			// We expected to read exactly one byte, so there is currently
			// nothing to read.  Pause and try again.
			logEntry := "ReadNextRTCM3MessageFrame: no data.  Pausing"
			handler.makeLogEntry(logEntry)
			handler.pause()
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
		case n < utils.LeaderLengthBytes+2:
			// We haven't read enough bytes to figure out the message length yet.
			continue

		case n == utils.LeaderLengthBytes+2:
			// We have the first three bytes of the frame so we have enough data to find
			// the length and the type of the message (which we will need in a later trip
			// around this loop).
			messageLength, messageType, err := handler.getMessageLengthAndType(frame)
			if err != nil {
				// We thought we'd found the start of a message, but it's something else
				// that happens to start with the start of frame byte.
				// Return the collected data.
				logEntry := fmt.Sprintf("ReadNextRTCM3MessageFrame: error getting length and type: %v", err)
				handler.makeLogEntry(logEntry)
				return frame, nil
			}

			logEntry1 := fmt.Sprintf("ReadNextRTCM3MessageFrame: found message type %d length %d", messageType, messageLength)
			handler.makeLogEntry(logEntry1)

			// The frame contains a 3-byte header, a variable-length message (for which
			// we now know the length) and a 3-byte CRC.  Now we just need to continue to
			// read bytes until we have the whole message.
			expectedFrameLength = messageLength + utils.LeaderLengthBytes + utils.CRCLengthBytes
			logEntry2 := fmt.Sprintf("ReadNextRTCM3MessageFrame: expecting a %d frame", expectedFrameLength)
			handler.makeLogEntry(logEntry2)

			// Now we read the rest of the message byte by byte, one byte every trip.
			// We know how many bytes we want, so we could just read that many using one
			// Read call, but if the input stream is a serial connection, we would
			// probably need several of those, so we might as well do it this way.
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
			logEntry := fmt.Sprintf("ReadNextRTCM3MessageFrame: returning an RTCM message frame, %d bytes, expected %d", n, expectedFrameLength)
			handler.makeLogEntry(logEntry)
			return frame, nil

		default:
			// In most trips around the loop, we just read the next byte and build up the
			// message frame.
			continue
		}
	}
}

// ReadNextRTCM3Message gets the next message frame from a reader, extracts
// and returns the message.  It returns any read error that it encounters,
// such as EOF.
func (handler *RTCM) ReadNextRTCM3Message(reader *bufio.Reader) (*rtcm3.Message, error) {

	frame, err1 := handler.ReadNextRTCM3MessageFrame(reader)
	if err1 != nil {
		return nil, err1
	}

	if len(frame) == 0 {
		return nil, nil
	}

	// Return the chunk as a Message.
	message, messageFetchError := handler.GetMessage(frame)
	return message, messageFetchError
}

// New creates an RTCM object using the given year, month and day to
// identify which week the times in the messages refer to.
func New(startTime time.Time, logger *log.Logger) *RTCM {

	handler := RTCM{logger: logger, WaitTimeOnEOF: defaultWaitTimeOnEOF}

	// Convert the start date to UTC.
	startTime = startTime.In(utils.LocationUTC)

	// Get the start of last Sunday in UTC. (If today is Sunday, the start
	// of today.)

	startOfWeekUTC := getStartOfLastSundayUTC(startTime)

	// GPS.  The GPS week starts gpsLeapSeconds before midnight at the
	// start of Sunday in UTC, ie on Saturday just before midnight.  So
	// most of Saturday UTC is the end of one GPS week but the last few
	// seconds are the beginning of the next.
	//
	if startTime.Weekday() == time.Saturday {
		// This is saturday, either in the old GPS week or the new one.
		// Get the time when the new GPS week starts (or started).
		sunday := startTime.AddDate(0, 0, 1)
		midnightNextSunday := getStartOfLastSundayUTC(sunday)
		gpsWeekStart := midnightNextSunday.Add(gpsTimeOffset)
		if startTime.Equal(gpsWeekStart) || startTime.After(gpsWeekStart) {
			// It's Saturday in the first few seconds of a new GPS week
			handler.startOfGPSWeek = gpsWeekStart
			// Galileo keeps GPS time.
			handler.startOfGalileoWeek = gpsWeekStart

		} else {
			// It's Saturday at the end of a GPS week.
			midnightLastSunday := getStartOfLastSundayUTC(startTime)
			handler.startOfGPSWeek = midnightLastSunday.Add(gpsTimeOffset)
			// Galileo keeps GPS time.
			handler.startOfGalileoWeek = midnightLastSunday.Add(gpsTimeOffset)
		}
	} else {
		// It's not Saturday.  The GPS week started just before midnight
		// at the end of last Saturday.
		midnightLastSunday := getStartOfLastSundayUTC(startTime)
		handler.startOfGPSWeek = midnightLastSunday.Add(gpsTimeOffset)
		// Galileo keeps GPS time
		handler.startOfGalileoWeek = midnightLastSunday.Add(gpsTimeOffset)
	}

	handler.timestampFromPreviousGPSMessage = (uint(startTime.Sub(handler.startOfGPSWeek).Milliseconds()))
	// Galileo keeps GPS time.
	handler.timestampFromPreviousGalileoMessage = handler.timestampFromPreviousGPSMessage

	// Beidou.
	// Get the start of this Beidou week.  Despite
	// https://www.unoosa.org/pdf/icg/2016/Beidou-Timescale2016.pdf
	// the correct offset appears to be +14 seconds!!!

	handler.startOfBeidouWeek = startOfWeekUTC.Add(beidouTimeOffset)

	if startTime.Before(handler.startOfBeidouWeek) {
		// The given start date is in the previous Beidou week.  (This
		// happens if it's within the first few seconds of Sunday UTC.)
		handler.startOfBeidouWeek = handler.startOfBeidouWeek.AddDate(0, 0, -7)
	}

	handler.timestampFromPreviousBeidouMessage =
		(uint(startTime.Sub(handler.startOfBeidouWeek).Milliseconds()))

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

func (r *RTCM) SetDisplayWriter(displayWriter io.Writer) {
	r.displayWriter = displayWriter

}

// GetMessage extracts an RTCM3 message from the given bit stream and returns it
// as an RTC3Message. If the bit stream is empty, it returns an error.  If the data
// doesn't contain a valid message, it returns a message with type NonRTCMMessage.
//
func (handler *RTCM) GetMessage(bitStream []byte) (*rtcm3.Message, error) {

	if len(bitStream) == 0 {
		return nil, errors.New("zero length message frame")
	}

	if bitStream[0] != startOfMessageFrame {
		// This is not an RTCM message.
		return rtcm3.NewNonRTCM(bitStream), nil
	}

	messageLength, messageType, formatError := handler.getMessageLengthAndType(bitStream)
	if formatError != nil {
		return rtcm3.New(messageType, formatError.Error(), bitStream), formatError
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
		warning := "incomplete message frame"
		return rtcm3.New(utils.NonRTCMMessage, warning, bitStream[:frameLength]), errors.New(warning)
	}

	// We have a complete message.

	// Check the CRC.
	if !CheckCRC(bitStream) {
		warning := "CRC is not valid"
		return rtcm3.New(utils.NonRTCMMessage, warning, bitStream[:frameLength]), errors.New(warning)
	}

	// The message is complete and the CRC check passes, so it's valid.
	message := rtcm3.New(messageType, "", bitStream[:expectedFrameLength])

	// If the message is an MSM7, get the time (for the heading if displaying)
	// The message frame is: 3 bytes of leader, a 12-bit message type, a 12-bit
	// station ID followed by the 30-bit epoch time, followed by lots of other
	// stuff and finally a 3-byte CRC.  If we get to here then the leader and
	// CRC are present and the message contains at least a complete header.

	const startBit = 48 // Leader plus 24 bits.
	const timestampLength = 30

	if utils.MSM(message.MessageType) {

		// The message is an MSM so get the timestamp and set the UTCTime.

		timestamp := uint(utils.GetBitsAsUint64(bitStream, startBit, timestampLength))

		utcTime, timeError := handler.getTimeFromTimeStamp(message.MessageType, timestamp)

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
func Analyse(message *rtcm3.Message) {
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

func analyseMSM4(messageBitStream []byte, message *rtcm3.Message) {
	msm4Message, msm4Error := msm4Message.GetMessage(messageBitStream)
	if msm4Error != nil {
		message.ErrorMessage = msm4Error.Error()
		return
	}

	message.Readable = msm4Message
}

func analyseMSM7(messageBitStream []byte, message *rtcm3.Message) {
	msm7Message, msm7Error := msm7Message.GetMessage(messageBitStream)
	if msm7Error != nil {
		message.ErrorMessage = msm7Error.Error()
		return
	}

	message.Readable = msm7Message
}

func analyse1005(messageBitStream []byte, message *rtcm3.Message) {
	message1005, message1005Error := message1005.GetMessage(messageBitStream)
	if message1005Error != nil {
		message.ErrorMessage = message1005Error.Error()
		return
	}

	message.Readable = message1005
}

// getTimeFromTimeStamp converts the 30-bit timestamp in the MSM header to a time value
// in the UTC timezone.  The message must be an MSM as others don't have a timestamp.
func (handler *RTCM) getTimeFromTimeStamp(messageType int, timestamp uint) (time.Time, error) {

	var zeroTimeValue time.Time

	// Convert the timestamp to UTC.  This requires keeping a notion of time
	// over many messages, potentially for many days, so it must be done by
	// this module.
	//
	// The Glonass timestamp has an invalid value, so the Glonass converter can
	// return an error.

	switch messageType {
	case utils.MessageTypeMSM4GPS:
		utcTime, err := handler.getUTCFromGPSTime(timestamp)
		return utcTime, err
	case utils.MessageTypeMSM7GPS:
		utcTime, err := handler.getUTCFromGPSTime(timestamp)
		return utcTime, err
	case utils.MessageTypeMSM4Glonass:
		utcTime, err := handler.getUTCFromGlonassTime(timestamp)
		return utcTime, err
	case utils.MessageTypeMSM7Glonass:
		utcTime, err := handler.getUTCFromGlonassTime(timestamp)
		return utcTime, err
	case utils.MessageTypeMSM4Galileo:
		utcTime, err := handler.getUTCFromGalileoTime(timestamp)
		return utcTime, err
	case utils.MessageTypeMSM7Galileo:
		utcTime, err := handler.getUTCFromGalileoTime(timestamp)
		return utcTime, err
	case utils.MessageTypeMSM4Beidou:
		utcTime, err := handler.getUTCFromBeidouTime(timestamp)
		return utcTime, err
	case utils.MessageTypeMSM7Beidou:
		utcTime, err := handler.getUTCFromBeidouTime(timestamp)
		return utcTime, err
	default:
		// This MSM is one that we don't know how to decode.
		return zeroTimeValue, errors.New("unknown message type")
	}
}

// GetUTCFromGPSTime converts a GPS time to UTC, using the start time
// to find the correct epoch.
//
func (handler *RTCM) getUTCFromGPSTime(timestamp uint) (time.Time, error) {
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
		timestamp, handler.timestampFromPreviousGPSMessage,
		handler.startOfGPSWeek)

	if err != nil {
		return timeFromTimestamp, err
	}

	// We may have moved into the next week.
	handler.startOfGPSWeek = newStartOfWeek

	// Get ready for the next call.
	handler.timestampFromPreviousGPSMessage = timestamp

	return timeFromTimestamp, nil
}

// GetUTCFromGlonassTimestamp converts a Glonass timestamp to UTC using
// the start time to give the correct Glonass week.
func (handler *RTCM) getUTCFromGlonassTime(timestamp uint) (time.Time, error) {
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
	if day != handler.glonassDayFromPreviousMessage {
		// The day has rolled over.
		handler.startOfGlonassDay =
			handler.startOfGlonassDay.AddDate(0, 0, 1)
	}

	// Add the millisecond offset from the timestamp
	offset := time.Duration(millis) * time.Millisecond
	timeFromTimestamp := handler.startOfGlonassDay.Add(offset)

	// Set the day ready for next time.
	handler.glonassDayFromPreviousMessage = uint(timeFromTimestamp.Weekday())

	return timeFromTimestamp, nil

}

// GetUTCFromGalileoTime converts a Galileo time to UTC, using the same epoch
// as the start time.
//
func (handler *RTCM) getUTCFromGalileoTime(timestamp uint) (time.Time, error) {
	// Galileo follows GPS time, but we keep separate state variables.
	//
	// Note: we have to be careful when the start time is Saturday
	// and close to midnight, because that is within the new GPS
	// week.  If create a handler around then, we have to specify
	// the start time carefully.

	timeFromTimestamp, newStartOfWeek, err := getUTCFromTimestamp(
		timestamp,
		handler.timestampFromPreviousGalileoMessage,
		handler.startOfGPSWeek)

	if err != nil {
		return timeFromTimestamp, err
	}

	// We may have moved into the next week.
	handler.startOfGalileoWeek = newStartOfWeek

	// Get ready for the next call.
	handler.timestampFromPreviousGalileoMessage = timestamp

	return timeFromTimestamp, nil
}

// GetUTCFromBeidouTime converts a Baidou time to UTC, using the Beidou
// epoch given by the start time.
//
func (handler *RTCM) getUTCFromBeidouTime(timestamp uint) (time.Time, error) {

	// The Beidou week starts at midnight at the start of Sunday
	// but Beidou time is behind UTC by a few seconds, so in UTC
	// terms the week starts a few seconds after Saturday/Sunday
	// midnight.
	//
	// Note: we have to be careful when the start time is Saturday
	// and just after midnight, because that is within the new Beidou
	// week.  If create a handler around then, we have to specify
	// the start time carefully.

	timeFromTimestamp, newStartOfWeek, err := getUTCFromTimestamp(
		timestamp, handler.timestampFromPreviousBeidouMessage,
		handler.startOfBeidouWeek)

	if err != nil {
		return timeFromTimestamp, err
	}

	// We may have moved into the next week.
	handler.startOfBeidouWeek = newStartOfWeek

	// Get ready for the next call.
	handler.timestampFromPreviousBeidouMessage = timestamp

	return timeFromTimestamp, nil
}

// getStartOfLastSundayUTC gets midnight at the start of the
// last Sunday (which may be today) in UTC.
//
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
//
func (handler *RTCM) DisplayMessage(message *rtcm3.Message) string {

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

	// In some cases the displayable is a simple string.
	m, ok := readable.(string)
	if ok {
		display += m + "\n"
		return display
	}

	if len(message.ErrorMessage) > 0 {
		display += message.ErrorMessage + "\n"
		// return display
	}

	// The message is a set of broken out fields.  Create a readable version.  If that reveals
	// an error, the Valid flag will be unset and a warning added to the message.
	switch {

	case message.MessageType == 1005:
		m, ok := readable.(*message1005.Message)
		if !ok {
			// Internal error:  the message says the data are a type 1005 (base position)
			// message but when decoded they are not.
			display += "the readable message should be a message type 1005\n"
			break
		}
		display += m.String()

	case utils.MSM4(message.MessageType):
		m, ok := readable.(*msm4Message.Message)
		if !ok {
			// Internal error:  the message says the data are an MSM4
			// message but when decoded they are not.
			display += "the readable message should be an MSM4\n"
			break
		}
		display += m.String()

	case utils.MSM7(message.MessageType):
		m, ok := readable.(*msm7Message.Message)
		if !ok {
			// Internal error:  the message says the data are an MSM4
			// message but when decoded they are not.
			display += "the readable message should be an MSM7\n"
			break
		}
		display += m.String()

		// The default case can't be reached - Readable is only set if the
		// rtcm handler's PrepareForDisplay has been called.  That calls
		// Analyse which sets Readable field to an error message if it can't
		// display the message.  That case was taken care of earlier.
		//
		// default:
		// 	display += "the message is not displayable\n"
	}

	return display
}

// PrepareForDisplay creates and returns the readable component of the message
// ready for String to display it.
func PrepareForDisplay(message *rtcm3.Message) interface{} {
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
	crcHiByte := frame[len(frame)-3]
	crcMiByte := frame[len(frame)-2]
	crcLoByte := frame[len(frame)-1]

	l := len(frame) - utils.CRCLengthBytes
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

// ParseGlonassTimeStamp separates out the two parts of a Glonass
// timestamp -3/27 day/milliseconds from start of day.
//
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

// pause sleeps for the time defined in the RTCM.
func (handler *RTCM) pause() {
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
func (handler *RTCM) makeLogEntry(s string) {
	if handler.logger == nil {
		log.Print(s)
	} else {
		handler.logger.Print(s)
	}
}
