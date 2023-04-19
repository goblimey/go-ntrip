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

	// Create the output channel.
	messageChan := make(chan rtcm.Message, 10)

	// Create and start a file handler feeding the rtcmHandler.  The file
	// handler reads the input bytes and messages appear on the message
	// channel.

	const waitTimeOnEOF = 0 // Do not wait when encountering End Of File.
	const timeoutOnEOF = 0  // Time out immediately on the first End Of File.

	fh := New(messageChan, waitTimeOnEOF, timeoutOnEOF)
	go fh.Handle(time.Now(), reader)

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
	// aggregate channel.  This test simulates that situation by calling Handle
	// twice using different bit streams each time.  The result on the aggregate 
	// message channel should be the messages from the two bit streams in order.

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

	// Create an aggregate message channel.  Each of the two phases below
	// will send messages to this.
	aggregateMessageChan := make(chan rtcm.Message, messageChannelCapacity)

	// Phase 1:  read from a data source until it's exhausted and send the
	// resulting messages to the aggregate channel.  This data source is a
	// complete message, which is how things will be in the field - the GNSS
	// device will send complete messages in bursts with a long(ish) delay
	// between each burst.

	// Create the temporary output channel.
	messageChan1 := make(chan rtcm.Message, messageChannelCapacity)

	// Create and start a file handler feeding the rtcmHandler.  The file
	// handler reads the input bytes and messages appear on the temporary
	// message channel.  When it's closed, the handler is done.

	fh1 := New(messageChan1, waitTimeOnEOF, timeoutOnEOF)
	go fh1.Handle(time.Now(), reader1)

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

	// Create a buffered reader connected to the test bit stream.
	r2 := bytes.NewReader(bitStream2)
	reader2 := bufio.NewReader(r2)

	// Create the output channel.
	messageChan2 := make(chan rtcm.Message, messageChannelCapacity)

	// Create and start a file handler feeding the rtcmHandler.  The file
	// handler reads the input bytes and messages appear on the message
	// channel.

	fh := New(messageChan2, waitTimeOnEOF, timeoutOnEOF)
	go fh.Handle(time.Now(), reader2)

	// Read the messages from the message channel and send them to the
	// aggregate channel.
	for {
		message, ok := <-messageChan2
		if !ok {
			break // Phase 1 is done.
		}

		aggregateMessageChan <- message
	}

	// We're done.

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