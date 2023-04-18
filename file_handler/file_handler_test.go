package filehandler

import (
	"bufio"
	"bytes"
	"testing"
	"time"

	rtcm "github.com/goblimey/go-ntrip/rtcm/handler"
	"github.com/goblimey/go-ntrip/rtcm/testdata"
	"github.com/goblimey/go-ntrip/rtcm/utils"
)

// // TestReadIncompleteMessage tests that an incomplete RTCM message is processed
// // correctly.  It should be returned as a non-RTCM message.
// func TestReadIncompleteMessage(t *testing.T) {

// 	// This is the message contents that should result.
// 	want := string(testdata.IncompleteMessage)

// 	r := bytes.NewReader(testdata.IncompleteMessage)
// 	imReader := bufio.NewReader(r)

// 	startTime := time.Date(2020, time.November, 13, 0, 0, 0, 0, utils.LocationUTC)
// 	rtcm := New(startTime, logger)
// 	rtcm.StopOnEOF = true

// 	// The first call should read the incomplete message, hit
// 	// EOF and ignore it.
// 	frame1, readError1 := rtcm.ReadNextRTCM3MessageFrame(imReader)
// 	if readError1 != nil {
// 		t.Fatal(readError1)
// 	}

// 	// The message is incomplete so expect an error.
// 	message, messageFetchError := rtcm.getMessage(frame1)
// 	if messageFetchError == nil {
// 		t.Error("expected to get an error (reading an incomplete message)")
// 	}

// 	if message.MessageType != utils.NonRTCMMessage {
// 		t.Errorf("expected message type %d, got %d",
// 			utils.NonRTCMMessage, message.MessageType)
// 	}

// 	got := string(message.RawData)

// 	if len(want) != len(got) {
// 		t.Errorf("expected a message body %d long, got %d", len(want), len(got))
// 	}

// 	if want != got {
// 		t.Errorf("message content doesn't match what we expected value")
// 	}

// 	// The second call should return nil and the EOF.
// 	frame2, readError2 := rtcm.ReadNextRTCM3MessageFrame(imReader)
// 	if readError2 == nil {
// 		t.Errorf("expected an error")
// 	}
// 	if readError2 != io.EOF {
// 		t.Errorf("expected EOF, got %v", readError2)
// 	}
// 	if frame2 != nil {
// 		t.Errorf("expected no frame, got %s", string(frame2))
// 	}
// }

// // TestReadInCompleteMessageFrame checks that ReadNextRTCM3MessageFrame correctly
// // handles a short frame.
// func TestReadInCompleteMessageFrame(t *testing.T) {
// 	data := []byte{
// 		0xd3, 0x00, 0xf4, 0x43, 0x50, 0x00, 0x49, 0x96, 0x84, 0x2e, 0x00, 0x00, 0x40, 0xa0, 0x85, 0x80,
// 		0x00, 0x00, 0x00, 0x20, 0x00, 0x80, 0x5f, 0xa9, 0xc8, 0x88, 0xea, 0x08, 0xe9, 0x88, 0x8a, 0x6a,
// 		0x60, 0x00, 0x00, 0x00, 0x00, 0xd6, 0x0a, 0x1b, 0xc5, 0x57, 0x9f, 0xf8, 0x92, 0xf2, 0x2e, 0x2d,
// 		0xb0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
// 		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x43,
// 		0x50, 0xd3, 0x00, 0xdc, 0x43, 0xf0, 0x00, 0x6e, 0x5c, 0x48, 0xee, 0x00, 0x00, 0x41, 0x83, 0x41,
// 		0x80, 0x00, 0x00, 0x00, 0x00, 0x20, 0x80, 0x00, 0xfd, 0xa4, 0x26, 0x22, 0xa4, 0x23, 0xa5, 0x22,
// 		0x20, 0x46, 0x68, 0x3d, 0xd4, 0xae, 0xca, 0x74, 0xd2, 0x20, 0x21, 0xc1, 0xf5, 0xcd, 0xa5, 0x85,
// 		0x67, 0xee, 0x70, 0x08, 0x9e, 0xd7, 0x80, 0xd6, 0xdf, 0xca, 0x00, 0x3a, 0x1b, 0x5c, 0xb9, 0xd2,
// 		0xf5, 0xe6, 0xf7, 0x5a, 0x37, 0x76, 0x78, 0x9f, 0x71, 0xa8, 0x7a, 0xde, 0xf7, 0xb5, 0x77, 0x86,
// 		0xa0, 0xd8, 0x6e, 0xbc, 0x60, 0xfe, 0x66, 0xd1, 0x8c, 0xed, 0x42, 0x68, 0x50, 0xee, 0xe8, 0x7b,
// 		0xd0, 0xa7, 0xcb, 0xdf, 0xcc, 0x10, 0xef, 0xd3, 0xef, 0xdf, 0xe4, 0xb8, 0x5f, 0xdf, 0xd6, 0x3f,
// 		0xe2, 0xad, 0x0f, 0xf6, 0x3c, 0x08, 0x01, 0x8a, 0x20, 0x66, 0xdf, 0x8d, 0x65, 0xb7, 0xbd, 0x9c,
// 		0x4f, 0xc5, 0xa2, 0x24, 0x35, 0x0c, 0xcc, 0x52, 0xcc, 0x95, 0x23, 0xcd, 0x93, 0x44, 0x8d, 0x23,
// 		0x40, 0x6f, 0xd4, 0xef, 0x32, 0x4c, 0x80, 0x00, 0x2b, 0x08, 0xc2, 0xa0, 0x98, 0x31, 0x0a, 0xc3,
// 		0x00, 0xa8, 0x2e, 0x0a, 0xc8, 0x18, 0x8d, 0x72, 0x48, 0x75}

