package filehandler

import (
	"bufio"
	"bytes"
	"testing"
	"time"

	"github.com/goblimey/go-ntrip/jsonconfig"
	rtcm "github.com/goblimey/go-ntrip/rtcm/handler"
	"github.com/goblimey/go-ntrip/rtcm/testdata"
	"github.com/goblimey/go-ntrip/rtcm/utils"
)

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

	config := jsonconfig.Config{
		WaitTimeOnEOFMilliseconds: waitTimeOnEOF,
		TimeoutOnEOFMilliSeconds:  timeoutOnEOF,
	}

	fh := New(messageChan, &config)
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

	const waitTimeOnEOF = 0 // Do not wait when encountering End Of File.
	const timeoutOnEOF = 0  // Time out immediately on the first End Of File.

	config := jsonconfig.Config{
		WaitTimeOnEOFMilliseconds: waitTimeOnEOF,
		TimeoutOnEOFMilliSeconds:  timeoutOnEOF,
	}

	const messageChannelCapacity = 100 // The capacity of the buffered message channels.

	// The first test bit stream contains 1 message, the second contains
	// 7 messages.
	bitStream1 := testdata.MessageFrameType1074_2
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

	fh1 := New(messageChan1, &config)
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

	fh2 := New(messageChan2, &config)
	go fh2.Handle(time.Now(), reader2)

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
