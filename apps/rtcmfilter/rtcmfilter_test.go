package main

import (
	"bytes"
	"io"
	"testing"
	"time"

	rtcm "github.com/goblimey/go-ntrip/rtcm/handler"
	"github.com/goblimey/go-ntrip/rtcm/testdata"
	"github.com/goblimey/go-ntrip/rtcm/utils"

	"github.com/kylelemons/godebug/diff"
)

// TestWriteReadableMessages checks that writeReadableMessages produces the correct results.
func TestWriteReadableMessages(t *testing.T) {

	const wantDisplay = `Message type 1024, Residuals, Plane Grid Representation
A coordinate transformation message.  Not often found in actual use.
Frame length 14 bytes:
00000000  d3 00 08 40 00 00 8a 00  00 00 00 4f 5e e7        |...@.......O^.|

message type 1024 currently cannot be displayed

`

	// Step 1: create a channel of messages using the input bit stream.

	byteChan := make(chan byte, 100000)

	for _, b := range testdata.UnhandledMessageType1024 {
		byteChan <- b
	}

	close(byteChan)

	messageChan := make(chan rtcm.Message, 10)

	rtcmHandler := rtcm.New(time.Now())

	rtcmHandler.HandleMessages(byteChan, messageChan)

	// Step 2: Create a dummy writer and write the message to it.

	var w bytes.Buffer
	writer := &w

	writeReadableMessages(messageChan, writer)

	// Check results.

	got := writer.Bytes()

	gotDisplay := string(got)

	if wantDisplay != gotDisplay {
		t.Error(diff.Diff(wantDisplay, gotDisplay))
	}
}

// TestWriteRTCMMessages checks that writeRTCMMessages produces the correct results.
func TestWriteRTCMMessages(t *testing.T) {

	var testData = []struct {
		description    string
		fun            func(ch chan rtcm.Message, writer io.Writer)
		inputBitStream []byte
		wantBitStream  []byte
	}{
		// writeRTCMMessages should send just the RTCM messages to the channel.
		{"writeRTCMMessages", writeRTCMMessages,
			testdata.MessageBatchWithJunk, testdata.WantResultFromProcessingMessageBatchWithJunk},
		// writeAllMessages should send everything it receives, so the output
		// should be a copy of the input.
		{"writeAllMessages", writeAllMessages,
			testdata.MessageBatchWithJunk, testdata.MessageBatchWithJunk},
	}

	for _, td := range testData {

		// Step 1: create a channel of messages using the input bit stream.

		byteChan := make(chan byte, 100000)

		for _, b := range td.inputBitStream {
			byteChan <- b
		}

		close(byteChan)

		messageChan := make(chan rtcm.Message, 10)

		startTime := time.Date(2020, time.November, 13, 0, 0, 0, 0, utils.LocationUTC)

		rtcmHandler := rtcm.New(startTime)

		rtcmHandler.HandleMessages(byteChan, messageChan)

		// Step 2: Create a dummy writer and write the message to it.

		var w bytes.Buffer
		writer := &w

		td.fun(messageChan, writer)

		// Check results.

		got := writer.Bytes()
		if len(td.wantBitStream) != len(got) {
			t.Errorf("%s: want %d bytes got %d",
				td.description, len(td.wantBitStream), len(got))
			continue
		}
	}
}
