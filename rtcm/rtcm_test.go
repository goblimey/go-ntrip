package rtcm

import (
	"bufio"
	"bytes"
	"io"
	"log"
	"testing"
	"time"

	msm4message "github.com/goblimey/go-ntrip/rtcm/msm4/message"
	msm7message "github.com/goblimey/go-ntrip/rtcm/msm7/message"
	"github.com/goblimey/go-ntrip/rtcm/utils"

	"github.com/goblimey/go-tools/switchwriter"
)

// A complete message frame (including 3-byte leader and 3-byte CRC).  The message
// type is the bottom half of byte 3 and all of byte 4 - 0x449 - decimal 1097.
var validMessageFrame = []byte{
	0xd3, 0x00, 0xaa, 0x44, 0x90, 0x00, 0x33, 0xf6, 0xea, 0xe2, 0x00, 0x00, 0x0c, 0x50, 0x00, 0x10,
	0x08, 0x00, 0x00, 0x00, 0x20, 0x01, 0x00, 0x00, 0x3f, 0xaa, 0xaa, 0xb2, 0x42, 0x8a, 0xea, 0x68,
	0x00, 0x00, 0x07, 0x65, 0xce, 0x68, 0x1b, 0xb4, 0xc8, 0x83, 0x7c, 0xe6, 0x11, 0x30, 0x10, 0x3f,
	0x05, 0xff, 0x4f, 0xfc, 0xe0, 0x4f, 0x61, 0x68, 0x59, 0xb6, 0x86, 0xb5, 0x1b, 0xa1, 0x31, 0xb9,
	0xd9, 0x71, 0x55, 0x57, 0x07, 0xa0, 0x00, 0xd3, 0x2e, 0x0c, 0x99, 0x01, 0x98, 0xc4, 0xfa, 0x16,
	0x0e, 0xfa, 0x6e, 0xac, 0x07, 0x19, 0x7a, 0x07, 0x3a, 0xa4, 0xfc, 0x53, 0xc4, 0xfb, 0xff, 0x97,
	0x00, 0x4c, 0x6f, 0xf8, 0x65, 0xda, 0x4e, 0x61, 0xe4, 0x75, 0x2c, 0x4b, 0x01, 0xe5, 0x21, 0x0d,
	0x4f, 0xc0, 0x0b, 0x02, 0xb0, 0xb0, 0x2f, 0x0c, 0x02, 0x70, 0x94, 0x23, 0x0b, 0xc3, 0xe9, 0xe0,
	0x97, 0xd1, 0x70, 0x63, 0x00, 0x45, 0x8d, 0xe9, 0x71, 0xd7, 0xe5, 0xeb, 0x5f, 0xf8, 0x78, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x4d, 0xf5, 0x5a,
}

var lenValidMessageInBits = len(validMessageFrame) * 8

// maxEpochTime is the value of a GPS and Beidou epoch time
// just before it rolls over.
const maxEpochTime uint = (7 * 24 * 3600 * 1000) - 1

// dateLayout defines the layout of dates when they are displayed.  It
// produces "yyyy-mm-dd hh:mm:ss.ms timeshift timezone".
const dateLayout = "2006-01-02 15:04:05.000 -0700 MST"

var london *time.Location
var paris *time.Location

var logger *log.Logger

func init() {

	london, _ = time.LoadLocation("Europe/London")
	paris, _ = time.LoadLocation("Europe/Paris")
	writer := switchwriter.New()
	logger = log.New(writer, "rtcm_test", 0)
}

// TestReadNextRTCM3MessageFrame checks that ReadNextRTCM3MessageFrame correctly
// handles a valid message.
func TestReadNextRTCM3MessageFrameWithValidMessage(t *testing.T) {

	const wantType = 1097

	r := bytes.NewReader(validMessageFrame)
	reader := bufio.NewReader(r)

	now := time.Now()
	handler := New(now, logger)
	handler.StopOnEOF = true

	frame1, readError1 := handler.ReadNextRTCM3MessageFrame(reader)
	if readError1 != nil {
		t.Error(readError1)
		return
	}

	message, messageFetchError := handler.GetMessage(frame1)
	if messageFetchError != nil {
		t.Error(messageFetchError)
	}

	if !message.Complete {
		t.Error("not complete")
	}

	if !message.CRCValid {
		t.Error("CRC check fails")
	}

	if !message.Valid {
		t.Error("invalid")
	}

	gotType := message.MessageType
	if wantType != gotType {
		t.Errorf("expected type %d got %d", wantType, gotType)
	}
}

// TestReadNextRTCM3MessageFrameWithShortBitStream checks that ReadNextRTCM3MessageFrame
// handles a short bitstream.
func TestReadNextRTCM3MessageFrameWithShortBitStream(t *testing.T) {
	messageFrameWithLengthZero := []byte{0xd3, 0x00, 0x00, 0x44, 0x90, 0x00}

	r := bytes.NewReader(messageFrameWithLengthZero)
	reader := bufio.NewReader(r)

	now := time.Now()
	handler := New(now, logger)
	handler.StopOnEOF = true

	// ReadNextRTCM3MessageFrame calls GetMessageLengthAndType.  That returns
	// an error because the message length is zero.  ReadNextRTCM3MessageFrame
	// should eat the error message and return a yte slice containing the five
	// bytes that it consumed.
	frame, err := handler.ReadNextRTCM3MessageFrame(reader)

	if err != nil {
		t.Errorf("want no error, got %v", err)
	}

	if frame == nil {
		t.Error("want a message frame, got nil")
	}

	if len(frame) != 5 {
		t.Errorf("want a message frame of length 5, got length %d", len(frame))
	}

	if frame[0] != 0xd3 {
		t.Errorf("want a message frame starting 0xd3, got first byte of 0x%02x", frame[0])
	}
}

// TestGetMessageLengthAndType checks GetMessageLengthAndType
func TestGetMessageLengthAndType(t *testing.T) {

	messageFrameWithIncorrectStart := []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff}
	messageFrameWithLengthTooBig := []byte{0xd3, 0xff, 0xff, 0xff, 0xff, 0xff}
	messageFrameWithLengthZero := []byte{0xd3, 0x00, 0x00, 0x44, 0x90, 0x00}

	var testData = []struct {
		description       string
		bitStream         []byte
		wantMessageType   int
		wantMessageLength uint
		wantError         string
	}{
		{"valid", validMessageFrame, 1097, 170, ""},
		{"invalid", validMessageFrame[:4], NonRTCMMessage, 0,
			"the message is too short to get the header and the length"},
		{"invalid", messageFrameWithIncorrectStart, NonRTCMMessage, 0,
			"message starts with 0xff not 0xd3"},
		{"invalid", messageFrameWithLengthZero, 1097, 0,
			"zero length message type 1097"},
		{"invalid", messageFrameWithLengthTooBig, NonRTCMMessage, 0,
			"bits 8-13 of header are 63, must be 0"},
	}
	for _, td := range testData {
		handler := New(time.Now(), logger)
		gotMessageLength, gotMessageType, gotError := handler.GetMessageLengthAndType(td.bitStream)
		if td.wantError != "" {
			if td.wantError != gotError.Error() {
				t.Errorf("%s: want %s, got %s", td.description, td.wantError, gotError.Error())
			}
		}
		if td.wantMessageLength != gotMessageLength {
			t.Errorf("%s: want %d, got %v", td.description, td.wantMessageLength, gotMessageLength)
		}

		if td.wantMessageType != gotMessageType {
			t.Errorf("%s: want %d, got %d", td.description, td.wantMessageType, gotMessageType)
		}
	}

}

func TestGetMessageLengthAndTypeWithShortBitstream(t *testing.T) {
	// The message bit stream is less than 5 bytes long.
}

