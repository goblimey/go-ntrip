package rtcm

// Incomplete message - looks like a real message with a length of 0xaa
// but we hit EOF while reading it.  Should be returned as a non-RTCM message.
var incompleteMessage = []byte{0xd3, 0x00, 0xaa, 0x46, 0x70, 0x00}

// Four bytes of junk which should be returned as a non-RTCM message,
// followed by the start of message byte.
var junkAtStart = []byte{'j', 'u', 'n', 'k', ' ', 'j', 'u', 'n', 'k', 0xd3}

// Four bytes of junk which should be returned as a non-RTCM message,
var allJunk = []byte{'j', 'u', 'n', 'k'}

// We should get this result when we read the junkAtStartReader or the junkReader.
const wantJunk = "junk"

var codeBias = []byte{0xd3, 0, 0x08,
	0x4c, 0xe0, 00, 0x8a, 0, 0, 0, 0,
	0xa8, 0xf7, 0x2a,
}

// testData contain a batch of RTCM3 messages.  In each message byte 0
// is 0xd3.  Bytes 1 and 2 form a sixteen bit unsigned number, the
// message length, but this is limited to 1023 so the top six bits
// are always 0.  The message follows and then three bytes of CRC.
var testData = []byte{
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
	0x51, 0x2c, 0x0d, 0xdf, 0x50, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xc4, 0xc9, 0x01,

	0xd3, 0x00,
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

// This is real data collected on the 13th November 2020 with some junk added
// to check that junk is handled properly as well as good data.
var realData = []byte{

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
	//        signal mask  cell mask (16 bits)
	//           0000 0000  0010 00|00 1000 0000 0000 0000 01|10 1101 11|11 1111
	//
	//            32-bit signal mask: signals 1 and 16.       16-bit cell mask - 0xdbff - 1101 1011 1111 1111
	//          0|010 0000  0000 0000  1000 0000  0000 0000 0|110 1101 1111 1111 1|010 1000
	//          |                                           |
	//           -----------v                       v-------    |Satellite Cells  16X8, 16X4, 16X10, 16X14 - 16X36 bits
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

	'j', // junk which should be returned as a non-RTCM message.

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

	'j', 'u', 'n', 'k', // junk which should be returned as a non-RTCM message.

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

	// Beidou
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

	// Incomplete message - should be returned as a non-RTCM message.
	0xd3, 0x00, 0xaa, 0x46, 0x70, 0x00, 0x66, 0xff, 0xbc, 0xa0, 0x00, 0x00, 0x00, 0x04, 0x00, 0x26,
	0x18, 0x00, 0x00, 0x00, 0x20, 0x02, 0x00, 0x00, 0x75, 0x53, 0xfa, 0x82, 0x42, 0x62, 0x9a, 0x80,
}

// This is what processMessages should send to the writer when it reads realData.
var expectedResultFromProcessMessages = []byte{
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
