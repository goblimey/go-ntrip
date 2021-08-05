package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/goblimey/go-ntrip/rtcm"
)

// This is real data collected on the 13th November 2020.  Some junk
// has been added between messages.
var inputDataBytes = [...]byte{

	// type 1077 - GPS.  Converted to RINEX, this gives:
	0xd3, 0x00, 0xdc, 0x43, 0x50, 0x00, 0x67, 0x00, 0x97, 0x62, 0x00, 0x00, 0x08, 0x40, 0xa0, 0x65,
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
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xfe,
	0x69, 0xe8,

	'j',

	// Type 1087 - Glonass
	//
	// R 5  23482521.703   125527502.441         886.891          36.000                                                                    23482518.744    97632475.638         689.879          37.000
	// R12  20829833.360   111269260.007        3266.930          48.000       20829832.826    86542668.996        2540.913          39.000
	// R13  19220908.037   102638574.587        -569.980          36.000       19220907.074    79830006.582        -443.200          33.000
	// R14  22228766.616   118491839.342       -3852.575          42.000       22228768.714    92160317.831       -2996.456          39.000
	// R22  20286899.487   108292911.973        2735.571          42.000       20286900.360    84227771.187        2127.874          29.000
	// R23  19954308.877   106742118.811       -2561.292          48.000       19954309.753    83021654.098       -1992.063          37.000
	// R24  22984791.448   122910027.290       -4164.178          40.000       22984791.701    95596674.871       -3238.890          39.000
	0xd3, 0x00, 0xc3, 0x43, 0xf0, 0x00, 0xa2, 0x93, 0x7c, 0x22, 0x00, 0x00, 0x04, 0x0e, 0x03, 0x80,
	0x00, 0x00, 0x00, 0x00, 0x20, 0x80, 0x00, 0x00, 0x7f, 0xfe, 0x9c, 0x8a, 0x80, 0x94, 0x86, 0x84,
	0x99, 0x0c, 0xa0, 0x95, 0x2a, 0x8b, 0xd8, 0x3a, 0x92, 0xf5, 0x74, 0x7d, 0x56, 0xfe, 0xb7, 0xec,
	0xe8, 0x0d, 0x41, 0x69, 0x7c, 0x00, 0x0e, 0xf0, 0x61, 0x42, 0x9c, 0xf0, 0x27, 0x38, 0x86, 0x2a,
	0xda, 0x62, 0x36, 0x3c, 0x8f, 0xeb, 0xc8, 0x27, 0x1b, 0x77, 0x6f, 0xb9, 0x4c, 0xbe, 0x36, 0x2b,
	0xe4, 0x26, 0x1d, 0xc1, 0x4f, 0xdc, 0xd9, 0x01, 0x16, 0x24, 0x11, 0x9a, 0xe0, 0x91, 0x02, 0x00,
	0x7a, 0xea, 0x61, 0x9d, 0xb4, 0xe1, 0x52, 0xf6, 0x1f, 0x22, 0xae, 0xdf, 0x26, 0x28, 0x3e, 0xe0,
	0xf6, 0xbe, 0xdf, 0x90, 0xdf, 0xb8, 0x01, 0x3f, 0x8e, 0x86, 0xbf, 0x7e, 0x67, 0x1f, 0x83, 0x8f,
	0x20, 0x51, 0x53, 0x60, 0x46, 0x60, 0x30, 0x43, 0xc3, 0x3d, 0xcf, 0x12, 0x84, 0xb7, 0x10, 0xc4,
	0x33, 0x53, 0x3d, 0x25, 0x48, 0xb0, 0x14, 0x00, 0x00, 0x04, 0x81, 0x28, 0x60, 0x13, 0x84, 0x81,
	0x08, 0x54, 0x13, 0x85, 0x40, 0xe8, 0x60, 0x12, 0x85, 0x01, 0x38, 0x5c, 0x67, 0xb7, 0x67, 0xa5,
	0xff, 0x4e, 0x71, 0xcd, 0xd3, 0x78, 0x27, 0x29, 0x0e, 0x5c, 0xed, 0xd9, 0xd7, 0xcc, 0x7e, 0x04,
	0xf8, 0x09, 0xc3, 0x73, 0xa0, 0x40, 0x70, 0xd9, 0x6d,

	'j', 'u', 'n', 'k',

	//1097 - Galileo
	0xd3, 0x00, 0xc3, 0x44, 0x90, 0x00, 0x67, 0x00, 0x97, 0x62, 0x00, 0x00, 0x21, 0x18, 0x00, 0xc0,
	0x08, 0x00, 0x00, 0x00, 0x20, 0x01, 0x00, 0x00, 0x7f, 0xfe, 0xae, 0xbe, 0x90, 0x98, 0xa6, 0x9c,
	0xb4, 0x00, 0x00, 0x00, 0x08, 0xc1, 0x4b, 0xc1, 0x32, 0xf8, 0x0b, 0x08, 0xc5, 0x83, 0xc8, 0x01,
	0xe8, 0x25, 0x3f, 0x74, 0x7c, 0xc4, 0x02, 0xa0, 0x4b, 0xc1, 0x47, 0x90, 0x12, 0x86, 0x62, 0x72,
	0x92, 0x28, 0x53, 0x18, 0x9d, 0x8d, 0x85, 0x82, 0xc6, 0xe1, 0x8a, 0x6a, 0x2f, 0xdd, 0x5e, 0xcd,
	0xd3, 0xe1, 0x1a, 0x15, 0x01, 0xa1, 0x2b, 0xdc, 0x56, 0x3f, 0xc4, 0xea, 0xc0, 0x5e, 0xdc, 0x40,
	0x48, 0xd3, 0x80, 0xb2, 0x25, 0x60, 0x9c, 0x7b, 0x7e, 0x32, 0xdd, 0x3e, 0x22, 0xf7, 0x01, 0xb6,
	0xf3, 0x81, 0xaf, 0xb7, 0x1f, 0x78, 0xe0, 0x7f, 0x6c, 0xaa, 0xfe, 0x9a, 0x7e, 0x7e, 0x94, 0x9f,
	0xbf, 0x06, 0x72, 0x3f, 0x15, 0x8c, 0xb1, 0x44, 0x56, 0xe1, 0xb1, 0x92, 0xdc, 0xb5, 0x37, 0x4a,
	0xd4, 0x5d, 0x17, 0x38, 0x4e, 0x30, 0x24, 0x14, 0x00, 0x04, 0xc1, 0x50, 0x3e, 0x0f, 0x85, 0x41,
	0x40, 0x52, 0x13, 0x85, 0x61, 0x50, 0x5a, 0x16, 0x04, 0xa1, 0x38, 0x12, 0x5b, 0x24, 0x7e, 0x03,
	0x6c, 0x07, 0x89, 0xdb, 0x93, 0xbd, 0xba, 0x0d, 0x34, 0x27, 0x68, 0x75, 0xd0, 0xa6, 0x72, 0x24,
	0xe4, 0x88, 0xdc, 0x61, 0xa9, 0x40, 0xb1, 0x9d, 0x0d,

	// Incomplete message (Beidou).  Should be ignored.
	0xd3, 0x00, 0xaa, 0x46, 0x70, 0x00, 0x66, 0xff, 0xbc, 0xa0, 0x00, 0x00, 0x00, 0x04, 0x00, 0x26,
	0x18, 0x00, 0x00, 0x00, 0x20, 0x02, 0x00, 0x00, 0x75, 0x53, 0xfa, 0x82, 0x42, 0x62, 0x9a, 0x80,
	0x00, 0x00, 0x06, 0x95, 0x4e, 0xa7, 0xa0, 0xbf, 0x1e, 0x78, 0x7f, 0x0a, 0x10, 0x08, 0x18, 0x7f,
}

