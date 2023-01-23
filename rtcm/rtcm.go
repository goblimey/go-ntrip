package rtcm

import (
	"bufio"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/goblimey/go-ntrip/rtcm/message1005"
	msm4message "github.com/goblimey/go-ntrip/rtcm/msm4/message"
	msm7message "github.com/goblimey/go-ntrip/rtcm/msm7/message"
	"github.com/goblimey/go-ntrip/rtcm/utils"

	crc24q "github.com/goblimey/go-crc24q/crc24q"
)

// The rtcm package contains logic to read and decode and display RTCM3
// messages produced by GNSS devices.  See the README for this repository
// for a description of the RTCM version 3 protocol.
//
//     handler := rtcm.New(time.Now(), logger)
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
// I tested the software using UBlox equipment.  For accurate positioning
// a UBlox rover requires message type 1005 and the MSM messages.  It also
// requires type 1230 (GLONASS code/phase biases) and type 4072, which is
// in a proprietary unpublished UBlox format.  I cannot currently decipher
// either of these messages.
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

// NonRTCMMessage indicates a Message that does contain RTCM data.  Typically
// it will be a stream of data in other formats (NMEA, UBX etc).
// According to the spec, message numbers start at 1, but I've
// seen messages of type 0.
const NonRTCMMessage = -1

// oneLightMillisecond is the distance in metres traveled by light in one
// millisecond.  The value can be used to convert a range in milliseconds to a
// distance in metres.  The speed of light is 299792458.0 metres/second.
const oneLightMillisecond float64 = 299792.458

// defaultWaitTimeOnEOF is the default value for RTCM.WaitTimeOnEOF.
const defaultWaitTimeOnEOF = 100 * time.Microsecond

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

// startOfMessageFrame is the value of the byte that starts an RTCM3 message frame.
const startOfMessageFrame byte = 0xd3

// leaderLengthBytes is the length of the message frame leader in bytes.
const leaderLengthBytes = 3

// leaderLengthBits is the length of the message frame leader in bits.
const leaderLengthBits = leaderLengthBytes * 8

// crcLengthBytes is the length of the Cyclic Redundancy check value in bytes.
const crcLengthBytes = 3

// crcLengthBits is the length of the Cyclic Redundancy check value in bits.
const crcLengthBits = crcLengthBytes * 8

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

// RTCM3Message contains an RTCM3 message, possibly broken out into readable form,
// or a stream of non-RTCM data.  RTCM3Message type NonRTCMMessage indicates the
// second case.
type RTCM3Message struct {
	// MessageType is the type of the RTCM message (the message number).
	// RTCM messages all have a positive message number.  Type NonRTCMMessage
	// is negative and indicates a stream of bytes that doesn't contain a
	// valid RTCM message, for example an NMEA message or a corrupt RTCM.
	MessageType int

	// Valid is true if the message is valid - complete and the CRC checks.
	Valid bool

	// Complete is true if the message is complete.  The last bytes in a
	// log of messages may not be complete.
	Complete bool

	// CRCValid is true if the Cyclic Redundancy Check bits are valid.
	CRCValid bool

	// Warning contains any error message encountered while fetching
	// the message.
	Warning string

	// RawData is the message frame in its original binary form
	//including the header and the CRC.
	RawData []byte

	// readable is a broken out version of the RTCM message.  It's accessed
	// via the Readable method and the message is only decoded on the
	// first call.  (Lazy evaluation.)
	readable interface{}
}

// Copy makes a copy of the message and its contents.
func (message *RTCM3Message) Copy() RTCM3Message {
	// Make a copy of the raw data.
	rawData := make([]byte, len(message.RawData))
	copy(rawData, message.RawData)
	// Create a new message.  Omit the readable part - it may not be needed
	// and if it is needed, it will be created automatically at that point.
	var newMessage = RTCM3Message{
		MessageType: message.MessageType,
		RawData:     rawData,
		Valid:       message.Valid,
		Complete:    message.Complete,
		CRCValid:    message.CRCValid,
		Warning:     message.Warning,
	}
	return newMessage
}

// MSM4 returns true if the message is an MSM type 4.
func (message *RTCM3Message) MSM4() bool {

	switch message.MessageType {
	// GPS
	case 1074: // GPS
		return true
	case 1084: // Glonass
		return true
	case 1094: // Galileo
		return true
	case 1104: // SBAS
		return true
	case 1114: // QZSS
		return true
	case 1124: // Beidou
		return true
	case 1134: // NavIC/IRNSS
		return true
	default:
		return false
	}
}

