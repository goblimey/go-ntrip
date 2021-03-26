package rtcm

import (
	"bytes"
	"fmt"
	"testing"
	"time"
)

// maxEpochTime is the value of a GPS and Beidout epoch time
// just before it rolls over.
const maxEpochTime uint = (7 * 24 * 3600 * 1000) - 1

// testDelta3 is the delta value used to test floating point
// values for equality to three decimal places.
const testDelta3 = 0.001

var london *time.Location
var paris *time.Location

// Data contain a batch of RTCM3 messages.  In each message byte 0
// is 0xd3.  Bytes 1 and 2 form a sixteen bit unsigned number, the
// message length, but this is limited to 1023 so the top six bits
// are always 0.  The message follows and then three bytes of CRC.
var d = [...]byte{
	0xd3, 0x00, 0x8a, 0x43, 0x20, 0x00, 0x8a, 0x0e, 0x1a, 0x26, 0x00, 0x00, 0x2f, 0x40, 0x00, 0x06,
	0x00, 0x00, 0x00, 0x00, 0x20, 0x00, 0x80, 0x00, 0x5f, 0xff, 0xa4, 0xa7, 0x25, 0xa4, 0xa4, 0x22,
	0xa9, 0x26, 0x30, 0x64, 0xab, 0x9f, 0x4e, 0x1d, 0xef, 0x58, 0xd5, 0x28, 0x60, 0x34, 0x00, 0xff,
	0xff, 0x98, 0x63, 0x48, 0xb0, 0x91, 0xab, 0x63, 0x4c, 0x72, 0x8c, 0x63, 0xa6, 0x24, 0x26, 0x44,
	0x04, 0x7f, 0x68, 0xf0, 0xb0, 0x42, 0xa0, 0x51, 0xfc, 0x1f, 0x39, 0x00, 0xc8, 0x90, 0x04, 0x21,
	0xa0, 0x6c, 0x9e, 0x81, 0x64, 0x7f, 0x06, 0x00, 0xe8, 0x1b, 0xd0, 0x7f, 0x35, 0x6e, 0xbd, 0x20,
	0x2a, 0x09, 0xcf, 0x34, 0x28, 0xa6, 0x10, 0x80, 0xf6, 0x41, 0xd9, 0xe4, 0x01, 0xa7, 0x20, 0x07,
	0x4e, 0xbf, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x00, 0x01, 0x75, 0x14, 0xd7, 0x3d, 0x76,
	0x65, 0x56, 0x16, 0x4b, 0x35, 0xb4, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x81, 0x51, 0xa5,
	0xd3, 0x00, 0x98, 0x43, 0xc0, 0x00, 0xd1, 0x07, 0x8e, 0xe6, 0x00, 0x00, 0x60, 0xb0, 0x61, 0x80,
	0x00, 0x00, 0x00, 0x00, 0x20, 0x80, 0x00, 0x00, 0x7f, 0x7f, 0xe8, 0x09, 0x48, 0xc8, 0xa9, 0x28,
	0xc9, 0xc9, 0xe9, 0x10, 0x6b, 0x10, 0x9d, 0xac, 0x13, 0x6a, 0xdb, 0xd9, 0xa8, 0xc0, 0xa1, 0x5c,
	0xa2, 0xb0, 0x1f, 0x3b, 0x3e, 0x6e, 0x70, 0xa8, 0xdf, 0xf5, 0x96, 0x87, 0x62, 0x96, 0xc3, 0x52,
	0x9d, 0x65, 0x05, 0x07, 0x14, 0x0e, 0x07, 0xd6, 0xa1, 0xaf, 0x83, 0x4c, 0x36, 0x96, 0xb0, 0xaf,
	0xf9, 0xc2, 0xb6, 0x78, 0xfc, 0x34, 0x47, 0xf8, 0x3b, 0xdf, 0x96, 0x90, 0x7e, 0x69, 0x76, 0xf2,
	0xe2, 0x67, 0xd6, 0xfc, 0x9f, 0x71, 0x76, 0x02, 0xca, 0x91, 0x0a, 0x5d, 0x54, 0x1c, 0xaa, 0x20,
	0x6c, 0x83, 0x7d, 0x65, 0x1f, 0xf5, 0xd2, 0x8f, 0xd8, 0x04, 0x4f, 0x52, 0x45, 0x7f, 0xff, 0xff,
	0xff, 0xff, 0xff, 0x32, 0xef, 0xfc, 0x00, 0x01, 0x74, 0xd6, 0xd7, 0x8d, 0x57, 0xe3, 0x56, 0x15,
	0x51, 0x2c, 0x0d, 0xdf, 0x50, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xc4, 0xc9, 0x01, 0xd3, 0x00,
	0xdc, 0x44, 0x90, 0x00, 0x8a, 0x0e, 0x1a, 0x26, 0x00, 0x00, 0x54, 0x41, 0x00, 0x81, 0x08, 0x00,
	0x00, 0x00, 0x20, 0x01, 0x00, 0x00, 0x3f, 0xff, 0xae, 0x2c, 0x27, 0x26, 0x2a, 0xaa, 0xab, 0xad,
	0x00, 0x00, 0x00, 0x00, 0x46, 0xf8, 0x52, 0x78, 0x4f, 0x1c, 0xfe, 0x2d, 0x0d, 0x2c, 0x7e, 0x1e,
	0x0e, 0x50, 0x0d, 0x9f, 0x55, 0x81, 0xae, 0x11, 0xe0, 0x1f, 0x7e, 0xc5, 0xfb, 0x67, 0xfe, 0x88,
	0x52, 0x68, 0x56, 0x99, 0x89, 0x90, 0x98, 0x44, 0xde, 0xf5, 0xba, 0xef, 0x0e, 0xae, 0x2d, 0x08,
	0x62, 0x8f, 0xf6, 0x1c, 0x1e, 0x63, 0xd7, 0xd1, 0x30, 0xe1, 0x93, 0x3d, 0x56, 0x9a, 0xa4, 0x6a,
	0x01, 0xff, 0xe8, 0x88, 0x97, 0xa1, 0x66, 0x9f, 0xa6, 0x31, 0xf8, 0x6a, 0x37, 0x70, 0x6d, 0x55,
	0xd7, 0xc2, 0x49, 0x77, 0xc5, 0x37, 0x87, 0x8c, 0x67, 0x0f, 0x8d, 0xed, 0x37, 0x76, 0x65, 0x8f,
	0x94, 0x5a, 0x58, 0x4e, 0x99, 0x88, 0x52, 0x6e, 0x07, 0xa5, 0xd3, 0xaf, 0xa1, 0xb4, 0x44, 0x17,
	0x15, 0x45, 0xf3, 0x5c, 0xd9, 0x42, 0x50, 0x92, 0x5c, 0x96, 0xe5, 0xc0, 0x12, 0x0c, 0x8b, 0x44,
	0xd1, 0x20, 0x00, 0x1f, 0x0a, 0x42, 0xd0, 0xc0, 0x2f, 0x0b, 0x82, 0xd0, 0xac, 0x2d, 0x08, 0xc2,
	0x00, 0xa8, 0x2c, 0x09, 0x42, 0xb0, 0xe4, 0xbe, 0x8c, 0x45, 0x1d, 0x98, 0xc4, 0xf1, 0x8c, 0x3a,
	0x41, 0xb4, 0x82, 0x1f, 0x84, 0x3e, 0xc0, 0x6c, 0x48, 0xd7, 0x50, 0xd1, 0x11, 0x97, 0xfc, 0xf7,
	0x39, 0xc2, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x8e, 0x85, 0xa1,
	0xd3, 0x00, 0x7b, 0x46, 0x40, 0x00, 0x8a, 0x0d, 0x3f, 0x66, 0x00, 0x00, 0x01, 0x30, 0x04, 0x28,
	0x08, 0x00, 0x00, 0x00, 0x20, 0x02, 0x00, 0x00, 0x3f, 0x55, 0x0d, 0x0c, 0xaa, 0x98, 0x9e, 0xa6,
	0xaf, 0x7b, 0x2d, 0xdd, 0x62, 0x1a, 0xdb, 0x3b, 0x26, 0x08, 0xb6, 0x4d, 0xe7, 0x1b, 0x44, 0x30,
	0xf6, 0x60, 0x40, 0x06, 0xce, 0x4b, 0xbb, 0x0f, 0x87, 0xb5, 0xb0, 0x58, 0xfd, 0xfd, 0xf9, 0xf4,
	0xf6, 0xff, 0x37, 0xc2, 0x2e, 0x0e, 0xfa, 0xb1, 0x41, 0x37, 0x24, 0x0a, 0x13, 0xfb, 0xc4, 0xad,
	0xbf, 0xe3, 0x72, 0x3f, 0xff, 0xff, 0xff, 0xf8, 0x00, 0xe4, 0xd1, 0xc7, 0x7e, 0x57, 0x57, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x8c, 0x60,
	0xc0, 0xd3, 0x00, 0xc3, 0x46, 0x70, 0x00, 0x8a, 0x0d, 0x3f, 0x64, 0x00, 0x00, 0x01, 0x30, 0x04,
	0x28, 0x08, 0x00, 0x00, 0x00, 0x20, 0x02, 0x00, 0x00, 0x3f, 0x55, 0x0d, 0x0c, 0xaa, 0x98, 0x9e,
	0xa6, 0xae, 0x00, 0x00, 0x00, 0x17, 0xb2, 0xdd, 0xd6, 0x21, 0xad, 0xb3, 0xb2, 0x60, 0x81, 0x4e,
	0x08, 0xdf, 0xb4, 0x9f, 0x4f, 0x84, 0x64, 0x01, 0x37, 0xc5, 0x22, 0xd8, 0xe9, 0xbc, 0xe0, 0x1b,
	0x44, 0x21, 0x87, 0xaf, 0xf8, 0x10, 0x0e, 0x0d, 0x9a, 0x04, 0xbb, 0xa1, 0x87, 0xbe, 0x5e, 0xd6,
	0xa2, 0x0b, 0x1f, 0xbd, 0xef, 0xcf, 0xa5, 0xed, 0xfe, 0x65, 0xe1, 0x17, 0x03, 0xdf, 0x56, 0x2c,
	0x09, 0xb9, 0x20, 0x14, 0x27, 0xf1, 0xe2, 0x56, 0xdb, 0xfc, 0x6e, 0x40, 0xf3, 0x41, 0xd0, 0xa4,
	0xe9, 0x3b, 0xd0, 0x51, 0x84, 0x8c, 0xe2, 0x80, 0x1c, 0x09, 0x82, 0x30, 0x8c, 0x2f, 0x0c, 0x82,
	0xe0, 0xac, 0x20, 0x03, 0x68, 0x08, 0x9d, 0x11, 0x28, 0xf4, 0x0a, 0xe7, 0x37, 0xda, 0xf4, 0xbf,
	0x9a, 0x0f, 0x9f, 0xb4, 0xe0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x8f, 0x9d, 0x1c,
}

