package handler

import (
	"bufio"
	"bytes"
	"math"
	"testing"
	"time"

	"github.com/goblimey/go-ntrip/rtcm/header"
	"github.com/goblimey/go-ntrip/rtcm/pushback"
	"github.com/goblimey/go-ntrip/rtcm/testdata"
	"github.com/goblimey/go-ntrip/rtcm/type1005"
	"github.com/goblimey/go-ntrip/rtcm/type1006"
	msm4message "github.com/goblimey/go-ntrip/rtcm/type_msm4/message"
	msm4satellite "github.com/goblimey/go-ntrip/rtcm/type_msm4/satellite"
	msm4signal "github.com/goblimey/go-ntrip/rtcm/type_msm4/signal"
	msm7message "github.com/goblimey/go-ntrip/rtcm/type_msm7/message"
	"github.com/goblimey/go-ntrip/rtcm/utils"

	"github.com/kylelemons/godebug/diff"
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

// timestampAtEndOfWeek is the value of a GPS, Galileo and Beidou
// timestamp at the end of the week, just before it rolls over.
const timestampAtEndOfWeek uint = (7 * 24 * 3600 * 1000) - 1

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
	var wantContents0 = testdata.MessageBatchWith1077[:226]
	const wantType1 = utils.NonRTCMMessage
	const wantLength1 = 9
	var wantContents1 = testdata.MessageBatchWith1077[226:]

	reader := bytes.NewReader(testdata.MessageBatchWith1077)

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
	rtcmHandler.HandleMessages(ch_source, ch_result)

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
		{"1006", testdata.MessageFrameType1006, utils.MessageType1006},
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

	// messageFrameWithLengthZero is a valid message frame but the length of the
	// contained message is zero.
	messageFrameWithLengthZero := []byte{0xd3, 0x00, 0x00, 0x44}

	var testData = []struct {
		description string
		frame       []byte
		wantError   string
		wantWarning string
	}{

		// GetMessage should return a nil message and an error.
		{"very short", messageFrame1077NoTimestamp,
			"incomplete message frame", ""},

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
				t.Errorf("%s: want warning - %s, got %s",
					td.description, td.wantWarning, gotMessage.ErrorMessage)
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
	rtcmHandler.HandleMessages(ch_source, ch_result)

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

	if message.Signals[0][0].Satellite.ID != 4 {
		t.Errorf("expected satelliteID 4, got %d",
			message.Signals[0][0].Satellite.ID)
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

// TestGetLastSundayUTC tests getLastSundayUTC.
func TestGetLastSundayUTC(t *testing.T) {

	// The 8th August 2020 was a Saturday.
	thisSaturday := time.Date(2020, time.August, 8, 0, 0, 0, 0, utils.LocationUTC)
	lastSunday := time.Date(2020, time.August, 2, 0, 0, 0, 0, utils.LocationUTC)
	thisSunday := time.Date(2020, time.August, 9, 0, 0, 0, 0, utils.LocationUTC)

	var testData = []struct {
		startTime  time.Time
		wantSunday time.Time
	}{
		// The very end of last week.
		{time.Date(2020, time.August, 7, 23, 59, 59, 999999999, utils.LocationUTC), lastSunday},

		// Saturday UTC.
		{thisSaturday, lastSunday},
		{time.Date(2020, time.August, 9, 0, 0, 0, 0, utils.LocationLondon), lastSunday},
		// The very end of Saturday.
		{time.Date(2020, time.August, 8, 23, 59, 59, 999999999, utils.LocationLondon), lastSunday},
		// Sunday
		{thisSunday, thisSunday},
		{time.Date(2020, time.August, 9, 12, 0, 0, 0, utils.LocationLondon), thisSunday},
		// The very end of sunday.
		{time.Date(2020, time.August, 9, 23, 59, 59, 999999999, utils.LocationUTC), thisSunday},
		// Wednesday.
		{time.Date(2020, time.August, 12, 0, 0, 0, 0, utils.LocationLondon), thisSunday},
	}

	for _, td := range testData {

		gotSunday := getStartOfLastSundayUTC(td.startTime)
		if !td.wantSunday.Equal(gotSunday) {
			t.Errorf("%s: expected %s result %s\n",
				td.startTime.Format(time.RFC3339Nano),
				td.wantSunday.Format(time.RFC3339Nano),
				gotSunday)
		}

		// Check that the result really is a Sunday.
		if time.Sunday != gotSunday.Weekday() {
			t.Errorf("%s: want %d got %d",
				td.startTime.Format(time.RFC3339Nano),
				time.Sunday,
				gotSunday.Weekday())
		}
	}

}

// TestStartTimes checks that New sets the correct start of week for GPS, Galileo
// and Beidou.  (Glonass keeps time using a slightly different system).
func TestStartTimes(t *testing.T) {

	// The timestamp is the number of milliseconds since the start of week, which is
	// different for each constellation.

	var testData = []struct {
		description     string
		constellation   string
		messageType     int
		startTime       time.Time
		wantStartOfWeek time.Time
	}{
		{
			"Glonass MSM4 start of the week", "Glonass", 1084,
			time.Date(2023, time.June, 11, 0, 0, 0, 0, utils.LocationMoscow),
			time.Date(2023, time.June, 11, 0, 0, 0, 0, utils.LocationMoscow),
		},
		{
			"Glonass MSM7 end of the week", "Glonass", 1087,
			time.Date(2023, time.June, 17, 23, 59, 59, 999, utils.LocationMoscow),
			time.Date(2023, time.June, 11, 0, 0, 0, 0, utils.LocationMoscow),
		},
		{
			//just before 1am BST is just before midnight UTC.
			"Sunday 2020/08/02 BST, just before the end of the week.", "GPS", 1074,
			time.Date(2020, time.August, 9, 00, 59, 41, int(999*time.Millisecond), utils.LocationLondon),
			time.Date(2020, time.August, 2, 0, 0, 0, 0, utils.LocationUTC).Add(utils.GPSTimeOffset),
		},
		{
			//just before 1am BST is just before midnight UTC.
			"Sunday 2020/08/02 BST, just before the end of the week.", "GPS", 1077,
			time.Date(2020, time.August, 9, 00, 59, 41, int(999*time.Millisecond), utils.LocationLondon),
			time.Date(2020, time.August, 2, 0, 0, 0, 0, utils.LocationUTC).Add(utils.GPSTimeOffset),
		},
		{
			"Saturday 2nd August, start of week", "GPS", 1077,
			time.Date(2020, time.August, 2, 0, 59, 42, 0, utils.LocationLondon),
			time.Date(2020, time.August, 2, 0, 0, 0, 0, utils.LocationUTC).Add(utils.GPSTimeOffset),
		},
		{
			"Wednesday 2020/08/05", "GPS", 1077,
			time.Date(2020, time.August, 5, 12, 0, 0, 0, utils.LocationLondon),
			time.Date(2020, time.August, 2, 0, 0, 0, 0, utils.LocationUTC).Add(utils.GPSTimeOffset),
		},
		{
			//just before 1am BST is just before midnight UTC.
			"Sunday 2020/08/02 BST, just before the end of the week.", "GPS", 1077,
			time.Date(2020, time.August, 9, 00, 59, 41, int(999*time.Millisecond), utils.LocationLondon),
			time.Date(2020, time.August, 2, 0, 0, 0, 0, utils.LocationUTC).Add(utils.GPSTimeOffset),
		},
		{
			"Sunday 2020/09/02 CET, at the start of the next week", "GPS", 1077,
			time.Date(2020, time.August, 9, 1, 59, 42, 0, utils.LocationParis),
			time.Date(2020, time.August, 8, 23, 59, 42, 0, utils.LocationUTC),
		},
		{
			"Sunday 2020/09/02 CET, just after the start of the next week", "GPS", 1077,
			time.Date(2020, time.August, 9, 1, 59, 42, int(999*time.Millisecond), utils.LocationParis),
			time.Date(2020, time.August, 8, 23, 59, 42, 0, utils.LocationUTC),
		},
		{
			"Saturday 2nd August, start of week", "Galileo", 1094,
			time.Date(2020, time.August, 2, 0, 59, 42, 0, utils.LocationLondon),
			time.Date(2020, time.August, 1, 23, 59, 42, 0, utils.LocationUTC),
		},
		{
			"Saturday 2nd August, start of week", "Galileo", 1097,
			time.Date(2020, time.August, 2, 0, 59, 42, 0, utils.LocationLondon),
			time.Date(2020, time.August, 1, 23, 59, 42, 0, utils.LocationUTC),
		},
		{
			"Wednesday 2020/08/05", "Galileo", 1097,
			time.Date(2020, time.August, 5, 12, 0, 0, 0, utils.LocationLondon),
			time.Date(2020, time.August, 1, 23, 59, 42, 0, utils.LocationUTC),
		},
		{
			"Sunday 2020/08/02 BST, just before the end of the week.", "Galileo", 1097,
			time.Date(2020, time.August, 9, 00, 59, 41, int(999*time.Millisecond), utils.LocationLondon),
			time.Date(2020, time.August, 1, 23, 59, 42, 0, utils.LocationUTC),
		},
		{
			"Sunday 2020/09/02 CET, at the start of the next week", "Galileo", 1097,
			time.Date(2020, time.August, 9, 1, 59, 43, 0, utils.LocationParis),
			time.Date(2020, time.August, 8, 23, 59, 42, 0, utils.LocationUTC),
		},
		{
			"Sunday 2020/09/02 CET, just after the start of the next week", "Galileo", 1097,
			time.Date(2020, time.August, 9, 1, 59, 44, 0, utils.LocationParis),
			time.Date(2020, time.August, 8, 23, 59, 42, 0, utils.LocationUTC),
		},
		{
			// This start time should be at the end of the previous week ...
			"Saturday 8th August, end of week", "Beidou", 1124,
			time.Date(2020, time.August, 8, 23, 59, 55, int(999*time.Millisecond), utils.LocationUTC),
			time.Date(2020, time.August, 2, 0, 0, 0, 0, utils.LocationUTC).Add(utils.BeidouTimeOffset),
		},
		{
			// This start time should be at the end of the previous week ...
			"Saturday 8th August, end of week", "Beidou", 1127,
			time.Date(2020, time.August, 8, 23, 59, 55, int(999*time.Millisecond), utils.LocationUTC),
			time.Date(2020, time.August, 2, 0, 0, 0, 0, utils.LocationUTC).Add(utils.BeidouTimeOffset),
		},
		{
			// ... and this one too.
			"Sunday 9th Aug", "Beidou", 1127,
			time.Date(2020, time.August, 9, 1, 0, 0, 0, utils.LocationUTC),
			time.Date(2020, time.August, 9, 0, 0, 0, 0, utils.LocationUTC).Add(utils.BeidouTimeOffset),
		},
		{
			// Saturday 15th, just before rollover.
			"4", "Beidou", 1127,
			time.Date(2020, time.August, 15, 23, 59, 55, int(999*time.Millisecond), utils.LocationUTC),
			time.Date(2020, time.August, 9, 0, 0, 0, 0, utils.LocationUTC).Add(utils.BeidouTimeOffset),
		},
		{
			"start of next week", "Beidou", 1127,
			time.Date(2020, time.August, 15, 23, 59, 56, 0, utils.LocationUTC),
			time.Date(2020, time.August, 16, 0, 0, 0, 0, utils.LocationUTC).Add(utils.BeidouTimeOffset),
		},
		{
			"one second after the start of the next week", "Beidou", 1127,
			time.Date(2020, time.August, 15, 23, 59, 57, 0, utils.LocationUTC),
			time.Date(2020, time.August, 16, 0, 0, 0, 0, utils.LocationUTC).Add(utils.BeidouTimeOffset),
		},
	}

	for _, td := range testData {

		h := New(td.startTime)
		startOfWeek, err := h.getStartOfWeek(td.messageType)
		if err != nil {
			t.Error(err)
		}
		if !td.wantStartOfWeek.Equal(startOfWeek) {
			t.Errorf("%s %s: want %s got %s\n",
				td.constellation, td.description,
				td.wantStartOfWeek.Format(time.RFC3339Nano),
				startOfWeek.Format(time.RFC3339Nano))
		}
	}
}

func TestGlonassStartTimes(t *testing.T) {

	var testData = []struct {
		description            string
		startTime              time.Time
		wantStartOfWeek        time.Time
		timestamp1             uint
		wantTimeFromTimestamp1 time.Time
		wantDay1               uint
		timestamp2             uint
		wantTimeFromTimestamp2 time.Time
		wantDay2               uint
	}{
		{
			"21:00 on Monday 3rd August UTC, 00:00 on the 4th in Moscow",
			time.Date(2020, time.August, 2, 5, 0, 0, 0, utils.LocationUTC),
			time.Date(2020, time.August, 1, 21, 0, 0, 0, utils.LocationUTC),
			// One day and four hours.
			uint((1 << 27) + (4 * 3600 * 1000)),
			time.Date(2020, time.August, 3, 1, 0, 0, 0, utils.LocationUTC),
			1,
			// Start of day 2.
			uint(2 << 27),
			time.Date(2020, time.August, 3, 21, 0, 0, 0, utils.LocationUTC),
			2,
		},
		{
			"22:59 on Monday 3rd August in Paris, 20:59 UTC, 23:59 on Sunday 2nd Moscow",
			time.Date(2020, time.August, 3, 22, 59, 59, 999999999, utils.LocationParis),
			time.Date(2020, time.August, 2, 0, 0, 0, 0, utils.LocationMoscow),
			// Two days and five hours.
			uint((2 << 27) + (5 * 3600 * 1000)),
			time.Date(2020, time.August, 4, 2, 00, 0, 0, utils.LocationUTC),
			2,
			// Day 3, 23:00.
			uint((3 << 27) + (23 * 3600 * 1000)),
			time.Date(2020, time.August, 5, 23, 0, 0, 0, utils.LocationMoscow),
			3,
		},
		{
			"11pm on Monday 3rd August in Paris, 00:00 on Tuesday 4th in Moscow",
			time.Date(2020, time.August, 3, 23, 00, 00, 0, utils.LocationParis),
			time.Date(2020, time.August, 1, 21, 0, 0, 0, utils.LocationUTC),
			// Day 3 11:00.
			uint((3 << 27) + (11 * 3600 * 1000)),
			time.Date(2020, time.August, 5, 8, 0, 0, 0, utils.LocationUTC),
			3,
			// Day 2 11:00 - roll over to week commencing 9th in Moscow.
			uint((2 << 27) + (11 * 3600 * 1000)),
			time.Date(2020, time.August, 11, 8, 0, 0, 0, utils.LocationUTC),
			2,
		},
	}

	for _, td := range testData {

		handler := New(td.startTime)

		if !td.wantStartOfWeek.Equal(handler.startOfGlonassWeek) {
			t.Errorf("start - %s: want %s got %s\n",
				td.description,
				td.wantStartOfWeek.Format(time.RFC3339Nano),
				handler.startOfGlonassWeek.Format(time.RFC3339Nano))
		}

		timeFromTimestamp, err := handler.getUTCFromGlonassTime(td.timestamp1)
		if err != nil {
			t.Error(err)
		}

		if !td.wantTimeFromTimestamp1.Equal(timeFromTimestamp) {
			t.Errorf("time1 - %s: want %s got %s\n",
				td.description,
				td.wantTimeFromTimestamp1.Format(time.RFC3339Nano),
				timeFromTimestamp.Format(time.RFC3339Nano))
		}

		if td.wantDay1 != handler.glonassDayFromPreviousMessage {
			t.Errorf("day1 - %s: want %d got %d\n",
				td.description,
				td.wantDay1, handler.glonassDayFromPreviousMessage)
		}

		timeFromTimestamp2, err := handler.getUTCFromGlonassTime(td.timestamp2)
		if err != nil {
			t.Error(err)
		}

		if !td.wantTimeFromTimestamp2.Equal(timeFromTimestamp2) {
			t.Errorf("time2 - %s: want %s got %s\n",
				td.description,
				td.wantTimeFromTimestamp2.Format(time.RFC3339Nano),
				timeFromTimestamp2.Format(time.RFC3339Nano))
		}

		if td.wantDay2 != handler.glonassDayFromPreviousMessage {
			t.Errorf("day2 - %s: want %d got %d\n",
				td.description,
				td.wantDay2, handler.glonassDayFromPreviousMessage)
		}
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

// TestAnalyseWith1005 checks that Analyse correctly handles a message type 1005 (base position).
func TestAnalyseWith1005(t *testing.T) {

	message := NewMessage(utils.MessageType1005, "", testdata.MessageFrameType1005)

	Analyse(message)

	if message.Readable == nil {
		t.Error("Readable is nil")
		return
	}

	_, ok := message.Readable.(*type1005.Message)

	if !ok {
		t.Error("expecting Readable to contain a message type 1005")
	}
}

// TestAnalyseWith1006 checks that Analyse correctly handles a message type 1006 (base position and height).
func TestAnalyseWith1006(t *testing.T) {

	message := NewMessage(utils.MessageType1006, "", testdata.MessageFrameType1006)

	Analyse(message)

	if message.Readable == nil {
		t.Error("Readable is nil")
		return
	}

	_, ok := message.Readable.(*type1006.Message)

	if !ok {
		t.Error("expecting Readable to contain a message type 1006")
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

func createHeader(messageType int, timestamp uint) *header.Header {
	return header.New(messageType, 0, timestamp, false, 0, 0, 0, 0, false, 0, 0, 0, 0)
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
		wantTimeFromTimestamp3 time.Time // The time from timestamp3.
	}{

		{
			"Beidou",
			// midnight Sunday 9th August - 8th just before midnight in Beidou time,
			// start of the Beidou week.  Stored timestamp is zero.
			time.Date(2020, time.August, 9, 0, 0, 0, 0, utils.LocationUTC).Add(utils.BeidouTimeOffset),
			utils.MessageTypeMSM7Beidou,
			// wantStartOfWeek1
			time.Date(2020, time.August, 8, 23, 59, 56, 0, utils.LocationUTC),
			// First messages of the week.
			0,
			// wantTimeFromTimestamp1
			time.Date(2020, time.August, 8, 23, 59, 56, 0, utils.LocationUTC),
			timestampAtEndOfWeek, // just before end of week.
			// wantTimeFromTimestamp1
			time.Date(2020, time.August, 14, 23, 59, 55, 999, utils.LocationUTC),
			500, // rolled over
			// wantStartOfWeek2
			time.Date(2020, time.August, 15, 23, 59, 56, 0, utils.LocationUTC),
			// wantTimeFromTimestamp
			time.Date(2020, time.August, 15, 23, 59, 56, int(500*time.Millisecond), utils.LocationUTC),
		},

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
			"Glonass",
			// StartTime - 22:00:00 on Monday 10th August Paris is midnight on the Tuesday
			// 11th in Russia - start of Glonass day 2.
			time.Date(2020, time.August, 10, 22, 0, 0, 0, utils.LocationParis),
			utils.MessageTypeMSM7Glonass,
			// wantStartOfWeek1 - 9th August 00:00 in Moscow, 8th August 21:00 UTC.
			time.Date(2020, time.August, 9, 0, 0, 0, 0, utils.LocationMoscow),
			// timestamp1 - Day = 2, hour = 4, which is 4am on Russian Tuesday,
			// which is 1am on Tuesday 11th UTC.
			2<<27 | (2 * 3600 * 1000),
			// timeFromTimestamp1
			time.Date(2020, time.August, 10, 23, 0, 0, 0, utils.LocationUTC),
			3<<27 | (5 * 3600 * 1000), // 5am next day in Moscow, 2am the next day in UTC.
			time.Date(2020, time.August, 11, 2, 0, 0, 0, utils.LocationUTC),
			1<<27 | (18 * 3600 * 1000), // rolled over to the next week
			// wantStartOfWeek2 - midnight 16th Aug in Moscow.
			time.Date(2020, time.August, 16, 0, 0, 0, 0, utils.LocationMoscow),
			// wantTimeFromTimestamp3 - Day = 1, hour = 18, which is 6pm on Russian
			// Monday, 17:00 Monday CET.
			time.Date(2020, time.August, 17, 17, 0, 0, 0, utils.LocationParis),
		},
		{
			"Galileo",
			// Monday 10th Aug.  Paris is two hours ahead of UTC.
			time.Date(2020, time.August, 10, 23, 0, 0, 0, utils.LocationParis),
			utils.MessageTypeMSM7Galileo,
			time.Date(2020, time.August, 8, 23, 59, 42, 0, utils.LocationUTC),
			(((52*3600)+1800)*1000 + 300), // 2 days, 4.5 hours  plus 300 ms in ms.
			time.Date(2020, time.August, 10, 04, 29, 42, int(300*time.Millisecond), utils.LocationUTC).
				Add(utils.GPSTimeOffset),
			(((74*3600)+30)*1000 + 700), // 3 days 2 hours 30 secondsand 400 ms.
			time.Date(2020, time.August, 12, 23, 59, 30, int(700*time.Millisecond), utils.LocationUTC),
			((2 * 3600 * 1000) + 4), // rolled over to the next day
			// 2020-08-08 23:59:42.

			time.Date(2020, time.August, 16, 00, 00, 00, 0, utils.LocationUTC).
				Add(utils.GPSTimeOffset),
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
		startOfPeriod1 := getStartOfWeek(td.messageType, handler)

		if !td.wantStartOfWeek1.Equal(startOfPeriod1) {
			t.Errorf("%s: startOfWeek1 want %s got %s",
				td.description, td.wantStartOfWeek1.Format(utils.DateLayout),
				startOfPeriod1.Format(utils.DateLayout))
		}

		_, err2 := handler.getTimeFromTimeStamp(td.messageType, td.timestamp2)

		if err2 != nil {
			t.Errorf("%s: %v", td.description, err2)
		}

		// Should be no rollover.
		if !td.wantStartOfWeek1.Equal(startOfPeriod1) {
			t.Errorf("%s: startOfWeek1 want %s got %s",
				td.description, td.wantStartOfWeek1.Format(utils.DateLayout),
				startOfPeriod1.Format(utils.DateLayout))
		}

		timeFromTimestamp, err3 := handler.getTimeFromTimeStamp(td.messageType, td.timestamp3)

		if err3 != nil {
			t.Errorf("%s: %v", td.description, err3)
		}

		// That should have provoked a rollover ...

		// ...so we should be in a new period ...

		// Get the start of the week for this message, or the
		// start of day if Glonass.
		startOfWeek2 := getStartOfWeek(td.messageType, handler)

		if !td.wantStartOfWeek2.Equal(startOfWeek2) {
			t.Errorf("%s: startOfWeek2 want %s got %s",
				td.description, td.wantStartOfWeek2.Format(utils.DateLayout),
				startOfWeek2.Format(utils.DateLayout))
		}

		// ... and so we should get this time from the timestamp.
		if !td.wantTimeFromTimestamp3.Equal(timeFromTimestamp) {
			t.Errorf("%s: timeFromTimestamp want %s got %s",
				td.description, td.wantTimeFromTimestamp3.Format(utils.DateLayout), timeFromTimestamp.Format(utils.DateLayout))
		}
	}
}

// getStartOfWeek is a helper function.   It gets the start of the constellation's
// current period. (For Glonass the start of the current day.  For the rest, the start
// of the current week.)
func getStartOfWeek(messageType int, handler *Handler) time.Time {
	// Get the start of the week for this constellation.
	constellation := utils.GetConstellation(messageType)
	switch constellation {
	case "GPS":
		return handler.startOfGPSWeek
	case "Galileo":
		return handler.startOfGalileoWeek
	case "Glonass":
		return handler.startOfGlonassWeek
	case "Beidou":
		return handler.startOfBeidouWeek
	}

	var zeroTime time.Time
	return zeroTime
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
		{"4 0", createHeader(utils.MessageTypeMSM4Glonass, (7 << 27)), "timestamp out of range"},
		{"7 1", createHeader(utils.MessageTypeMSM7Glonass, ((7 << 27) | 1)), "timestamp out of range"},
		{"7 max", createHeader(utils.MessageTypeMSM7Glonass, ((7 << 27) | (1 << 25))), "timestamp out of range"},
		// This timestamp is an int, which is 32 bits on some machines, 64 on others.  For safety, only
		// set the timestamp to the max value of an int32.
		{"max int32", createHeader(utils.MessageTypeMSM4Glonass, math.MaxInt32), "timestamp out of range"},
		// Try all of the other constellations with a value bigger than 30 bits.
		{"GPS MSM4", createHeader(utils.MessageTypeMSM4GPS, utils.MaxTimestamp+1), "timestamp out of range"},
		{"GPS MSM7", createHeader(utils.MessageTypeMSM7GPS, math.MaxInt32), "timestamp out of range"},
		{"Galileo MSM4", createHeader(utils.MessageTypeMSM4Galileo, utils.MaxTimestamp+1), "timestamp out of range"},
		{"Galileo MSM7", createHeader(utils.MessageTypeMSM7Galileo, math.MaxInt32/2+1), "timestamp out of range"},
		{"Beidou MSM4", createHeader(utils.MessageTypeMSM4Beidou, utils.MaxTimestamp+2), "timestamp out of range"},
		{"Beidou MSM7", createHeader(utils.MessageTypeMSM7Beidou, 0x40000000), "timestamp out of range"},
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
	const want = "timestamp out of range"

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

func TestCheckCRC(t *testing.T) {

	// CRCCheck checks that the frame is at least 6 bytes long.
	shortFrame := []byte{1, 2, 3, 4, 5}

	var testData = []struct {
		description string
		bitStream   []byte
		want        bool
	}{
		{"valid", testdata.MessageBatchWith1077, true},
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
		rtcmHandler.HandleMessages(ch_source, ch_result)

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
const wantTimestamp = 2
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
	hdr := header.New(wantMessageType, wantStationID, wantTimestamp, wantMultipleMessage,
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
func createRTCMWithMSM4(msm4 *msm4message.Message, startOfWeek time.Time) *Message {
	message := NewMessage(utils.MessageTypeMSM4GPS, "", testdata.MessageFrameType1074_1)
	message.Readable = msm4
	// In the real world these values would be set by handler.GetMessage.
	message.SentAt = "Time 2023-02-11 23:59:42.002 +0000 UTC"
	message.StartOfWeek =
		"Start of GPS week 2023-02-11 23:59:42 +0000 UTC plus timestamp 2 (0d 0h 0m 0s 2ms)"

	return message
}

// TestNewMessage checks that NewMessage creates a message correctly.
func TestNewMessage(t *testing.T) {

	const wantType = utils.MessageTypeMSM4QZSS
	const wantWarning = "a warning"
	wantBitstream := testdata.UnhandledMessageType1024
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

	if wantReadable != message.Readable {
		t.Errorf("want %v got %v", wantReadable, message.Readable)
	}
}

// TestNewNonRTCM checks that NewNonRTCM creates a non-RTCM message correctly.
func TestNewNonRTCM(t *testing.T) {

	const wantType = utils.NonRTCMMessage
	const wantWarning = ""
	var wantBitstream = []byte{'j', 'u', 'n', 'k'}
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

	if wantReadable != message.Readable {
		t.Errorf("want %v got %v", wantReadable, message.Readable)
	}
}

// TestString checks the String method using various message types.
func TestString(t *testing.T) {

	const wantNonRTCM = `Message type -1, Non-RTCM data
Data which is not in RTCM3 format, for example NMEA messages.
Frame length 4 bytes:
00000000  6a 75 6e 6b                                       |junk|

`

	const wantShort1005Frame = `Message type 1005, Stationary RTK Reference Station Antenna Reference Point (ARP)
Commonly called the Station Description this message includes the ECEF location of the ARP of the antenna (not the phase center) and also the quarter phase alignment details.  The datum field is not used/defined, which often leads to confusion if a local datum is used. See message types 1006 and 1032. The 1006 message also adds a height about the ARP value.
Frame length 7 bytes:
00000000  d3 00 13 3e d0 02 0f                              |...>...|

overrun - expected 152 bits in a message type 1005, got 8
`

	const wantShort1006Frame = `Message type 1006, Stationary RTK Reference Station ARP with Antenna Height
Commonly called the Station Description this message includes the ECEF location of the antenna (the antenna reference point (ARP) not the phase center) and also the quarter phase alignment details.  The height about the ARP value is also provided. The datum field is not used/defined, which often leads to confusion if a local datum is used. See message types 1005 and 1032. The 1005 message does not convey the height about the ARP value.
Frame length 7 bytes:
00000000  d3 00 15 3e e0 02 0f                              |...>...|

overrun - expected 168 bits in a message type 1006, got 8
`

	const want1024 = `Message type 1024, Residuals, Plane Grid Representation
A coordinate transformation message.  Not often found in actual use.
Frame length 14 bytes:
00000000  d3 00 08 40 00 00 8a 00  00 00 00 4f 5e e7        |...@.......O^.|

message type 1024 currently cannot be displayed
`

	// The satellite mask says that there are two satellites but
	// the multiple message flag is set and not all satellites need
	// to be included.  In this case there's only one satellite.
	const wantCompleteMSM4 = `Message type 1074, GPS Full Pseudoranges and PhaseRanges plus Carrier to Noise Ratio
The type 4 Multiple Signal Message format for the American GPS system.
Time 2023-02-11 23:59:42.002 +0000 UTC
Start of GPS week 2023-02-11 23:59:42 +0000 UTC plus timestamp 2 (0d 0h 0m 0s 2ms)
Frame length 42 bytes:
00000000  d3 04 32 43 20 01 00 00  00 04 00 00 08 00 00 00  |..2C ...........|
00000010  00 00 00 00 20 00 80 00  60 28 00 40 01 00 02 00  |.... ...` + "`" + `(.@....|
00000020  00 40 00 00 68 8e 80 6e  75 44                    |.@..h..nuD|

stationID 1, multiple message, issue of data station 3
session transmit time 4, clock steering 5, external clock 6
divergence free smoothing true, smoothing interval 7
Satellite mask:
0000 0000 0000 0000  0000 0000 0000 0000  0000 0000 0000 0000  0000 0000 0000 0011
Signal mask: 0000 0000 0000 0000  0000 0000 0000 0111
cell mask: fff fft
2 satellites, 3 signal types, 1 signals
Satellite ID {approx range - whole, frac, millis, metres}
 8 {9, 10, 9.010, 2701059.783}
Signals:
Sat ID Sig ID {(range delta, delta m, range m), (phase range delta, cycles) lock time ind, half cycle ambiguity, Carrier Noise Ratio, wavelength}
 8 11 {(12, 0.214, 2701059.997), (13, 168816.237), 14, true, 15, 16.000}
`

	const wantIncompleteMSM4 = `Message type 1074, GPS Full Pseudoranges and PhaseRanges plus Carrier to Noise Ratio
The type 4 Multiple Signal Message format for the American GPS system.
Time 2023-02-11 23:59:42.002 +0000 UTC
Start of GPS week 2023-02-11 23:59:42 +0000 UTC plus timestamp 2 (0d 0h 0m 0s 2ms)
Frame length 42 bytes:
00000000  d3 04 32 43 20 01 00 00  00 04 00 00 08 00 00 00  |..2C ...........|
00000010  00 00 00 00 20 00 80 00  60 28 00 40 01 00 02 00  |.... ...` + "`" + `(.@....|
00000020  00 40 00 00 68 8e 80 6e  75 44                    |.@..h..nuD|

stationID 1, multiple message, issue of data station 3
session transmit time 4, clock steering 5, external clock 6
divergence free smoothing true, smoothing interval 7
Satellite mask:
0000 0000 0000 0000  0000 0000 0000 0000  0000 0000 0000 0000  0000 0000 0000 0011
Signal mask: 0000 0000 0000 0000  0000 0000 0000 0111
cell mask: fff fft
2 satellites, 3 signal types, 1 signals
No Satellites
No Signals
`

	const wantComplete1077Display = testdata.MessageFrameType1077Heading +
		"Time 2023-02-17 00:00:05 +0000 UTC\n" +
		"Start of GPS week 2023-02-11 23:59:42 +0000 UTC plus timestamp 432023000 (5d 0h 0m 23s 0ms)\n" +
		testdata.MessageFrameType1077HexDump +
		testdata.WantHeaderFromMessageFrameType1077 + "\n" +
		testdata.WantSatelliteListFromMessageFrameType1077 + "\n" +
		testdata.WantSignalListFromMessageFrameType1077 + "\n"

	const wantCrazy1005 = `Message type 1005, Stationary RTK Reference Station Antenna Reference Point (ARP)
Commonly called the Station Description this message includes the ECEF location of the ARP of the antenna (not the phase center) and also the quarter phase alignment details.  The datum field is not used/defined, which often leads to confusion if a local datum is used. See message types 1006 and 1032. The 1006 message also adds a height about the ARP value.
Frame length 225 bytes:
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
	const wantCrazyMSM4 = `Message type 1124, BeiDou Full Pseudoranges and PhaseRanges plus Carrier to Noise Ratio
The type 4 Multiple Signal Message format for China’s BeiDou system.
Time 2023-02-11 23:59:42.002 +0000 UTC
Start of GPS week 2023-02-11 23:59:42 +0000 UTC plus timestamp 2 (0d 0h 0m 0s 2ms)
Frame length 225 bytes:
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

	const wantCrazyMSM7 = `Message type 1097, Galileo Full Pseudoranges and PhaseRanges plus Carrier to Noise Ratio (high resolution)
The type 7 Multiple Signal Message format for Europe’s Galileo system.
Time 2023-02-11 23:59:42.002 +0000 UTC
Start of GPS week 2023-02-11 23:59:42 +0000 UTC plus timestamp 2 (0d 0h 0m 0s 2ms)
Frame length 42 bytes:
00000000  d3 04 32 43 20 01 00 00  00 04 00 00 08 00 00 00  |..2C ...........|
00000010  00 00 00 00 20 00 80 00  60 28 00 40 01 00 02 00  |.... ...` + "`" + `(.@....|
00000020  00 40 00 00 68 8e 80 6e  75 44                    |.@..h..nuD|

message type 1074 is not an MSM7
`

	// nonRTCMMessage is a Non-RTCM message built from a frame that
	// doesn't contain any RTCM material.
	nonRTCMMessage := NewNonRTCM(testdata.AllJunk)

	// messageFromShort1005Frame is a message built from a type 1005 frame too
	// short to make sense of - the embedded message is only one byte long so
	// the message length is incomplete.  The display will contain an error message.
	messageFromShort1005Frame := NewMessage(utils.MessageType1005, "", testdata.MessageFrameType1005[:7])

	// messageFromShort1006Frame is a message built from a type 1005 frame too
	// short to make sense of - the embedded message is only one byte long so
	// the message length is incomplete.  The display will contain an error message.
	messageFromShort1006Frame := NewMessage(utils.MessageType1006, "", testdata.MessageFrameType1006[:7])

	// message1024 is a message of type 1024.  It's not handled and
	// displaying it produces a Readable field which is just a
	// string containing a warning message.
	message1024 := NewMessage(1024, "", testdata.UnhandledMessageType1024)

	// message1005 is a message type 1005 - base position.
	message1005 := NewMessage(utils.MessageType1005, "", testdata.MessageFrameType1005)

	// message1006 is a message type 1006 - base position and height.
	message1006 := NewMessage(utils.MessageType1006, "", testdata.MessageFrameType1006)

	// The start times for MSM messages.
	startTime := time.Date(2023, time.February, 14, 1, 2, 3, 0, utils.LocationUTC)
	startOfWeek := time.Date(2023, time.February, 11, 23, 59, 42, 0, utils.LocationUTC)

	// completeMessage has a header, satellites and Signals.
	msm4 := createMSM4()
	completeMSM4Message := createRTCMWithMSM4(msm4, startOfWeek)

	const scaleFactor = 0x20000000

	// The MSM4 within incompleteMessage has just a header
	incompleteMSM4 := createMSM4()
	incompleteMSM4.Satellites = nil
	incompleteMSM4.Signals = nil

	incompleteMessage := NewMessage(utils.MessageTypeMSM4GPS, "", testdata.MessageFrameType1074_1)
	incompleteMessage.Readable = incompleteMSM4
	incompleteMessage.SentAt = "Time 2023-02-11 23:59:42.002 +0000 UTC"
	incompleteMessage.StartOfWeek =
		"Start of GPS week 2023-02-11 23:59:42 +0000 UTC plus timestamp 2 (0d 0h 0m 0s 2ms)"

	// These messages have the wrong message type, which are
	// treated as special cases.
	crazy1005 := NewMessage(utils.MessageType1005, "", testdata.MessageFrameType1077)
	crazyMSM4 := NewMessage(utils.MessageTypeMSM4Beidou, "", testdata.MessageFrameType1077)
	crazyMSM4.SentAt = "Time 2023-02-11 23:59:42.002 +0000 UTC"
	crazyMSM4.StartOfWeek =
		"Start of GPS week 2023-02-11 23:59:42 +0000 UTC plus timestamp 2 (0d 0h 0m 0s 2ms)"
	// This one is an MSM4 but the message type is forced to be MSM7.
	crazyMSM7 := NewMessage(utils.MessageTypeMSM7Galileo, "", testdata.MessageFrameType1074_1)
	crazyMSM7.MessageType = utils.MessageTypeMSM7Galileo
	crazyMSM7.SentAt = "Time 2023-02-11 23:59:42.002 +0000 UTC"
	crazyMSM7.StartOfWeek =
		"Start of GPS week 2023-02-11 23:59:42 +0000 UTC plus timestamp 2 (0d 0h 0m 0s 2ms)"

	rtcmHandler := New(startTime)

	completeMSM7Message, err := rtcmHandler.GetMessage(testdata.MessageFrameType1077)
	if err != nil {
		t.Error(err)
	}

	glonassMSM7WithIllegalDay, glonassError := rtcmHandler.GetMessage(testdata.GlonassMSM7WithIllegalDay)
	if glonassError == nil {
		t.Error("expected an error - timestamp out of range")
	}

	var testData = []struct {
		description string
		message     *Message
		want        string
	}{
		{"complete MSM4", completeMSM4Message, wantCompleteMSM4},
		{"complete MSM7", completeMSM7Message, wantComplete1077Display},
		{"glonass with illegal day", glonassMSM7WithIllegalDay, testdata.GlonassMSM7WithIllegalDayDisplay},
		{"1005", message1005, testdata.MessageFrameType1005Display},
		{"1006", message1006, testdata.MessageFrameType1006Display},
		{"not handled", message1024, want1024},
		{"no RTCM", nonRTCMMessage, wantNonRTCM},
		{"short 1005 frame", messageFromShort1005Frame, wantShort1005Frame},
		{"short 1006 frame", messageFromShort1006Frame, wantShort1006Frame},
		{"incomplete MSM4", incompleteMessage, wantIncompleteMSM4},
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
	const wantUTCTime = 0
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

	if wantReadable != message.Readable {
		t.Errorf("want %v got %v", wantReadable, message.Readable)
	}
}

// TestDisplayable checks the displayable function.
func TestDisplayable(t *testing.T) {
	var testData = []struct {
		messageType int
		want        bool
	}{
		{utils.NonRTCMMessage, false},
		{1005, true},
		{1006, true},
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

// TestGetTimeDisplayFromTimestamp checks getTimeDisplayFromTimestamp.
func TestGetTimeDisplayFromTimestamp(t *testing.T) {

	const timestampTooBig = 0x40000000

	var testData = []struct {
		messageType int
		timestamp   uint
		wantError   string
		wantDisplay string
	}{
		{utils.MessageTypeMSM7Galileo, 3 * 24 * 3600 * 1000, "", "Time 2023-02-14 23:59:42 +0000 UTC"},
		{utils.MessageTypeMSM7GPS, timestampTooBig, "timestamp out of range", "Time (timestamp out of range)"},
	}
	for _, td := range testData {

		startTime := time.Date(2023, time.February, 14, 0, 0, 0, 0, utils.LocationUTC)

		h := New(startTime)

		display, displayErr := h.getTimeDisplayFromTimestamp(td.messageType, td.timestamp)

		if len(td.wantError) > 0 {
			if displayErr == nil {
				t.Error("expected an error")
			}

			if td.wantError != displayErr.Error() {
				t.Error(diff.Diff(td.wantError, displayErr.Error()))
			}
		} else {
			if displayErr != nil {
				t.Error(displayErr)
			}
		}

		if td.wantDisplay != display {
			t.Error(diff.Diff(td.wantDisplay, display))
		}
	}
}

// TestGetStartTimeDisplay checks getStartTimeDisplay.
func TestGetStartTimeDisplay(t *testing.T) {

	const maxTimestamp = ((((6*24 + 23) * 3600) + 59*60 + 59) * 1000) + 999
	const timestampTooBig = utils.MaxTimestamp + 1
	const glonassTimestampTooBig = utils.MaxTimestampGlonass + 1

	var testData = []struct {
		messageType int
		timestamp   uint
		wantDisplay string
	}{
		{utils.MessageTypeMSM7Glonass, (6 << 27) + (24 * 3600 * 1000) - 1,
			"Start of Glonass week 2023-02-11 21:00:00 +0000 UTC plus timestamp 891706367 (6d 23h 59m 59s 999ms)"},
		{utils.MessageTypeMSM7Galileo, maxTimestamp,
			"Start of Galileo week 2023-02-11 23:59:42 +0000 UTC plus timestamp 604799999 (6d 23h 59m 59s 999ms)"},
		{utils.MessageTypeMSM7SBAS, 1,
			"Start of SBAS week (don't know the start of week for message type 1107) plus timestamp 1 (0d 0h 0m 0s 1ms)"},
		{utils.MessageTypeMSM7Glonass, glonassTimestampTooBig,
			"Start of Glonass week 2023-02-11 21:00:00 +0000 UTC plus timestamp out of range - 0x35265c00 (6/86400000)"},
		{utils.MessageTypeMSM7Galileo, timestampTooBig,
			"Start of Galileo week 2023-02-11 23:59:42 +0000 UTC plus timestamp out of range - 604800000 (7/0)"},
	}
	for _, td := range testData {

		startTime := time.Date(2023, time.February, 14, 0, 0, 0, 0, utils.LocationUTC)

		h := New(startTime)

		display := h.getStartTimeDisplay(td.messageType, td.timestamp)

		if td.wantDisplay != display {
			t.Error(diff.Diff(td.wantDisplay, display))
		}
	}
}
