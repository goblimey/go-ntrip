package rtcm

import (
	"bufio"
	"bytes"
	"io"
	"log"

	"os"
	"testing"
	"time"

	"github.com/goblimey/go-tools/switchwriter"
)

// maxEpochTime is the value of a GPS and Beidou epoch time
// just before it rolls over.
const maxEpochTime uint = (7 * 24 * 3600 * 1000) - 1

// testDelta3 is the delta value used to test floating point
// values for equality to three decimal places.
const testDelta3 = 0.001

// testDelta5 is the delta value used to test floating point
// values for equality to five decimal places.
const testDelta5 = 0.00001

var london *time.Location
var paris *time.Location

var logger *log.Logger

func init() {

	london, _ = time.LoadLocation("Europe/London")
	paris, _ = time.LoadLocation("Europe/Paris")
	writer := switchwriter.New()
	logger = log.New(writer, "rtcm_test", 0)
}

func TestReadValidMessage(t *testing.T) {
	d := [...]byte{
		// Message type   --------- 0x449 - 1097.
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
	const wantType = 1097

	validMessage := d[:]
	r := bytes.NewReader(validMessage)
	reader := bufio.NewReader(r)

	now := time.Now()
	handler := New(now, logger)
	handler.StopOnEOF = true

	frame1, readError1 := handler.ReadNextFrame(reader)
	if readError1 != nil {
		t.Fatal(readError1)
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

func Test(t *testing.T) {

	const wantType = 1077

	f, openError := os.Open("/home/simon/goprojects/go-ntrip/rtcmfilter/data.2022-03-17.rtcm3")
	if openError != nil {
		t.Error(openError)
	}

	reader := bufio.NewReader(f)

	now := time.Now()
	handler := New(now, logger)
	handler.StopOnEOF = true

	frame1, readError1 := handler.ReadNextFrame(reader)
	if readError1 != nil {
		t.Fatal(readError1)
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

	channels := make([]chan Message, 0)
	ch := make(chan Message, 10)
	channels = append(channels, ch)
	rtcmHandler := New(time.Now(), nil)
	rtcmHandler.StopOnEOF = true

	// Test
	rtcmHandler.HandleMessages(reader, channels)
	// Close the channel so that a channel reader knows when it's finished.
	close(ch)

	// Check.  Read the data back from the channel and check the message type
	// and validity flags.
	messages := make([]Message, 0)
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
	message0, err0 := rtcmHandler.ReadNextMessage(resultReader0)
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
	message1, err1 := rtcmHandler.ReadNextMessage(resultReader1)
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
	frame1, readError1 := rtcm.ReadNextFrame(imReader)
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
	frame2, readError2 := rtcm.ReadNextFrame(imReader)
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
	d := [...]byte{
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

	var data []byte = d[:]

	r := bytes.NewReader(data)
	imReader := bufio.NewReader(r)

	startTime := time.Date(2020, time.November, 13, 0, 0, 0, 0, locationUTC)
	rtcm := New(startTime, logger)
	rtcm.StopOnEOF = true

	// The first call should read the incomplete message, hit
	// EOF and ignore it.
	frame1, readError1 := rtcm.ReadNextFrame(imReader)
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

	frame, err1 := rtcm.ReadNextFrame(junkAtStartReader)
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

	frame, err1 := rtcm.ReadNextFrame(junkReader)

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

	frame2, err2 := rtcm.ReadNextFrame(junkReader)

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

//TestDisplayMSM4 checks that MSM type 4 messages are handled correctly.
func TestReadMSM4(t *testing.T) {
	r := bytes.NewReader(msm4Data)
	msm4Reader := bufio.NewReader(r)
	// This test uses real data collected on the 17th June 2022.
	startTime := time.Date(2022, time.June, 17, 0, 0, 0, 0, locationUTC)
	rtcm := New(startTime, logger)
	rtcm.StopOnEOF = true

	frame, err1 := rtcm.ReadNextFrame(msm4Reader)

	if err1 != nil {
		t.Error(err1.Error())
		return
	}

	message, messageFetchError := rtcm.GetMessage(frame)
	if messageFetchError != nil {
		t.Errorf("error getting message - %v", messageFetchError)
		return
	}

	if message.MessageType != 1124 {
		t.Errorf("expected message type 1124 got %d", message.MessageType)
		return
	}

	// Get the message in display form.
	display, ok := message.Readable(rtcm).(*MSMMessage)
	if !ok {
		t.Error("expected the readable message to be *MSMMessage\n")
		return
	}

	if len(display.Satellites) != 7 {
		t.Errorf("expected 7 satellites, got %d", len(display.Satellites))
	}

	_ = display.Satellites[0].RangeWholeMillis

}

// TestDecode1230GlonassCodeBias checks that a message of type 1230
// (Glonass code bias) with two bias values is decoded correctly.
// The test value is taken from real data.  It has an L1 CA bias
// value of zero and an L2 CA bias value of zero but no l1 P bias
// value or L2 P bias value.  Experience shows that these messages
// are common in my location.
//
func TestDecode1230GlonassCodeBias(t *testing.T) {
	const wantType = 1230
	r := bytes.NewReader(codeBias)
	codeBiasReader := bufio.NewReader(r)
	startTime := time.Date(2020, time.November, 13, 0, 0, 0, 0, locationUTC)
	rtcmHandler := New(startTime, logger)
	rtcmHandler.StopOnEOF = true

	frame, err1 := rtcmHandler.ReadNextFrame(codeBiasReader)

	if err1 != nil {
		t.Fatal(err1.Error())
	}

	message, messageFetchError := rtcmHandler.GetMessage(frame)
	if messageFetchError != nil {
		t.Errorf("error getting message - %v", messageFetchError)
		return
	}

	gotType := message.MessageType
	if wantType != gotType {
		t.Errorf("expected type %d, got %d", wantType, gotType)
		return
	}

	rtcmHandler.Analyse(message)

	// The message should be a Message1230.
	m := message.Readable(rtcmHandler).(*Message1230)

	// All code biases should be valid and 0.
	if !m.L1_C_A_Bias_valid {
		t.Error("L1_C_A_Bias should be valid")

	}
	if !m.L1_P_Bias_valid {
		t.Error("L1_P_Bias should be valid")

	}
	if !m.L2_C_A_Bias_valid {
		t.Error("L2_C_A_Bias should be valid")

	}
	if !m.L2_P_Bias_valid {
		t.Error("L2_P_Bias should be valid")

	}

	if m.L1_C_A_Bias != 0 {
		t.Errorf("want L1_C_A_Bias to be 0, got %d", m.L1_C_A_Bias)

	}

	if m.L1_P_Bias != 0 {
		t.Errorf("want L1_P_Bias to be 0, got %d", m.L1_P_Bias)

	}

	if m.L2_C_A_Bias != 0 {
		t.Errorf("want L2_C_A_Bias to be 0, got %d", m.L2_C_A_Bias)

	}

	if m.L2_P_Bias != 0 {
		t.Errorf("want L2_P_Bias to be 0, got %d", m.L2_P_Bias)

	}
}

func TestReadNextMessageFrame(t *testing.T) {
	r := bytes.NewReader(realData)
	realDataReader := bufio.NewReader(r)
	startTime := time.Date(2020, time.November, 13, 0, 0, 0, 0, locationUTC)
	rtcmHandler := New(startTime, logger)
	rtcmHandler.StopOnEOF = true

	frame, err1 := rtcmHandler.ReadNextFrame(realDataReader)
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

// TestRealData reads the first message from the real data and checks it in detail.
func TestRealData(t *testing.T) {
	r := bytes.NewReader(realData)
	realDataReader := bufio.NewReader(r)
	startTime := time.Date(2020, time.November, 13, 0, 0, 0, 0, locationUTC)
	rtcm := New(startTime, logger)
	rtcm.StopOnEOF = true

	m, readError := rtcm.ReadNextMessage(realDataReader)

	if readError != nil {
		t.Errorf("error reading data - %s", readError.Error())
		return
	}

	if m.MessageType != 1077 {
		t.Errorf("expected message type 1007, got %d", m.MessageType)
		return
	}

	message, ok := m.Readable(rtcm).(*MSMMessage)

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

	if message.Satellites[0].RangeWholeMillis != 81 {
		t.Errorf("expected range whole  of 81, got %d",
			message.Satellites[0].RangeWholeMillis)
		return
	}

	if message.Satellites[0].RangeFractionalMillis != 435 {
		t.Errorf("expected range fractional 435, got %d",
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
	for i, _ := range message.Signals {
		numSignals1 += len(message.Signals[i])
	}

	if numSignals1 != 14 {
		t.Errorf("expected 14 GPS signals, got %d", numSignals1)
		return
	}

	// A signal cell contains a Satellite which is an index into the Satellite array.
	// The satellite has an ID.
	if message.Satellites[message.Signals[0][0].Satellite].SatelliteID != 4 {
		t.Errorf("expected satelliteID 4, got %d",
			message.Satellites[message.Signals[0][0].Satellite].SatelliteID)
		return
	}

	if message.Signals[0][0].RangeDelta != -26835 {
		t.Errorf("expected range delta -26835, got %d",
			message.Signals[0][0].RangeDelta)
		return
	}

	// Checking the resulting range in metres against the value
	// in the RINEX data produced from this message.

	if !floatsEqualWithin3(24410527.355, message.Signals[0][0].RangeMetres) {
		t.Errorf("expected range 24410527.355 metres, got %3.6f",
			message.Signals[0][0].RangeMetres)
		return
	}
}

func TestGetbitu(t *testing.T) {
	i := Getbitu(testData, 8, 0)
	if i != 0 {
		t.Errorf("expected 0, got 0x%x", i)
	}

	i = Getbitu(testData, 16, 4)
	if i != 8 {
		t.Errorf("expected 8, got 0x%x", i)
	}

	i = Getbitu(testData, 16, 8)
	if i != 0x8a {
		t.Errorf("expected 0x8a, got 0x%x", i)
	}

	i = Getbitu(testData, 16, 16)
	if i != 0x8a43 {
		t.Errorf("expected 0x8a43, got 0x%x", i)
	}

	// try a full 64-byte number
	i = Getbitu(testData, 12, 64)
	if i != 0x08a4320008a0e1a2 {
		t.Errorf("expected 0x08a4320008a0e1a2, got 0x%x", i)
	}
}

func TestGetbits(t *testing.T) {
	var b1 = [...]byte{
		0x7f, 0x00, 0xff, 0x00, 0xff, 0x00, 0xff, 0x00,
	}
	var testdata1 []byte = b1[:]

	i := Getbits(testdata1, 0, 64)
	if i != 0x7f00ff00ff00ff00 {
		t.Errorf("expected 0x7f00ff00ff00ff00, got 0x%x", i)
	}

	var b2 = [...]byte{
		0x7f, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
	}
	var testdata2 []byte = b2[:]

	i = Getbits(testdata2, 0, 64)
	if i != 0x7fffffffffffffff {
		t.Errorf("expected 0x7fffffffffffffff, got 0x%x", i)
	}

	var b3 = [...]byte{0xfb /* 1111 1011 */}
	var testdata3 []byte = b3[:]

	i = Getbits(testdata3, 0, 8)
	if i != -5 {
		t.Errorf("expected -5, got %d, 0x%x", i, i)
	}

	var b4 = [...]byte{0xff, 0xff}
	var testdata4 []byte = b4[:]

	i = Getbits(testdata4, 0, 16)
	if i != -1 {
		t.Errorf("expected -1, got %d, 0x%x", i, i)
	}

	var b5 = [...]byte{
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
	}
	var testdata5 []byte = b5[:]

	i = Getbits(testdata5, 0, 64)
	if i != -1 {
		t.Errorf("expected -1, got %d, 0x%x", i, i)
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

// TestGetSatellites tests GetSatellites.
func TestGetSatellites(t *testing.T) {
	// Set the bitstream to "junk" followed by a 64-bit
	// mask with bit 63 (sat 1) 55 (sat 9) and 0 (sat 64)
	var bitstream = []byte{'j', 'u', 'n', 'k', 0x80, 0x80, 0, 0, 0, 0, 0, 1}
	var expectedSatellites = []uint{1, 9, 64}
	satellites := GetSatellites(bitstream, 32)

	if !slicesEqual(expectedSatellites, satellites) {
		t.Errorf("expected %v, got %v\n",
			expectedSatellites, satellites)
		return
	}
}

// TestGetSignals tests GetSignals.
func TestGetSignals(t *testing.T) {
	// Set the bitstream to "junk" followed by a 32-bit
	// mask with bit 31 (sat 1) 23 (sat 9) and 0 (sat 32).
	var bitstream = []byte{'j', 'u', 'n', 'k', 0x80, 0x80, 0, 1}
	var expectedSignals = []uint{1, 9, 32}
	signals := GetSignals(bitstream, 32)

	if !slicesEqual(expectedSignals, signals) {
		t.Errorf("expected %v, got %v\n",
			expectedSignals, signals)
		return
	}
}

// TestScaled5ToFloat tests Scaled5ToFloat with positive and negative values.
func TestScaled5ToFloat(t *testing.T) {
	var scaled5 int64 = 9812345
	fPos := Scaled5ToFloat(scaled5)
	if !floatsEqualWithin5(981.2345, fPos) {
		t.Errorf("expected 981.2345, got %f\n", fPos)
	}

	fZero := Scaled5ToFloat(0)
	if !floatsEqualWithin5(0.0, fZero) {
		t.Errorf("expected 0.0, got %f\n", fZero)
	}

	fNeg := Scaled5ToFloat(-7654321)
	if !floatsEqualWithin5(-765.4321, fNeg) {
		t.Errorf("expected -765.4321, got %f\n", fNeg)
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

// TestGetrangeMSM7 checks that getMSMRangeInMetres works
// for an MSM7 message.
func TestGetRangeMSM7(t *testing.T) {
	const expectedRange float64 = (128.5 + P2_11) * oneLightMillisecond // 38523477.236036
	var rangeMillisWhole uint = 0x80                                    // 1000 0000
	var rangeMillisFractional uint = 0x200                              // 10 bits 1000 ...
	rangeDelta := int(0x40000)                                          // 20 bits 0100 ...

	header := MSMHeader{MessageType: 1077}

	satellite := MSMSatelliteCell{
		RangeValid:            true,
		RangeWholeMillis:      rangeMillisWhole,
		RangeFractionalMillis: rangeMillisFractional}

	signal := MSMSignalCell{
		RangeDeltaValid: true,
		RangeDelta:      rangeDelta}

	rangeM1 := getMSMRangeInMetres(&header, &satellite, &signal)

	if !floatsEqualWithin3(expectedRange, rangeM1) {
		t.Errorf("expected %f got %f", expectedRange, rangeM1)
		return
	}

	// Test values from real data.

	const expectedRange2 = 24410527.355

	satellite2 := MSMSatelliteCell{
		RangeValid:            true,
		RangeWholeMillis:      81,
		RangeFractionalMillis: 435}

	signal2 := MSMSignalCell{
		RangeDeltaValid: true,
		RangeDelta:      -26835}

	rangeM2 := getMSMRangeInMetres(&header, &satellite2, &signal2)

	if !floatsEqualWithin3(expectedRange2, rangeM2) {
		t.Errorf("expected %f got %f", expectedRange2, rangeM2)
		return
	}
}

func TestGetPhaseRangeGPS(t *testing.T) {
	wavelength := CLight / freq2
	range1 := (128.5 + P2_9)
	range1M := range1 * oneLightMillisecond
	expectedPhaseRange1 := range1M / wavelength
	var signalID uint = 16
	var rangeMillisWhole uint = 0x80       // 1000 0000
	var rangeMillisFractional uint = 0x200 // 10 0000 0000
	var phaseRangeDelta int = 0x400000     // 24 bits 01000 ...

	header := MSMHeader{
		MessageType:   1077,
		Constellation: "GPS",
	}

	satellite1 := MSMSatelliteCell{
		RangeValid:            true,
		RangeWholeMillis:      rangeMillisWhole,
		RangeFractionalMillis: rangeMillisFractional}

	signal1 := MSMSignalCell{
		PhaseRangeDeltaValid: true,
		SignalID:             signalID,
		PhaseRangeDelta:      phaseRangeDelta,
	}

	rangeCycles1, err := getMSMPhaseRange(&header, &satellite1, &signal1)

	if err != nil {
		t.Errorf(err.Error())
		return
	}

	if !floatsEqualWithin3(expectedPhaseRange1, rangeCycles1) {
		t.Errorf("expected %f got %f", expectedPhaseRange1, rangeCycles1)
		return
	}

	// Test using real data.

	const expectedPhaseRange2 = 128278179.264

	satellite2 := MSMSatelliteCell{
		RangeValid:            true,
		RangeWholeMillis:      81,
		RangeFractionalMillis: 435}

	signal2 := MSMSignalCell{
		PhaseRangeDeltaValid: true,
		SignalID:             2,
		PhaseRangeDelta:      -117960}

	rangeCycles2, err := getMSMPhaseRange(&header, &satellite2, &signal2)

	if err != nil {
		t.Errorf(err.Error())
		return
	}

	if !floatsEqualWithin3(expectedPhaseRange2, rangeCycles2) {
		t.Errorf("expected %f got %f", expectedPhaseRange2, rangeCycles2)
		return
	}
}

func TestGetPhaseRangeRate(t *testing.T) {
	// wavelength := CLight / freq2
	// range1 := (128.5 + P2_9)
	// range1M := range1 * oneLightMillisecond
	// expectedPhaseRangeRate1 := range1M / wavelength
	// var signalID uint = 16
	// var rangeMillisWhole uint = 0x80       // 1000 0000
	// var rangeMillisFractional uint = 0x200 // 10 0000 0000
	// var phaseRangeDelta int = 0x400000     // 24 bits 01000 ...

	const expectedPhaseRangeRate2 = float64(709.992)

	header2 := MSMHeader{Constellation: "GPS"}
	satellite2 := MSMSatelliteCell{
		PhaseRangeRate: -135}

	signal2 := MSMSignalCell{
		SignalID:            2,
		PhaseRangeRateDelta: -1070}

	rate2, err := getMSMPhaseRangeRate(&header2, &satellite2, &signal2)
	if err != nil {
		t.Errorf(err.Error())
		return
	}

	if floatsEqualWithin3(expectedPhaseRangeRate2, rate2) {
		t.Errorf("expected %f got %f", expectedPhaseRangeRate2, rate2)
		return
	}
}

// TestHandleMessages tests the verbatim logging of messages.

// slicesEqual returns true if uint slices a and b contain the same
// elements.  A nil argument is equivalent to an empty slice.
// https://yourbasic.org/golang/compare-slices/
func slicesEqual(a, b []uint) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

// floatsEqualWithin3 returns true if two float values are equal
// within 3 decimal places.
func floatsEqualWithin3(f1, f2 float64) bool {
	if f1 > f2 {
		return (f1 - f2) < testDelta3
	}

	return (f2 - f1) < testDelta3
}

// floatsEqualWithin5 returns true if two float values are equal
// within 5 decimal places.
func floatsEqualWithin5(f1, f2 float64) bool {
	if f1 > f2 {
		return (f1 - f2) < testDelta5
	}

	return (f2 - f1) < testDelta5
}