var data []byte = d[:]

// This is real data collected on the 13th November 2020.
var realDataArray = [...]byte{

	// type 1077 - GPS.  Converted to RINEX, this gives:
	//
	// > 2020 11 13  0  0 23.0000000  0 26
	// G 4  24410527.355   128278179.264         709.992          40.000
	// G 9
	// G16  22915780.724   120423177.179       -3139.070          40.000
	// G18  21506547.550   113017684.727       -2482.645          44.000
	// G25  23345103.037   122679365.321        3327.570          40.000
	// G26  20662003.308   108579565.367       -1538.436          46.000
	// G29  21136079.188   111070868.860        2016.750          46.000
	// G31  21670772.711   113880577.055        2325.559          46.000
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

	// Type 1127 - Beidou
	0xd3, 0x00, 0xaa, 0x46, 0x70, 0x00, 0x66, 0xff, 0xbc, 0xa0, 0x00, 0x00, 0x00, 0x04, 0x00, 0x26,
	0x18, 0x00, 0x00, 0x00, 0x20, 0x02, 0x00, 0x00, 0x75, 0x53, 0xfa, 0x82, 0x42, 0x62, 0x9a, 0x80,
	0x00, 0x00, 0x06, 0x95, 0x4e, 0xa7, 0xa0, 0xbf, 0x1e, 0x78, 0x7f, 0x0a, 0x10, 0x08, 0x18, 0x7f,
	0x35, 0x04, 0xab, 0xee, 0x50, 0x77, 0x8a, 0x86, 0xf0, 0x51, 0xf1, 0x4d, 0x82, 0x46, 0x38, 0x29,
	0x0a, 0x8c, 0x35, 0x57, 0x23, 0x87, 0x82, 0x24, 0x2a, 0x01, 0xb5, 0x40, 0x07, 0xeb, 0xc5, 0x01,
	0x37, 0xa8, 0x80, 0xb3, 0x88, 0x03, 0x23, 0xc4, 0xfc, 0x61, 0xe0, 0x4f, 0x33, 0xc4, 0x73, 0x31,
	0xcd, 0x90, 0x54, 0xb2, 0x02, 0x70, 0x90, 0x26, 0x0b, 0x42, 0xd0, 0x9c, 0x2b, 0x0c, 0x02, 0x97,
	0xf4, 0x08, 0x3d, 0x9e, 0xc7, 0xb2, 0x6e, 0x44, 0x0f, 0x19, 0x48, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xe5, 0x1e, 0xd8,
}

