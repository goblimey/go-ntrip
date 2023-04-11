package filehandler

import (
	"bufio"
	"io"
	"time"

	rtcm "github.com/goblimey/go-ntrip/rtcm/handler"
)

// Handler provides code to handle a text file containing RTCM3 messages, possibly
// interspersed with messages of other formats.  It's assumed that the file is no
// longer being written to.  (To handle a file which is being written to, such as
// a USB connection fed by an NTRIP source, see the serial line handler.)
//
type Handler struct {
	RTCMHandler rtcm.Handler
	MessageChan chan rtcm.Message
}

// New creates a handler using the given start time.
func New(startTime time.Time, messageChan chan rtcm.Message) *Handler {
	rtcmHandler := rtcm.New(startTime)
	handler := Handler{RTCMHandler: *rtcmHandler, MessageChan: messageChan}
	return &handler
}

// Handle reads the file.  Extracted RTCM messages are sent to the handler's message
// channel. If there is a read error (other than EOF), it's returned.
//
func (handler *Handler) Handle(reader *bufio.Reader) error {

	// Create a buffered channel big enough to hold a few messages.
	sourceChan := make(chan byte, 10000)

	// Set up an RTCM handler connected to the input and output channels.
	go handler.RTCMHandler.HandleMessagesFromChannel(sourceChan, handler.MessageChan)

	// Read the file and send the data to the channel.
	for {
		buf := make([]byte, 1)
		n, err := reader.Read(buf)
		if err != nil {
			close(sourceChan)
			if err == io.EOF {
				return nil // EOF - expected behaviour.
			} else {
				// Some kind of file handling error.  (This is VERY difficult
				// to provoke during testing.)
				return err
			}
		}

		// We have read a byte.  Send it to the channel to be handled.
		if n > 0 {
			sourceChan <- buf[0]
		}
	}
}

// // ReadNextRTCM3Message gets the next message frame from a reader, extracts
// // and returns the message.  It returns any read error that it encounters,
// // such as EOF.
// func (handler *Handler) ReadNextRTCM3Message(reader *bufio.Reader) (*rtcm.Message, error) {

// 	frame, err1 := handler.readNextRTCM3MessageFrame(reader)
// 	if err1 != nil {
// 		return nil, err1
// 	}

// 	if len(frame) == 0 {
// 		return nil, nil
// 	}

// 	// Return the chunk as a Message.
// 	message, messageFetchError := handler.RTCMHandler.getMessage(frame)
// 	return message, messageFetchError
// }

// // readNextRTCM3MessageFrame gets the next message frame from a reader.  The
// // incoming byte stream contains RTCM messages interspersed with messages in
// // other formats such as NMEA, UBX etc.   The resulting slice contains either
// // a single valid message or some non-RTCM text that precedes a message.  If
// // the function encounters a fatal read error and it has not yet read any text,
// // it returns the error.  If it has read some text, it just returns that (on
// // the assumption that the next call will get no text and the same error).
// //
// func (rtcmHandler *Handler) readNextRTCM3MessageFrame(reader *bufio.Reader) ([]byte, error) {

// 	// A valid RTCM message frame is a header containing the start of message
// 	// byte and two bytes containing a 10-bit message length, zero padded to
// 	// the left, for example 0xd3, 0x00, 0x8a.  The variable-length message
// 	// comes next and always starts with a two-byte message type.  It may be
// 	// padded with zero bytes at the end.  The message frame then ends with a
// 	// 3-byte Cyclic Redundancy Check value.