var inputData []byte = inputDataBytes[:]

// This is what processMessages is supposed to send to the writer.
var expectedResultBytes = [...]byte{

	// type 1077 - GPS.
	0xd3, 0x00, 0xdc, 0x43, 0x50, 0x00, 0x67, 0x00, 0x97, 0x62, 0x00, 0x00, 0x08, 0x40, 0xa0, 0x65,
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
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xfe,
	0x69, 0xe8,

	// Type 1087 - Glonass
	0xd3, 0x00, 0xc3, 0x43, 0xf0, 0x00, 0xa2, 0x93, 0x7c, 0x22, 0x00, 0x00, 0x04, 0x0e, 0x03, 0x80,
	0x00, 0x00, 0x00, 0x00, 0x20, 0x80, 0x00, 0x00, 0x7f, 0xfe, 0x9c, 0x8a, 0x80, 0x94, 0x86, 0x84,
	0x99, 0x0c, 0xa0, 0x95, 0x2a, 0x8b, 0xd8, 0x3a, 0x92, 0xf5, 0x74, 0x7d, 0x56, 0xfe, 0xb7, 0xec,
	0xe8, 0x0d, 0x41, 0x69, 0x7c, 0x00, 0x0e, 0xf0, 0x61, 0x42, 0x9c, 0xf0, 0x27, 0x38, 0x86, 0x2a,
	0xda, 0x62, 0x36, 0x3c, 0x8f, 0xeb, 0xc8, 0x27, 0x1b, 0x77, 0x6f, 0xb9, 0x4c, 0xbe, 0x36, 0x2b,
	0xe4, 0x26, 0x1d, 0xc1, 0x4f, 0xdc, 0xd9, 0x01, 0x16, 0x24, 0x11, 0x9a, 0xe0, 0x91, 0x02, 0x00,
	0x7a, 0xea, 0x61, 0x9d, 0xb4, 0xe1, 0x52, 0xf6, 0x1f, 0x22, 0xae, 0xdf, 0x26, 0x28, 0x3e, 0xe0,
	0xf6, 0xbe, 0xdf, 0x90, 0xdf, 0xb8, 0x01, 0x3f, 0x8e, 0x86, 0xbf, 0x7e, 0x67, 0x1f, 0x83, 0x8f,
	0x20, 0x51, 0x53, 0x60, 0x46, 0x60, 0x30, 0x43, 0xc3, 0x3d, 0xcf, 0x12, 0x84, 0xb7, 0x10, 0xc4,
	0x33, 0x53, 0x3d, 0x25, 0x48, 0xb0, 0x14, 0x00, 0x00, 0x04, 0x81, 0x28, 0x60, 0x13, 0x84, 0x81,
	0x08, 0x54, 0x13, 0x85, 0x40, 0xe8, 0x60, 0x12, 0x85, 0x01, 0x38, 0x5c, 0x67, 0xb7, 0x67, 0xa5,
	0xff, 0x4e, 0x71, 0xcd, 0xd3, 0x78, 0x27, 0x29, 0x0e, 0x5c, 0xed, 0xd9, 0xd7, 0xcc, 0x7e, 0x04,
	0xf8, 0x09, 0xc3, 0x73, 0xa0, 0x40, 0x70, 0xd9, 0x6d,

	//1097 - Galileo
	0xd3, 0x00, 0xc3, 0x44, 0x90, 0x00, 0x67, 0x00, 0x97, 0x62, 0x00, 0x00, 0x21, 0x18, 0x00, 0xc0,
	0x08, 0x00, 0x00, 0x00, 0x20, 0x01, 0x00, 0x00, 0x7f, 0xfe, 0xae, 0xbe, 0x90, 0x98, 0xa6, 0x9c,
	0xb4, 0x00, 0x00, 0x00, 0x08, 0xc1, 0x4b, 0xc1, 0x32, 0xf8, 0x0b, 0x08, 0xc5, 0x83, 0xc8, 0x01,
	0xe8, 0x25, 0x3f, 0x74, 0x7c, 0xc4, 0x02, 0xa0, 0x4b, 0xc1, 0x47, 0x90, 0x12, 0x86, 0x62, 0x72,
	0x92, 0x28, 0x53, 0x18, 0x9d, 0x8d, 0x85, 0x82, 0xc6, 0xe1, 0x8a, 0x6a, 0x2f, 0xdd, 0x5e, 0xcd,
	0xd3, 0xe1, 0x1a, 0x15, 0x01, 0xa1, 0x2b, 0xdc, 0x56, 0x3f, 0xc4, 0xea, 0xc0, 0x5e, 0xdc, 0x40,
	0x48, 0xd3, 0x80, 0xb2, 0x25, 0x60, 0x9c, 0x7b, 0x7e, 0x32, 0xdd, 0x3e, 0x22, 0xf7, 0x01, 0xb6,
	0xf3, 0x81, 0xaf, 0xb7, 0x1f, 0x78, 0xe0, 0x7f, 0x6c, 0xaa, 0xfe, 0x9a, 0x7e, 0x7e, 0x94, 0x9f,
	0xbf, 0x06, 0x72, 0x3f, 0x15, 0x8c, 0xb1, 0x44, 0x56, 0xe1, 0xb1, 0x92, 0xdc, 0xb5, 0x37, 0x4a,
	0xd4, 0x5d, 0x17, 0x38, 0x4e, 0x30, 0x24, 0x14, 0x00, 0x04, 0xc1, 0x50, 0x3e, 0x0f, 0x85, 0x41,
	0x40, 0x52, 0x13, 0x85, 0x61, 0x50, 0x5a, 0x16, 0x04, 0xa1, 0x38, 0x12, 0x5b, 0x24, 0x7e, 0x03,
	0x6c, 0x07, 0x89, 0xdb, 0x93, 0xbd, 0xba, 0x0d, 0x34, 0x27, 0x68, 0x75, 0xd0, 0xa6, 0x72, 0x24,
	0xe4, 0x88, 0xdc, 0x61, 0xa9, 0x40, 0xb1, 0x9d, 0x0d,
}