// 	r := bytes.NewReader(data)
// 	imReader := bufio.NewReader(r)

// 	startTime := time.Date(2020, time.November, 13, 0, 0, 0, 0, utils.LocationUTC)
// 	rtcm := New(startTime, logger)
// 	rtcm.StopOnEOF = true

// 	// The first call should read the incomplete message, hit
// 	// EOF and ignore it.
// 	frame1, readError1 := rtcm.ReadNextRTCM3MessageFrame(imReader)
// 	if readError1 != nil {
// 		t.Fatal(readError1)
// 	}

// 	// The message is incomplete so expect an error.
// 	message, messageFetchError := rtcm.getMessage(frame1)
// 	if messageFetchError == nil {
// 		t.Error("expected to get an error (reading an incomplete message)")
// 	}

// 	t.Log(len(message.RawData))

// }

// func TestReadEmptyBuffer(t *testing.T) {
// 	data := []byte{}

// 	r := bytes.NewReader(data)
// 	imReader := bufio.NewReader(r)

// 	startTime := time.Date(2020, time.November, 13, 0, 0, 0, 0, utils.LocationUTC)
// 	rtcm := New(startTime, logger)
// 	rtcm.StopOnEOF = false

// 	// This should read the empty buffer, hit EOF and ignore it.
// 	m, err := rtcm.ReadNextRTCM3Message(imReader)

// 	if err != nil {
// 		t.Errorf("Expected no error, got %s", err.Error())
// 	}

// 	if m != nil {
// 		if m.RawData == nil {
// 			t.Errorf("want nil RTCM3Message, got a struct with nil RawData")
// 		}
// 		t.Errorf("Expected nil frame, got %d bytes of RawData", len(m.RawData))
// 	}
// }

// // TestReadJunk checks that ReadNextChunk handles interspersed junk correctly.
// func TestReadJunk(t *testing.T) {
// 	r := bytes.NewReader(testdata.JunkAtStart)
// 	junkAtStartReader := bufio.NewReader(r)
// 	ch := make(chan byte, 100)
// 	for _, j := range testdata.JunkAtStart {
// 		ch <- j
// 	}
// 	startTime := time.Date(2020, time.November, 13, 0, 0, 0, 0, utils.LocationUTC)
// 	rtcm := New(startTime, logger)
// 	rtcm.StopOnEOF = true

// 	frame, err1 := rtcm.ReadNextRTCM3MessageFrame(junkAtStartReader)
// 	if err1 != nil {
// 		t.Fatal(err1.Error())
// 	}

// 	message, messageFetchError := rtcm.getMessage(frame)
// 	if messageFetchError != nil {
// 		t.Errorf("error getting message - %v", messageFetchError)
// 	}

// 	if message.MessageType != utils.NonRTCMMessage {
// 		t.Errorf("expected message type %d, got %d",
// 			utils.NonRTCMMessage, message.MessageType)
// 	}

// 	gotBody := string(message.RawData[:4])