// MSM7 returns true if the message is an MSM type 7.
func (message *RTCM3Message) MSM7() bool {
	switch message.MessageType {
	case 1077:
		return true
	case 1087:
		return true
	case 1097:
		return true
	case 1107:
		return true
	case 1117:
		return true
	case 1127:
		return true
	case 1137:
		return true
	default:
		return false
	}
}

// MSM returns true if the message type in the header is an Multiple Signal
// Message of any type.
func (message *RTCM3Message) MSM() bool {
	if message.MessageType == NonRTCMMessage {
		return false
	}

	return message.MSM4() || message.MSM7()
}

// displayable is true if the message type is one that we know how
// to display in a readable form.
func (message *RTCM3Message) displayable() bool {
	// we currently can display messages of type 1005, MSM4 and MSM7.

	if message.MessageType == NonRTCMMessage {
		return false
	}

	if message.MSM() || message.MessageType == 1005 {
		return true
	}

	return false
}

// HandleMessages reads from the input stream until it's exhausted, extracting any
// valid RTCM messages and copying them to those output channels which are not nil.
//
func (rtcm *RTCM) HandleMessages(reader io.Reader, channels []chan RTCM3Message) {
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
		message, messageFetchError := rtcm.ReadNextRTCM3Message(bufferedReader)
		if messageFetchError != nil {
			if message == nil {
				return
			} else {
				logEntry := fmt.Sprintf("HandleMessages ignoring error %v", messageFetchError)
				rtcm.makeLogEntry(logEntry)
			}
		}

		if message == nil {
			// There is no message yet.  Pause and try again.
			rtcm.makeLogEntry("HandleMessages: nil message - pausing")
			rtcm.pause()
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

// GetMessageLengthAndType extracts the message length and the message type from an
// RTCMs message frame or returns an error, implying that this is not the start of a
// valid message.  The bit stream must be at least 5 bytes long.
func (handler *RTCM) GetMessageLengthAndType(bitStream []byte) (uint, int, error) {

	if len(bitStream) < leaderLengthBytes+2 {
		return 0, NonRTCMMessage, errors.New("the message is too short to get the header and the length")
	}

	// The message header is 24 bits.  The top byte is startOfMessage.
	if bitStream[0] != startOfMessageFrame {
		message := fmt.Sprintf("message starts with 0x%0x not 0xd3", bitStream[0])
		return 0, NonRTCMMessage, errors.New(message)
	}

	// The next six bits must be zero.  If not, we've just come across
	// a 0xd3 byte in a stream of binary data.
	sanityCheck := utils.GetBitsAsUint64(bitStream, 8, 6)
	if sanityCheck != 0 {
		errorMessage := fmt.Sprintf("bits 8 -13 of header are %d, must be 0", sanityCheck)
		return 0, NonRTCMMessage, errors.New(errorMessage)
	}

	// The bottom ten bits of the header is the message length.
	length := uint(utils.GetBitsAsUint64(bitStream, 14, 10))
	message, messageFetchError := rtcm.GetMessage(frame)

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
	buf := make([]byte, 1)
	for {
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
		case n < leaderLengthBytes+2:
			continue

		case n == leaderLengthBytes+2:
			// We have the first three bytes of the frame so we have enough data to find
			// the length and the type of the message (which we will need in a later trip
			// around this loop).
			messageLength, messageType, err := handler.GetMessageLengthAndType(frame)
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
			expectedFrameLength = messageLength + leaderLengthBytes + crcLengthBytes
			logEntry2 := fmt.Sprintf("ReadNextRTCM3MessageFrame: expecting a %d frame", expectedFrameLength)
			handler.makeLogEntry(logEntry2)

			// Now we read the rest of the message byte by byte, one byte every trip.
			// We know how many bytes we want, so we could just read that many using one
			// Read call, but if the input stream is a serial connection, we would
			// probably need several of those, so we might as well do it this way.
			continue

		case expectedFrameLength == 0:
			// We haven't figured out the message length yet.
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
func (rtcm *RTCM) ReadNextRTCM3Message(reader *bufio.Reader) (*RTCM3Message, error) {

	frame, err1 := rtcm.ReadNextRTCM3MessageFrame(reader)
	if err1 != nil {
		return nil, err1
	}

	if len(frame) == 0 {
		return nil, nil
	}

	// Return the chunk as a Message.
	message, messageFetchError := rtcm.GetMessage(frame)
	return message, messageFetchError
}

// PrepareForDisplay returns a broken out version of the message - for example,
// if the message type is 1005, it's a Message1005.
func (m *RTCM3Message) PrepareForDisplay(r *RTCM) interface{} {
	var err error
	if m.readable == nil {
		err = r.Analyse(m)
		if err != nil {
			// Message can't be analysed.  Log an error and mark the message
			// as not valid.
			log.Println(err.Error())
			m.Valid = false
		}
	}

	return m.readable
}

func (m *RTCM3Message) SetReadable(r interface{}) {
	m.readable = r
}

func init() {
	locationUTC, _ = time.LoadLocation("UTC")
	locationGMT, _ = time.LoadLocation("GMT")
	locationMoscow, _ = time.LoadLocation("Europe/Moscow")
}

// New creates an RTCM object using the given year, month and day to
// identify which week the times in the messages refer to.
func New(startTime time.Time, logger *log.Logger) *RTCM {

	rtcm := RTCM{logger: logger, WaitTimeOnEOF: defaultWaitTimeOnEOF}

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

func (r *RTCM) SetDisplayWriter(displayWriter io.Writer) {
	r.displayWriter = displayWriter

}

// GetMessage extracts an RTCM3 message from the given bit stream and returns it
// as an RTC3Message. If the data doesn't contain a valid message, it returns a
// message with type NonRTCMMessage.
//
func (handler *RTCM) GetMessage(bitStream []byte) (*RTCM3Message, error) {

	if len(bitStream) == 0 {
		return nil, errors.New("zero length message frame")
	}

	if bitStream[0] != startOfMessageFrame {
		// This is not an RTCM message.
		message := RTCM3Message{
			MessageType: NonRTCMMessage,
			RawData:     bitStream,
		}
		return &message, nil
	}

	messageLength, messageType, formatError := handler.GetMessageLengthAndType(bitStream)
	if formatError != nil {
		message := RTCM3Message{
			MessageType: messageType,
			RawData:     bitStream,
			Warning:     formatError.Error(),
		}
		return &message, formatError
	}

	if messageType == NonRTCMMessage {
		message := RTCM3Message{MessageType: messageType, RawData: bitStream}
		return &message, nil
	}

	// The message frame should contain a header, the variable-length message and
	// the CRC.  We now know the message length, so we can check that we have the
	// whole thing.

	frameLength := uint(len(bitStream))
	expectedFrameLength := messageLength + leaderLengthBytes + crcLengthBytes
	// The message is analysed only when necessary (lazy evaluation).  For
	// now, just copy the byte stream into the Message.
	if expectedFrameLength > frameLength {
		// The message is incomplete, return what we have.
		// (This can happen if it's the last message in the input stream.)
		warning := "incomplete message frame"
		message := RTCM3Message{
			MessageType: messageType,
			RawData:     bitStream[:frameLength],
			Warning:     warning,
		}
		return &message, errors.New(warning)
	}

	// We have a complete message.

	message := RTCM3Message{
		MessageType: messageType,
		RawData:     bitStream[:expectedFrameLength],
		Complete:    true,
	}

	// Check the CRC.
	if !CheckCRC(bitStream) {
		errorMessage := "CRC failed"
		message.Warning = errorMessage
		return &message, errors.New(errorMessage)
	}
	message.CRCValid = true

	// the message is complete and the CRC check passes, so it's valid.
	message.Valid = true

	return &message, nil
}

// Analyse decodes the raw byte stream and fills in the broken out message.
func (rtcm *RTCM) Analyse(message *RTCM3Message) error {
	var readable interface{}

	// The raw data contains the whole message frame: the leader, the MSM message
	// and the CRC.  Reduce it to just the MSM message.
	low := leaderLengthBytes
	high := len(message.RawData) - crcLengthBytes
	messageBitStream := message.RawData[low:high]

	switch {
	case message.MSM4():
		msm4Message, err := msm4message.GetMessage(messageBitStream)
		// Convert the EpochTime to UTC.  This requires keeping a notion of time
		// over many messages, potentially for many days, so it must be done by
		// this module.
		switch msm4Message.Header.MessageType {
		case 1074:
			msm4Message.Header.UTCTime =
				rtcm.GetUTCFromGPSTime(msm4Message.Header.EpochTime)
		case 1084:
			msm4Message.Header.UTCTime =
				rtcm.GetUTCFromGlonassTime(msm4Message.Header.EpochTime)
		case 1094:
			msm4Message.Header.UTCTime =
				rtcm.GetUTCFromGalileoTime(msm4Message.Header.EpochTime)
		case 1124:
			msm4Message.Header.UTCTime =
				rtcm.GetUTCFromBeidouTime(msm4Message.Header.EpochTime)
		default:
			// This MSM is one that we don't know how to decode.
			// Leave the UTCTime set to zero.
		}
		message.SetReadable(msm4Message)
		return err
	case message.MSM7():
		msm7Message, err := msm7message.GetMessage(messageBitStream)
		// Convert the EpochTime to UTC.  This requires keeping a notion of time
		// over many messages, potentially for many days, so it must be done by
		// this module.
		switch msm7Message.Header.MessageType {
		case 1077:
			msm7Message.Header.UTCTime =
				rtcm.GetUTCFromGPSTime(msm7Message.Header.EpochTime)
		case 1087:
			msm7Message.Header.UTCTime =
				rtcm.GetUTCFromGlonassTime(msm7Message.Header.EpochTime)
		case 1097:
			msm7Message.Header.UTCTime =
				rtcm.GetUTCFromGalileoTime(msm7Message.Header.EpochTime)
		case 1127:
			msm7Message.Header.UTCTime =
				rtcm.GetUTCFromBeidouTime(msm7Message.Header.EpochTime)
		default:
			// This MSM is one that we don't know how to decode.
			// Leave the UTCTime set to zero.
		}
		message.SetReadable(msm7Message)
		return err

	case message.MessageType == 1005:
		readable, err := message1005.GetMessage(messageBitStream)
		message.SetReadable(readable)
		return err
	case message.MessageType == 1230:
		readable = "(Message type 1230 - GLONASS code-phase biases - don't know how to decode this.)"
		message.SetReadable(readable)
		return nil
	case message.MessageType == 4072:
		readable = "(Message type 4072 is in an unpublished format defined by U-Blox.)"
		message.SetReadable(readable)
		return nil
	default:
		errorMessage := fmt.Sprintf("message type %d currently cannot be displayed",
			message.MessageType)
		message.SetReadable(errorMessage)
		return errors.New(errorMessage)
	}
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
func (handler *RTCM) DisplayMessage(message *RTCM3Message) string {

	if message.MessageType == NonRTCMMessage {
		return fmt.Sprintf("not RTCM, %d bytes, %s\n%s\n",
			len(message.RawData), message.Warning, hex.Dump(message.RawData))
	}

	status := ""
	if message.Valid {
		status = "valid"
	} else {
		if message.Complete {
			status = "complete "
		} else {
			status = "incomplete "
		}
		if message.CRCValid {
			status += "CRC check passed"
		} else {
			status += "CRC check failed"
		}
	}

	leader := fmt.Sprintf("message type %d, frame length %d %s %s\n",
		message.MessageType, len(message.RawData), status, message.Warning)
	leader += fmt.Sprintf("%s\n", hex.Dump(message.RawData))

	if !message.Valid {
		return leader
	}

	if !message.displayable() {
		return leader
	}

	switch {

	case message.MessageType == 1005:
		m, ok := message.PrepareForDisplay(handler).(*message1005.Message)
		if !ok {
			return ("expected the readable message to be *Message1005\n")
		}
		return leader + m.Display()

	case message.MSM4():
		m, ok := message.PrepareForDisplay(handler).(*msm4message.Message)
		if !ok {
			return ("expected the readable message to be an MSM4\n")
		}
		return leader + m.Display()

	case message.MSM7():
		m, ok := message.PrepareForDisplay(handler).(*msm7message.Message)
		if !ok {
			return ("expected the readable message to be an MSM7\n")
		}
		return leader + m.Display()

	default:
		return leader + "\n"
	}
}

// CheckCRC checks the CRC of a message frame.
func CheckCRC(frame []byte) bool {
	if len(frame) < leaderLengthBytes+crcLengthBytes {
		return false
	}
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

// ParseGlonassEpochTime separates out the two parts of a Glonass
// epoch time value -3/27 day/milliseconds from start of day.
//
func ParseGlonassEpochTime(epochTime uint) (uint, uint) {
	// fmt.Printf("ParseGlonassEpochTime %x\n", epochTime)
	day := (epochTime & glonassDayBitMask) >> 27
	millis := epochTime &^ glonassDayBitMask
	return day, millis
}

// pause sleeps for the time defined in the RTCM.
func (rtcm *RTCM) pause() {
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
func (rtcm *RTCM) makeLogEntry(s string) {
	if rtcm.logger == nil {
		log.Print(s)
	} else {
		rtcm.logger.Print(s)
	}
}
