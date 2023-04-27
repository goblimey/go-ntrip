package main

import (
	"bytes"
	"io"
	"testing"
	"time"

	rtcm "github.com/goblimey/go-ntrip/rtcm/handler"
	"github.com/goblimey/go-ntrip/rtcm/testdata"
)

// TestWriteRTCMMessages checks that writeRTCMMessages produces the correct results.
func TestWriteRTCMMessages(t *testing.T) {

	const wantDisplay = `message type 1024, frame length 14
00000000  d3 00 08 40 00 00 8a 00  00 00 00 4f 5e e7        |...@.......O^.|

message type 1024 currently cannot be displayed

`

	wantDisplayBytes := []byte(wantDisplay)

	var testData = []struct {
		description    string
		fun            func(ch chan rtcm.Message, writer io.Writer)
		inputBitStream []byte
		wantBitStream  []byte
	}{
		{"writeReadableMessages", writeReadableMessages,
			testdata.UnhandledMessageType1024, wantDisplayBytes},
		{"writeRTCMMessages", writeRTCMMessages,
			testdata.MessageBatchWithJunk, testdata.WantResultFromProcessingMessageBatchWithJunk},
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

		rtcmHandler := rtcm.New(time.Now())

		rtcmHandler.HandleMessages(byteChan, messageChan)

		// Step 2: Create a dummy writer and write the message to it.

		var w bytes.Buffer
		writer := &w

		td.fun(messageChan, writer)

		// Check results.

		got := writer.Bytes()

		gotString := string(got)

		_ = gotString

		if len(td.wantBitStream) != len(got) {
			t.Errorf("%s: want %d bytes got %d",
				td.description, len(td.wantBitStream), len(got))
			continue
		}

		// Compare the slices byte by byte.
		for i := range td.wantBitStream {
			if td.wantBitStream[i] != got[i] {
				t.Errorf("%s: slices differ at byte %d - 0x%x 0x%x",
					td.description, i, td.wantBitStream, got[i])
				continue
			}
		}
	}
}