// TestHandleMessages tests the handleMessages method.
func TestHandleMessages(t *testing.T) {
	// handleMessages reads the given data and writes any valid messages to
	// the given channel.  The test data contains one valid message of type
	// 1077 and some junk so the channel should contain a message of type
	// 1077 and a message of type NonRTCMMessage.

	var messageDataArray = [...]byte{

		// RTCM message type 1077 - signals from GPS satellites:
		0xd3, 0x00, 0xdc, // header - message length (0xdc - 220)
		// message, starting with  12-bit message type (0x435 - 1077), padded with null bytes
		// at the end.
		0x43, 0x50, 0x00, 0x67, 0x00, 0x97, 0x62, 0x00, 0x00, 0x08, 0x40, 0xa0, 0x65,
		0x00, 0x00, 0x00, 0x00, 0x20, 0x00, 0x80, 0x00, 0x6d, 0xff, 0xa8, 0xaa, 0x26, 0x23, 0xa6, 0xa2,
		0x23, 0x24, 0x00, 0x00, 0x00, 0x00, 0x36, 0x68, 0xcb, 0x83, 0x7a, 0x6f, 0x9d, 0x7c, 0x04, 0x92,
		0xfe, 0xf2, 0x05, 0xb0, 0x4a, 0xa0, 0xec, 0x7b, 0x0e, 0x09, 0x27, 0xd0, 0x3f, 0x23, 0x7c, 0xb9,
		0x6f, 0xbd, 0x73, 0xee, 0x1f, 0x01, 0x64, 0x96, 0xf5, 0x7b, 0x27, 0x46, 0xf1, 0xf2, 0x1a, 0xbf,
		0x19, 0xfa, 0x08, 0x41, 0x08, 0x7b, 0xb1, 0x1b, 0x67, 0xe1, 0xa6, 0x70, 0x71, 0xd9, 0xdf, 0x0c,
		0x61, 0x7f, 0x19, 0x9c, 0x7e, 0x66, 0x66, 0xfb, 0x86, 0xc0, 0x04, 0xe9, 0xc7, 0x7d, 0x85, 0x83,
		0x7d, 0xac, 0xad, 0xfc, 0xbe, 0x2b, 0xfc, 0x3c, 0x84, 0x02, 0x1d, 0xeb, 0x81, 0xa6, 0x9c, 0x87,
		0x17, 0x5d, 0x86, 0xf5, 0x60, 0xfb, 0x66, 0x72, 0x7b, 0xfa, 0x2f, 0x48, 0xd2, 0x29, 0x67, 0x08,
		0xc8, 0x72, 0x15, 0x0d, 0x37, 0xca, 0x92, 0xa4, 0xe9, 0x3a, 0x4e, 0x13, 0x80, 0x00, 0x14, 0x04,
		0xc0, 0xe8, 0x50, 0x16, 0x04, 0xc1, 0x40, 0x46, 0x17, 0x05, 0x41, 0x70, 0x52, 0x17, 0x05, 0x01,
		0xef, 0x4b, 0xde, 0x70, 0x4c, 0xb1, 0xaf, 0x84, 0x37, 0x08, 0x2a, 0x77, 0x95, 0xf1, 0x6e, 0x75,
		0xe8, 0xea, 0x36, 0x1b, 0xdc, 0x3d, 0x7a, 0xbc, 0x75, 0x42, 0x80, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		// 24-bit Cyclic Redundancy Check
		0xfe, 0x69, 0xe8,

		's', 'o', 'm', 'e', ' ', 'j', 'u', 'n', 'k', // junk which should be returned as a non-RTCM message.
	}

	var messageData []byte = messageDataArray[:]

	const wantNumMessages = 2
	const wantType0 = 1077
	const wantLength0 = 226
	var wantContents0 = messageData[:226]
	const wantType1 = NonRTCMMessage
	const wantLength1 = 9
	var wantContents1 = messageData[226:]

	reader := bytes.NewReader(messageData)

	channels := make([]chan RTCM3Message, 0)
	ch := make(chan RTCM3Message, 10)
	channels = append(channels, ch)
	rtcmHandler := New(time.Now(), nil)
	rtcmHandler.StopOnEOF = true

	// Test
	rtcmHandler.HandleMessages(reader, channels)
	// Close the channel so that a channel reader knows when it's finished.
	close(ch)

	// Check.  Read the data back from the channel and check the message type
	// and validity flags.
	messages := make([]RTCM3Message, 0)
	for {
		message, ok := <-ch
		if !ok {
			// Done - chan is drained.
			break
		}
		messages = append(messages, message)
	}

	if len(messages) != wantNumMessages {
		t.Errorf("want %d message, got %d messages", wantNumMessages, len(messages))
	}

	if !messages[0].Complete {
		t.Error("not complete")
	}

	if !messages[0].CRCValid {
		t.Error("CRC check fails")
	}

	if !messages[0].Valid {
		t.Error("invalid")
	}

	if messages[1].Complete {
		t.Error("should be incomplete")
	}

	if messages[1].CRCValid {
		t.Error("CRC check should fail")
	}

	if messages[1].Valid {
		t.Error("should be invalid")
	}

	r0 := bytes.NewReader(messages[0].RawData)
	resultReader0 := bufio.NewReader(r0)
	message0, err0 := rtcmHandler.ReadNextRTCM3Message(resultReader0)
	if err0 != nil {
		t.Fatal(err0)
	}

	if message0 == nil {
		t.Errorf("message 0 is empty")
		return
	}

	got0 := message0.MessageType
	if wantType0 != got0 {
		t.Errorf("want message type %d, got message type %d", wantType0, got0)
	}

	if message0.RawData == nil {
		t.Errorf("raw data in message 0 is nil")
		return
	}

	gotLength0 := len(message0.RawData)

	if wantLength0 != gotLength0 {
		t.Errorf("want message length %d got %d", wantLength0, gotLength0)
	}

	gotContents0 := message0.RawData

	if !bytes.Equal(wantContents0, gotContents0) {
		t.Error("contents of message 0 is not correct")
	}

	if !message0.Complete {
		t.Error("not complete")
	}

	if !message0.CRCValid {
		t.Error("CRC check fails")
	}

	if !message0.Valid {
		t.Error("invalid")
	}

	r1 := bytes.NewReader(messages[1].RawData)
	resultReader1 := bufio.NewReader(r1)
	message1, err1 := rtcmHandler.ReadNextRTCM3Message(resultReader1)
	if err1 != nil {
		t.Fatal(err1)
	}

	if message1 == nil {
		t.Fatal("message 1 is empty")
		return
	}

	got1 := message1.MessageType
	if wantType1 != got1 {
		t.Errorf("want message type %d, got message type %d", wantType1, got1)
	}

	if message0.RawData == nil {
		t.Errorf("raw data in message 0 is nil")
		return
	}

	gotLength1 := len(message1.RawData)

	if wantLength1 != gotLength1 {
		t.Errorf("want message length %d got %d", wantLength1, gotLength1)
	}

	gotContents1 := message1.RawData

	if !bytes.Equal(wantContents1, gotContents1) {
		t.Error("contents of message 1 is not correct")
	}

	if message1.Complete {
		t.Error("should be incomplete")
	}

	if message1.CRCValid {
		t.Error("CRC check should fail")
	}

	if message1.Valid {
		t.Error("should be invalid")
	}
}

// TestReadIncompleteMessage tests that an incomplete RTCM message is processed
// correctly.  It should be returned as a non-RTCM message.
func TestReadIncompleteMessage(t *testing.T) {

	// This is the message contents that should result.
	want := string(incompleteMessage)

	r := bytes.NewReader(incompleteMessage)
	imReader := bufio.NewReader(r)

	startTime := time.Date(2020, time.November, 13, 0, 0, 0, 0, locationUTC)
	rtcm := New(startTime, logger)
	rtcm.StopOnEOF = true

	// The first call should read the incomplete message, hit
	// EOF and ignore it.
	frame1, readError1 := rtcm.ReadNextRTCM3MessageFrame(imReader)
	if readError1 != nil {
		t.Fatal(readError1)
	}

	// The message is incomplete so expect an error.
	message, messageFetchError := rtcm.GetMessage(frame1)
	if messageFetchError == nil {
		t.Error("expected to get an error (reading an incomplete message)")
	}

	if message.MessageType != 1127 {
		t.Errorf("expected message type %d, got %d",
			1127, message.MessageType)
	}

	if message.Valid {
		t.Error("expected an invalid message")
	}

	if message.Complete {
		t.Error("expected an incomplete message")
	}

	got := string(message.RawData)

	if len(want) != len(got) {
		t.Errorf("expected a message body %d long, got %d", len(want), len(got))
	}

	if want != got {
		t.Errorf("message content doesn't match what we expected value")
	}

	// The second call should return nil and the EOF.
	frame2, readError2 := rtcm.ReadNextRTCM3MessageFrame(imReader)
	if readError2 == nil {
		t.Errorf("expected an error")
	}
	if readError2 != io.EOF {
		t.Errorf("expected EOF, got %v", readError2)
	}
	if frame2 != nil {
		t.Errorf("expected no frame, got %s", string(frame2))
	}
}

