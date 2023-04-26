package handler

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"math"
	"testing"
	"time"

	"github.com/goblimey/go-crc24q/crc24q"
	"github.com/goblimey/go-ntrip/rtcm/header"
	"github.com/goblimey/go-ntrip/rtcm/pushback"
	"github.com/goblimey/go-ntrip/rtcm/testdata"
	"github.com/goblimey/go-ntrip/rtcm/type1005"
	msm4message "github.com/goblimey/go-ntrip/rtcm/type_msm4/message"
	msm4satellite "github.com/goblimey/go-ntrip/rtcm/type_msm4/satellite"
	msm4signal "github.com/goblimey/go-ntrip/rtcm/type_msm4/signal"
	msm7message "github.com/goblimey/go-ntrip/rtcm/type_msm7/message"
	"github.com/goblimey/go-ntrip/rtcm/utils"

	"github.com/goblimey/go-tools/switchwriter"

	"github.com/kylelemons/godebug/diff"
)

// This "test" is used to calculate the CRC when hand-crafting a bit stream.
// Run te test in a debugger and examine the value of crc.
func Test(t *testing.T) {

	var bitStream = []byte{
		0xd3, 0x00, 0x08,
		0x4c, 0xe0, 00, 0x8a, 0, 0, 0, 0,
	}

	crc := crc24q.Hash(bitStream)

	_ = crc
}

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

// maxEpochTime is the value of a GPS, Galileo and Beidou
// timestamp at the end of the week, just before it rolls over.
const maxEpochTime uint = (7 * 24 * 3600 * 1000) - 1

var logger *log.Logger

func init() {
	writer := switchwriter.New()
	logger = log.New(writer, "rtcm_test", 0)
}

// TestHandleMessagesFromChannel tests the HandleMessagesFromChannel method.
func TestHandleMessagesFromChannel(t *testing.T) {
	// HandleMessagesFromChannel consumes the data on the given input channel and sends
	// any valid messages to the given output channel.  Bytes 0-225 of the test data
	// contain one valid message of type 1077 and bytes 226 onwards contain non-RTCM
	// data, so the channel should contain a message of type 1077 and a message of
	// type utils.NonRTCMMessage.

	const wantNumMessages = 2
	const wantType0 = 1077
	const wantLength0 = 226
	var wantContents0 = testdata.MesageBatchWith1077[:226]
	const wantType1 = utils.NonRTCMMessage
	const wantLength1 = 9
	var wantContents1 = testdata.MesageBatchWith1077[226:]

	reader := bytes.NewReader(testdata.MesageBatchWith1077)

	// Create a buffered channel big enough to hold the test data, send the
	// data to it and close it.
	ch_source := make(chan byte, 10000)
	for {
		buf := make([]byte, 1)
		n, err := reader.Read(buf)
		if err != nil {
			// We've read all the test data.  Done.
			close(ch_source)
			break
		}

		if n > 0 {
			ch_source <- buf[0]
		}
	}

	// Expect the resulting messages on this channel.
	ch_result := make(chan Message, 10)

	rtcmHandler := New(time.Now())

	// Test
	rtcmHandler.HandleMessagesFromChannel(ch_source, ch_result)

	// Check.  Read the data back from the channel and check the message type.
	messages := make([]Message, 0)
	for {
		message, ok := <-ch_result
		if !ok {
			// Done - chan is drained.
			break
		}
		messages = append(messages, message)
	}

	if len(messages) != wantNumMessages {
		t.Errorf("want %d message, got %d messages", wantNumMessages, len(messages))
	}

	if wantType0 != messages[0].MessageType {
		t.Errorf("want message type %d, got message type %d", wantType0, messages[0].MessageType)
	}

	if messages[0].RawData == nil {
		t.Errorf("raw data in message 0 is nil")
		return
	}

	if wantLength0 != len(messages[0].RawData) {
		t.Errorf("want message length %d got %d", wantLength0, len(messages[0].RawData))
	}

	if !bytes.Equal(wantContents0, messages[0].RawData) {
		t.Error("contents of message 0 is not correct")
	}

	if wantType1 != messages[1].MessageType {
		t.Errorf("want message type %d, got message type %d", wantType1, messages[1].MessageType)
	}

	if messages[1].RawData == nil {
		t.Errorf("raw data in message 1 is nil")
		return
	}

	if wantLength1 != len(messages[1].RawData) {
		t.Errorf("want message length %d got %d", wantLength1, len(messages[1].RawData))
	}

	if !bytes.Equal(wantContents1, messages[1].RawData) {
		t.Error("contents of message 1 is not correct")
	}
}

// TestFetchNextMessageFrame checks that FetchNextMessageFrame correctly
// reads a message frame.
func TestFetchNextMessageFrame(t *testing.T) {

	var testData = []struct {
		description     string
		bitStream       []byte
		wantMessageType int
		wantError       string
	}{
		// provokes an error while scanning the message length (phase 2)
		{"very short", testdata.MessageFrameType1077[:4], utils.NonRTCMMessage, ""},
		// provokes an error while scanning the rest of the message (phase 3)
		{"incomplete", testdata.IncompleteMessage, utils.NonRTCMMessage, ""},
		// The message length must be no more than 1024 bytes.
		{"Length too big", testdata.MessageFrameWithLengthTooBig, utils.NonRTCMMessage, ""},
		{"bad CRC", testdata.MessageFrameWithCRCFailure, utils.NonRTCMMessage, "CRC is not valid"},
		{"junk followed by RTCM", testdata.JunkAtStart, utils.NonRTCMMessage, ""},
		{"all junk", testdata.AllJunk, utils.NonRTCMMessage, ""},
		{"1077", testdata.MessageFrameType1077, utils.MessageTypeMSM7GPS, ""},
		{"1024", testdata.UnhandledMessageType1024, 1024, ""},
	}

	for _, td := range testData {

		// Create a ByteChannel containing the data from the bit stream.
		ch := make(chan byte, 10000)
		for _, b := range td.bitStream {
			ch <- b
		}
		bc := pushback.New(ch)
		bc.Close()

		// Tuesday 29/8/23.
		startDate := time.Date(2023, time.August, 29, 00, 00, 00, 0, utils.LocationUTC)
		handler := New(startDate)

		gotMessage, gotError := handler.FetchNextMessageFrame(bc)

		// We expect a message even when there is an error.
		if td.wantMessageType != gotMessage.MessageType {
			t.Errorf("%s: want %d got %d", td.description, td.wantMessageType, gotMessage.MessageType)
		}

		if td.wantError != "" {
			if gotError == nil {
				t.Error("want an error")
				continue
			}
			if td.wantError != gotError.Error() {
				t.Errorf("%s: want %s got %s", td.description, td.wantError, gotError.Error())
			}
		}

	}
}

// TestFetchNextMessageFrameWithNilOrEmptyFrame checks that FetchNextMessageFrame
// correctly handles the case when the channel is nil or empty - we get an error
// but no message.
func TestFetchNextMessageFrameWithNilOrEmptyFrame(t *testing.T) {

	var testData = []struct {
		description     string
		bitStream       []byte
		wantMessageType int
		wantError       string
	}{
		{"nil frame", nil, utils.MessageTypeMSM4QZSS, "done"},
		{"zero length", testdata.EmptyFrame, utils.MessageTypeGCPB, "done"},
	}

	for _, td := range testData {

		// Create a ByteChannel containing the data from the bit stream.
		ch := make(chan byte, 10000)
		for _, b := range td.bitStream {
			ch <- b
		}
		bc := pushback.New(ch)
		bc.Close()

		// Tuesday 29/8/23.
		startDate := time.Date(2023, time.August, 29, 00, 00, 00, 0, utils.LocationUTC)
		handler := New(startDate)

		gotMessage, gotError := handler.FetchNextMessageFrame(bc)

		if gotMessage != nil {
			t.Error("expected a nil message pointer")
		}

		if td.wantError != "" {
			if gotError == nil {
				t.Error("want an error")
				continue
			}
			if td.wantError != gotError.Error() {
				t.Errorf("%s: want %s got %s", td.description, td.wantError, gotError.Error())
			}
		}
	}

}