var realData []byte = realDataArray[:]

func init() {

	london, _ = time.LoadLocation("Europe/London")
	paris, _ = time.LoadLocation("Europe/Paris")
}

func TestGetChar(t *testing.T) {

	r := bytes.NewReader(realData)
	n := 0

	c, err := getChar(r)
	if err != nil {
		t.Fatal(err.Error())
	}
	if c != 0xd3 {
		t.Fatalf("expected d3 got %d", c)
	}
	n++

	c, err = getChar(r)
	if err != nil {
		t.Fatal(err.Error())
	}
	if c != 0 {
		t.Fatalf("expected 0 got %x", c)
	}
	n++

	c, err = getChar(r)
	if err != nil {
		t.Fatal(err.Error())
	}
	if c != 0xdc {
		t.Fatalf("expected dc got %x", c)
	}
	n++

	c, err = getChar(r)
	if err != nil {
		t.Fatal(err.Error())
	}
	if c != 0x43 {
		t.Fatalf("expected 43 got %x", c)
	}
	n++

	// We should read exactly 804 bytes
	for {
		c, err = getChar(r)
		if err != nil {
			if n != 804 {
				t.Fatalf("expected 804 bytes got %d", n)
			}

			break // Success!
		}
		n++
	}
}
func TestReadNextMessageFrame(t *testing.T) {
	r := bytes.NewReader(realData)

	startTime := time.Date(2020, time.November, 13, 0, 0, 0, 0, locationUTC)
	rtcm := New(startTime)

	frame, err1 := rtcm.ReadNextMessageFrame(r)
	if err1 != nil {
		t.Fatal(err1.Error())
	}

	message, err2 := rtcm.GetMessage(frame)
	if err2 != nil {
		t.Fatal(err2.Error())
	}

	if message.MessageType != 1077 {
		t.Fatalf("expected message type 1077, got %d", message.MessageType)
	}
}

