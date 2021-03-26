// displayrtcm3 reads bytes from stdin, ignores anything that's not
// in RTCM version 3 format and writes a readable form of the RTCM to
// stdout.  (Raw RTCM3 is a tightly compressed binary format.)  There
// are many different types of RTCM message, all in different formats.
// The most important for accurate GNSS systems are message type 1005,
// which gives the position of the base station, and Multiple Signal
// Messages type 7 (MSM7), which give the base station's observations
// of satellite signals to a high precision.  The tool displays
// these messages is a readable format.  It handles MSM7 data for
// GPS, Galileo, GLONASS and BeiDou satellites.  These can be observed
// from anywhere in the world, assuming an open view of the sky. The
// RTCM data may contain other messages and these are displayed in their
// raw binary format.
//
// The program takes one argument which should be in the format
// "yyyy-mm-dd".  The RTCM input should begin some time on that day.
// If no start date is specified it's assumed that the data is being
// generated live and the current date/time is used.
//
// Some RTCM messages contain a timestamp represented as
// milliseconds since the start of an epoch, which is midnight at
// the start of Sunday in some time zone.  This timestamp rolls over
// every week, so the tool needs to know the data of the first
// message.
//
// The GPS timestamp rolls over a few seconds before midnight at the
// start of Sunday in UTC (in 2021, 18 seconds before midnight).
// Galileo uses the same timestamp as GPS.  The Beidou timestamp
// rolls over 14 seconds AFTER midnight on Sunday.  The GLONASS
// timestamp contains two fields, day and milliseconds since the start
// of the day in the Moscow time zone.  Day 0 is Sunday, which start
// at 21:00 on Saturday evening in UTC.
//
// If the tool is run without specifying a start date it uses the
// current date and time as its starting point for decoding the
// timestamps.  It's therefore most important that the system clock is
// correct, especially if the tool is started around 9pm on a Saturday
// or around midnight on a Sunday, which is when the timestamps roll
// over.
//
package main

import (
	"fmt"
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
	}

	rtcm := rtcm.New(startDate)
	fmt.Printf("RTCM data\n")
	fmt.Printf("\nNote: times are in UTC.  RINEX format uses GPS time, which is currently (Jan 2021)\n")
	fmt.Printf("18 seconds ahead of UTC\n\n")

	for {
		rawMessage, err := rtcm.ReadNextMessageFrame(os.Stdin)
		if err != nil {
			// Probably EOF.
			return
		}

		decode(rtcm, rawMessage)

	}
}

func decode(rtcm *rtcm.RTCM, rawMessage []byte) {

	message, err := rtcm.GetMessage(rawMessage)
	if err != nil {
		fmt.Printf("illegal message - %s", err.Error())
		return
	}

	// write the decoded message.
	fmt.Printf("%s\n", rtcm.DisplayMessage(message))
}