// 	// Call ReadBytes until we get some text or a fatal error.
// 	var frame = make([]byte, 0)
// 	var eatError error
// 	for {
// 		// Eat bytes until we see the start of message byte.
// 		frame, eatError = reader.ReadBytes(utils.StartOfMessageFrame)
// 		if eatError != nil {
// 			// We only deal with an error if there's nothing in the buffer.
// 			// If there is any text, we deal with that and assume that we will see
// 			// any hard error again on the next call.
// 			if len(frame) == 0 {
// 				// An error and no bytes in the frame.  Deal with the error.
// 				if eatError == io.EOF {
// 					if rtcmHandler.StopOnEOF {
// 						// EOF is fatal for the kind of input file we are reading.
// 						logEntry := "ReadNextRTCM3MessageFrame: hard EOF while eating"
// 						rtcmHandler.makeLogEntry(logEntry)
// 						return nil, eatError
// 					} else {
// 						// For this kind of input, EOF just means that there is nothing
// 						// to read just yet, but there may be something later.  So we
// 						// just return, expecting the caller to call us again.
// 						logEntry := "ReadNextRTCM3MessageFrame: non-fatal EOF while eating"
// 						rtcmHandler.makeLogEntry(logEntry)
// 						return nil, nil
// 					}
// 				} else {
// 					// Any error other than EOF is always fatal.  Return immediately.
// 					logEntry := fmt.Sprintf("ReadNextRTCM3MessageFrame: error at start of eating - %v", eatError)
// 					rtcmHandler.makeLogEntry(logEntry)
// 					return nil, eatError
// 				}
// 			} else {
// 				logEntry := fmt.Sprintf("ReadNextRTCM3MessageFrame: continuing after error,  eaten %d bytes - %v",
// 					len(frame), eatError)
// 				rtcmHandler.makeLogEntry(logEntry)
// 			}
// 		}

// 		if len(frame) == 0 {
// 			// We've got nothing.  Pause and try again.
// 			logEntry := "ReadNextRTCM3MessageFrame: frame is empty while eating, but no error"
// 			rtcmHandler.makeLogEntry(logEntry)
// 			continue
// 		}

// 		// We've read some text.
// 		break
// 	}

// 	// Figure out what ReadBytes has returned.  Could be a start of message byte,
// 	// some other text followed by the start of message byte or just some other
// 	// text.
// 	if len(frame) > 1 {
// 		// We have some non-RTCM, possibly followed by a start of message
// 		// byte.
// 		logEntry := fmt.Sprintf("ReadNextRTCM3MessageFrame: read %d bytes", len(frame))
// 		rtcmHandler.makeLogEntry(logEntry)
// 		if frame[len(frame)-1] == utils.StartOfMessageFrame {
// 			// non-RTCM followed by start of message byte.  Push the start
// 			// byte back so we see it next time and return the rest of the
// 			// buffer as a non-RTCM message.
// 			logEntry1 := "ReadNextRTCM3MessageFrame: found d3 - unreading"
// 			rtcmHandler.makeLogEntry(logEntry1)
// 			reader.UnreadByte()
// 			frameWithoutTrailingStartByte := frame[:len(frame)-1]
// 			logEntry2 := fmt.Sprintf("ReadNextRTCM3MessageFrame: returning %d bytes %s",
// 				len(frameWithoutTrailingStartByte),
// 				hex.Dump(frameWithoutTrailingStartByte))
// 			rtcmHandler.makeLogEntry(logEntry2)
// 			return frameWithoutTrailingStartByte, nil
// 		} else {
// 			// Just some non-RTCM.
// 			logEntry := fmt.Sprintf("ReadNextRTCM3MessageFrame: got: %d bytes %s",
// 				len(frame),
// 				hex.Dump(frame))
// 			rtcmHandler.makeLogEntry(logEntry)
// 			return frame, nil
// 		}
// 	}

// 	// The buffer contains just a start of message byte so
// 	// we may have the start of an RTCM message frame.
// 	// Get the rest of the message frame.
// 	logEntry := "ReadNextRTCM3MessageFrame: found d3 immediately"
// 	rtcmHandler.makeLogEntry(logEntry)
// 	var n int = 1
// 	var expectedFrameLength uint = 0
// 	for {
// 		// Read and handle the next byte.
// 		buf := make([]byte, 1)
// 		l, readErr := reader.Read(buf)
// 		// We've read some text, so log any read error, but ignore it.  If it's
// 		// a hard error it will be caught on the next call.
// 		if readErr != nil {
// 			if readErr != io.EOF {
// 				// Any error other than EOF is always fatal, but it will be caught
// 				logEntry := fmt.Sprintf("ReadNextRTCM3MessageFrame: ignoring error while reading message - %v", readErr)
// 				rtcmHandler.makeLogEntry(logEntry)
// 				return frame, nil
// 			}