func TestReadAlmostCompleteMessage(t *testing.T) {
	data := []byte{
		0xd3, 0x00, 0xf4, 0x43, 0x50, 0x00, 0x49, 0x96, 0x84, 0x2e, 0x00, 0x00, 0x40, 0xa0, 0x85, 0x80,
		0x00, 0x00, 0x00, 0x20, 0x00, 0x80, 0x5f, 0xa9, 0xc8, 0x88, 0xea, 0x08, 0xe9, 0x88, 0x8a, 0x6a,
		0x60, 0x00, 0x00, 0x00, 0x00, 0xd6, 0x0a, 0x1b, 0xc5, 0x57, 0x9f, 0xf8, 0x92, 0xf2, 0x2e, 0x2d,
		0xb0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x43,
		0x50, 0xd3, 0x00, 0xdc, 0x43, 0xf0, 0x00, 0x6e, 0x5c, 0x48, 0xee, 0x00, 0x00, 0x41, 0x83, 0x41,
		0x80, 0x00, 0x00, 0x00, 0x00, 0x20, 0x80, 0x00, 0xfd, 0xa4, 0x26, 0x22, 0xa4, 0x23, 0xa5, 0x22,
		0x20, 0x46, 0x68, 0x3d, 0xd4, 0xae, 0xca, 0x74, 0xd2, 0x20, 0x21, 0xc1, 0xf5, 0xcd, 0xa5, 0x85,
		0x67, 0xee, 0x70, 0x08, 0x9e, 0xd7, 0x80, 0xd6, 0xdf, 0xca, 0x00, 0x3a, 0x1b, 0x5c, 0xb9, 0xd2,
		0xf5, 0xe6, 0xf7, 0x5a, 0x37, 0x76, 0x78, 0x9f, 0x71, 0xa8, 0x7a, 0xde, 0xf7, 0xb5, 0x77, 0x86,
		0xa0, 0xd8, 0x6e, 0xbc, 0x60, 0xfe, 0x66, 0xd1, 0x8c, 0xed, 0x42, 0x68, 0x50, 0xee, 0xe8, 0x7b,
		0xd0, 0xa7, 0xcb, 0xdf, 0xcc, 0x10, 0xef, 0xd3, 0xef, 0xdf, 0xe4, 0xb8, 0x5f, 0xdf, 0xd6, 0x3f,
		0xe2, 0xad, 0x0f, 0xf6, 0x3c, 0x08, 0x01, 0x8a, 0x20, 0x66, 0xdf, 0x8d, 0x65, 0xb7, 0xbd, 0x9c,
		0x4f, 0xc5, 0xa2, 0x24, 0x35, 0x0c, 0xcc, 0x52, 0xcc, 0x95, 0x23, 0xcd, 0x93, 0x44, 0x8d, 0x23,
		0x40, 0x6f, 0xd4, 0xef, 0x32, 0x4c, 0x80, 0x00, 0x2b, 0x08, 0xc2, 0xa0, 0x98, 0x31, 0x0a, 0xc3,
		0x00, 0xa8, 0x2e, 0x0a, 0xc8, 0x18, 0x8d, 0x72, 0x48, 0x75}

	r := bytes.NewReader(data)
	imReader := bufio.NewReader(r)

	startTime := time.Date(2020, time.November, 13, 0, 0, 0, 0, locationUTC)
	rtcm := New(startTime, logger)
	rtcm.StopOnEOF = true

	// The first call should read the incomplete message, hit
	// EOF and ignore it.
	frame1, readError1 := rtcm.ReadNextRTCM3MessageFrame(imReader)
	if readError1 != nil {
		t.Fatal(readError1)
	}

	// The message is incomplete so expect an error.
	message, messageFetchError := rtcm.GetMessage(frame1)
	if messageFetchError == nil {
		t.Error("expected to get an error (reading an incomplete message)")
	}

	t.Log(len(message.RawData))

}

func TestReadEmptyBuffer(t *testing.T) {
	data := []byte{}

	r := bytes.NewReader(data)
	imReader := bufio.NewReader(r)

	startTime := time.Date(2020, time.November, 13, 0, 0, 0, 0, locationUTC)
	rtcm := New(startTime, logger)
	rtcm.StopOnEOF = false

	// This should read the empty buffer, hit EOF and ignore it.
	m, err := rtcm.ReadNextRTCM3Message(imReader)

	if err != nil {
		t.Errorf("Expected no error, got %s", err.Error())
	}

	if m != nil {
		if m.RawData == nil {
			t.Errorf("want nil RTCM3Message, got a struct with nil RawData")
		}
		t.Errorf("Expected nil frame, got %d bytes of RawData", len(m.RawData))
	}
}

// TestReadJunk checks that ReadNextChunk handles interspersed junk correctly.
func TestReadJunk(t *testing.T) {
	r := bytes.NewReader(junkAtStart)
	junkAtStartReader := bufio.NewReader(r)
	ch := make(chan byte, 100)
	for _, j := range junkAtStart {
		ch <- j
	}
	startTime := time.Date(2020, time.November, 13, 0, 0, 0, 0, locationUTC)
	rtcm := New(startTime, logger)
	rtcm.StopOnEOF = true

	frame, err1 := rtcm.ReadNextRTCM3MessageFrame(junkAtStartReader)
	if err1 != nil {
		t.Fatal(err1.Error())
	}

	message, messageFetchError := rtcm.GetMessage(frame)
	if messageFetchError != nil {
		t.Errorf("error getting message - %v", messageFetchError)
	}

	if message.MessageType != NonRTCMMessage {
		t.Errorf("expected message type %d, got %d",
			NonRTCMMessage, message.MessageType)
	}

	gotBody := string(message.RawData[:4])

	if wantJunk != gotBody {
		t.Errorf("expected %s, got %s", wantJunk, gotBody)
	}
}

func TestReadOnlyJunk(t *testing.T) {
	r := bytes.NewReader(allJunk)
	junkReader := bufio.NewReader(r)
	startTime := time.Date(2020, time.November, 13, 0, 0, 0, 0, locationUTC)
	rtcm := New(startTime, logger)
	rtcm.StopOnEOF = true

	frame, err1 := rtcm.ReadNextRTCM3MessageFrame(junkReader)

	if err1 != nil {
		t.Fatal(err1.Error())
	}

	message, messageFetchError := rtcm.GetMessage(frame)
	if messageFetchError != nil {
		t.Errorf("error getting message - %v", messageFetchError)
	}

	if message.MessageType != NonRTCMMessage {
		t.Errorf("expected message type %d, got %d",
			NonRTCMMessage, message.MessageType)
	}

	gotBody := string(message.RawData)

	if wantJunk != gotBody {
		t.Errorf("expected %s, got %s", wantJunk, gotBody)
	}

	// Call again - expect EOF.

	frame2, err2 := rtcm.ReadNextRTCM3MessageFrame(junkReader)

	if err2 == nil {
		t.Fatal("expected EOF error")
	}
	if err2.Error() != "EOF" {
		t.Errorf("expected EOF error, got %s", err2.Error())
	}

	if len(frame2) != 0 {
		t.Errorf("expected frame to be empty, got %s", string(frame2))
	}
}

