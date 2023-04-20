package main

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/goblimey/go-ntrip/rtcm/testdata"
	"github.com/goblimey/go-ntrip/rtcm/utils"

	"github.com/kylelemons/godebug/diff"
)

// TestGetTime tests getTime
func TestGetTime(t *testing.T) {
	const dateTimeLayout = "2006-01-02 15:04:05.000000000 MST"

	expectedTime1 := time.Date(2020, time.November, 13, 0, 0, 0, 0, utils.LocationUTC)
	time1, err1 := getTime("2020-11-13")
	if err1 != nil {
		t.Error(err1)
	}
	if !expectedTime1.Equal(time1) {
		t.Errorf("expected %s got %s",
			expectedTime1.Format(dateTimeLayout),
			time1.Format(dateTimeLayout))
	}

	expectedTime2 := time.Date(2020, time.November, 13, 9, 10, 11, 0, utils.LocationUTC)
	time2, err2 := getTime("2020-11-13T09:10:11Z")
	if err2 != nil {
		t.Error(err2)
	}
	if !expectedTime2.Equal(time2) {
		t.Errorf("expected %s got %s",
			expectedTime2.Format(dateTimeLayout),
			time2.Format(dateTimeLayout))
	}

	expectedTime3 := time.Date(2020, time.November, 13, 9, 10, 11, 0, utils.LocationParis)
	time3, err3 := getTime("2020-11-13T09:10:11+01:00")
	if err3 != nil {
		t.Error(err3)
	}
	if !expectedTime3.Equal(time3) {
		t.Errorf("expected %s got %s",
			expectedTime3.Format(dateTimeLayout),
			time3.Format(dateTimeLayout))
	}

	expectedTime4 := time.Date(2020, time.November, 13, 9, 10, 11, 0, utils.LocationUTC)
	time4, err4 := getTime("2020-11-13T09:10:11Z")
	if err4 != nil {
		t.Error(err4)
	}
	if !expectedTime4.Equal(time4) {
		t.Errorf("expected %s got %s",
			expectedTime4.Format(dateTimeLayout),
			time4.Format(dateTimeLayout))
	}

	// Test time values that should fail.

	junk1 := "2020-11-13T09:10:11+junk"
	_, err5 := getTime(junk1)
	if err5 == nil {
		t.Errorf("timestring %s parsed but it should have failed", junk1)
	}

	junk2 := "2020-11-13T09:10:11+junk"
	_, err6 := getTime(junk2)
	if err6 == nil {
		t.Errorf("time string %s parsed but it should have failed", junk2)
	}
}

// TestDisplayMessages checks that DisplayMessages correctly displays input
// containing a single message.
func TestDisplayMessage(t *testing.T) {

	const want = `RTCM data

Note: times are in UTC.  RINEX format uses GPS time, which is currently (Jan 2021)
18 seconds ahead of UTC

message type 1230, frame length 14
00000000  d3 00 08 4c e0 00 8a 00  00 00 00 a8 f7 2a        |...L.........*|

(Message type 1230 - GLONASS code-phase biases - don't know how to decode this)

`

	reader := bytes.NewReader(testdata.Fake1230)

	bufferBytes := make([]byte, 0, 1000)
	buffer := bytes.NewBuffer(bufferBytes)

	err := DisplayMessages(time.Now(), reader, buffer, 0, 0)

	if err != nil {
		t.Error(err)
		return
	}

	gotBytes := make([]byte, 1000)
	n, readError := buffer.Read(gotBytes)

	if readError != nil {
		t.Error(readError)
	}

	got := string(gotBytes[:n])

	if want != got {
		t.Errorf(diff.Diff(want, got))
	}
}

// TestOpenFile tests the openFile function using the test file testdata.rtcm.
func TestOpenFile(t *testing.T) {
	reader, openError := openFile("testdata.rtcm")

	if openError != nil {
		t.Error(openError)
	}

	// The file starts with a 0xd3 byte.  Check that we can read it.
	buffer := make([]byte, 1)

	n, readError := reader.Read(buffer)

	if readError != nil {
		t.Error(readError)
	}

	if n != 1 {
		t.Errorf("want 1 byte got %d", n)
		return
	}

	if buffer[0] != 0xd3 {
		t.Errorf("want 0xd3 byte got 0x%2x", buffer[0])
		return
	}

}

// TestOpenFileWithError tests the openFile function using a file name that
// doesn't exist.
func TestOpenFileWithError(t *testing.T) {
	const filename = "junk"
	want := fmt.Sprintf("open %s: no such file or directory", filename)
	_, openError := openFile("junk")

	if openError == nil {
		t.Error("expected an error")
	}

	got := openError.Error()

	if got != want {
		t.Errorf("want %s got %s", want, got)
	}
}