// 			if rtcmHandler.StopOnEOF {
// 				// EOF is fatal for the kind of input file we are reading.
// 				logEntry := "ReadNextRTCM3MessageFrame: ignoring fatal EOF"
// 				rtcmHandler.makeLogEntry(logEntry)
// 				return frame, nil
// 			} else {
// 				// For this kind of input, EOF just means that there is nothing
// 				// to read just yet, but there may be something later.  So we
// 				// just pause and try again.
// 				logEntry := "ReadNextRTCM3MessageFrame: ignoring non-fatal EOF"
// 				rtcmHandler.makeLogEntry(logEntry)
// 				continue
// 			}
// 		}

// 		if l < 1 {
// 			// We expected to read exactly one byte, so there is currently
// 			// nothing to read.  Pause and try again.
// 			logEntry := "ReadNextRTCM3MessageFrame: no data.  Pausing"
// 			rtcmHandler.makeLogEntry(logEntry)
// 			continue
// 		}

// 		frame = append(frame, buf[0])
// 		n++

// 		// What we do next depends upon how much of the message we have read.
// 		// On the first few trips around the loop we read the header bytes and
// 		// the 10-bit expected message length l.  Once we know l, we can work
// 		// out the total length of the frame (which is l+6) and we can then
// 		// read the remaining bytes of the frame.
// 		switch {
// 		case n < utils.LeaderLengthBytes+2:
// 			// We haven't read enough bytes to figure out the message length yet.
// 			continue

// 		case n == utils.LeaderLengthBytes+2:
// 			// We have the first three bytes of the frame so we have enough data to find
// 			// the length and the type of the message (which we will need in a later trip
// 			// around this loop).
// 			messageLength, messageType, err := rtcmHandler.getMessageLengthAndType(frame)
// 			if err != nil {
// 				// We thought we'd found the start of a message, but it's something else
// 				// that happens to start with the start of frame byte.
// 				// Return the collected data.
// 				logEntry := fmt.Sprintf("ReadNextRTCM3MessageFrame: error getting length and type: %v", err)
// 				rtcmHandler.makeLogEntry(logEntry)
// 				return frame, nil
// 			}

// 			logEntry1 := fmt.Sprintf("ReadNextRTCM3MessageFrame: found message type %d length %d", messageType, messageLength)
// 			rtcmHandler.makeLogEntry(logEntry1)

// 			// The frame contains a 3-byte header, a variable-length message (for which
// 			// we now know the length) and a 3-byte CRC.  Now we just need to continue to
// 			// read bytes until we have the whole message.
// 			expectedFrameLength = messageLength + utils.LeaderLengthBytes + utils.CRCLengthBytes
// 			logEntry2 := fmt.Sprintf("ReadNextRTCM3MessageFrame: expecting a %d frame", expectedFrameLength)
// 			rtcmHandler.makeLogEntry(logEntry2)

// 			// Now we read the rest of the message byte by byte, one byte every trip.
// 			// We know how many bytes we want, so we could just read that many using one
// 			// Read call, but if the input stream is a serial connection, we would
// 			// probably need several of those, so we might as well do it this way.
// 			continue

// 		case n >= int(expectedFrameLength):
// 			// By this point the expected frame length has been decoded and set to a
// 			// non-zero value (otherwise the previous case would have triggered) and we have
// 			// read that many bytes.  So we are done.  Return the complete message frame.
// 			// The CRC will be checked later.
// 			//
// 			// (The case condition could use ==, but using >= guarantees that the loop will
// 			// terminate eventually even if my logic is faulty and the loop overruns!)
// 			//
// 			logEntry := fmt.Sprintf("ReadNextRTCM3MessageFrame: returning an RTCM message frame, %d bytes, expected %d", n, expectedFrameLength)
// 			rtcmHandler.makeLogEntry(logEntry)
// 			return frame, nil

// 		default:
// 			// In most trips around the loop, we just read the next byte and build up the
// 			// message frame.
// 			continue
// 		}
// 	}
// }