//TestGetMessageWithRealData checks that GetMessage correctly handles an MSM4 message extracted from
// real data.
func TestGetMessageWithRealData(t *testing.T) {

	// These data were collected on the 17th June 2022.
	startTime := time.Date(2022, time.June, 17, 0, 0, 0, 0, locationUTC)
	var msm4 = []byte{
		0xd3, 0x00, 0x7b, 0x46, 0x40, 0x00, 0x78, 0x4e, 0x56, 0xfe, 0x00, 0x00, 0x00, 0x58, 0x16, 0x00,
		0x20, 0x00, 0x00, 0x00, 0x20, 0x02, 0x00, 0x00, 0x7f, 0x55, 0x0e, 0xa2, 0xa2, 0xa4, 0x9a, 0x92,
		0xa3, 0x10, 0xe2, 0x4a, 0xd0, 0xa9, 0xba, 0x91, 0x8f, 0xc0, 0x62, 0x40, 0x8d, 0xa6, 0xa4, 0x4c,
		0x4d, 0x9f, 0xdb, 0x3c, 0x65, 0x87, 0x9f, 0x4f, 0x16, 0x3b, 0xf2, 0x55, 0x40, 0x72, 0xe7, 0x01,
		0x4d, 0x8c, 0x1a, 0x85, 0x40, 0x63, 0x1d, 0x42, 0x07, 0x3e, 0x07, 0xf3, 0x15, 0xe3, 0x36, 0x77,
		0xb0, 0x29, 0xde, 0x66, 0x68, 0x84, 0x9b, 0xf7, 0xff, 0xff, 0xff, 0xff, 0xfe, 0x00, 0x3d, 0x15,
		0x15, 0x4f, 0x6d, 0x78, 0x63, 0x58, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x5b, 0xa7,
		0x0c,
		0xd3, 0x00, 0x7b, 0x44, 0x60, 0x00, 0x78, 0x4f, 0x31, 0xbe, 0x00, 0x00, 0x21, 0x84, 0x00,
		0x60, 0x40, 0x00, 0x00, 0x00, 0x20, 0x01, 0x00, 0x00, 0x7f, 0xbe, 0xb2, 0x9e, 0xa2, 0xae, 0xb8,
		0xa4, 0xad, 0x04, 0x04, 0x5a, 0x33, 0xa2, 0x16, 0x93, 0x1e, 0x6f, 0xd8, 0x9f, 0xbb, 0xdd, 0x3d,
		0x3a, 0x7e, 0xee, 0x9a, 0xdc, 0x4c, 0x3e, 0xc8, 0x80, 0x97, 0x06, 0x83, 0x77, 0xc6, 0xcc, 0xc2,
		0x6a, 0x04, 0xae, 0xff, 0x1b, 0x83, 0xfd, 0xcb, 0xbf, 0xc9, 0x2b, 0xff, 0x33, 0x78, 0xf9, 0x91,
		0xe3, 0xeb, 0x7c, 0x50, 0x87, 0xae, 0x02, 0x2c, 0x1e, 0xf8, 0x15, 0x20, 0x3a, 0xb8, 0x50, 0xeb,
		0xbb, 0xc0, 0xb4, 0xf5, 0x03, 0x15, 0x07, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xc0, 0x01, 0x3d,
		0x17, 0xdd, 0x7d, 0x54, 0x52, 0xf5, 0xf6, 0xd7, 0x48, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x76,
		0xfb, 0x6f,
		0xd3, 0x00, 0x7b, 0x43, 0xc0, 0x00, 0xb3, 0xe2, 0x16, 0x7e, 0x00, 0x00, 0x0c, 0x07,
		0x0c, 0x00, 0x00, 0x00, 0x00, 0x00, 0x20, 0x80, 0x00, 0x00, 0x7f, 0xfe, 0x86, 0x82, 0x94, 0x8c,
		0x9a, 0x88, 0x93, 0x2c, 0xd2, 0x39, 0x44, 0x70, 0xc6, 0xf5, 0x49, 0xb7, 0xf0, 0x6f, 0xc9, 0x86,
		0x69, 0x8c, 0x8d, 0x00, 0x85, 0x01, 0x69, 0xe2, 0xdb, 0xc8, 0x31, 0x5e, 0x52, 0xab, 0xdb, 0x13,
		0xf6, 0x19, 0x09, 0xe8, 0x12, 0xf3, 0xfe, 0x94, 0xc0, 0x0d, 0xa7, 0xe1, 0xc2, 0x56, 0x07, 0x9e,
		0x68, 0x00, 0x0b, 0x90, 0x02, 0xb0, 0x7f, 0xb9, 0xe9, 0x7f, 0x01, 0x9a, 0x15, 0xc5, 0x08, 0x57,
		0x78, 0xfe, 0xd7, 0x0e, 0x7b, 0x8c, 0x9a, 0x0a, 0x89, 0x78, 0x56, 0x8a, 0x1f, 0xff, 0xfd, 0xff,
		0xf7, 0x5f, 0xff, 0xe0, 0x00, 0x65, 0x5e, 0x56, 0xc5, 0x0d, 0xf5, 0x44, 0xf5, 0x15, 0x5f, 0x38,
		0x5d, 0xa9, 0x5d,
		0xd3, 0x00, 0x98, 0x43, 0x20, 0x00, 0x78, 0x4f, 0x31, 0xbc, 0x00, 0x00, 0x2b,
		0x50, 0x08, 0x06, 0x00, 0x00, 0x00, 0x00, 0x20, 0x00, 0x80, 0x00, 0x5f, 0xfd, 0xe9, 0x49, 0xa8,
		0xe9, 0x08, 0xa8, 0xc9, 0x2a, 0x69, 0xc3, 0x2b, 0x30, 0xfc, 0x5d, 0xba, 0x3d, 0x14, 0x76, 0x18,
		0xf0, 0xc8, 0xe5, 0xdc, 0x8d, 0xf8, 0xfb, 0xbb, 0x8b, 0x76, 0xf4, 0x02, 0x5e, 0x01, 0x70, 0xa6,
		0xf9, 0x4a, 0x41, 0x56, 0x02, 0x74, 0x48, 0x6f, 0xe0, 0x84, 0xc0, 0x1c, 0x3f, 0x44, 0x7c, 0xc0,
		0x3f, 0x05, 0x1e, 0x5b, 0x97, 0xf9, 0xd9, 0x83, 0xf9, 0xcb, 0x07, 0xe4, 0x72, 0xe0, 0x38, 0xdf,
		0x01, 0x09, 0x4e, 0x18, 0x42, 0xf8, 0x66, 0xdd, 0x20, 0xc1, 0x5a, 0x83, 0x25, 0xa2, 0x0f, 0x65,
		0x17, 0x83, 0xe8, 0x3e, 0x04, 0x23, 0x84, 0x6b, 0x9e, 0x12, 0xf7, 0x67, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xf7, 0xf8, 0x00, 0x05, 0xf5, 0xcb, 0x6d, 0x57, 0x4f, 0x85, 0x97, 0x57, 0x6c, 0xcf,
		0x53, 0x30, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xce, 0xce,
		0x4c,
	}

	const wantNumSatellites = 7
	const wantMessageType = 1124

	r := bytes.NewReader(msm4)
	reader := bufio.NewReader(r)

	rtcm := New(startTime, logger)
	rtcm.StopOnEOF = true

	frame, err1 := rtcm.ReadNextRTCM3MessageFrame(reader)

	if err1 != nil {
		t.Error(err1.Error())
		return
	}

	message, messageFetchError := rtcm.GetMessage(frame)
	if messageFetchError != nil {
		t.Errorf("error getting message - %v", messageFetchError)
		return
	}

	if message.MessageType != wantMessageType {
		t.Errorf("expected message type 1124 got %d", message.MessageType)
		return
	}

	// Get the message in display form.
	display, ok := message.PrepareForDisplay(rtcm).(*msm4message.Message)
	if !ok {
		t.Error("expected the readable message to be *MSMMessage\n")
		return
	}

	if len(display.Satellites) != wantNumSatellites {
		t.Errorf("expected %d satellites, got %d", wantNumSatellites, len(display.Satellites))
	}

	// The outer slice should be the same size as the satellite slice.

	if len(display.Signals) != wantNumSatellites {
		t.Errorf("expected %d sets of signals, got %d", wantNumSatellites, len(display.Satellites))
	}
}

//TestGetMessageWithNoData checks that GetMessage correctly a bit stream with no data.
func TestGetMessageWithNoData(t *testing.T) {

	empty := make([]byte, 0)
	some := make([]byte, 1)

	var testData = []struct {
		description string
		frame       []byte
		want        string
	}{
		{"nil", nil, "zero length message frame"},
		{"empty", empty, "zero length message frame"},
		{"some", some, ""},
	}
	for _, td := range testData {
		startTime := time.Date(2020, time.November, 13, 0, 0, 0, 0, locationUTC)
		handler := New(startTime, logger)
		gotMessage, gotError := handler.GetMessage(td.frame)
		if td.want == "" {
			if gotMessage == nil {
				t.Error("expected a message")
			}
			if gotError != nil {
				t.Errorf("%s: want a message, got error %v", td.description, gotError.Error())
			}
		} else {
			if gotMessage != nil {
				t.Error("expected a nil message")
			}
			if gotError.Error() != td.want {
				t.Errorf("%s: want %s, got %v", td.description, td.want, gotError)
			}
		}
	}
}