func TestRealData(t *testing.T) {
	const expectedNumberOfMessages = 4
	startTime := time.Date(2020, time.November, 13, 0, 0, 0, 0, locationUTC)
	rtcm := New(startTime)
	messages := rtcm.GetMessages(realData)

	// Examine the first message in detail.  For the others, only examine the values
	// affected by the frequency map, which is different for each constellation.
	if expectedNumberOfMessages != len(messages) {
		t.Fatalf("expected message length %d, got %d",
			expectedNumberOfMessages, len(messages))
	}

	if messages[0].MessageType != 1077 {
		t.Fatalf("expected message type 1007, got %d", messages[0].MessageType)
	}

	message, ok := messages[0].Readable.(*MSM7Message)

	if !ok {
		t.Fatalf("expected message 0 to contain a type 1077 message but readable is nil")
	}

	fmt.Printf(rtcm.DisplayMessage(messages[0]))

	if message.Header.Constellation != "GPS" {
		t.Fatalf("expected GPS, got %s", message.Header.Constellation)
	}

	if len(message.Satellites) != 8 {
		t.Fatalf("expected 8 GPS satellites, got %d",
			len(message.Satellites))
	}

	if message.Satellites[0].RangeMillisWhole != 81 {
		t.Fatalf("expected range whole  of 81, got %d",
			message.Satellites[0].RangeMillisWhole)
	}

	if message.Satellites[0].RangeMillisFractional != 435 {
		t.Fatalf("expected range fractional 435, got %d",
			message.Satellites[0].RangeMillisFractional)
	}

	// There should be one signal list per satellite
	if len(message.Signals) != len(message.Satellites) {
		t.Fatalf("expected %d GPS signal lists, got %d",
			len(message.Signals), len(message.Satellites))
	}

	numSignals1 := 0
	for _, list := range message.Signals {
		for range list {
			numSignals1++
		}
	}

	if numSignals1 != 14 {
		t.Fatalf("expected 14 GPS signals, got %d", numSignals1)
	}

	// A signal cell contains a Satellite which is an index into the Satellite array.
	// The satellite has an ID.
	if message.Satellites[message.Signals[0][0].Satellite].SatelliteID != 4 {
		t.Fatalf("expected satelliteID 4, got %d",
			message.Satellites[message.Signals[0][0].Satellite].SatelliteID)
	}

	if message.Signals[0][0].RangeDelta != -26835 {
		t.Fatalf("expected range delta -26835, got %d",
			message.Signals[0][0].RangeDelta)
	}

	// Checking the resulting range in metres against the value
	// in the RINEX data produced from this message.

	if !floatsEqualWithin3(24410527.355, message.Signals[0][0].RangeMetres) {
		t.Fatalf("expected range 24410527.355 metres, got %3.6f",
			message.Signals[0][0].RangeMetres)
	}
}