// 	if testdata.WantJunk != gotBody {
// 		t.Errorf("expected %s, got %s", testdata.WantJunk, gotBody)
// 	}
// }

// func TestReadOnlyJunk(t *testing.T) {
// 	r := bytes.NewReader(testdata.AllJunk)
// 	junkReader := bufio.NewReader(r)
// 	startTime := time.Date(2020, time.November, 13, 0, 0, 0, 0, utils.LocationUTC)
// 	rtcm := New(startTime, logger)
// 	rtcm.StopOnEOF = true

// 	frame, err1 := rtcm.ReadNextRTCM3MessageFrame(junkReader)

// 	if err1 != nil {
// 		t.Fatal(err1.Error())
// 	}

// 	message, messageFetchError := rtcm.getMessage(frame)
// 	if messageFetchError != nil {
// 		t.Errorf("error getting message - %v", messageFetchError)
// 	}

// 	if message.MessageType != utils.NonRTCMMessage {
// 		t.Errorf("expected message type %d, got %d",
// 			utils.NonRTCMMessage, message.MessageType)
// 	}

// 	gotBody := string(message.RawData)

// 	if testdata.WantJunk != gotBody {
// 		t.Errorf("expected %s, got %s", testdata.WantJunk, gotBody)
// 	}

// 	// Call again - expect EOF.

// 	frame2, err2 := rtcm.ReadNextRTCM3MessageFrame(junkReader)

// 	if err2 == nil {
// 		t.Fatal("expected EOF error")
// 	}
// 	if err2.Error() != "EOF" {
// 		t.Errorf("expected EOF error, got %s", err2.Error())
// 	}

// 	if len(frame2) != 0 {
// 		t.Errorf("expected frame to be empty, got %s", string(frame2))
// 	}
// }

// TestHandle checks that Handle correctly processes a bit stream containing a
// set of messages.
func TestHandle(t *testing.T) {

	// The test bit stream contains 7 messages.
	bitStream := testdata.MessageBatchWithJunk

	// These are the expected message types.
	wantMessageType := []int{
		1077,
		utils.NonRTCMMessage,
		1087,
		utils.NonRTCMMessage,
		1097,
		1127,
		utils.NonRTCMMessage,
	}

	// Create a buffered reader connected to the test bit stream.
	r := bytes.NewReader(bitStream)
	reader := bufio.NewReader(r)

	// Create the input channel for the RTCM handler.
	byteChan := make(chan byte)

	// Create the output channel.
	messageChan := make(chan rtcm.Message, 10)

	// Create and start a file handler feeding the rtcmHandler.  The file
	// handler reads the input bytes and messages appear on the message
	// channel.

	const waitTimeOnEOF = 0 // Do not wait when encountering End Of File.
	const timeoutOnEOF = 0  // Time out immediately on the first End Of File.
	fh := New(messageChan, waitTimeOnEOF, timeoutOnEOF)
	go fh.Handle(time.Now(), reader, byteChan)

	// Handle doesn't close the byte channel.  That decision is left to the
	// caller.  Pause briefly to let the data drain and then close it.  That
	// causes the rtcmHandler to stop waiting on any more data coming in and
	// to process the partial messages that ends the test data.

	time.Sleep(time.Second)
	close(byteChan)

	// Fetch the messages from the channel.
	messages := make([]rtcm.Message, 0)
	for {
		message, ok := <-messageChan
		if !ok {
			break
		}
		messages = append(messages, message)
	}

	// Check the number of messages.
	gotNumMessages := len(messages)
	if len(wantMessageType) != gotNumMessages {
		t.Errorf("want %d got %d", len(wantMessageType), gotNumMessages)
	}

	// Check the message types.
	for i, message := range messages {
		if wantMessageType[i] != message.MessageType {
			t.Errorf("%d: want type %d got %d", i, wantMessageType[i], message.MessageType)
		}
	}
}