// TestReadGetMessageWithShortBitStream checks that GetMessage handles a short
// bitstream correctly.
func TestReadGetMessageWithShortBitStream(t *testing.T) {
	messageFrameWithLengthZero := []byte{0xd3, 0x00, 0x00, 0x44}

	now := time.Now()
	handler := New(now, logger)
	handler.StopOnEOF = true

	const wantWarning = "the message is too short to get the header and the length"

	// GetMessage calls GetMessageLengthAndType.  That returns an error because the
	// bit stream is too short.  GetMessage should
	// should eat the error message and return a byte slice containing the five
	// bytes that it consumed.
	message, err := handler.GetMessage(messageFrameWithLengthZero)

	if err == nil {
		t.Error("want an error")
		return
	}

	if err.Error() != wantWarning {
		t.Errorf("want error - %s, got %v", wantWarning, err)
	}

	if message == nil {
		t.Error("want a message, got nil")
		return
	}

	if message.RawData == nil {
		t.Error("want some raw data, got nil")
		return
	}

	if len(message.RawData) != 4 {
		t.Errorf("want a message frame of length 4, got length %d", len(message.RawData))
	}

	if message.Warning != wantWarning {
		t.Errorf("want warning - %s, got %s", wantWarning, message.Warning)
	}
}

// TestGetMessageWithNonRTCMMessage checks that GetMessage handles a bit stream
// containing a non-RTCM message correctly.
func TestGetMessageWithNonRTCMMessage(t *testing.T) {
	// A bit stream containing just the non_RTCM message "plain text".
	plainTextBytes := []byte{'p', 'l', 'a', 'i', 'n', ' ', 't', 'e', 'x', 't'}
	// A bit stream containing the non_RTCM message "plain text" followed by the
	// start of an RTCM message.
	plainTextBytesWithD3 := []byte{'p', 'l', 'a', 'i', 'n', ' ', 't', 'e', 'x', 't', 0xd3}
	const plainText = "plain text"

	var testData = []struct {
		description string
		bitStream   []byte
	}{
		{"plain text", plainTextBytes},
		{"plain text with D3", plainTextBytesWithD3},
	}
	for _, td := range testData {
		startTime := time.Now()
		handler := New(startTime, logger)
		handler.StopOnEOF = true
		// ReadNextMessageFrame should strip off any training D3 byte.
		r := bytes.NewReader(td.bitStream)
		messageReader := bufio.NewReader(r)
		bitStream, frameError := handler.ReadNextRTCM3MessageFrame(messageReader)
		if frameError != nil {
			t.Error(frameError)
			return
		}

		gotMessage, gotError := handler.GetMessage(bitStream)

		if gotError != nil {
			t.Error(gotError)
			// return
		}

		if gotMessage == nil {
			t.Error("want a message, got nil")
			return
		}

		if gotMessage.MessageType != NonRTCMMessage {
			t.Errorf("want a NONRTCMMessage, got message type %d", gotMessage.MessageType)
			return
		}

		if gotMessage.RawData == nil {
			t.Error("want some raw data, got nil")
			return
		}

		if len(gotMessage.RawData) != len(plainText) {
			t.Errorf("want a message frame of length %d, got length %d",
				len(plainText), len(gotMessage.RawData))
		}

		if string(gotMessage.RawData) != plainText {
			t.Errorf("want raw data - %s, got %s",
				plainText, string(gotMessage.RawData))
		}
	}
}

func TestReadNextMessageFrame(t *testing.T) {
	r := bytes.NewReader(realData)
	realDataReader := bufio.NewReader(r)
	startTime := time.Date(2020, time.November, 13, 0, 0, 0, 0, locationUTC)
	rtcmHandler := New(startTime, logger)
	rtcmHandler.StopOnEOF = true

	frame, err1 := rtcmHandler.ReadNextRTCM3MessageFrame(realDataReader)
	if err1 != nil {
		t.Fatal(err1.Error())
	}

	message, messageFetchError := rtcmHandler.GetMessage(frame)
	if messageFetchError != nil {
		t.Errorf("error getting message - %v", messageFetchError)
		return
	}

	if message.MessageType != 1077 {
		t.Errorf("expected message type 1077, got %d", message.MessageType)
		return
	}
}

// TestWithRealData reads the first message from the real data and checks it in detail.
func TestWithRealData(t *testing.T) {

	const wantRangeWholeMilliSecs = 81
	const wantRangeFractionalMilliSecs = 435
	// The data was produced by a real device and then converted to RINEX format.
	// These values were taken from the RINEX.
	const wantRange = 24410527.355

	r := bytes.NewReader(realData)
	realDataReader := bufio.NewReader(r)
	startTime := time.Date(2020, time.November, 13, 0, 0, 0, 0, locationUTC)
	rtcm := New(startTime, logger)
	rtcm.StopOnEOF = true

	m, readError := rtcm.ReadNextRTCM3Message(realDataReader)

	if readError != nil {
		t.Errorf("error reading data - %s", readError.Error())
		return
	}

	if m.MessageType != 1077 {
		t.Errorf("expected message type 1077, got %d", m.MessageType)
		return
	}

	message, ok := m.PrepareForDisplay(rtcm).(*msm7message.Message)

	if !ok {
		t.Errorf("expected message 0 to contain a type 1077 message but readable is nil")
		return
	}

	if message.Header.Constellation != "GPS" {
		t.Errorf("expected GPS, got %s", message.Header.Constellation)
		return
	}

	if len(message.Satellites) != 8 {
		t.Errorf("expected 8 GPS satellites, got %d",
			len(message.Satellites))
		return
	}

	if message.Satellites[0].RangeWholeMillis != wantRangeWholeMilliSecs {
		t.Errorf("expected range whole  of %d, got %d",
			wantRangeWholeMilliSecs,
			message.Satellites[0].RangeWholeMillis)
		return
	}

	if message.Satellites[0].RangeFractionalMillis != wantRangeFractionalMilliSecs {
		t.Errorf("expected range fractional %d, got %d",
			wantRangeFractionalMilliSecs,
			message.Satellites[0].RangeFractionalMillis)
		return
	}

	// There should be one signal list per satellite
	if len(message.Signals) != len(message.Satellites) {
		t.Errorf("expected %d GPS signal lists, got %d",
			len(message.Signals), len(message.Satellites))
		return
	}

	numSignals1 := 0
	for i := range message.Signals {
		numSignals1 += len(message.Signals[i])
	}

	if numSignals1 != 14 {
		t.Errorf("expected 14 GPS signals, got %d", numSignals1)
		return
	}

	if message.Signals[0][0].SatelliteID != 4 {
		t.Errorf("expected satelliteID 4, got %d",
			message.Signals[0][0].SatelliteID)
		return
	}

	if message.Signals[0][0].RangeDelta != -26835 {
		t.Errorf("expected range delta -26835, got %d",
			message.Signals[0][0].RangeDelta)
		return
	}

	rangeMetres := message.Signals[0][0].RangeInMetres()

	if !utils.EqualWithin(3, wantRange, rangeMetres) {
		t.Errorf("expected range %f metres, got %3.6f", wantRange, rangeMetres)
		return
	}
}

// TestGetMessage checks GetMessage with a valid message.
func TestGetMessage(t *testing.T) {
	const expectedLength = 0x8a + 6
	startTime := time.Date(2020, time.December, 9, 0, 0, 0, 0, locationUTC)
	rtcm := New(startTime, logger)
	rtcm.StopOnEOF = true

	message, messageFetchError := rtcm.GetMessage(testData)
	if messageFetchError != nil {
		t.Errorf("error getting message - %v", messageFetchError)
		return
	}

	if expectedLength != len(message.RawData) {
		t.Errorf("expected message length %d, got %d",
			expectedLength, len(message.RawData))
		return
	}
}