func TestGetbitu(t *testing.T) {
	i := Getbitu(data, 8, 0)
	if i != 0 {
		t.Errorf("expected 0, got 0x%x", i)
	}

	i = Getbitu(data, 16, 4)
	if i != 8 {
		t.Errorf("expected 8, got 0x%x", i)
	}

	i = Getbitu(data, 16, 8)
	if i != 0x8a {
		t.Errorf("expected 0x8a, got 0x%x", i)
	}

	i = Getbitu(data, 16, 16)
	if i != 0x8a43 {
		t.Errorf("expected 0x8a43, got 0x%x", i)
	}

	// try a full 64-byte number
	i = Getbitu(data, 12, 64)
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

func TestGetMessage(t *testing.T) {
	const expectedLength = 0x8a + 6
	startTime := time.Date(2020, time.December, 9, 0, 0, 0, 0, locationUTC)
	rtcm := New(startTime)
	m, err := rtcm.GetMessage(data)
	if err != nil {
		t.Fatal(err.Error())
	}
	if expectedLength != len(m.RawData) {
		t.Fatalf("expected message length %d, got %d",
			expectedLength, len(m.RawData))
	}
}

func TestGetMessages(t *testing.T) {
	const expectedNumberOfMessages = 5
	startTime := time.Date(2020, time.November, 11, 0, 0, 0, 0, locationUTC)
	rtcm := New(startTime)
	messages := rtcm.GetMessages(data)
	if expectedNumberOfMessages != len(messages) {
		t.Fatalf("expected message length %d, got %d",
			expectedNumberOfMessages, len(messages))
	}

	for _, m := range messages {
		fmt.Println(rtcm.DisplayMessage(m))
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
		t.Fatalf("expected %v, got %v\n",
			expectedSatellites, satellites)
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
		t.Fatalf("expected %v, got %v\n",
			expectedSignals, signals)
	}
}

// TestGetScaled5 tests GetScaled5 with positive and negative values.
func TestGetScaled5(t *testing.T) {
	var scaled5 int64 = 9812345
	whole, part := GetScaled5(scaled5)
	if !(98 == whole && 12345 == part) {
		t.Errorf("expected 98.12345, got %d.%d\n", whole, part)
	}

	scaled5 = 0
	whole, part = GetScaled5(scaled5)
	if !(0 == whole && 0 == part) {
		t.Errorf("expected 0.0, got %d.%d\n", whole, part)
	}

	scaled5 = -7654321
	whole, part = GetScaled5(scaled5)
	if !(-76 == whole && 54321 == part) {
		t.Errorf("expected -76.54321, got %d.%d\n", whole, part)
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
	rtcm1 := New(dateTime1)
	if expectedEpochStart != rtcm1.startOfThisGPSWeek {
		t.Fatalf("expected %s result %s\n",
			expectedEpochStart.Format(dateLayout),
			rtcm1.startOfThisGPSWeek.Format(dateLayout))
	}
	if expectedNextEpochStart != rtcm1.startOfNextGPSWeek {
		t.Fatalf("expected %s result %s\n",
			expectedEpochStart.Format(dateLayout),
			rtcm1.startOfThisGPSWeek.Format(dateLayout))
	}

	// Wednesday 2020/08/05
	dateTime2 := time.Date(2020, time.August, 5, 12, 0, 0, 0, london)
	rtcm2 := New(dateTime2)
	if expectedEpochStart != rtcm2.startOfThisGPSWeek {
		t.Fatalf("expected %s result %s\n",
			expectedEpochStart.Format(dateLayout),
			rtcm2.startOfThisGPSWeek.Format(dateLayout))
	}

	// Sunday 2020/08/02 BST, just before the end of the GPS epoch
	dateTime3 := time.Date(2020, time.August, 9, 00, 59, 60-gpsLeapSeconds-1, 999999999, london)
	rtcm3 := New(dateTime3)
	if expectedEpochStart != rtcm3.startOfThisGPSWeek {
		t.Fatalf("expected %s result %s\n",
			expectedEpochStart.Format(dateLayout),
			rtcm3.startOfThisGPSWeek.Format(dateLayout))
	}

	// Sunday 2020/08/02 BST, at the start of the next GPS epoch.
	dateTime4 := time.Date(2020, time.August, 9, 1, 59, 60-gpsLeapSeconds, 0, paris)
	startOfNext := time.Date(2020, time.August, 8, 23, 59, 60-gpsLeapSeconds, 0, locationUTC)

	rtcm4 := New(dateTime4)
	if startOfNext != rtcm4.startOfThisGPSWeek {
		t.Fatalf("expected %s result %s\n",
			startOfNext.Format(dateLayout),
			rtcm4.startOfThisGPSWeek.Format(dateLayout))
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
	rtcm1 := New(startTime1)

	if !expectedStartOfPreviousWeek.Equal(rtcm1.startOfThisBeidouWeek) {
		t.Errorf("expected %s result %s\n",
			expectedStartOfPreviousWeek.Format(dateLayout), rtcm1.startOfThisBeidouWeek.Format(dateLayout))
	}

	// ... and so should this.
	startTime2 := time.Date(2020, time.August, 9, 0, 0, beidouLeapSeconds-1, 999999999, locationUTC)
	rtcm2 := New(startTime2)

	if !expectedStartOfPreviousWeek.Equal(rtcm2.startOfThisBeidouWeek) {
		t.Errorf("expected %s result %s\n",
			expectedStartOfPreviousWeek.Format(dateLayout), rtcm2.startOfThisBeidouWeek.Format(dateLayout))
	}

	// This start time should be in this week.
	startTime3 := time.Date(2020, time.August, 9, 0, 0, beidouLeapSeconds, 0, locationUTC)
	rtcm3 := New(startTime3)

	if !expectedStartOfThisWeek.Equal(rtcm3.startOfThisBeidouWeek) {
		t.Errorf("expected %s result %s\n",
			expectedStartOfThisWeek.Format(dateLayout), rtcm3.startOfThisBeidouWeek.Format(dateLayout))
	}

	// This start time should be just at the end of this Beidou week.
	startTime4 :=
		time.Date(2020, time.August, 16, 0, 0, beidouLeapSeconds-1, 999999999, locationUTC)
	rtcm4 := New(startTime4)

	if !expectedStartOfThisWeek.Equal(rtcm4.startOfThisBeidouWeek) {
		t.Errorf("expected %s result %s\n",
			expectedStartOfThisWeek.Format(dateLayout), rtcm4.startOfThisBeidouWeek.Format(dateLayout))
	}

	// This start time should be just at the start of the next Beidou week.
	startTime5 :=
		time.Date(2020, time.August, 16, 0, 0, beidouLeapSeconds, 0, locationUTC)
	rtcm5 := New(startTime5)

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
	rtcm1 := New(startTime1)
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
	rtcm2 := New(startTime2)
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
	rtcm := New(startTime)

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
	rtcm := New(startTime1)

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
	rtcm := New(startTime)

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
	rtcm1 := New(startTime1)

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

func TestGetRange(t *testing.T) {
	const expectedRange float64 = (128.5 + P2_11) * oneLightMillisecond // 38523477.236036
	var rangeMillisWhole uint = 0x80                                    // 1000 0000
	var rangeMillisFractional uint = 0x200                              // 10 bits 1000 ...
	rangeDelta := int(0x40000)                                          // 20 bits 0100 ...

	satellite := MSM7SatelliteCell{
		RangeMillisWhole:      rangeMillisWhole,
		RangeMillisFractional: rangeMillisFractional}

	signal := MSM7SignalCell{
		RangeDelta: rangeDelta}

	rangeM1 := getRangeInMetres(&satellite, &signal)

	if !floatsEqualWithin3(expectedRange, rangeM1) {
		t.Fatalf("expected %f got %f", expectedRange, rangeM1)
	}

	// Test values from real data.

	const expectedRange2 = 24410527.355

	satellite2 := MSM7SatelliteCell{
		RangeMillisWhole:      81,
		RangeMillisFractional: 435}

	signal2 := MSM7SignalCell{
		RangeDelta: -26835}

	rangeM2 := getRangeInMetres(&satellite2, &signal2)

	if !floatsEqualWithin3(expectedRange2, rangeM2) {
		t.Fatalf("expected %f got %f", expectedRange2, rangeM2)
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

	header := MSMHeader{Constellation: "GPS"}

	satellite1 := MSM7SatelliteCell{
		RangeMillisWhole:      rangeMillisWhole,
		RangeMillisFractional: rangeMillisFractional}

	signal1 := MSM7SignalCell{
		SignalID:        signalID,
		PhaseRangeDelta: phaseRangeDelta}

	rangeCycles1, err := getPhaseRangeCycles(&header, &satellite1, &signal1)

	if err != nil {
		t.Fatalf(err.Error())
	}

	if !floatsEqualWithin3(expectedPhaseRange1, rangeCycles1) {
		t.Fatalf("expected %f got %f", expectedPhaseRange1, rangeCycles1)
	}

	// Test using real data.

	const expectedPhaseRange2 = 128278179.264

	satellite2 := MSM7SatelliteCell{
		RangeMillisWhole:      81,
		RangeMillisFractional: 435}

	signal2 := MSM7SignalCell{
		SignalID:        2,
		PhaseRangeDelta: -117960}

	rangeCycles2, err := getPhaseRangeCycles(&header, &satellite2, &signal2)

	if err != nil {
		t.Fatalf(err.Error())
	}

	if !floatsEqualWithin3(expectedPhaseRange2, rangeCycles2) {
		t.Fatalf("expected %f got %f", expectedPhaseRange2, rangeCycles2)
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
	satellite2 := MSM7SatelliteCell{
		PhaseRangeRate: -135}

	signal2 := MSM7SignalCell{
		SignalID:            2,
		PhaseRangeRateDelta: -1070}

	rate2, err := getPhaseRangeRate(&header2, &satellite2, &signal2)
	if err != nil {
		t.Fatalf(err.Error())
	}

	if floatsEqualWithin3(expectedPhaseRangeRate2, rate2) {
		t.Fatalf("expected %f got %f", expectedPhaseRangeRate2, rate2)
	}
}

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