// TestHandleManyCalls checks that Handle correctly processes a number of bit streams
// containing messages.
func TestHandleManyCalls(t *testing.T) {

	// If the input is a serial line with a GNSS device on the end, Handle will
	// be called many times and the messages from each call will be sent to an
	// aggregate channel.  This call simulates that situation using two test
	// bit streams.  The result on the aggregate message channel should be the
	// messages from the two bit streams in order.

	const waitTimeOnEOF = 0            // Do not wait when encountering End Of File.
	const timeoutOnEOF = 0             // Time out immediately on the first End Of File.
	const messageChannelCapacity = 100 // The capacity of the buffered message channels.

	// The first test bit stream contains 1 message, the second contains
	// 7 messages.
	bitStream1 := testdata.MessageFrameType1074
	bitStream2 := testdata.MessageBatchWithJunk

	// These are the expected message types.
	wantMessageType := []int{
		1074,
		1077,
		utils.NonRTCMMessage,
		1087,
		utils.NonRTCMMessage,
		1097,
		1127,
		utils.NonRTCMMessage,
	}

	// Create a buffered reader connected to the first test bit stream.
	r1 := bytes.NewReader(bitStream1)
	reader1 := bufio.NewReader(r1)

	// Create an aggregate message channel.  Each o the two phases below
	// will send messages to this.
	aggregateMessageChan := make(chan rtcm.Message, messageChannelCapacity)

	// Phase 1:  read from a data source until it's exhausted and send the
	// resulting messages to the aggregate channel.  This data source is a
	// complete message, which is how things will be in the field - the GNSS
	// device will send complete messages in bursts with a long(ish) delay
	// between each burst.

	// Create the input channel for the RTCM handler.  It persists
	// over both calls of Handle.
	byteChan1 := make(chan byte)

	// Create the temporary output channel.
	messageChan1 := make(chan rtcm.Message, messageChannelCapacity)

	// Create and start a file handler feeding the rtcmHandler.  The file
	// handler reads the input bytes and messages appear on the temporary
	// message channel.  When it's closed, the handler is done.

	fh1 := New(messageChan1, waitTimeOnEOF, timeoutOnEOF)
	go fh1.Handle(time.Now(), reader1, byteChan1)

	// Read the messages from the message channel and send them to the
	// aggregate channel.
	for {
		message, ok := <-messageChan1
		if !ok {
			break // Phase 1 is done.
		}

		aggregateMessageChan <- message
	}

	// phase 2:  same again but with different input data.

	// Create the input channel for the RTCM handler.  It persists
	// over both calls of Handle.
	byteChan2 := make(chan byte)

	// Create a buffered reader connected to the test bit stream.
	r2 := bytes.NewReader(bitStream2)
	reader2 := bufio.NewReader(r2)

	// Create the output channel.
	messageChan2 := make(chan rtcm.Message, messageChannelCapacity)

	// Create and start a file handler feeding the rtcmHandler.  The file
	// handler reads the input bytes and messages appear on the message
	// channel.

	fh := New(messageChan2, waitTimeOnEOF, timeoutOnEOF)
	go fh.Handle(time.Now(), reader2, byteChan2)

	// Read the messages from the message channel and send them to the
	// aggregate channel.
	for {
		message, ok := <-messageChan2
		if !ok {
			break // Phase 1 is done.
		}

		aggregateMessageChan <- message
	}

	// We're done.  Handle doesn't close the byte channel.  That decision is
	// left to the caller.  Pause briefly to let the data drain and then close
	// it.  That causes the rtcmHandler to stop waiting on any more data
	// coming in and to process the partial message that ends the second
	// bit stream.

	// time.Sleep(time.Second)
	// close(byteChan)

	close(aggregateMessageChan)

	// Fetch the messages from the aggregate channel.
	messages := make([]rtcm.Message, 0)
	for {
		message, ok := <-aggregateMessageChan
		if !ok {
			break
		}
		messages = append(messages, message)
	}

	// Check the number of messages.
	gotNumMessages := len(messages)
	if len(wantMessageType) != gotNumMessages {
		t.Errorf("want %d got %d", len(wantMessageType), gotNumMessages)
	}

	// Check the message types.
	for i, message := range messages {
		if wantMessageType[i] != message.MessageType {
			t.Errorf("%d: want type %d got %d", i, wantMessageType[i], message.MessageType)
		}
	}
}

// // TestReadNextRTCM3MessageFrameWithShortBitStream checks that ReadNextRTCM3MessageFrame
// // handles a short bitstream.
// func TestReadNextRTCM3MessageFrameWithInvalidFrame(t *testing.T) {
// 	var testData = []struct {
// 		description string
// 		bitStream   []byte
// 		wantLength  int
// 	}{
// 		{"zero length", testdata.MessageFrameWithLengthZero, 5},
// 		{"too big", testdata.MessageFrameWithLengthTooBig, 5},
// 	}

// 	for _, td := range testData {