// TestGPSEpochTimes tests that New sets up the correct start times
// for the GPS epochs.
func TestGPSEpochTimes(t *testing.T) {

	expectedEpochStart :=
		time.Date(2020, time.August, 1, 23, 59, 60-gpsLeapSeconds, 0, locationUTC)
	expectedNextEpochStart :=
		time.Date(2020, time.August, 8, 23, 59, 60-gpsLeapSeconds, 0, locationUTC)

	// Sunday 2020/08/02 BST, just after the start of the GPS epoch
	dateTime1 := time.Date(2020, time.August, 2, 1, 0, 0, (60 - gpsLeapSeconds), london)
	rtcm1 := New(dateTime1, logger)
	if expectedEpochStart != rtcm1.startOfThisGPSWeek {
		t.Errorf("expected %s result %s\n",
			expectedEpochStart.Format(dateLayout),
			rtcm1.startOfThisGPSWeek.Format(dateLayout))
		return
	}
	if expectedNextEpochStart != rtcm1.startOfNextGPSWeek {
		t.Errorf("expected %s result %s\n",
			expectedEpochStart.Format(dateLayout),
			rtcm1.startOfThisGPSWeek.Format(dateLayout))
		return
	}

	// Wednesday 2020/08/05
	dateTime2 := time.Date(2020, time.August, 5, 12, 0, 0, 0, london)
	rtcm2 := New(dateTime2, logger)
	if expectedEpochStart != rtcm2.startOfThisGPSWeek {
		t.Errorf("expected %s result %s\n",
			expectedEpochStart.Format(dateLayout),
			rtcm2.startOfThisGPSWeek.Format(dateLayout))
		return
	}

	// Sunday 2020/08/02 BST, just before the end of the GPS epoch
	dateTime3 := time.Date(2020, time.August, 9, 00, 59, 60-gpsLeapSeconds-1, 999999999, london)
	rtcm3 := New(dateTime3, logger)
	if expectedEpochStart != rtcm3.startOfThisGPSWeek {
		t.Errorf("expected %s result %s\n",
			expectedEpochStart.Format(dateLayout),
			rtcm3.startOfThisGPSWeek.Format(dateLayout))
		return
	}

	// Sunday 2020/08/02 BST, at the start of the next GPS epoch.
	dateTime4 := time.Date(2020, time.August, 9, 1, 59, 60-gpsLeapSeconds, 0, paris)
	startOfNext := time.Date(2020, time.August, 8, 23, 59, 60-gpsLeapSeconds, 0, locationUTC)

	rtcm4 := New(dateTime4, logger)
	if startOfNext != rtcm4.startOfThisGPSWeek {
		t.Errorf("expected %s result %s\n",
			startOfNext.Format(dateLayout),
			rtcm4.startOfThisGPSWeek.Format(dateLayout))
		return
	}
}

// TestBeidouEpochTimes checks that New ssets the correct start times
// for this and the next Beidou epoch.
func TestBeidouEpochTimes(t *testing.T) {
	// Like GPS time, the Beidou time rolls over every seven days,
	// but it uses a different number of leap seconds.

	// The first few seconds of Sunday UTC are in the previous Beidou week.
	expectedStartOfPreviousWeek :=
		time.Date(2020, time.August, 2, 0, 0, beidouLeapSeconds, 0, locationUTC)
	expectedStartOfThisWeek :=
		time.Date(2020, time.August, 9, 0, 0, beidouLeapSeconds, 0, locationUTC)
	expectedStartOfNextWeek :=
		time.Date(2020, time.August, 16, 0, 0, beidouLeapSeconds, 0, locationUTC)

	// The 9th is Sunday.  This start time should be in the previous week ...
	startTime1 := time.Date(2020, time.August, 9, 0, 0, 0, 0, locationUTC)
	rtcm1 := New(startTime1, logger)

	if !expectedStartOfPreviousWeek.Equal(rtcm1.startOfThisBeidouWeek) {
		t.Errorf("expected %s result %s\n",
			expectedStartOfPreviousWeek.Format(dateLayout), rtcm1.startOfThisBeidouWeek.Format(dateLayout))
	}

	// ... and so should this.
	startTime2 := time.Date(2020, time.August, 9, 0, 0, beidouLeapSeconds-1, 999999999, locationUTC)
	rtcm2 := New(startTime2, logger)

	if !expectedStartOfPreviousWeek.Equal(rtcm2.startOfThisBeidouWeek) {
		t.Errorf("expected %s result %s\n",
			expectedStartOfPreviousWeek.Format(dateLayout), rtcm2.startOfThisBeidouWeek.Format(dateLayout))
	}

	// This start time should be in this week.
	startTime3 := time.Date(2020, time.August, 9, 0, 0, beidouLeapSeconds, 0, locationUTC)
	rtcm3 := New(startTime3, logger)

	if !expectedStartOfThisWeek.Equal(rtcm3.startOfThisBeidouWeek) {
		t.Errorf("expected %s result %s\n",
			expectedStartOfThisWeek.Format(dateLayout), rtcm3.startOfThisBeidouWeek.Format(dateLayout))
	}

	// This start time should be just at the end of this Beidou week.
	startTime4 :=
		time.Date(2020, time.August, 16, 0, 0, beidouLeapSeconds-1, 999999999, locationUTC)
	rtcm4 := New(startTime4, logger)

	if !expectedStartOfThisWeek.Equal(rtcm4.startOfThisBeidouWeek) {
		t.Errorf("expected %s result %s\n",
			expectedStartOfThisWeek.Format(dateLayout), rtcm4.startOfThisBeidouWeek.Format(dateLayout))
	}

	// This start time should be just at the start of the next Beidou week.
	startTime5 :=
		time.Date(2020, time.August, 16, 0, 0, beidouLeapSeconds, 0, locationUTC)
	rtcm5 := New(startTime5, logger)

	if !expectedStartOfNextWeek.Equal(rtcm5.startOfThisBeidouWeek) {
		t.Errorf("expected %s result %s\n",
			expectedStartOfNextWeek.Format(dateLayout), rtcm5.startOfThisBeidouWeek.Format(dateLayout))
	}
}

// TestGlonassEpochTimes tests that New sets up the correct start time
// for the Glonass epochs.
func TestGlonassEpochTimes(t *testing.T) {

	// expect 9pm Saturday 1st August - midnight Sunday 2nd August in Russia - Glonass day 0.
	expectedEpochStart1 :=
		time.Date(2020, time.August, 1, 21, 0, 0, 0, locationUTC)
	// expect 9pm Sunday 2nd August - midnight Monday 3rd August - Glonass day 1
	expectedNextEpochStart1 :=
		time.Date(2020, time.August, 2, 21, 0, 0, 0, locationUTC)
		// expect Glonass day 0.
	expectedGlonassDay1 := uint(0)

	startTime1 :=
		time.Date(2020, time.August, 2, 5, 0, 0, 0, locationUTC)
	rtcm1 := New(startTime1, logger)
	if expectedEpochStart1 != rtcm1.startOfThisGlonassDay {
		t.Errorf("expected %s result %s\n",
			expectedEpochStart1.Format(dateLayout),
			rtcm1.startOfThisGlonassDay.Format(dateLayout))
	}
	if expectedNextEpochStart1 != rtcm1.startOfNextGlonassDay {
		t.Errorf("expected %s result %s\n",
			expectedEpochStart1.Format(dateLayout),
			rtcm1.startOfThisGlonassDay.Format(dateLayout))
	}
	if expectedGlonassDay1 != rtcm1.previousGlonassDay {
		t.Errorf("expected %d result %d\n",
			expectedGlonassDay1, rtcm1.previousGlonassDay)
	}

	// 21:00 on Monday 3rd August - 00:00 on Tuesday in Moscow - Glonass day 2.
	expectedEpochStart2 :=
		time.Date(2020, time.August, 3, 21, 0, 0, 0, locationUTC)
	// 21:00 on Tuesday 4th August - 00:00 on Wednesday in Moscow - Glonass day 3
	expectedNextEpochStart2 :=
		time.Date(2020, time.August, 4, 21, 0, 0, 0, locationUTC)
	expectedGlonassDay2 := uint(2)

	// Start just before 9pm on Tuesday 3rd August - just before the end of
	// Tuesday in Moscow - day 2
	startTime2 :=
		time.Date(2020, time.August, 3, 22, 59, 59, 999999999, locationUTC)
	rtcm2 := New(startTime2, logger)
	if expectedEpochStart2 != rtcm2.startOfThisGlonassDay {
		t.Errorf("expected %s result %s\n",
			expectedEpochStart2.Format(dateLayout),
			rtcm1.startOfThisGlonassDay.Format(dateLayout))
	}
	if expectedNextEpochStart2 != rtcm2.startOfNextGlonassDay {
		t.Errorf("expected %s result %s\n",
			expectedEpochStart2.Format(dateLayout),
			rtcm1.startOfThisGlonassDay.Format(dateLayout))
	}
	if expectedGlonassDay2 != rtcm2.previousGlonassDay {
		t.Errorf("expected %d result %d\n",
			expectedGlonassDay2, rtcm2.previousGlonassDay)
	}
}

