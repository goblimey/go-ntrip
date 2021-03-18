// The rtcmfilter reads bytes from stdin, filters out anything
// that's not an rtcm message and writes the remaining data
// to stdout.  It's meant to be used to clean up a stream of
// incoming data from some unreliable channel and pass it on
// to an ntrip client, which in turn passes it to an upstream
// server over a tcp/ip connection.  The tcp/ip protocol
// protects the data from any further corruption.
//
// An rtcm message (strictly a message frame) is a stream of
// bits starting with 0xd3.  That's the start of a 24-bit
// 3-byte header.  Bit 0 is the top bit of the first byte, so
// the 0xd3 is bits 0 to 7.  Bits 8-13 are always zero, bits
// 14-23 are a 10-bit unsigned value giving the length in bytes
// of the embedded message.  The message comes next, followed
// by a 24-bit 3-byte cyclic redundancy check (CRC) value:
//
//     header message CRC
//     < message frame  >
//
// The CRC is created using the Qalcomm algorithm and can be
// checked by taking the message not including the CRC, creating
// a new CRC value and comparing that with the given value.
//
// The incoming stream of data can be a mixture of RTCM and other
// messages.  It's assumed to come from a GNSS device which is
// issuing messages continuously on some noisy channel. (In my
// case it's a Ublox ZED-FP9 sending data on a serial USB or IRC
// connection.  These are both prone to dropping or scrambling
// the occasional character.)  When the filter starts up, it picks
// up the messages at some arbitrary point and scans for a 0xd3
// byte.  This may or may not be the start of a valid message.
// It reads the next two bytes, assuming that it's the header,
// gets the length and reads the rest of the message.  It checks
// the CRC.  That check may fail if the 0xd3 byte was not the
// start of a message, or if the message was scrambled in transit.
//
// If the message is valid, it's written to standard output and
// the filter scans for the next one.  Intervening text and
// message frames that fail the CRC check are discarded.
//
// The embedded rtcm message is binary so it may contain 0xd3
// values by simple coincidence.  If you read this byte, you
// can't be certain that it's the start of a message frame.  You
// need to read the next two bytes to get what you assume is the
// header, check that bits 8-13 are zero, get the message length,
// read the whole message frame and check the CRC.
//
// The first 12 bits of the embedded message (bits 24-35 of the
// message frame) contain the message number (message type) 0-4095.
// Each message type has a different format, all starting with
// this 12-bit type number.  However, for the purposes of accurate
// GNSS, only a few type matter: 1005, which gives the position of
// the device, and a Multiple Signal Message type 7 (MSM7) for each
// satellite constellation:
//
// Type 1005: base station position
// type 1077: high-resolution GPS signals
// type 1087: high-resolution Glonass signals
// type 1097: high resolution Galileo signals
// tupe 1107: high resolution SBAS signals
// type 1117: high resolution QZSS signals
// type 1127: high resolution Beidou signals
// type 1137: high resolution NavIC/IRNSS signals
//
// MSM7 messages contain a timestamp, milliseconds from the
// constellation's epoch, which rolls over every week.  To
// convert this to a UTC time you need to know which week you
// are in when at startup time.  The program assumes that the
// data is coming from a live source and gets the system time
// when it starts up.  So your machine's system time must be set
// reasonably accurately.  This matters particularly if you
// start it around the time of an epoch rollover.  For example
// if you start it at midnight at the end of Saturday and your
// system clock is out by a few seconds, the program might
// assume the wrong GPS week.
//
// The GNSS device should be configured to send a batch of
// messages every second so there should only be a short delay
// between each batch of messages arriving.  If there is, then
// the host machine has probably lost contact with the GNSS device.
// The filter should close the input channel, reopen it and
// continue.
//
// My filter runs on a Raspberry Pi in a setup like this:
//
//     -----------------                          ----------------
//     | Ublox ZED-F9P | -----------------------> | Raspberry Pi |
//     -----------------    IRC or serial USB     ----------------
//
//
// The Raspberry Pi has four USB ports and four devices
// /dev/ttyACM0, /dev/ttyACM1, and so on.  These only exist when a
// USB device is connected.  When a device connects, the system
// creates one of the devices to handle the traffic.  It can use
// any one. If the device disconnects and reconnects, the system may
// choose a different device.
//
// The filter looks for a file rtcmfilter.json in the current
// directory when it starts up.  This contains a list of devices
// to try on startup and on reconnection.
//
// My device scans the satellites once every second for signals
// and issues one MSM7 message for each constellation.  From
// where I am in the UK it can see four constellations, GPS,
// Glonass, Galileo and Beido.  The MSM7 messages list the
// satellites that it detected and the signals sent by each.  My
// receiver is a dual-band device, so I get up to 2 signals from
// each satellite.  The result is a batch of four or five messages
// every second, each 500 to 1,000 characters long.  (The fifth
// message is a type 1005, which is not sent every second.)
//
// I can only test the handling of messages that my device can see,
// so I've only tested types 1005, 1077, 1087, 1097 and 1227.
//
package main

import (
	"log"
	"os"
	"time"

	"github.com/goblimey/go-ntrip/rtcm"
)

func main() {
	// The format of arg[1] should be yyyy-mm-dd.
	var startDate time.Time
	var err error
	if len(os.Args) > 1 {
		const dateLayout = "2006-01-02"
		startDate, err = time.Parse(dateLayout, os.Args[1])
		if err != nil {
			log.Fatal("usage: go-ntrip yyyy-mm-dd")
		}
	} else {
		startDate = time.Now()
	}

	rtcm := rtcm.New(startDate)

	for {
		// Get the next message, discarding any intervening non-RTCM3 material.
		frame, err := rtcm.ReadNextMessageFrame(os.Stdin)
		if err != nil {
			// Probably EOF.
			return
		}

		message, err := rtcm.GetMessage(frame)

		// Copy the RTCM message to stdout
		os.Stdout.Write(message.RawData)
	}
}