// 		r := bytes.NewReader(td.bitStream)
// 		reader := bufio.NewReader(r)

// 		now := time.Now()
// 		handler := New(now, logger)
// 		handler.StopOnEOF = true
// 		frame, err := handler.ReadNextRTCM3MessageFrame(reader)

// 		if err != nil {
// 			t.Errorf("want no error, got %v", err)
// 		}

// 		if frame == nil {
// 			t.Error("want a message frame, got nil")
// 		}

// 		if len(frame) != td.wantLength {
// 			t.Errorf("want a message frame of length %d, got length %d", td.wantLength, len(frame))
// 		}

// 		if frame[0] != 0xd3 {
// 			t.Errorf("want a message frame starting 0xd3, got first byte of 0x%02x", frame[0])
// 		}
// 	}
// }

// // TestGetMessageWithNonRTCMMessage checks that GetMessage handles a bit stream
// // containing a non-RTCM message correctly.
// func TestGetMessageWithNonRTCMMessage(t *testing.T) {
// 	// A bit stream containing just the non_RTCM message "plain text".
// 	plainTextBytes := []byte{'p', 'l', 'a', 'i', 'n', ' ', 't', 'e', 'x', 't'}
// 	// A bit stream containing the non_RTCM message "plain text" followed by the
// 	// start of an RTCM message.
// 	plainTextBytesWithD3 := []byte{'p', 'l', 'a', 'i', 'n', ' ', 't', 'e', 'x', 't', 0xd3}
// 	const plainText = "plain text"

// 	var testData = []struct {
// 		description string
// 		bitStream   []byte
// 	}{
// 		{"plain text", plainTextBytes},
// 		{"plain text with D3", plainTextBytesWithD3},
// 	}
// 	for _, td := range testData {
// 		startTime := time.Now()
// 		handler := New(startTime, logger)
// 		handler.StopOnEOF = true
// 		// ReadNextMessageFrame strips off any trailing D3 byte.
// 		r := bytes.NewReader(td.bitStream)
// 		messageReader := bufio.NewReader(r)
// 		bitStream, frameError := handler.ReadNextRTCM3MessageFrame(messageReader)
// 		if frameError != nil {
// 			t.Error(frameError)
// 			return
// 		}

// 		gotMessage, gotError := handler.getMessage(bitStream)

// 		if gotError != nil {
// 			t.Error(gotError)
// 			// return
// 		}

// 		if gotMessage == nil {
// 			t.Error("want a message, got nil")
// 			return
// 		}

// 		if gotMessage.MessageType != utils.NonRTCMMessage {
// 			t.Errorf("want a NONRTCMMessage, got message type %d", gotMessage.MessageType)
// 			return
// 		}

// 		if gotMessage.RawData == nil {
// 			t.Error("want some raw data, got nil")
// 			return
// 		}

// 		if len(gotMessage.RawData) != len(plainText) {
// 			t.Errorf("want a message frame of length %d, got length %d",
// 				len(plainText), len(gotMessage.RawData))
// 		}

// 		if string(gotMessage.RawData) != plainText {
// 			t.Errorf("want raw data - %s, got %s",
// 				plainText, string(gotMessage.RawData))
// 		}
// 	}
// }

// func TestReadNextMessageFrame(t *testing.T) {
// 	r := bytes.NewReader(testdata.MessageBatchWithJunk)
// 	realDataReader := bufio.NewReader(r)
// 	startTime := time.Date(2020, time.November, 13, 0, 0, 0, 0, utils.LocationUTC)
// 	rtcmHandler := New(startTime, logger)
// 	rtcmHandler.StopOnEOF = true

// 	frame, err1 := rtcmHandler.ReadNextRTCM3MessageFrame(realDataReader)
// 	if err1 != nil {
// 		t.Fatal(err1.Error())
// 	}

// 	message, messageFetchError := rtcmHandler.getMessage(frame)
// 	if messageFetchError != nil {
// 		t.Errorf("error getting message - %v", messageFetchError)
// 		return
// 	}

// 	if message.MessageType != 1077 {
// 		t.Errorf("expected message type 1077, got %d", message.MessageType)
// 		return
// 	}
// }

// //TestGetMessageWithRealData checks that GetMessage correctly handles an MSM4 message extracted from
// // real data.
// func TestGetMessageWithRealData(t *testing.T) {