// TestGetMessageLengthAndType checks GetMessageLengthAndType
func TestGetMessageLengthAndType(t *testing.T) {

	var testData = []struct {
		description       string
		bitStream         []byte
		wantMessageType   int
		wantMessageLength uint
		wantError         string
	}{
		{"valid", validMessageFrame, 1097, 170, ""},
		{"short", validMessageFrame[:4], utils.NonRTCMMessage, 0,
			"the message is too short to get the header and the length"},
		{"incorrect start", testdata.MessageFrameWithIncorrectStart, utils.NonRTCMMessage, 0,
			"message starts with 0xff not 0xd3"},
		{"zero length", testdata.MessageFrameWithLengthZero, 1097, 0,
			"zero length message, type 1097"},
		{"invalid", testdata.MessageFrameWithLengthTooBig, utils.NonRTCMMessage, 0,
			"bits 8-13 of header are 63, must be 0"},
	}
	for _, td := range testData {
		handler := New(time.Now())
		gotMessageLength, gotMessageType, gotError := handler.getMessageLengthAndType(td.bitStream)
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

// TestGetMessage checks GetMessage with valid messages.
func TestGetMessage(t *testing.T) {
	var testData = []struct {
		description string
		bitStream   []byte
		want        int
	}{
		{"junk", testdata.AllJunk, utils.NonRTCMMessage},
		{"1230", testdata.Fake1230, utils.MessageTypeGCPB},
		{"1074", testdata.MessageBatch, utils.MessageTypeMSM4GPS},
		{"1005", testdata.MessageFrameType1005, utils.MessageType1005},
	}
	for _, td := range testData {
		startTime := time.Date(2020, time.December, 9, 0, 0, 0, 0, utils.LocationUTC)
		handler := New(startTime)

		got, messageFetchError := handler.GetMessage(td.bitStream)
		if messageFetchError != nil {
			t.Errorf("%s: error getting message - %v", td.description, messageFetchError)
			return
		}

		if td.want != got.MessageType {
			t.Errorf("%s: expected message type %d, got %d",
				td.description, td.want, got.MessageType)
			return
		}
	}
}

var FrameWithIllegalGlonassDay = []byte{
	0xd3, 0x00, 0x10,
	//       |      |   timestamp (1110 0000 ..)
	0x43, 0xc0, 0x00, 0xe0, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
}

// TestGetMessageWithErrors checks that GetMessage returns correct error messages.
func TestGetMessageWithErrors(t *testing.T) {

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
		startTime := time.Now()
		handler := New(startTime)
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
	// messageFrame1077NoTimestamp is a message frame with a valid CRC but the 1077
	// message that it contains is too short to contain a complete header.
	var messageFrame1077NoTimestamp = []byte{
		0xd3, 0x00, 0x8a,
		0x43, 0x20, 0x00, 0x8a, 0x0e, 0x1a, 0x26, 0x00, 0x00, 0x2f, 0x40, 0x00,
		0x4a, 0x0a, 0x0b,
		// CRC
		0, 0, 0,
	}

	// frameWithIllegalGlonassDay contains message type 1084 (MSM4 Glonass) but the
	// day in the timestamp is the illegal value 7.
	messageFrameWithIllegalGlonassDay := []byte{
		0xd3, 0x00, 0x10,
		//       |      |   timestamp (1110 0000 ..)
		0x43, 0xc0, 0x00, 0xe0, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x42, 0xf2, 0x8f,
		// CRC
		0, 0, 0,
	}

	// messageFrameWithLengthZero is a valid message frame but the length of the
	// contained message is zero.
	messageFrameWithLengthZero := []byte{0xd3, 0x00, 0x00, 0x44}

	var testData = []struct {
		description string
		frame       []byte
		wantError   string
		wantWarning string
	}{

		{"illegal", messageFrameWithIllegalGlonassDay,
			"invalid day", "invalid day"},

		// GetMessage should return a nil message and an error.
		{"very short", messageFrame1077NoTimestamp,
			"incomplete message frame", ""},

		// GetMessage should return an error AND a message with the warning set.
		{"illegal", messageFrameWithIllegalGlonassDay,
			"invalid day", "invalid day"},

		// GetMessage calls GetMessageLengthAndType.  That returns an error because the
		// bit stream is too short.  GetMessage should return the error message AND a
		// byte slice containing the five bytes that it consumed, plus a warning.
		{"short", messageFrameWithLengthZero,
			"the message is too short to get the header and the length",
			"the message is too short to get the header and the length"},
	}
	for _, td := range testData {
		startTime := time.Now()
		handler := New(startTime)
		gotMessage, gotError := handler.GetMessage(td.frame)

		if len(td.wantError) > 0 {
			if gotError == nil {
				t.Errorf("%s: want an error got nil", td.description)
				return
			}

			if td.wantError != gotError.Error() {
				t.Errorf("%s: want error - %s, got %s", td.description, td.wantError, gotError.Error())
			}
		}

		if len(td.wantWarning) > 0 {
			if gotMessage == nil {
				t.Errorf("%s: want a message got nil", td.description)
				return
			}

			if gotMessage.RawData == nil {
				t.Errorf("%s: want some raw data, got nil", td.description)
				return
			}

			if td.wantWarning != gotMessage.ErrorMessage {
				t.Errorf("want warning - %s, got %s", td.wantWarning, gotMessage.ErrorMessage)
			}
		}
	}
}

// TestPrepareForDisplayWithRealData reads the first message from the real data,
// prepares it for display and checks the result in detail.
func TestPrepareForDisplayWithRealData(t *testing.T) {

	const wantRangeWholeMilliSecs = 81
	const wantRangeFractionalMilliSecs = 435
	// The data was produced by a real device and then converted to RINEX format.
	// These values were taken from the RINEX.
	const wantRange = 24410527.355

	r := bytes.NewReader(testdata.MessageBatchWithJunk)
	realDataReader := bufio.NewReader(r)

	// Create a buffered channel big enough to hold the test data, send the
	// data to it and close it.
	ch_source := make(chan byte, 10000)
	for {
		buf := make([]byte, 1)
		n, err := realDataReader.Read(buf)
		if err != nil {
			// We've read all the test data.  Done.
			close(ch_source)
			break
		}

		if n > 0 {
			ch_source <- buf[0]
		}
	}

	// Expect the resulting messages on this channel.
	ch_result := make(chan Message, 10)

	startTime := time.Date(2020, time.November, 13, 0, 0, 0, 0, utils.LocationUTC)
	rtcmHandler := New(startTime)

	// Test
	rtcmHandler.HandleMessagesFromChannel(ch_source, ch_result)

	// Check.  Read the first message back from the channel and check it.
	m, ok := <-ch_result
	if !ok {
		// Done - chan is drained.
		t.Error("expected a message")
	}

	if m.MessageType != 1077 {
		t.Errorf("expected message type 1077, got %d", m.MessageType)
		return
	}

	message, ok := PrepareForDisplay(&m).(*msm7message.Message)

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

// TestGPSEpochTimes tests that New sets up the correct start times
// for the GPS epochs.
func TestGPSEpochTimes(t *testing.T) {

	expectedEpochStart :=
		time.Date(2020, time.August, 1, 23, 59, 60-gpsLeapSeconds, 0, utils.LocationUTC)

	// Sunday 2020/08/02 BST, just after the start of the GPS epoch
	dateTime1 := time.Date(2020, time.August, 2, 1, 0, 0, (60 - gpsLeapSeconds), utils.LocationLondon)
	rtcm1 := New(dateTime1)
	if expectedEpochStart != rtcm1.startOfGPSWeek {
		t.Errorf("expected %s result %s\n",
			expectedEpochStart.Format(utils.DateLayout),
			rtcm1.startOfGPSWeek.Format(utils.DateLayout))
		return
	}

	// Wednesday 2020/08/05
	dateTime2 := time.Date(2020, time.August, 5, 12, 0, 0, 0, utils.LocationLondon)
	rtcm2 := New(dateTime2)
	if expectedEpochStart != rtcm2.startOfGPSWeek {
		t.Errorf("expected %s result %s\n",
			expectedEpochStart.Format(utils.DateLayout),
			rtcm2.startOfGPSWeek.Format(utils.DateLayout))
		return
	}

	// Sunday 2020/08/02 BST, just before the end of the GPS epoch
	dateTime3 := time.Date(2020, time.August, 9, 00, 59, 60-gpsLeapSeconds-1, 999999999, utils.LocationLondon)
	rtcm3 := New(dateTime3)
	if expectedEpochStart != rtcm3.startOfGPSWeek {
		t.Errorf("expected %s result %s\n",
			expectedEpochStart.Format(utils.DateLayout),
			rtcm3.startOfGPSWeek.Format(utils.DateLayout))
		return
	}

	// Sunday 2020/08/02 BST, at the start of the next GPS epoch.
	dateTime4 := time.Date(2020, time.August, 9, 1, 59, 60-gpsLeapSeconds, 0, utils.LocationParis)
	startOfNext := time.Date(2020, time.August, 8, 23, 59, 60-gpsLeapSeconds, 0, utils.LocationUTC)

	rtcm4 := New(dateTime4)
	if startOfNext != rtcm4.startOfGPSWeek {
		t.Errorf("expected %s result %s\n",
			startOfNext.Format(utils.DateLayout),
			rtcm4.startOfGPSWeek.Format(utils.DateLayout))
		return
	}
}

// TestBeidouEpochTimes checks that New sets the correct start times
// for this and the next Beidou epoch.
func TestBeidouEpochTimes(t *testing.T) {
	// Like GPS time, the Beidou time rolls over every seven days,
	// but it uses a different number of leap seconds.

	// The first few seconds of Sunday UTC are in the previous Beidou week.
	expectedStartOfPreviousWeek :=
		time.Date(2020, time.August, 2, 0, 0, beidouLeapSeconds, 0, utils.LocationUTC)
	expectedStartOfThisWeek :=
		time.Date(2020, time.August, 9, 0, 0, beidouLeapSeconds, 0, utils.LocationUTC)
	expectedStartOfNextWeek :=
		time.Date(2020, time.August, 16, 0, 0, beidouLeapSeconds, 0, utils.LocationUTC)

	// The 9th is Sunday.  This start time should be in the previous week ...
	startTime1 := time.Date(2020, time.August, 9, 0, 0, 0, 0, utils.LocationUTC)
	rtcm1 := New(startTime1)

	if !expectedStartOfPreviousWeek.Equal(rtcm1.startOfBeidouWeek) {
		t.Errorf("expected %s result %s\n",
			expectedStartOfPreviousWeek.Format(utils.DateLayout), rtcm1.startOfBeidouWeek.Format(utils.DateLayout))
	}

	// ... and so should this.
	startTime2 := time.Date(2020, time.August, 9, 0, 0, beidouLeapSeconds-1, 999999999, utils.LocationUTC)
	rtcm2 := New(startTime2)

	if !expectedStartOfPreviousWeek.Equal(rtcm2.startOfBeidouWeek) {
		t.Errorf("expected %s result %s\n",
			expectedStartOfPreviousWeek.Format(utils.DateLayout), rtcm2.startOfBeidouWeek.Format(utils.DateLayout))
	}

	// This start time should be in this week.
	startTime3 := time.Date(2020, time.August, 9, 0, 0, beidouLeapSeconds, 0, utils.LocationUTC)
	rtcm3 := New(startTime3)

	if !expectedStartOfThisWeek.Equal(rtcm3.startOfBeidouWeek) {
		t.Errorf("expected %s result %s\n",
			expectedStartOfThisWeek.Format(utils.DateLayout), rtcm3.startOfBeidouWeek.Format(utils.DateLayout))
	}

	// This start time should be just at the end of this Beidou week.
	startTime4 :=
		time.Date(2020, time.August, 16, 0, 0, beidouLeapSeconds-1, 999999999, utils.LocationUTC)
	rtcm4 := New(startTime4)

	if !expectedStartOfThisWeek.Equal(rtcm4.startOfBeidouWeek) {
		t.Errorf("expected %s result %s\n",
			expectedStartOfThisWeek.Format(utils.DateLayout), rtcm4.startOfBeidouWeek.Format(utils.DateLayout))
	}

	// This start time should be just at the start of the next Beidou week.
	startTime5 :=
		time.Date(2020, time.August, 16, 0, 0, beidouLeapSeconds, 0, utils.LocationUTC)
	rtcm5 := New(startTime5)

	if !expectedStartOfNextWeek.Equal(rtcm5.startOfBeidouWeek) {
		t.Errorf("expected %s result %s\n",
			expectedStartOfNextWeek.Format(utils.DateLayout), rtcm5.startOfBeidouWeek.Format(utils.DateLayout))
	}
}

// TestGlonassEpochTimes tests that New sets up the correct start time
// for the Glonass epochs.
func TestGlonassEpochTimes(t *testing.T) {

	// expect 9pm Saturday 1st August - midnight Sunday 2nd August in Russia - Glonass day 0.
	expectedEpochStart1 :=
		time.Date(2020, time.August, 1, 21, 0, 0, 0, utils.LocationUTC)

		// expect Glonass day 0.
	expectedGlonassDay1 := uint(0)

	startTime1 :=
		time.Date(2020, time.August, 2, 5, 0, 0, 0, utils.LocationUTC)
	rtcm1 := New(startTime1)
	if expectedEpochStart1 != rtcm1.startOfGlonassDay {
		t.Errorf("expected %s result %s\n",
			expectedEpochStart1.Format(utils.DateLayout),
			rtcm1.startOfGlonassDay.Format(utils.DateLayout))
	}

	if expectedGlonassDay1 != rtcm1.glonassDayFromPreviousMessage {
		t.Errorf("expected %d result %d\n",
			expectedGlonassDay1, rtcm1.glonassDayFromPreviousMessage)
	}

	// 21:00 on Monday 3rd August - 00:00 on Tuesday in Moscow - Glonass day 2.
	expectedEpochStart2 :=
		time.Date(2020, time.August, 3, 21, 0, 0, 0, utils.LocationUTC)
	// 21:00 on Tuesday 4th August - 00:00 on Wednesday in Moscow - Glonass day 3

	expectedGlonassDay2 := uint(2)

	// Start just before 9pm on Tuesday 3rd August - just before the end of
	// Tuesday in Moscow - day 2
	startTime2 :=
		time.Date(2020, time.August, 3, 22, 59, 59, 999999999, utils.LocationUTC)
	rtcm2 := New(startTime2)
	if expectedEpochStart2 != rtcm2.startOfGlonassDay {
		t.Errorf("expected %s result %s\n",
			expectedEpochStart2.Format(utils.DateLayout),
			rtcm1.startOfGlonassDay.Format(utils.DateLayout))
	}
	if expectedGlonassDay2 != rtcm2.glonassDayFromPreviousMessage {
		t.Errorf("expected %d result %d\n",
			expectedGlonassDay2, rtcm2.glonassDayFromPreviousMessage)
	}
}

// TestPrepareForDisplayWithErrorMessage checks that PrepareforDisplay
// handles an error message correctly.
func TestPrepareForDisplayWithErrorMessage(t *testing.T) {
	// PrepareForDisplay checks that the message hasn't already been
	// analysed.  If not, it calls Analyse.  If that returns an error
	// message it displays that.  Force Analyse to fail using
	// an incomplete bit stream.

	const wantType = 1077
	const wantErrorMessage = "bitstream is too short for an MSM header - got 80 bits, expected at least 169"

	shortBitStream := testdata.MessageBatchWithJunk[:16]

	message := NewMessage(utils.MessageTypeMSM7GPS, "", shortBitStream)

	PrepareForDisplay(message)

	if message.MessageType != wantType {
		t.Errorf("expected a type %d got %d", wantType, message.MessageType)
	}

	if len(message.ErrorMessage) == 0 {
		t.Error("expected an error message")
	}

	if message.ErrorMessage != wantErrorMessage {
		t.Errorf("expected error message %s got %s",
			wantErrorMessage, message.ErrorMessage)
	}
}

// TestAnalyseWithMSM4 checks that Analyse correctly handles an MSM4.
func TestAnalyseWithMSM4(t *testing.T) {

	message := NewMessage(utils.MessageTypeMSM4GPS, "", testdata.MessageFrameType1074_2)

	Analyse(message)

	if message.Readable == nil {
		t.Error("Readable is nil")
		return
	}

	m, ok := message.Readable.(*msm4message.Message)

	if !ok {
		t.Error("expecting Readable to contain an MSM4")
	}

	if !(utils.MSM4(m.Header.MessageType)) {
		t.Errorf("expecting an MSM4, got %d", m.Header.MessageType)
	}
}

// TestAnalyseWithMSM7 checks that Analyse correctly handles an MSM7.
func TestAnalyseWithMSM7(t *testing.T) {

	message := NewMessage(utils.MessageTypeMSM7GPS, "", testdata.MessageFrame1077)

	Analyse(message)

	if message.Readable == nil {
		t.Error("Readable is nil")
		return
	}

	m, ok := message.Readable.(*msm7message.Message)

	if !ok {
		t.Error("expecting Readable to contain an MSM7")
	}

	if !(utils.MSM7(m.Header.MessageType)) {
		t.Errorf("expecting an MSM7, got %d", m.Header.MessageType)
	}
}

// TestAnalyseWith1005 checks that Analyse correctly handles an MSM7.
func TestAnalyseWith1005(t *testing.T) {

	message := NewMessage(utils.MessageType1005, "", testdata.MessageFrameType1005)

	Analyse(message)

	if message.Readable == nil {
		t.Error("Readable is nil")
		return
	}

	_, ok := message.Readable.(*type1005.Message)

	if !ok {
		t.Error("expecting Readable to contain an MSM7")
	}
}

// TestAnalyseWith1230 checks that Analyse correctly handles a message of type 1230
// (the correct behaviour being to set the Readable field to a string).
func TestAnalyseWith1230(t *testing.T) {

	message := NewMessage(utils.MessageTypeGCPB, "", testdata.Fake1230)

	Analyse(message)

	_, ok := message.Readable.(string)

	if !ok {
		t.Error("expecting Readable to contain a string")
	}
}

// TestDisplayMessage checks DisplayMessage.
func TestDisplayMessage(t *testing.T) {

	const resultForNotRTCM = `message type -1, frame length 4
00000000  6a 75 6e 6b                                       |junk|

`

	const resultForIncomplete = `message type 1127, frame length 6
00000000  d3 00 aa 46 70 00                                 |...Fp.|

bitstream is too short for an MSM header - got 0 bits, expected at least 169
`

	// The hex dump includes a ` so we have to create this string by glueing parts together.
	const resultForMSM4 = `message type 1074, frame length 42
00000000  d3 00 24 43 20 01 00 00  00 04 00 00 08 00 00 00  |..$C ...........|
` +
		"00000010  00 00 00 00 20 00 80 00  60 28 00 40 01 00 02 00  |.... ...`(.@....|" + `
00000020  00 40 00 00 68 8e 80 83  f7 4b                    |.@..h....K|

type 1074 GPS Full Pseudoranges and PhaseRanges plus CNR
stationID 1, timestamp 1, single message, sequence number 0
session transmit time 0, clock steering 0, external clock 0
divergence free smoothing false, smoothing interval 0
1 satellites, 2 signal types, 2 signals
1 Satellites
Satellite ID {range ms}
 4 {374740.573}
1 Signals
Sat ID Sig ID {range (delta), lock time ind, half cycle ambiguity, Carrier Noise Ratio}
 4  2 {374758.870, 1970044.248, 3, false, 7}
 4 16 {374777.168, 1534500.000, 4, true, 16}
`

	const resultForMSM7 = `message type 1077, frame length 841
00000000  d3 00 dc 43 50 00 67 00  97 62 00 00 08 40 a0 65  |...CP.g..b...@.e|
00000010  00 00 00 00 20 00 80 00  6d ff a8 aa 26 23 a6 a2  |.... ...m...&#..|
00000020  23 24 00 00 00 00 36 68  cb 83 7a 6f 9d 7c 04 92  |#$....6h..zo.|..|
00000030  fe f2 05 b0 4a a0 ec 7b  0e 09 27 d0 3f 23 7c b9  |....J..{..'.?#|.|
00000040  6f bd 73 ee 1f 01 64 96  f5 7b 27 46 f1 f2 1a bf  |o.s...d..{'F....|
00000050  19 fa 08 41 08 7b b1 1b  67 e1 a6 70 71 d9 df 0c  |...A.{..g..pq...|
00000060  61 7f 19 9c 7e 66 66 fb  86 c0 04 e9 c7 7d 85 83  |a...~ff......}..|
00000070  7d ac ad fc be 2b fc 3c  84 02 1d eb 81 a6 9c 87  |}....+.<........|
00000080  17 5d 86 f5 60 fb 66 72  7b fa 2f 48 d2 29 67 08  |.]..` + "`" + `.fr{./H.)g.|
00000090  c8 72 15 0d 37 ca 92 a4  e9 3a 4e 13 80 00 14 04  |.r..7....:N.....|
000000a0  c0 e8 50 16 04 c1 40 46  17 05 41 70 52 17 05 01  |..P...@F..ApR...|
000000b0  ef 4b de 70 4c b1 af 84  37 08 2a 77 95 f1 6e 75  |.K.pL...7.*w..nu|
000000c0  e8 ea 36 1b dc 3d 7a bc  75 42 80 00 00 00 00 00  |..6..=z.uB......|
000000d0  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 fe  |................|
000000e0  69 e8 6a d3 00 c3 43 f0  00 a2 93 7c 22 00 00 04  |i.j...C....|"...|
000000f0  0e 03 80 00 00 00 00 20  80 00 00 7f fe 9c 8a 80  |....... ........|
00000100  94 86 84 99 0c a0 95 2a  8b d8 3a 92 f5 74 7d 56  |.......*..:..t}V|
00000110  fe b7 ec e8 0d 41 69 7c  00 0e f0 61 42 9c f0 27  |.....Ai|...aB..'|
00000120  38 86 2a da 62 36 3c 8f  eb c8 27 1b 77 6f b9 4c  |8.*.b6<...'.wo.L|
00000130  be 36 2b e4 26 1d c1 4f  dc d9 01 16 24 11 9a e0  |.6+.&..O....$...|
00000140  91 02 00 7a ea 61 9d b4  e1 52 f6 1f 22 ae df 26  |...z.a...R.."..&|
00000150  28 3e e0 f6 be df 90 df  b8 01 3f 8e 86 bf 7e 67  |(>........?...~g|
00000160  1f 83 8f 20 51 53 60 46  60 30 43 c3 3d cf 12 84  |... QS` + "`F`" + `0C.=...|
00000170  b7 10 c4 33 53 3d 25 48  b0 14 00 00 04 81 28 60  |...3S=%H......(` + "`" + `|
00000180  13 84 81 08 54 13 85 40  e8 60 12 85 01 38 5c 67  |....T..@.` + "`" + `...8\g|
00000190  b7 67 a5 ff 4e 71 cd d3  78 27 29 0e 5c ed d9 d7  |.g..Nq..x').\...|
000001a0  cc 7e 04 f8 09 c3 73 a0  40 70 d9 6d 6a 75 6e 6b  |.~....s.@p.mjunk|
000001b0  d3 00 c3 44 90 00 67 00  97 62 00 00 21 18 00 c0  |...D..g..b..!...|
000001c0  08 00 00 00 20 01 00 00  7f fe ae be 90 98 a6 9c  |.... ...........|
000001d0  b4 00 00 00 08 c1 4b c1  32 f8 0b 08 c5 83 c8 01  |......K.2.......|
000001e0  e8 25 3f 74 7c c4 02 a0  4b c1 47 90 12 86 62 72  |.%?t|...K.G...br|
000001f0  92 28 53 18 9d 8d 85 82  c6 e1 8a 6a 2f dd 5e cd  |.(S........j/.^.|
00000200  d3 e1 1a 15 01 a1 2b dc  56 3f c4 ea c0 5e dc 40  |......+.V?...^.@|
00000210  48 d3 80 b2 25 60 9c 7b  7e 32 dd 3e 22 f7 01 b6  |H...%` + "`" + `.{~2.>"...|
00000220  f3 81 af b7 1f 78 e0 7f  6c aa fe 9a 7e 7e 94 9f  |.....x..l...~~..|
00000230  bf 06 72 3f 15 8c b1 44  56 e1 b1 92 dc b5 37 4a  |..r?...DV.....7J|
00000240  d4 5d 17 38 4e 30 24 14  00 04 c1 50 3e 0f 85 41  |.].8N0$....P>..A|
00000250  40 52 13 85 61 50 5a 16  04 a1 38 12 5b 24 7e 03  |@R..aPZ...8.[$~.|
00000260  6c 07 89 db 93 bd ba 0d  34 27 68 75 d0 a6 72 24  |l.......4'hu..r$|
00000270  e4 88 dc 61 a9 40 b1 9d  0d d3 00 aa 46 70 00 66  |...a.@......Fp.f|
00000280  ff bc a0 00 00 00 04 00  26 18 00 00 00 20 02 00  |........&.... ..|
00000290  00 75 53 fa 82 42 62 9a  80 00 00 06 95 4e a7 a0  |.uS..Bb......N..|
000002a0  bf 1e 78 7f 0a 10 08 18  7f 35 04 ab ee 50 77 8a  |..x......5...Pw.|
000002b0  86 f0 51 f1 4d 82 46 38  29 0a 8c 35 57 23 87 82  |..Q.M.F8)..5W#..|
000002c0  24 2a 01 b5 40 07 eb c5  01 37 a8 80 b3 88 03 23  |$*..@....7.....#|
000002d0  c4 fc 61 e0 4f 33 c4 73  31 cd 90 54 b2 02 70 90  |..a.O3.s1..T..p.|
000002e0  26 0b 42 d0 9c 2b 0c 02  97 f4 08 3d 9e c7 b2 6e  |&.B..+.....=...n|
000002f0  44 0f 19 48 00 00 00 00  00 00 00 00 00 00 00 00  |D..H............|
00000300  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00  |................|
00000310  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00  |................|
00000320  00 00 00 00 00 00 e5 1e  d8 d3 00 aa 46 70 00 66  |............Fp.f|
00000330  ff bc a0 00 00 00 04 00  26 18 00 00 00 20 02 00  |........&.... ..|
00000340  00 75 53 fa 82 42 62 9a  80                       |.uS..Bb..|

type 1077 GPS Full Pseudoranges and PhaseRanges plus CNR (high resolution)
stationID 0, timestamp 432023000, multiple message, sequence number 0
session transmit time 0, clock steering 0, external clock 0
divergence free smoothing false, smoothing interval 0
8 satellites, 2 signal types, 16 signals
Satellite ID {approx range m, extended info, phase range rate}:
 4 {24410542.339, 0, -135}
 9 {25264833.738, 0, 182}
16 {22915678.774, 0, 597}
18 {21506595.669, 0, 472}
25 {23345166.602, 0, -633}
26 {20661965.550, 0, 292}
29 {21135953.821, 0, -383}
31 {21670837.435, 0, -442}
Signals: sat ID sig ID {range m, phase range, lock time ind, half cycle ambiguity, Carrier Noise Ratio, phase range rate}:
 4  2 {24410527.355, 128282115.527, 513, false, 80, -136.207}
 4 16 {24410523.313, 99955313.523, 320, false, 82, -134.869}
 9 16 {25264751.952, 103451227.387, 606, false, 78, 182.267}
16  2 {22915780.724, 120426622.169, 40, true, 86, 597.233}
18  2 {21506547.550, 113021883.224, 968, false, 84, 471.599}
18 16 {21506542.760, 88061705.706, 37, false, 90, 472.270}
25  2 {23345103.037, 122677706.869, 51, true, 88, -632.317}
25 16 {23345100.838, 95594616.762, 78, false, 74, -634.411}
26  2 {20662003.308, 108581645.522, 329, false, 78, 291.597}
26 16 {20662000.914, 84606022.946, 80, false, 18, 290.429}
29  2 {21136079.188, 111074207.143, 664, false, 364, -382.650}
29 16 {21136074.598, 86547263.526, 787, false, 583, -382.997}
31  2 {21670772.711, 113885408.778, 710, true, 896, -443.036}
31 16 {21670767.783, 88736342.592, 779, false, 876, -441.969}
`

	const resultForUnhandledMessageType = `message type 1024, frame length 14
00000000  d3 00 08 40 00 00 8a 00  00 00 00 4f 5e e7        |...@.......O^.|

message type 1024 currently cannot be displayed
`

	const resultFor1005 = `message type 1005, frame length 25
00000000  d3 00 13 3e d0 02 0f c0  00 01 e2 40 40 00 03 94  |...>.......@@...|
00000010  47 80 00 05 46 4e 5b 90  5f                       |G...FN[._|

message type 1005 - Base Station Information
stationID 2, ITRF realisation year 3, ignored 0xf,
x 123456, ignored 0x1, y 234567, ignored 0x2, z 345678,
ECEF coords in metres (12.3456, 23.4567, 34.5678)
`

	var testData = []struct {
		description string
		bitStream   []byte
		messageType int
		want        string
	}{
		{"not RTCM", testdata.AllJunk, utils.NonRTCMMessage, resultForNotRTCM},
		{"incomplete", testdata.IncompleteMessage, 1127, resultForIncomplete},
		{"1005", testdata.MessageFrameType1005, 1005, resultFor1005},
		{"msm4", testdata.MessageFrameType1074_2, 1074, resultForMSM4},
		{"msm7", testdata.MessageBatchWithJunk, 1077, resultForMSM7},
		{"1024", testdata.UnhandledMessageType1024, 1024, resultForUnhandledMessageType},
	}
	for _, td := range testData {
		message := NewMessage(td.messageType, "", td.bitStream)
		startTime := time.Date(2020, time.November, 13, 0, 0, 0, 0, utils.LocationUTC)
		handler := New(startTime)

		got := handler.DisplayMessage(message)
		if got != td.want {
			t.Errorf("%s:\n%s, ", td.description, diff.Diff(td.want, got))

		}
	}
}

// TestDisplayMessageWithErrors creates some obscure error conditions and checks
// that DisplayMessage handles them.
func TestDisplayMessageWithErrors(t *testing.T) {

	// RTCM3 messages that will produce an error when DisplayMessage is called.
	// In most cases the problem is that the type value in the message doesn't match
	// the value in the raw data.
	messageWithErrorMessage := NewMessage(utils.MessageType1005, "an error message", testdata.MessageFrameType1005)
	fake1005 := NewMessage(utils.MessageType1005, "", testdata.MessageFrame1077)
	fakeMSM4 := NewMessage(utils.MessageTypeMSM4Beidou, "", testdata.MessageFrame1077)
	fakeMSM7 := NewMessage(utils.MessageTypeMSM7Glonass, "", testdata.MessageFrameType1074_2)
	// Expected results.
	resultForMessageWithWarning := `message type 1005, frame length 25
00000000  d3 00 13 3e d0 02 0f c0  00 01 e2 40 40 00 03 94  |...>.......@@...|
00000010  47 80 00 05 46 4e 5b 90  5f                       |G...FN[._|

an error message
`

	resultForFake1005 := `message type 1005, frame length 226
00000000  d3 00 dc 43 50 00 67 00  97 62 00 00 08 40 a0 65  |...CP.g..b...@.e|
00000010  00 00 00 00 20 00 80 00  6d ff a8 aa 26 23 a6 a2  |.... ...m...&#..|
00000020  23 24 00 00 00 00 36 68  cb 83 7a 6f 9d 7c 04 92  |#$....6h..zo.|..|
00000030  fe f2 05 b0 4a a0 ec 7b  0e 09 27 d0 3f 23 7c b9  |....J..{..'.?#|.|
00000040  6f bd 73 ee 1f 01 64 96  f5 7b 27 46 f1 f2 1a bf  |o.s...d..{'F....|
00000050  19 fa 08 41 08 7b b1 1b  67 e1 a6 70 71 d9 df 0c  |...A.{..g..pq...|
00000060  61 7f 19 9c 7e 66 66 fb  86 c0 04 e9 c7 7d 85 83  |a...~ff......}..|
00000070  7d ac ad fc be 2b fc 3c  84 02 1d eb 81 a6 9c 87  |}....+.<........|
00000080  17 5d 86 f5 60 fb 66 72  7b fa 2f 48 d2 29 67 08  |.]..` + "`" + `.fr{./H.)g.|
00000090  c8 72 15 0d 37 ca 92 a4  e9 3a 4e 13 80 00 14 04  |.r..7....:N.....|
000000a0  c0 e8 50 16 04 c1 40 46  17 05 41 70 52 17 05 01  |..P...@F..ApR...|
000000b0  ef 4b de 70 4c b1 af 84  37 08 2a 77 95 f1 6e 75  |.K.pL...7.*w..nu|
000000c0  e8 ea 36 1b dc 3d 7a bc  75 42 80 00 00 00 00 00  |..6..=z.uB......|
000000d0  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 fe  |................|
000000e0  69 e8                                             |i.|

expected message type 1005 got 1077
`

	const resultForFakeMSM4 = `message type 1124, frame length 226
00000000  d3 00 dc 43 50 00 67 00  97 62 00 00 08 40 a0 65  |...CP.g..b...@.e|
00000010  00 00 00 00 20 00 80 00  6d ff a8 aa 26 23 a6 a2  |.... ...m...&#..|
00000020  23 24 00 00 00 00 36 68  cb 83 7a 6f 9d 7c 04 92  |#$....6h..zo.|..|
00000030  fe f2 05 b0 4a a0 ec 7b  0e 09 27 d0 3f 23 7c b9  |....J..{..'.?#|.|
00000040  6f bd 73 ee 1f 01 64 96  f5 7b 27 46 f1 f2 1a bf  |o.s...d..{'F....|
00000050  19 fa 08 41 08 7b b1 1b  67 e1 a6 70 71 d9 df 0c  |...A.{..g..pq...|
00000060  61 7f 19 9c 7e 66 66 fb  86 c0 04 e9 c7 7d 85 83  |a...~ff......}..|
00000070  7d ac ad fc be 2b fc 3c  84 02 1d eb 81 a6 9c 87  |}....+.<........|
00000080  17 5d 86 f5 60 fb 66 72  7b fa 2f 48 d2 29 67 08  |.]..` + "`" + `.fr{./H.)g.|
00000090  c8 72 15 0d 37 ca 92 a4  e9 3a 4e 13 80 00 14 04  |.r..7....:N.....|
000000a0  c0 e8 50 16 04 c1 40 46  17 05 41 70 52 17 05 01  |..P...@F..ApR...|
000000b0  ef 4b de 70 4c b1 af 84  37 08 2a 77 95 f1 6e 75  |.K.pL...7.*w..nu|
000000c0  e8 ea 36 1b dc 3d 7a bc  75 42 80 00 00 00 00 00  |..6..=z.uB......|
000000d0  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 fe  |................|
000000e0  69 e8                                             |i.|

message type 1077 is not an MSM4
`

	const resultForFakeMSM7 = `message type 1087, frame length 42
00000000  d3 00 24 43 20 01 00 00  00 04 00 00 08 00 00 00  |..$C ...........|
00000010  00 00 00 00 20 00 80 00  60 28 00 40 01 00 02 00  |.... ...` + "`" + `(.@....|
00000020  00 40 00 00 68 8e 80 83  f7 4b                    |.@..h....K|

message type 1074 is not an MSM7
`

	var testData = []struct {
		description string
		message     *Message
		want        string
	}{
		{"error message", messageWithErrorMessage, resultForMessageWithWarning},
		{"1005", fake1005, resultForFake1005},
		{"msm4", fakeMSM4, resultForFakeMSM4},
		{"msm7", fakeMSM7, resultForFakeMSM7},
	}
	for _, td := range testData {
		startTime := time.Now()
		handler := New(startTime)
		got := handler.DisplayMessage(td.message)
		if td.want != got {
			t.Errorf("%s\n%s", td.description, diff.Diff(td.want, got))
		}
	}
}

func createHeader(messageType int, epochTime uint) *header.Header {
	return header.New(messageType, 0, epochTime, false, 0, 0, 0, 0, false, 0, 0, 0, 0)
}

// TestConversionOfTimeToUTC checks that the converting a timestamp to
// a time works when the timestamp rolls over
func TestConversionOfTimeToUTC(t *testing.T) {

	// The timestamp is an offset in milliseconds from the start of the week.  The week
	// starts at a different time for each constellation.  The handler keeps track of the
	// current start of week and remembers the last timestamp.  If the given timestamp is
	// smaller than the last one, the handler rolls over into a new week, updating its
	// start of week value.
	//
	// The test uses three timestamps 1, 2 and 3.  2 is bigger than 1, so there should be
	// no rollover.  3 is bigger than 2 so that should provoke a rollover to the next
	// week.

	var testData = []struct {
		description            string
		startTime              time.Time // The start of the week for this constellation.
		messageType            int       // The message type (which gives the constellation).
		wantStartOfWeek1       time.Time // start of week before rollover.
		timestamp1             uint      // First timestamp - before the rollover.
		wantTimeFromTimestamp1 time.Time // The time from timestamp1.
		timestamp2             uint      // second timestamp, bigger than 1 so still no rollover.
		wantTimeFromTimestamp2 time.Time // The time from timestamp2.
		timestamp3             uint      // Third timestamp, smaller than 2 to provoke rollover.
		wantStartOfWeek2       time.Time // The start of week after the rollover.
		wantTimeFromTimestamp  time.Time // The time from timestamp3.
	}{
		{
			"GPS",
			// Monday 10th August BST.  2am is 1am UTC.
			time.Date(2020, time.August, 10, 2, 0, 0, 0, utils.LocationLondon),
			utils.MessageTypeMSM4GPS,
			// The GPS week of 10th August starts at GPS midnight 9th August,
			// in UTC, just before midnight at the end of the 8th.
			time.Date(2020, time.August, 8, 23, 59, 42, 0, utils.LocationUTC),
			47 * 3600 * 1000, // 1 day 23 hours into the week.
			time.Date(2020, time.August, 10, 22, 59, 42, 0, utils.LocationUTC),
			(7 * 24 * 3600 * 1000) - 1, // just before end of week.
			time.Date(2020, time.August, 15, 23, 59, 41, 0, utils.LocationUTC),
			500, // rolled over
			time.Date(2020, time.August, 15, 23, 59, 42, 0, utils.LocationUTC),
			// A timestamp value of 500 milliseconds should give 15th (02:00:00.500 less the leap seconds)
			// CET on Sunday 16th August.
			time.Date(2020, time.August, 15, 23, 59, 42, 500000000, utils.LocationUTC),
		},
		{
			"Beidou",
			// Sunday 9th August.
			time.Date(2020, time.August, 9, 0, 0, beidouLeapSeconds, 0, utils.LocationUTC),
			utils.MessageTypeMSM7Beidou,
			time.Date(2020, time.August, 9, 00, 00, 14, 0, utils.LocationUTC),
			0, // Start of the week.
			time.Date(2020, time.August, 9, 00, 00, 14, 0, utils.LocationUTC),
			maxEpochTime, // just before end of week.
			time.Date(2020, time.August, 15, 00, 00, 13, 999, utils.LocationUTC),
			500, // rolled over
			time.Date(2020, time.August, 16, 00, 00, 14, 0, utils.LocationUTC),
			time.Date(2020, time.August, 16, 00, 00, 14, 500000000, utils.LocationUTC),
		},
		{
			"Glonass",
			// 23:00:00 on Monday 10th August Paris is midnight on the Tuesday
			// 11th in Russia - start of Glonass day 2.
			time.Date(2020, time.August, 10, 23, 0, 0, 0, utils.LocationParis),
			utils.MessageTypeMSM7Glonass,
			time.Date(2020, time.August, 11, 0, 0, 0, 0, utils.LocationMoscow),
			// Day = 2, glonassTime = (4*3600*1000), which is 2 am on Russian Tuesday,
			// which is 11pm on Monday 10th UTC.
			2<<27 | (2 * 3600 * 1000),
			time.Date(2020, time.August, 12, 23, 0, 0, 0, utils.LocationUTC),
			2<<27 | (5 * 3600 * 1000), // later that day in Moscow, 2am the next day in UTC.
			time.Date(2020, time.August, 13, 2, 0, 0, 0, utils.LocationUTC),
			3<<27 | (18 * 3600 * 1000), // rolled over to the next day
			// 3pm Tuesday 11th August Paris - 4pm in Russia.
			time.Date(2020, time.August, 11, 0, 0, 0, 0, utils.LocationMoscow).AddDate(0, 0, 1),
			// Day = 3, glonassTime = (18*3600*1000), which is 6pm on Russian Wednesday,
			// which is 3pm on Wednesday 12th UTC, 5pm CET.
			time.Date(2020, time.August, 12, 17, 0, 0, 0, utils.LocationParis),
		},
		{
			"Galileo",
			// Monday 10th Aug.  Paris is two hours ahead of UTC.
			time.Date(2020, time.August, 10, 23, 0, 0, 0, utils.LocationParis),
			utils.MessageTypeMSM7Galileo,
			time.Date(2020, time.August, 8, 23, 59, 42, 0, utils.LocationUTC),
			(((52*3600)+1800)*1000 + 300), // 2 days, 4.5 hours  plus 300 ms in ms.
			time.Date(2020, time.August, 10, 04, 29, 42, int(300*time.Millisecond), utils.LocationUTC).
				Add(gpsTimeOffset),
			(((74*3600)+30)*1000 + 700), // 3 days 2 hours 30 secondsand 400 ms.
			time.Date(2020, time.August, 12, 23, 59, 30, int(700*time.Millisecond), utils.LocationUTC),
			((2 * 3600 * 1000) + 4), // rolled over to the next day
			// 2020-08-08 23:59:42.

			time.Date(2020, time.August, 16, 00, 00, 00, 0, utils.LocationUTC).
				Add(gpsTimeOffset),
			time.Date(2020, time.August, 16, 1, 59, 42, int(4*time.Millisecond), utils.LocationUTC),
		},
	}
	for _, td := range testData {

		handler := New(td.startTime)

		_, err1 := handler.getTimeFromTimeStamp(td.messageType, td.timestamp1)

		if err1 != nil {
			t.Errorf("%s: %v", td.description, err1)
		}

		// Get the start of the week for this message, or the
		// start of day if Glonass.
		startOfPeriod1 := getStartOfPeriod(td.messageType, handler)

		if !td.wantStartOfWeek1.Equal(startOfPeriod1) {
			t.Errorf("%s: want %s got %s",
				td.description, td.wantStartOfWeek1.Format(utils.DateLayout),
				startOfPeriod1.Format(utils.DateLayout))
		}

		_, err2 := handler.getTimeFromTimeStamp(td.messageType, td.timestamp2)

		if err2 != nil {
			t.Errorf("%s: %v", td.description, err2)
		}

		// Should be no rollover.
		if !td.wantStartOfWeek1.Equal(startOfPeriod1) {
			t.Errorf("%s: want %s got %s",
				td.description, td.wantStartOfWeek1.Format(utils.DateLayout),
				startOfPeriod1.Format(utils.DateLayout))
		}

		t3, err3 := handler.getTimeFromTimeStamp(td.messageType, td.timestamp3)

		if err3 != nil {
			t.Errorf("%s: %v", td.description, err3)
		}

		// That should have provoked a rollover ...

		// ...so we should be in a new period ...

		// Get the start of the week for this message, or the
		// start of day if Glonass.
		startOfPeriod2 := getStartOfPeriod(td.messageType, handler)

		if !td.wantStartOfWeek2.Equal(startOfPeriod2) {
			t.Errorf("%s: want %s got %s",
				td.description, td.wantStartOfWeek2.Format(utils.DateLayout),
				startOfPeriod2.Format(utils.DateLayout))
		}

		// ... and so we should get this time from the timestamp.
		if !td.wantTimeFromTimestamp.Equal(t3) {
			t.Errorf("%s: want %s got %s",
				td.description, td.wantTimeFromTimestamp.Format(utils.DateLayout), t3.Format(utils.DateLayout))
		}
	}
}

// getStartOfPeriod is a helper function for TestConversionOfTimeToUTCWithRollover.
// It gets the start of the constellation's current period (week or, for Glonass,  day)
func getStartOfPeriod(messageType int, handler *Handler) time.Time {
	// Get the start of the week for this message, or the
	// start of day if Glonass.
	constellation := utils.GetConstellation(messageType)
	var startOfPeriod time.Time
	switch constellation {
	case "GPS":
		startOfPeriod = handler.startOfGPSWeek
	case "Galileo":
		startOfPeriod = handler.startOfGalileoWeek
	case "Glonass":
		startOfPeriod = handler.startOfGlonassDay
	case "Beidou":
		startOfPeriod = handler.startOfBeidouWeek
	}

	return startOfPeriod
}

// TestGetTimeFromTimeStampWithError checks that getTimeFromTimeStampWithError when
// it's given an illegal Glonass timestamp.  (Glonass is the only constellation that
// can have an illegal timestamp).
func TestGetTimeFromTimeStampWithError(t *testing.T) {

	// The timestamp in the header is 30 bits.  For Glonass the top three bits hold the day
	// number and bits 0-26 hold the millis from midnight on that day.  Day values 0-6 are
	// legal, 7 is illegal and produces an error.
	//
	// For all constellations any timestamp longer than 30 bits is illegal.

	var testData = []struct {
		description string
		hdr         *header.Header
		wantError   string
	}{
		{"4 0", createHeader(utils.MessageTypeMSM4Glonass, (7 << 27)), "invalid day"},
		{"7 1", createHeader(utils.MessageTypeMSM7Glonass, ((7 << 27) | 1)), "invalid day"},
		{"7 max", createHeader(utils.MessageTypeMSM7Glonass, ((7 << 27) | (1 << 25))), "invalid day"},
		// This timestamp is an int, which is 32 bits on some machines, 64 on others.  For safety, only
		// set the timestamp to the max value of an int32.
		{"max int32", createHeader(utils.MessageTypeMSM4Glonass, math.MaxInt32), "out of range"},
		// Try all of the other constellations with a value bigger than 30 bits.
		{"GPS MSM4", createHeader(utils.MessageTypeMSM4GPS, maxTimestamp+1), "out of range"},
		{"GPS MSM7", createHeader(utils.MessageTypeMSM7GPS, math.MaxInt32), "out of range"},
		{"Galileo MSM4", createHeader(utils.MessageTypeMSM4Galileo, maxTimestamp+1), "out of range"},
		{"Galileo MSM7", createHeader(utils.MessageTypeMSM7Galileo, math.MaxInt32/2+1), "out of range"},
		{"Beidou MSM4", createHeader(utils.MessageTypeMSM4Beidou, maxTimestamp+2), "out of range"},
		{"Beidou MSM7", createHeader(utils.MessageTypeMSM7Beidou, 0x40000000), "out of range"},
		{"SBAS MSM7", createHeader(utils.MessageTypeMSM7SBAS, 0x40000000), "unknown message type"},
	}

	for _, td := range testData {
		// The start time is irrelevant so any will do.
		startTime := time.Now()
		handler := New(startTime)
		var zeroTime time.Time // zeroTime is the default time value.

		gotTime, gotError := handler.getTimeFromTimeStamp(td.hdr.MessageType, td.hdr.Timestamp)

		if !gotTime.Equal(zeroTime) {
			t.Errorf("%s: got time %v", td.description, gotTime)
		}

		if gotError == nil {
			t.Errorf("%s: expected an error", td.description)
			continue
		}
		if td.wantError != gotError.Error() {
			t.Errorf("%s: want error %s, got %s", td.description, td.wantError, gotError.Error())
		}
	}

}

// TestGetUTCFromGlonassTimeWithIllegalDay tests GetUTCFromGlonassTime
// when the day value is 7 (which is the illegal value)
func TestGetUTCFromGlonassTimeWithIllegalDay(t *testing.T) {
	// The timestamp is 30 bits.  The top 3 bits are the day.  Days 0-6 are
	// valid day values, day 7 is illegal and should produce an error.

	const illegal1 = 0x3c000000 // 11 1100 0000 0000 0000 0000 0000 0000
	const illegal2 = 0x3c000000 // 11 1100 0000 0000 0000 0000 0000 0000
	const want = "invalid day"

	var zeroTimeValue time.Time // utcTime should be set to this.

	var testData = []struct {
		description string
		timestamp   uint
	}{
		{"day/0", illegal1},
		{"day/timestamp", illegal2},
	}
	for _, td := range testData {
		handler := New(time.Now())
		utcTime, err := handler.getUTCFromGlonassTime(td.timestamp)

		if !utcTime.Equal(zeroTimeValue) {
			t.Errorf("expected the time to be %s, got %s",
				zeroTimeValue.Format(utils.DateLayout),
				utcTime.Format(utils.DateLayout))
		}

		if err == nil {
			t.Error("expected an error")
			continue
		}

		if want != err.Error() {
			t.Errorf("want error %s got %s", want, err.Error())
		}
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

	day1, millis1, err1 := ParseGlonassTimeStamp(uint(epochTime1))

	if err1 != nil {
		t.Error(err1)
	}

	if expectedDay1 != day1 {
		t.Errorf("expected day %d result %d", expectedDay1, day1)
	}
	if expectedMillis1 != millis1 {
		t.Errorf("expected millis %d result %d", maxMillis, millis1)
	}

	// Day = 0, millis = max
	const expectedDay2 uint = 0
	const epochTime2 = maxMillis

	day2, millis2, err2 := ParseGlonassTimeStamp(uint(epochTime2))

	if err2 != nil {
		t.Error(err2)
	}

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

	day3, millis3, err3 := ParseGlonassTimeStamp(uint(epochTime3))

	if err3 != nil {
		t.Error(err3)
	}

	if expectedDay3 != day3 {
		t.Errorf("expected day %d result %d", expectedDay3, day3)
	}
	if expectedMillis3 != millis3 {
		t.Errorf("expected millis %d result %d", expectedMillis3, millis3)
	}

	// Day = max, mills = max..
	const expectedDay4 uint = 6

	const epochTime4 = 6<<27 | uint(maxMillis)

	day4, millis4, err4 := ParseGlonassTimeStamp(uint(epochTime4))

	if err4 != nil {
		t.Error(err4)
	}

	if expectedDay4 != day4 {
		t.Errorf("expected day %d result %d", expectedDay4, day4)
	}
	if maxMillis != millis4 {
		t.Errorf("expected millis %d result %d", maxMillis, millis4)
	}

	// These values can't actually happen in a Glonass epoch - the day can only
	// be up to 6 and the millis only run up to 24hours minus 1 milli.  However.
	// we'll test the logic anyway.
	const expectedDay5 uint = 7
	const expectedMillis5 uint = 0x7ffffff

	const epochTime5 uint = 0x3fffffff // 11 1111 1111 ... (30 bits).

	day5, millis5, err5 := ParseGlonassTimeStamp(uint(epochTime5))

	if err5 != nil {
		t.Error(err5)
	}

	if expectedDay5 != day5 {
		t.Errorf("expected day %d result %d", expectedDay5, day5)
	}
	if expectedMillis5 != millis5 {
		t.Errorf("expected millis %x result %x", expectedMillis5, millis5)
	}
}

func TestCheckCRC(t *testing.T) {

	// CRCCheck checks that the frame is at least 6 bytes long.
	shortFrame := []byte{1, 2, 3, 4, 5}

	var testData = []struct {
		description string
		bitStream   []byte
		want        bool
	}{
		{"valid", testdata.MesageBatchWith1077, true},
		{"CRC failure", testdata.MessageFrameWithCRCFailure, false},
		{"short", shortFrame, false},
	}
	for _, td := range testData {
		r := bytes.NewReader(td.bitStream)
		reader := bufio.NewReader(r)

		// Create a buffered channel big enough to hold the test data, send the
		// data to it and close it.
		ch_source := make(chan byte, 10000)
		for {
			buf := make([]byte, 1)
			n, err := reader.Read(buf)
			if err != nil {
				// We've read all the test data.  Done.
				close(ch_source)
				break
			}

			if n > 0 {
				ch_source <- buf[0]
			}
		}

		// Expect the resulting messages on this channel.
		ch_result := make(chan Message, 10)

		rtcmHandler := New(time.Now())

		// Test
		rtcmHandler.HandleMessagesFromChannel(ch_source, ch_result)

		// Check.  Read the data back from the channel and check the CRC.

		message, ok := <-ch_result
		if !ok {
			// Done - chan is drained.
			t.Errorf("%s: expected a message", td.description)
		}

		got := CheckCRC(message.RawData)

		if td.want != got {
			t.Errorf("%s: want %v got %v", td.description, td.want, got)
		}
	}
}

const wantSatelliteMask = 3
const wantSignalMask = 7
const wantCellMask = 1
const wantMessageType = 1074
const wantStationID = 1
const wantEpochTime = 2
const wantMultipleMessage = true
const wantIssue = 3
const wantTransTime = 4
const wantClockSteeringIndicator = 5
const wantExternalClockSteeringIndicator = 6
const wantSmoothing = true
const wantSmoothingInterval = 7

const wantSatelliteID = 8
const wantRangeWhole uint = 9
const wantRangeFractional uint = 10

const wantSignalID = 11
const wantRangeDelta = 12
const wantPhaseRangeDelta = 13
const wantLockTimeIndicator = 14
const wantHalfCycleAmbiguity = true
const wantCNR = 15
const wantWavelength = 16.0

func createMSM4() *msm4message.Message {
	hdr := header.New(wantMessageType, wantStationID, wantEpochTime, wantMultipleMessage,
		wantIssue, wantTransTime, wantClockSteeringIndicator, wantExternalClockSteeringIndicator,
		wantSmoothing, wantSmoothingInterval, wantSatelliteMask, wantSignalMask, wantCellMask)
	sat := msm4satellite.New(wantSatelliteID, wantRangeWhole, wantRangeFractional)
	satellites := []msm4satellite.Cell{*sat}
	sig := msm4signal.New(wantSignalID, sat, wantRangeDelta, wantPhaseRangeDelta,
		wantLockTimeIndicator, wantHalfCycleAmbiguity, wantCNR, wantWavelength)
	signals := [][]msm4signal.Cell{{*sig}}
	return msm4message.New(hdr, satellites, signals)
}

// createRTCMWithMSM4 creates an RTCM message containing the given MSM4,
// setting the time to utcTime.  The Readable doesn't match the RawData.
func createRTCMWithMSM4(msm4 *msm4message.Message, utcTime time.Time) *Message {
	message := NewMessage(utils.MessageTypeMSM4GPS, "", testdata.MessageFrameType1074_1)
	message.Readable = msm4
	message.UTCTime = &utcTime

	return message
}

// TestNew checks that New creates a message correctly.
func TestNew(t *testing.T) {

	const wantType = utils.MessageTypeMSM4QZSS
	const wantWarning = "a warning"
	wantBitstream := testdata.UnhandledMessageType1024
	const wantValid = false
	const wantComplete = false
	const wantCRCValid = false
	var wantUTCTime *time.Time = nil
	var wantReadable interface{} = nil

	message := NewMessage(wantType, wantWarning, wantBitstream)

	if wantType != message.MessageType {
		t.Errorf("want %d got %d", wantType, message.MessageType)
	}

	if wantWarning != message.ErrorMessage {
		t.Errorf("want %s got %s", wantWarning, message.ErrorMessage)
	}

	// Can't compare the bit streams so convert them to strings.
	want := string(wantBitstream)
	got := string(message.RawData)
	if want != got {
		t.Errorf("want %s got %s", want, got)
	}

	// Check the fields that should never be set by New

	if wantUTCTime != message.UTCTime {
		t.Errorf("want %v got %v", wantUTCTime, message.UTCTime)
	}

	if wantReadable != message.Readable {
		t.Errorf("want %v got %v", wantReadable, message.Readable)
	}
}

// TestNewNonRTCM checks that NewNonRTCM creates a non-RTCM message correctly.
func TestNewNonRTCM(t *testing.T) {

	const wantType = utils.NonRTCMMessage
	const wantWarning = ""
	var wantBitstream = []byte{'j', 'u', 'n', 'k'}
	var wantUTCTime *time.Time = nil
	var wantReadable interface{} = nil

	message := NewNonRTCM(wantBitstream)

	if wantType != message.MessageType {
		t.Errorf("want %d got %d", wantType, message.MessageType)
	}

	if wantWarning != message.ErrorMessage {
		t.Errorf("want %s got %s", wantWarning, message.ErrorMessage)
	}

	// Can't compare the bit streams so convert them to strings.
	want := string(wantBitstream)
	got := string(message.RawData)
	if want != got {
		t.Errorf("want %s got %s", want, got)
	}

	// Check the fields that should never be set by NewNonRTCM

	if wantUTCTime != message.UTCTime {
		t.Errorf("want %v got %v", wantUTCTime, message.UTCTime)
	}

	if wantReadable != message.Readable {
		t.Errorf("want %v got %v", wantReadable, message.Readable)
	}
}

// TestString checks the message.String method for various message types.
func TestString(t *testing.T) {

	const wantNonRTCM = `message type -1, frame length 4
00000000  6a 75 6e 6b                                       |junk|

`

	const wantShortFrame = `message type 1005, frame length 7
00000000  d3 00 13 3e d0 02 0f                              |...>...|

overrun - expected 152 bits in a message type 1005, got 8
`

	const want1024 = `message type 1024, frame length 14
00000000  d3 00 08 40 00 00 8a 00  00 00 00 4f 5e e7        |...@.......O^.|

message type 1024 currently cannot be displayed
`

	const want1005 = `message type 1005, frame length 25
00000000  d3 00 13 3e d0 02 0f c0  00 01 e2 40 40 00 03 94  |...>.......@@...|
00000010  47 80 00 05 46 4e 5b 90  5f                       |G...FN[._|

message type 1005 - Base Station Information
stationID 2, ITRF realisation year 3, ignored 0xf,
x 123456, ignored 0x1, y 234567, ignored 0x2, z 345678,
ECEF coords in metres (12.3456, 23.4567, 34.5678)
`
	const resultTemplateMSM4Complete = `2023-02-14 01:02:03.004 +0000 UTC
message type 1074, frame length 42
00000000  d3 04 32 43 20 01 00 00  00 04 00 00 08 00 00 00  |..2C ...........|
00000010  00 00 00 00 20 00 80 00  60 28 00 40 01 00 02 00  |.... ...` + "`" + `(.@....|
00000020  00 40 00 00 68 8e 80 6e  75 44                    |.@..h..nuD|

type 1074 GPS Full Pseudoranges and PhaseRanges plus CNR
stationID 1, timestamp 2, multiple message, sequence number 3
session transmit time 4, clock steering 5, external clock 6
divergence free smoothing true, smoothing interval 7
2 satellites, 3 signal types, 6 signals
1 Satellites
Satellite ID {range ms}
 8 {%.3f}
1 Signals
Sat ID Sig ID {range (delta), lock time ind, half cycle ambiguity, Carrier Noise Ratio}
 8 11 {%.3f, %.3f, 14, true, 15}
`

	const wantIncompleteMSM4 = `message type 1074, frame length 42
00000000  d3 04 32 43 20 01 00 00  00 04 00 00 08 00 00 00  |..2C ...........|
00000010  00 00 00 00 20 00 80 00  60 28 00 40 01 00 02 00  |.... ...` + "`" + `(.@....|
00000020  00 40 00 00 68 8e 80 6e  75 44                    |.@..h..nuD|

type 1074 GPS Full Pseudoranges and PhaseRanges plus CNR
stationID 1, timestamp 2, multiple message, sequence number 3
session transmit time 4, clock steering 5, external clock 6
divergence free smoothing true, smoothing interval 7
2 satellites, 3 signal types, 6 signals
No Satellites
No Signals
`

	// This result is copied from rtcm_test.go.
	const wantMSM7 = `2023-02-14 01:02:03.004 +0000 UTC
message type 1077, frame length 838
00000000  43 50 00 67 00 97 62 00  00 08 40 a0 65 00 00 00  |CP.g..b...@.e...|
00000010  00 20 00 80 00 6d ff a8  aa 26 23 a6 a2 23 24 00  |. ...m...&#..#$.|
00000020  00 00 00 36 68 cb 83 7a  6f 9d 7c 04 92 fe f2 05  |...6h..zo.|.....|
00000030  b0 4a a0 ec 7b 0e 09 27  d0 3f 23 7c b9 6f bd 73  |.J..{..'.?#|.o.s|
00000040  ee 1f 01 64 96 f5 7b 27  46 f1 f2 1a bf 19 fa 08  |...d..{'F.......|
00000050  41 08 7b b1 1b 67 e1 a6  70 71 d9 df 0c 61 7f 19  |A.{..g..pq...a..|
00000060  9c 7e 66 66 fb 86 c0 04  e9 c7 7d 85 83 7d ac ad  |.~ff......}..}..|
00000070  fc be 2b fc 3c 84 02 1d  eb 81 a6 9c 87 17 5d 86  |..+.<.........].|
00000080  f5 60 fb 66 72 7b fa 2f  48 d2 29 67 08 c8 72 15  |.` + "`" + `.fr{./H.)g..r.|
00000090  0d 37 ca 92 a4 e9 3a 4e  13 80 00 14 04 c0 e8 50  |.7....:N.......P|
000000a0  16 04 c1 40 46 17 05 41  70 52 17 05 01 ef 4b de  |...@F..ApR....K.|
000000b0  70 4c b1 af 84 37 08 2a  77 95 f1 6e 75 e8 ea 36  |pL...7.*w..nu..6|
000000c0  1b dc 3d 7a bc 75 42 80  00 00 00 00 00 00 00 00  |..=z.uB.........|
000000d0  00 00 00 00 00 00 00 00  00 00 00 00 fe 69 e8 6a  |.............i.j|
000000e0  d3 00 c3 43 f0 00 a2 93  7c 22 00 00 04 0e 03 80  |...C....|"......|
000000f0  00 00 00 00 20 80 00 00  7f fe 9c 8a 80 94 86 84  |.... ...........|
00000100  99 0c a0 95 2a 8b d8 3a  92 f5 74 7d 56 fe b7 ec  |....*..:..t}V...|
00000110  e8 0d 41 69 7c 00 0e f0  61 42 9c f0 27 38 86 2a  |..Ai|...aB..'8.*|
00000120  da 62 36 3c 8f eb c8 27  1b 77 6f b9 4c be 36 2b  |.b6<...'.wo.L.6+|
00000130  e4 26 1d c1 4f dc d9 01  16 24 11 9a e0 91 02 00  |.&..O....$......|
00000140  7a ea 61 9d b4 e1 52 f6  1f 22 ae df 26 28 3e e0  |z.a...R.."..&(>.|
00000150  f6 be df 90 df b8 01 3f  8e 86 bf 7e 67 1f 83 8f  |.......?...~g...|
00000160  20 51 53 60 46 60 30 43  c3 3d cf 12 84 b7 10 c4  | QS` + "`F`" + `0C.=......|
00000170  33 53 3d 25 48 b0 14 00  00 04 81 28 60 13 84 81  |3S=%H......(` + "`" + `...|
00000180  08 54 13 85 40 e8 60 12  85 01 38 5c 67 b7 67 a5  |.T..@.` + "`" + `...8\g.g.|
00000190  ff 4e 71 cd d3 78 27 29  0e 5c ed d9 d7 cc 7e 04  |.Nq..x').\....~.|
000001a0  f8 09 c3 73 a0 40 70 d9  6d 6a 75 6e 6b d3 00 c3  |...s.@p.mjunk...|
000001b0  44 90 00 67 00 97 62 00  00 21 18 00 c0 08 00 00  |D..g..b..!......|
000001c0  00 20 01 00 00 7f fe ae  be 90 98 a6 9c b4 00 00  |. ..............|
000001d0  00 08 c1 4b c1 32 f8 0b  08 c5 83 c8 01 e8 25 3f  |...K.2........%?|
000001e0  74 7c c4 02 a0 4b c1 47  90 12 86 62 72 92 28 53  |t|...K.G...br.(S|
000001f0  18 9d 8d 85 82 c6 e1 8a  6a 2f dd 5e cd d3 e1 1a  |........j/.^....|
00000200  15 01 a1 2b dc 56 3f c4  ea c0 5e dc 40 48 d3 80  |...+.V?...^.@H..|
00000210  b2 25 60 9c 7b 7e 32 dd  3e 22 f7 01 b6 f3 81 af  |.%` + "`" + `.{~2.>"......|
00000220  b7 1f 78 e0 7f 6c aa fe  9a 7e 7e 94 9f bf 06 72  |..x..l...~~....r|
00000230  3f 15 8c b1 44 56 e1 b1  92 dc b5 37 4a d4 5d 17  |?...DV.....7J.].|
00000240  38 4e 30 24 14 00 04 c1  50 3e 0f 85 41 40 52 13  |8N0$....P>..A@R.|
00000250  85 61 50 5a 16 04 a1 38  12 5b 24 7e 03 6c 07 89  |.aPZ...8.[$~.l..|
00000260  db 93 bd ba 0d 34 27 68  75 d0 a6 72 24 e4 88 dc  |.....4'hu..r$...|
00000270  61 a9 40 b1 9d 0d d3 00  aa 46 70 00 66 ff bc a0  |a.@......Fp.f...|
00000280  00 00 00 04 00 26 18 00  00 00 20 02 00 00 75 53  |.....&.... ...uS|
00000290  fa 82 42 62 9a 80 00 00  06 95 4e a7 a0 bf 1e 78  |..Bb......N....x|
000002a0  7f 0a 10 08 18 7f 35 04  ab ee 50 77 8a 86 f0 51  |......5...Pw...Q|
000002b0  f1 4d 82 46 38 29 0a 8c  35 57 23 87 82 24 2a 01  |.M.F8)..5W#..$*.|
000002c0  b5 40 07 eb c5 01 37 a8  80 b3 88 03 23 c4 fc 61  |.@....7.....#..a|
000002d0  e0 4f 33 c4 73 31 cd 90  54 b2 02 70 90 26 0b 42  |.O3.s1..T..p.&.B|
000002e0  d0 9c 2b 0c 02 97 f4 08  3d 9e c7 b2 6e 44 0f 19  |..+.....=...nD..|
000002f0  48 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00  |H...............|
00000300  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00  |................|
00000310  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00  |................|
00000320  00 00 00 e5 1e d8 d3 00  aa 46 70 00 66 ff bc a0  |.........Fp.f...|
00000330  00 00 00 04 00 26 18 00  00 00 20 02 00 00 75 53  |.....&.... ...uS|
00000340  fa 82 42 62 9a 80                                 |..Bb..|

type 1077 GPS Full Pseudoranges and PhaseRanges plus CNR (high resolution)
stationID 0, timestamp 432023000, multiple message, sequence number 0
session transmit time 0, clock steering 0, external clock 0
divergence free smoothing false, smoothing interval 0
8 satellites, 2 signal types, 16 signals
Satellite ID {approx range m, extended info, phase range rate}:
 4 {24410542.339, 0, -135}
 9 {25264833.738, 0, 182}
16 {22915678.774, 0, 597}
18 {21506595.669, 0, 472}
25 {23345166.602, 0, -633}
26 {20661965.550, 0, 292}
29 {21135953.821, 0, -383}
31 {21670837.435, 0, -442}
Signals: sat ID sig ID {range m, phase range, lock time ind, half cycle ambiguity, Carrier Noise Ratio, phase range rate}:
 4  2 {24410527.355, 128282115.527, 513, false, 80, -136.207}
 4 16 {24410523.313, 99955313.523, 320, false, 82, -134.869}
 9 16 {25264751.952, 103451227.387, 606, false, 78, 182.267}
16  2 {22915780.724, 120426622.169, 40, true, 86, 597.233}
18  2 {21506547.550, 113021883.224, 968, false, 84, 471.599}
18 16 {21506542.760, 88061705.706, 37, false, 90, 472.270}
25  2 {23345103.037, 122677706.869, 51, true, 88, -632.317}
25 16 {23345100.838, 95594616.762, 78, false, 74, -634.411}
26  2 {20662003.308, 108581645.522, 329, false, 78, 291.597}
26 16 {20662000.914, 84606022.946, 80, false, 18, 290.429}
29  2 {21136079.188, 111074207.143, 664, false, 364, -382.650}
29 16 {21136074.598, 86547263.526, 787, false, 583, -382.997}
31  2 {21670772.711, 113885408.778, 710, true, 896, -443.036}
31 16 {21670767.783, 88736342.592, 779, false, 876, -441.969}
`

	const wantCrazy1005 = `message type 1005, frame length 225
00000000  d3 00 db 43 50 00 67 00  97 62 00 00 08 40 a0 65  |...CP.g..b...@.e|
00000010  00 00 00 00 20 00 80 00  6d ff a8 aa 26 23 a6 a2  |.... ...m...&#..|
00000020  23 24 00 00 00 00 36 68  cb 83 7a 6f 9d 7c 04 92  |#$....6h..zo.|..|
00000030  fe f2 05 b0 4a a0 ec 7b  0e 09 27 d0 3f 23 7c b9  |....J..{..'.?#|.|
00000040  6f bd 73 ee 1f 01 64 96  f5 7b 27 46 f1 f2 1a bf  |o.s...d..{'F....|
00000050  19 fa 08 41 08 7b b1 1b  67 e1 a6 70 71 d9 df 0c  |...A.{..g..pq...|
00000060  61 7f 19 9c 7e 66 66 fb  86 c0 04 e9 c7 7d 85 83  |a...~ff......}..|
00000070  7d ac ad fc be 2b fc 3c  84 02 1d eb 81 a6 9c 87  |}....+.<........|
00000080  17 5d 86 f5 60 fb 66 72  7b fa 2f 48 d2 29 67 08  |.]..` + "`" + `.fr{./H.)g.|
00000090  c8 72 15 0d 37 ca 92 a4  e9 3a 4e 13 80 00 14 04  |.r..7....:N.....|
000000a0  c0 e8 50 16 04 c1 40 46  17 05 41 70 52 17 05 01  |..P...@F..ApR...|
000000b0  ef 4b de 70 4c b1 af 84  37 08 2a 77 95 f1 6e 75  |.K.pL...7.*w..nu|
000000c0  e8 ea 36 1b dc 3d 7a bc  75 42 80 00 00 00 00 00  |..6..=z.uB......|
000000d0  00 00 00 00 00 00 00 00  00 00 00 00 00 00 0c 2d  |...............-|
000000e0  f3                                                |.|

expected message type 1005 got 1077
`
	const wantCrazyMSM4 = `message type 1124, frame length 225
00000000  d3 00 db 43 50 00 67 00  97 62 00 00 08 40 a0 65  |...CP.g..b...@.e|
00000010  00 00 00 00 20 00 80 00  6d ff a8 aa 26 23 a6 a2  |.... ...m...&#..|
00000020  23 24 00 00 00 00 36 68  cb 83 7a 6f 9d 7c 04 92  |#$....6h..zo.|..|
00000030  fe f2 05 b0 4a a0 ec 7b  0e 09 27 d0 3f 23 7c b9  |....J..{..'.?#|.|
00000040  6f bd 73 ee 1f 01 64 96  f5 7b 27 46 f1 f2 1a bf  |o.s...d..{'F....|
00000050  19 fa 08 41 08 7b b1 1b  67 e1 a6 70 71 d9 df 0c  |...A.{..g..pq...|
00000060  61 7f 19 9c 7e 66 66 fb  86 c0 04 e9 c7 7d 85 83  |a...~ff......}..|
00000070  7d ac ad fc be 2b fc 3c  84 02 1d eb 81 a6 9c 87  |}....+.<........|
00000080  17 5d 86 f5 60 fb 66 72  7b fa 2f 48 d2 29 67 08  |.]..` + "`" + `.fr{./H.)g.|
00000090  c8 72 15 0d 37 ca 92 a4  e9 3a 4e 13 80 00 14 04  |.r..7....:N.....|
000000a0  c0 e8 50 16 04 c1 40 46  17 05 41 70 52 17 05 01  |..P...@F..ApR...|
000000b0  ef 4b de 70 4c b1 af 84  37 08 2a 77 95 f1 6e 75  |.K.pL...7.*w..nu|
000000c0  e8 ea 36 1b dc 3d 7a bc  75 42 80 00 00 00 00 00  |..6..=z.uB......|
000000d0  00 00 00 00 00 00 00 00  00 00 00 00 00 00 0c 2d  |...............-|
000000e0  f3                                                |.|

message type 1077 is not an MSM4
`

	const wantCrazyMSM7 = `message type 1097, frame length 42
00000000  d3 04 32 43 20 01 00 00  00 04 00 00 08 00 00 00  |..2C ...........|
00000010  00 00 00 00 20 00 80 00  60 28 00 40 01 00 02 00  |.... ...` + "`" + `(.@....|
00000020  00 40 00 00 68 8e 80 6e  75 44                    |.@..h..nuD|

message type 1074 is not an MSM7
`

	// nonRTCMMessage is a Non-RTCM message built from a frame that
	// doesn't contain any RTCM material.
	nonRTCMMessage := NewNonRTCM(testdata.AllJunk)

	// messageFromShortFrame is a message built from a frame too short
	// to make sense of - the embedded message is only one byte long so
	// the message length is incomplete.  The display will contain an
	// error message.
	messageFromShortFrame := NewMessage(utils.MessageType1005, "", testdata.MessageFrameType1005[:7])

	// message1024 is a message of type 1024.  It's not handled and
	// displaying it produces a Readable field which is just a
	// string containing a warning message.
	message1024 := NewMessage(1024, "", testdata.UnhandledMessageType1024)

	// message1005 is a message type 1005 - base position.
	message1005 := NewMessage(utils.MessageType1005, "", testdata.MessageFrameType1005)

	// A message containing an MSM4 or an MSM7 has a date attached.
	// Use this one.
	utcTime := time.Date(2023, time.February, 14, 1, 2, 3, int(4*time.Millisecond), utils.LocationUTC)

	// completeMessage has a header, satellites and Signals.
	msm4 := createMSM4()
	completeMSM4Message := createRTCMWithMSM4(msm4, utcTime)

	// Work out the values for the template and produce the wanted string for the
	// complete message.

	approxRangeScaled := utils.GetScaledRange(wantRangeWhole, wantRangeFractional, 0)

	const scaleFactor = 0x20000000
	approxRangeInMillis := float64(approxRangeScaled) / float64(scaleFactor)

	// Use the speed of light to convert that to the distance from the
	// satellite to the receiver.
	approxRangeInMetres := approxRangeInMillis * utils.OneLightMillisecond

	rangeFromSignal := msm4.Signals[0][0].RangeInMetres()

	phaseRangeFromSignal := msm4.Signals[0][0].PhaseRange()

	wantCompleteMSM4 :=
		fmt.Sprintf(resultTemplateMSM4Complete, approxRangeInMetres, rangeFromSignal, phaseRangeFromSignal)

	// The MSM4 within incompleteMessage has just a header
	incompleteMSM4 := createMSM4()
	incompleteMSM4.Satellites = nil
	incompleteMSM4.Signals = nil

	incompleteMessage := NewMessage(utils.MessageTypeMSM4GPS, "", testdata.MessageFrameType1074_1)
	incompleteMessage.Readable = incompleteMSM4

	// testdata.MessageBatchWithJunk starts with a message type 1077 (a GPS MSM7)
	msm7, createError := msm7message.GetMessage(testdata.MessageBatchWithJunk)
	if createError != nil {
		t.Error(createError)
	}

	completeMSM7Message := NewMessage(msm7.Header.MessageType, "", testdata.MessageBatchWithJunk[3:841])
	completeMSM7Message.Readable = msm7
	completeMSM7Message.UTCTime = &utcTime

	// These messages have the wrong message type, which
	// String() treats as special cases.
	crazy1005 := NewMessage(utils.MessageType1005, "", testdata.MessageFrameType1077)
	crazyMSM4 := NewMessage(utils.MessageTypeMSM4Beidou, "", testdata.MessageFrameType1077)
	// This one is an MSM4 but the message type is forced to be MSM7.
	crazyMSM7 := NewMessage(utils.MessageTypeMSM7Galileo, "", testdata.MessageFrameType1074_1)
	crazyMSM7.MessageType = utils.MessageTypeMSM7Galileo

	var testData = []struct {
		description string
		message     *Message
		want        string
	}{
		{"not handled", message1024, want1024},
		{"no RTCM", nonRTCMMessage, wantNonRTCM},
		{"short frame", messageFromShortFrame, wantShortFrame},
		{"complete MSM4", completeMSM4Message, wantCompleteMSM4},
		{"incomplete MSM4", incompleteMessage, wantIncompleteMSM4},
		{"1005", message1005, want1005},
		{"complete MSM7", completeMSM7Message, wantMSM7},
		{"crazy 1005", crazy1005, wantCrazy1005},
		{"crazy MSM4", crazyMSM4, wantCrazyMSM4},
		{"crazy MSM7", crazyMSM7, wantCrazyMSM7},
	}

	for _, td := range testData {
		got := td.message.String()

		if td.want != got {
			t.Errorf("%s\n%s", td.description, diff.Diff(td.want, got))
		}
	}
}

// TestStringWithNilReadable checks that a String fills in the Readable field
// of a message when it's nil.
func TestStringWithNilReadable(t *testing.T) {
	message := NewMessage(utils.MessageTypeMSM4GPS, "", testdata.MessageFrameType1074_2)

	if message.Readable != nil {
		t.Error("expected the Readable part to be nil")
	}

	_ = message.String()

	if message.Readable == nil {
		t.Error("expected the Readable part to be not nil after String has been called")
	}
}

// TestCopy checks that Copy copies a message.
func TestCopy(t *testing.T) {

	const wantType = utils.MessageTypeMSM4QZSS
	const wantWarning = "a warning"
	const wantValid = false
	const wantComplete = false
	const wantCRCValid = false
	var wantUTCTime *time.Time = nil
	var wantReadable interface{} = nil
	wantBitstream := testdata.UnhandledMessageType1024

	firstMessage := NewMessage(wantType, wantWarning, wantBitstream)

	message := firstMessage.Copy()

	if wantType != message.MessageType {
		t.Errorf("want %d got %d", wantType, message.MessageType)
	}

	if wantWarning != message.ErrorMessage {
		t.Errorf("want %s got %s", wantWarning, message.ErrorMessage)
	}

	// Can't compare the bitstreams so convert them to strings.
	want := string(wantBitstream)
	got := string(message.RawData)
	if want != got {
		t.Errorf("want %s got %s", want, got)
	}

	// Check the fields that should never be set by Copy

	if wantUTCTime != message.UTCTime {
		t.Errorf("want %v got %v", wantUTCTime, message.UTCTime)
	}

	if wantReadable != message.Readable {
		t.Errorf("want %v got %v", wantReadable, message.Readable)
	}
}

// TestDispayable checks the displayable function.
func TestDispayable(t *testing.T) {
	var testData = []struct {
		messageType int
		want        bool
	}{
		{utils.NonRTCMMessage, false},
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
		message := NewMessage(td.messageType, "", nil)
		got := message.displayable()
		if got != td.want {
			t.Errorf("%d: want %v, got %v", td.messageType, td.want, got)
		}
	}
}
