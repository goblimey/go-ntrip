package testdata

import (
	"testing"

	"github.com/goblimey/go-crc24q/crc24q"
)

// This "test" is used to calculate the CRC when hand-crafting a bit stream.
// Run te test in a debugger and examine the value of crc.
func Test(t *testing.T) {

	var bitStream = []byte{
		0xd3, 0x00, 0xc3,
	//               | timestamp: 3-bit day, 27-bit milliseconds.
	0x43, 0xf0, 0x00, 0xf0, 0x00, 0x00, 0x06, 0x00, 0x00, 0x04, 0x0e, 0x03, 0x80,
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
	0xf8, 0x09, 0xc3, 0x73, 0xa0, 0x40,
	}

	crc := crc24q.Hash(bitStream)

	_ = crc
}