// 	// These data were collected on the 17th June 2022.
// 	startTime := time.Date(2022, time.June, 17, 0, 0, 0, 0, utils.LocationUTC)
// 	var msm4 = []byte{
// 		0xd3, 0x00, 0x7b, 0x46, 0x40, 0x00, 0x78, 0x4e, 0x56, 0xfe, 0x00, 0x00, 0x00, 0x58, 0x16, 0x00,
// 		0x20, 0x00, 0x00, 0x00, 0x20, 0x02, 0x00, 0x00, 0x7f, 0x55, 0x0e, 0xa2, 0xa2, 0xa4, 0x9a, 0x92,
// 		0xa3, 0x10, 0xe2, 0x4a, 0xd0, 0xa9, 0xba, 0x91, 0x8f, 0xc0, 0x62, 0x40, 0x8d, 0xa6, 0xa4, 0x4c,
// 		0x4d, 0x9f, 0xdb, 0x3c, 0x65, 0x87, 0x9f, 0x4f, 0x16, 0x3b, 0xf2, 0x55, 0x40, 0x72, 0xe7, 0x01,
// 		0x4d, 0x8c, 0x1a, 0x85, 0x40, 0x63, 0x1d, 0x42, 0x07, 0x3e, 0x07, 0xf3, 0x15, 0xe3, 0x36, 0x77,
// 		0xb0, 0x29, 0xde, 0x66, 0x68, 0x84, 0x9b, 0xf7, 0xff, 0xff, 0xff, 0xff, 0xfe, 0x00, 0x3d, 0x15,
// 		0x15, 0x4f, 0x6d, 0x78, 0x63, 0x58, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
// 		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x5b, 0xa7,
// 		0x0c,
// 		0xd3, 0x00, 0x7b, 0x44, 0x60, 0x00, 0x78, 0x4f, 0x31, 0xbe, 0x00, 0x00, 0x21, 0x84, 0x00,
// 		0x60, 0x40, 0x00, 0x00, 0x00, 0x20, 0x01, 0x00, 0x00, 0x7f, 0xbe, 0xb2, 0x9e, 0xa2, 0xae, 0xb8,
// 		0xa4, 0xad, 0x04, 0x04, 0x5a, 0x33, 0xa2, 0x16, 0x93, 0x1e, 0x6f, 0xd8, 0x9f, 0xbb, 0xdd, 0x3d,
// 		0x3a, 0x7e, 0xee, 0x9a, 0xdc, 0x4c, 0x3e, 0xc8, 0x80, 0x97, 0x06, 0x83, 0x77, 0xc6, 0xcc, 0xc2,
// 		0x6a, 0x04, 0xae, 0xff, 0x1b, 0x83, 0xfd, 0xcb, 0xbf, 0xc9, 0x2b, 0xff, 0x33, 0x78, 0xf9, 0x91,
// 		0xe3, 0xeb, 0x7c, 0x50, 0x87, 0xae, 0x02, 0x2c, 0x1e, 0xf8, 0x15, 0x20, 0x3a, 0xb8, 0x50, 0xeb,
// 		0xbb, 0xc0, 0xb4, 0xf5, 0x03, 0x15, 0x07, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xc0, 0x01, 0x3d,
// 		0x17, 0xdd, 0x7d, 0x54, 0x52, 0xf5, 0xf6, 0xd7, 0x48, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x76,
// 		0xfb, 0x6f,
// 		0xd3, 0x00, 0x7b, 0x43, 0xc0, 0x00, 0xb3, 0xe2, 0x16, 0x7e, 0x00, 0x00, 0x0c, 0x07,
// 		0x0c, 0x00, 0x00, 0x00, 0x00, 0x00, 0x20, 0x80, 0x00, 0x00, 0x7f, 0xfe, 0x86, 0x82, 0x94, 0x8c,
// 		0x9a, 0x88, 0x93, 0x2c, 0xd2, 0x39, 0x44, 0x70, 0xc6, 0xf5, 0x49, 0xb7, 0xf0, 0x6f, 0xc9, 0x86,
// 		0x69, 0x8c, 0x8d, 0x00, 0x85, 0x01, 0x69, 0xe2, 0xdb, 0xc8, 0x31, 0x5e, 0x52, 0xab, 0xdb, 0x13,
// 		0xf6, 0x19, 0x09, 0xe8, 0x12, 0xf3, 0xfe, 0x94, 0xc0, 0x0d, 0xa7, 0xe1, 0xc2, 0x56, 0x07, 0x9e,
// 		0x68, 0x00, 0x0b, 0x90, 0x02, 0xb0, 0x7f, 0xb9, 0xe9, 0x7f, 0x01, 0x9a, 0x15, 0xc5, 0x08, 0x57,
// 		0x78, 0xfe, 0xd7, 0x0e, 0x7b, 0x8c, 0x9a, 0x0a, 0x89, 0x78, 0x56, 0x8a, 0x1f, 0xff, 0xfd, 0xff,
// 		0xf7, 0x5f, 0xff, 0xe0, 0x00, 0x65, 0x5e, 0x56, 0xc5, 0x0d, 0xf5, 0x44, 0xf5, 0x15, 0x5f, 0x38,
// 		0x5d, 0xa9, 0x5d,
// 		0xd3, 0x00, 0x98, 0x43, 0x20, 0x00, 0x78, 0x4f, 0x31, 0xbc, 0x00, 0x00, 0x2b,
// 		0x50, 0x08, 0x06, 0x00, 0x00, 0x00, 0x00, 0x20, 0x00, 0x80, 0x00, 0x5f, 0xfd, 0xe9, 0x49, 0xa8,
// 		0xe9, 0x08, 0xa8, 0xc9, 0x2a, 0x69, 0xc3, 0x2b, 0x30, 0xfc, 0x5d, 0xba, 0x3d, 0x14, 0x76, 0x18,
// 		0xf0, 0xc8, 0xe5, 0xdc, 0x8d, 0xf8, 0xfb, 0xbb, 0x8b, 0x76, 0xf4, 0x02, 0x5e, 0x01, 0x70, 0xa6,
// 		0xf9, 0x4a, 0x41, 0x56, 0x02, 0x74, 0x48, 0x6f, 0xe0, 0x84, 0xc0, 0x1c, 0x3f, 0x44, 0x7c, 0xc0,
// 		0x3f, 0x05, 0x1e, 0x5b, 0x97, 0xf9, 0xd9, 0x83, 0xf9, 0xcb, 0x07, 0xe4, 0x72, 0xe0, 0x38, 0xdf,
// 		0x01, 0x09, 0x4e, 0x18, 0x42, 0xf8, 0x66, 0xdd, 0x20, 0xc1, 0x5a, 0x83, 0x25, 0xa2, 0x0f, 0x65,
// 		0x17, 0x83, 0xe8, 0x3e, 0x04, 0x23, 0x84, 0x6b, 0x9e, 0x12, 0xf7, 0x67, 0xff, 0xff, 0xff, 0xff,
// 		0xff, 0xff, 0xf7, 0xf8, 0x00, 0x05, 0xf5, 0xcb, 0x6d, 0x57, 0x4f, 0x85, 0x97, 0x57, 0x6c, 0xcf,
// 		0x53, 0x30, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xce, 0xce,
// 		0x4c,
// 	}