// TestGetUTCFromGPSTime tests GetUTCFromGPSTime
func TestGetUTCFromGPSTime(t *testing.T) {

	// Use Monday August 10th BST as the start date
	startTime := time.Date(2020, time.August, 10, 2, 0, 0, 0, london)
	rtcm := New(startTime, logger)

	// Tha should give an epoch start just before midnight on Saturday 8th August.
	startOfThisEpoch :=
		time.Date(2020, time.August, 9, 1, 0, 0, 0, london).Add(gpsTimeOffset)

	// This epoch time us two days after the start of the week.
	millis := uint(48 * 3600 * 1000)

	expectedTime1 := startOfThisEpoch.AddDate(0, 0, 2)

	timeUTC1 := rtcm.GetUTCFromGPSTime(millis)

	if timeUTC1.Location().String() != locationUTC.String() {
		t.Errorf("expected location to be UTC, got %s", timeUTC1.Location().String())
	}
	if !expectedTime1.Equal(timeUTC1) {
		t.Errorf("expected %s result %s\n",
			expectedTime1.Format(dateLayout), timeUTC1.Format(dateLayout))
	}

	// The GPS clock counts milliseconds until (23:59:59.999 GMT less the leap
	// seconds() on the next Saturday (15th August).  In August that's (00:59:59.999 BST
	// less the leap seconds) on the next Sunday (16th August).
	const maxMillis uint = (7 * 24 * 3600 * 1000) - 1

	expectedTime2 :=
		time.Date(2020, time.August, 16, 0, 59, 59, 999000000, london).Add(gpsTimeOffset)

	timeUTC2 := rtcm.GetUTCFromGPSTime(maxMillis)

	if timeUTC2.Location().String() != locationUTC.String() {
		t.Errorf("expected location to be UTC, got %s", timeUTC2.Location().String())
	}
	if !expectedTime2.Equal(timeUTC2) {
		t.Errorf("expected %s result %s\n",
			expectedTime2.Format(dateLayout), timeUTC2.Format(dateLayout))
	}

	// The previous call of GetUTCTimeFromGPS was just before the rollover, so the
	// next call will roll the clock over, putting us into the next week.  A time
	// value of 500 milliseconds should give (02:00:00.500 less the leap seconds)
	// CET on Sunday 16th August.

	expectedTime3 :=
		time.Date(2020, time.August, 16, 2, 0, 0, 500000000, paris).Add(gpsTimeOffset)

	var gpsMillis3 uint = 500
	timeUTC3 := rtcm.GetUTCFromGPSTime(gpsMillis3)

	if timeUTC3.Location().String() != locationUTC.String() {
		t.Errorf("expected location to be UTC, got %s", timeUTC3.Location().String())
	}
	if !expectedTime3.Equal(timeUTC3) {
		t.Errorf("expected %s result %s\n",
			expectedTime3.Format(dateLayout), timeUTC3.Format(dateLayout))
	}

	// GPS time 20,000 with the week starting 16th August means Sunday
	// 2020/08/16 02:00:20 CET less GPS leap seconds, which is 00:00:20 BST less
	// GPS leap seconds.
	var gpsMillis5 uint = 20000
	expectedTime5 :=
		time.Date(2020, time.August, 16, 2, 0, 20, 0, paris).Add(gpsTimeOffset)

	timeUTC5 := rtcm.GetUTCFromGPSTime(gpsMillis5)

	if timeUTC5.Location().String() != locationUTC.String() {
		t.Errorf("expected location to be UTC, got %s", timeUTC5.Location().String())
	}
	if !expectedTime5.Equal(timeUTC5) {
		t.Errorf("expected %s result %s\n",
			expectedTime5.Format(dateLayout), timeUTC5.Format(dateLayout))
	}

	// GPS time for Monday 2020/08/17 14:00:00 + 500 ms CET
	// (12:00:00 + 500 ms UTC).
	gpsMillis6 := uint((38 * 3600 * 1000) + 500)
	expectedTime6 :=
		time.Date(2020, time.August, 17, 16, 0, 0, 500000000, paris).Add(gpsTimeOffset)

	timeUTC6 := rtcm.GetUTCFromGPSTime(gpsMillis6)

	if timeUTC6.Location().String() != locationUTC.String() {
		t.Errorf("expected location to be UTC, got %s", timeUTC6.Location().String())
	}
	if !expectedTime6.Equal(timeUTC6) {
		t.Errorf("expected %s result %s\n",
			expectedTime6.Format(dateLayout), timeUTC6.Format(dateLayout))
	}
}

// TestGetUTCFromGlonassTime tests GetUTCFromGlonassTime
func TestGetUTCFromGlonassTime(t *testing.T) {

	// Expect 3pm Tuesday 11th August Paris - 4pm in Russia.
	expectedTime1 := time.Date(2020, time.August, 11, 3, 0, 0, 0, paris)
	const expectedGlonassDay1 uint = 2
	// Start at 23:00:00 on Monday 10th August Paris, midnight on the Tuesday
	// 11th in Russia - start of Glonass day 2.
	startTime1 := time.Date(2020, time.August, 10, 23, 0, 0, 0, paris)
	rtcm := New(startTime1, logger)

	if expectedGlonassDay1 != rtcm.previousGlonassDay {
		t.Errorf("expected %d result %d", expectedGlonassDay1, rtcm.previousGlonassDay)
	}

	// Day = 2, glonassTime = (4*3600*1000), which is 4 am on Russian Tuesday,
	// which in UTC is 1 am on Tuesday 10th, in CEST, 3 am.
	day1 := uint(2)
	millis1 := uint(4 * 3600 * 1000)
	epochTime1 := day1<<27 | millis1

	dateUTC1 := rtcm.GetUTCFromGlonassTime(epochTime1)

	if dateUTC1.Location().String() != locationUTC.String() {
		t.Errorf("expected location to be UTC, got %s", dateUTC1.Location().String())
	}
	if !expectedTime1.Equal(dateUTC1) {
		t.Errorf("expected %s result %s\n",
			expectedTime1.Format(dateLayout), dateUTC1.Format(dateLayout))
	}

	// Day = 3, glonassTime = (18*3600*1000), which is 6pm on Russian Wednesday,
	// which in UTC is 3pm on Wednesday 12th, in CEST, 5pm.
	// Day was 2 in the last call, 3 in this one, which causes the day to roll
	// over.
	expectedTime2 := time.Date(2020, time.August, 12, 17, 0, 0, 0, paris)
	day2 := uint(3)
	millis2 := uint(18 * 3600 * 1000)
	epochTime2 := day2<<27 | millis2

	dateUTC2 := rtcm.GetUTCFromGlonassTime(epochTime2)

	if !expectedTime2.Equal(dateUTC2) {
		t.Errorf("expected %s result %s\n",
			expectedTime2.Format(dateLayout), dateUTC2.Format(dateLayout))
	}
}

// TestGetUTCFromGalileoTime tests GetUTCFromGalileoTime
func TestGetUTCFromGalileoTime(t *testing.T) {

	// Galileo time follows GPS time.

	startTime := time.Date(2020, time.August, 9, 23, 0, 0, 0, paris)
	rtcm := New(startTime, logger)

	// 6 am plus 300 ms Paris on Monday is 4am plus 300 ms GMT on Monday.
	// GPS time is a few seconds earlier.
	millis1 := uint(28*3600*1000 + 300) // 4 hours  plus 300 ms in ms.

	expectedTime :=
		time.Date(2020, time.August, 10, 6, 0, 0, 300000000, paris).Add(gpsTimeOffset)

	gpsTime := rtcm.GetUTCFromGPSTime(millis1)

	dateUTC := rtcm.GetUTCFromGalileoTime(millis1)

	if dateUTC.Location().String() != locationUTC.String() {
		t.Errorf("expected location to be UTC, got %s", dateUTC.Location().String())
	}
	if !expectedTime.Equal(dateUTC) {
		t.Errorf("expected %s result %s\n",
			expectedTime.Format(dateLayout), dateUTC.Format(dateLayout))
	}
	if !gpsTime.Equal(dateUTC) {
		t.Errorf("expected %s result %s\n",
			gpsTime.Format(dateLayout), dateUTC.Format(dateLayout))
	}
}