// This slice contains the expected result.
var expectedResult []byte = expectedResultBytes[:]

func TestReadNextMessageFrame(t *testing.T) {
	reader := bytes.NewReader(inputData)
	var buffer bytes.Buffer

	// For this test, any time will do.
	rtcmHandler := rtcm.New(time.Now())

	processMessages(rtcmHandler, reader, &buffer)

	if bytes.Compare(expectedResult, buffer.Bytes()) != 0 {
		t.Fatalf(
			"result not as expected - expected length %d, result length %d\n",
			len(expectedResult), len(buffer.Bytes()))
	}
}

// TestGetTime tests getTime
func TestGetTime(t *testing.T) {
	const dateTimeLayout = "2006-01-02 15:04:05.000000000 MST"
	locationUTC, _ := time.LoadLocation("UTC")
	locationParis, _ := time.LoadLocation("Europe/Paris")

	expectedTime1 := time.Date(2020, time.November, 13, 0, 0, 0, 0, locationUTC)
	time1, err1 := getTime("2020-11-13")
	if err1 != nil {
		t.Error(err1)
	}
	if !expectedTime1.Equal(time1) {
		t.Errorf("expected %s got %s",
			expectedTime1.Format(dateTimeLayout),
			time1.Format(dateTimeLayout))
	}

	expectedTime2 := time.Date(2020, time.November, 13, 9, 10, 11, 0, locationUTC)
	time2, err2 := getTime("2020-11-13T09:10:11Z")
	if err2 != nil {
		t.Error(err2)
	}
	if !expectedTime2.Equal(time2) {
		t.Errorf("expected %s got %s",
			expectedTime2.Format(dateTimeLayout),
			time2.Format(dateTimeLayout))
	}

	expectedTime3 := time.Date(2020, time.November, 13, 9, 10, 11, 0, locationParis)
	time3, err3 := getTime("2020-11-13T09:10:11+01:00")
	if err3 != nil {
		t.Error(err3)
	}
	if !expectedTime3.Equal(time3) {
		t.Errorf("expected %s got %s",
			expectedTime3.Format(dateTimeLayout),
			time3.Format(dateTimeLayout))
	}

	expectedTime4 := time.Date(2020, time.November, 13, 9, 10, 11, 0, locationUTC)
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
		t.Errorf("timestring %s parsed but it should have failed", junk2)
	}
}