// 	const wantNumSatellites = 7
// 	const wantMessageType = 1124

// 	r := bytes.NewReader(msm4)
// 	reader := bufio.NewReader(r)

// 	rtcm := New(startTime, logger)
// 	rtcm.StopOnEOF = true

// 	frame, err1 := rtcm.ReadNextRTCM3MessageFrame(reader)

// 	if err1 != nil {
// 		t.Error(err1.Error())
// 		return
// 	}

// 	message, messageFetchError := rtcm.getMessage(frame)
// 	if messageFetchError != nil {
// 		t.Errorf("error getting message - %v", messageFetchError)
// 		return
// 	}

// 	if message.MessageType != wantMessageType {
// 		t.Errorf("expected message type 1124 got %d", message.MessageType)
// 		return
// 	}

// 	// Get the message in display form.
// 	display, ok := PrepareForDisplay(message).(*msm4message.Message)
// 	if !ok {
// 		t.Error("expected the readable message to be *MSMMessage\n")
// 		return
// 	}

// 	if len(display.Satellites) != wantNumSatellites {
// 		t.Errorf("expected %d satellites, got %d", wantNumSatellites, len(display.Satellites))
// 	}

// 	// The outer slice should be the same size as the satellite slice.

// 	if len(display.Signals) != wantNumSatellites {
// 		t.Errorf("expected %d sets of signals, got %d", wantNumSatellites, len(display.Satellites))
// 	}
// }