// TestTestGetUTCFromBeidouTime tests TestGetUTCFromBeidouTime
func TestGetUTCFromBeidouTime(t *testing.T) {

	// Set the start time to the start of a Beidou epoch.
	startTime1 :=
		time.Date(2020, time.August, 9, 0, 0, beidouLeapSeconds, 0, locationUTC)
	rtcm1 := New(startTime1, logger)

	expectedTime1 :=
		time.Date(2020, time.August, 9, 0, 0, 0, 0, locationUTC).Add(beidouTimeOffset)

	dateUTC1 := rtcm1.GetUTCFromBeidouTime(0)

	if dateUTC1.Location().String() != locationUTC.String() {
		t.Errorf("expected location to be UTC, got %s", dateUTC1.Location().String())
	}
	if !expectedTime1.Equal(dateUTC1) {
		t.Errorf("expected %s result %s\n",
			expectedTime1.Format(dateLayout), dateUTC1.Format(dateLayout))
	}

	dateUTC2 := rtcm1.GetUTCFromBeidouTime(maxEpochTime)
	expectedTime2 :=
		time.Date(2020, time.August, 16, 1, 59, 59, 999000000, paris).Add(beidouTimeOffset)

	if dateUTC2.Location().String() != locationUTC.String() {
		t.Errorf("expected location to be UTC, got %s", dateUTC2.Location().String())
	}
	if !expectedTime2.Equal(dateUTC2) {
		t.Errorf("expected %s result %s\n",
			expectedTime2.Format(dateLayout), dateUTC2.Format(dateLayout))
	}
}

// TestParseGlonassEpochTime tests ParseGlonassEpochTime
func TestParseGlonassEpochTime(t *testing.T) {
	// Maximum expected millis - twenty four hours of millis, less 1.
	const maxMillis = (24 * 3600 * 1000) - 1

	// Day = 0, millis = 0
	const expectedDay1 uint = 0
	const expectedMillis1 = 0
	const epochTime1 = 0

	day1, millis1 := ParseGlonassEpochTime(uint(epochTime1))

	if expectedDay1 != day1 {
		t.Errorf("expected day %d result %d", expectedDay1, day1)
	}
	if expectedMillis1 != millis1 {
		t.Errorf("expected millis %d result %d", maxMillis, millis1)
	}

	// Day = 0, millis = max
	const expectedDay2 uint = 0
	const epochTime2 = maxMillis

	day2, millis2 := ParseGlonassEpochTime(uint(epochTime2))

	if expectedDay2 != day2 {
		t.Errorf("expected day %d result %d", expectedDay2, day2)
	}
	if maxMillis != millis2 {
		t.Errorf("expected millis %d result %d", maxMillis, millis2)

	}

	// Day = max, millis = 0
	const expectedDay3 uint = 6
	const expectedMillis3 uint = 0
	const epochTime3 = 6 << 27

	day3, millis3 := ParseGlonassEpochTime(uint(epochTime3))

	if expectedDay3 != day3 {
		t.Errorf("expected day %d result %d", expectedDay3, day3)
	}
	if expectedMillis3 != millis3 {
		t.Errorf("expected millis %d result %d", expectedMillis3, millis3)
	}

	// Day = max, mills = max..
	const expectedDay4 uint = 6

	const epochTime4 = 6<<27 | uint(maxMillis)

	day4, millis4 := ParseGlonassEpochTime(uint(epochTime4))

	if expectedDay4 != day4 {
		t.Errorf("expected day %d result %d", expectedDay4, day4)
	}
	if maxMillis != millis4 {
		t.Errorf("expected millis %d result %d", maxMillis, millis4)
	}

	// Thess values can't actually happen in a Glonass epoch - the day can only
	// be up to 6 and the millis only run up to 24hours minus 1 milli.  However.
	// we'll test the logic anyway.
	const expectedDay5 uint = 7
	const expectedMillis5 uint = 0x7ffffff

	const epochTime5 uint = 0x3fffffff // 11 1111 1111 ... (30 bits).

	day5, millis5 := ParseGlonassEpochTime(uint(epochTime5))

	if expectedDay5 != day5 {
		t.Errorf("expected day %d result %d", expectedDay5, day5)
	}
	if expectedMillis5 != millis5 {
		t.Errorf("expected millis %x result %x", expectedMillis5, millis5)
	}
}

// TestMSM4 checks the MSM4 function.
func TestMSM4(t *testing.T) {

	var testData = []struct {
		messageType int
		want        bool
	}{
		{NonRTCMMessage, false},
		{1073, false},
		{1074, true},
		{1075, false},
		{1084, true},
		{1094, true},
		{1104, true},
		{1114, true},
		{1124, true},
		{1133, false},
		{1134, true},
		{1135, false},
	}
	for _, td := range testData {
		message := RTCM3Message{MessageType: td.messageType}
		got := message.MSM4()
		if got != td.want {
			t.Errorf("%d: want %v, got %v", td.messageType, td.want, got)
		}
	}
}

// TestMSM7 checks the MSM7 function.
func TestMSM7(t *testing.T) {
	var testData = []struct {
		messageType int
		want        bool
	}{
		{NonRTCMMessage, false},
		{1076, false},
		{1077, true},
		{1078, false},
		{1087, true},
		{1094, false},
		{1097, true},
		{1104, false},
		{1127, true},
		{1136, false},
		{1137, true},
		{1138, false},
	}
	for _, td := range testData {
		message := RTCM3Message{MessageType: td.messageType}
		got := message.MSM7()
		if got != td.want {
			t.Errorf("%d: want %v, got %v", td.messageType, td.want, got)
		}
	}
}

// TestMSM checks the MSM function.
func TestMSM(t *testing.T) {
	var testData = []struct {
		messageType int
		want        bool
	}{
		{NonRTCMMessage, false},
		{1076, false},
		{1074, true},
		{1077, true},
		{1107, true},
		{1116, false},
		{1117, true},
		{1118, false},
		{1127, true},
		{1134, true},
		{1137, true},
		{1136, false},
		{1137, true},
		{1138, false},
	}
	for _, td := range testData {
		message := RTCM3Message{MessageType: td.messageType}
		got := message.MSM()
		if got != td.want {
			t.Errorf("%d: want %v, got %v", td.messageType, td.want, got)
		}
	}
}

// TestMSM checks the displayable function.
func TestDispayable(t *testing.T) {
	var testData = []struct {
		messageType int
		want        bool
	}{
		{NonRTCMMessage, false},
		{1005, true},
		{1076, false},
		{1074, true},
		{1077, true},
		{1107, true},
		{1116, false},
		{1117, true},
		{1118, false},
		{1127, true},
		{1134, true},
		{1137, true},
		{1136, false},
		{1137, true},
		{1138, false},
	}
	for _, td := range testData {
		message := RTCM3Message{MessageType: td.messageType}
		got := message.displayable()
		if got != td.want {
			t.Errorf("%d: want %v, got %v", td.messageType, td.want, got)
		}
	}
}

// TestPrepareForDisplayWithInvalidMessage checks that PrepareforDisplay
// handles an invalid message correctly.
func TestPrepareForDisplayWithInvalidMessage(t *testing.T) {
	// PrepareForDisplay checks that the message hasn't already been
	// analysed.  If not, it calls Analyse.  If that returns an error,
	// it marks the message as invalid.  Force Analyse to fail using
	// an incomplete bit stream.

	shortBitStream := testData[:16]
	startTime := time.Date(2020, time.November, 13, 0, 0, 0, 0, locationUTC)
	rtcm := New(startTime, logger)
	rtcm.StopOnEOF = true

	message := RTCM3Message{MessageType: 1077, RawData: shortBitStream, Valid: true}

	message.PrepareForDisplay(rtcm)

	if message.Valid {
		t.Error("want the message to be marked as invalid")
	}
}

// TestSetDisplayWriter checks SetDisplayWriter
func TestSetDisplayWriter(t *testing.T) {
	startTime := time.Date(2020, time.November, 13, 0, 0, 0, 0, locationUTC)
	handler := New(startTime, logger)
	handler.SetDisplayWriter(logger.Writer())

	if handler.displayWriter != logger.Writer() {
		t.Error("SetDisplayWriter failed to set the writer")
	}
}