// TestJSONControl tests that the correct data is produced when the
// text from a JSON control file is unmarshalled.
//
func TestGetJSONControl(t *testing.T) {
	reader := strings.NewReader(`{"input": [
		{"name": "a"}, 
		{"name": "b"}
	]}`)

	inputFiles := getJSONControl(reader)

	if inputFiles == nil {
		t.Fatal("parsing json failed - nil")
	}

	if len(inputFiles.Files) != 2 {
		t.Fatalf("parsing json, expected 2 files, got %d", len(inputFiles.Files))
	}

	if inputFiles.Files[0].Name != "a" {
		t.Fatalf("parsing json, expected file 0 to be a, got %s", inputFiles.Files[0].Name)
	}

	if inputFiles.Files[0].Name != "b" {
		t.Fatalf("parsing json, expected file 0 to be b, got %s", inputFiles.Files[1]
		/home/simon/goprojects/go-ntrip/rtcmfilter/rtcmfilter_file_test.go:4:2: imported and not used: "bytes"
		/home/simon/goprojects/go-ntrip/rtcmfilter/rtcmfilter_file_test.go:10:2: imported and not used: "time"
		/home/simon/goprojects/go-ntrip/rtcmfilter/rtcmfilter_file_test.go:12:2: imported and not used: "github.com/goblimey/go-ntrip/rtcm"
		/home/simon/goprojects/go-ntrip/rtcmfilter/rtcmfilter_test.go:14:5: inputDataBytes.Name)
	}
}
