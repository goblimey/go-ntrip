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
	"io"
	"log"
	"os"
	"time"

	"github.com/goblimey/go-ntrip/rtcm"
)

func main() {

	var startDate time.Time
	var reader io.Reader
	if len(os.Args) < 3 {
		log.Fatalf("usage: %s file yyyy-mm-dd", os.Args[0])
	} else {
		appName := os.Args[0]

		// The format of arg[2] should be yyyy-mm-dd.
		var timeError error
		startDate, timeError = getTime(os.Args[2])
		if timeError != nil {
			log.Fatalf("usage: %s file yyyy-mm-dd", appName)
		}

		fileName := os.Args[1]
		var openError error
		reader, openError = openFile(fileName)
		if openError != nil {
			log.Fatalf("%s: cannot open %s - %v", appName, fileName, openError)
		}

	}

	rtcmHandler := rtcm.New(startDate, log.Default())
	// Process the input file and then stop.
	rtcmHandler.StopOnEOF = true

	// The output is always to stdout.

	// Write the heading.
	fmt.Printf("RTCM data\n")
	fmt.Printf("\nNote: times are in UTC.  RINEX format uses GPS time, which is currently (Jan 2021)\n")
	fmt.Printf("18 seconds ahead of UTC\n\n")

	// Run HandleMessages with a single channel connected to a
	// goroutine that displays each message in readable form.
	var channels []chan rtcm.RTCM3Message
	const channelCap = 1000
	stdoutChannel := make(chan rtcm.RTCM3Message, channelCap)
	defer close(stdoutChannel)
	channels = append(channels, stdoutChannel)
	go writeReadableMessages(stdoutChannel, rtcmHandler, os.Stdout)

	rtcmHandler.HandleMessages(reader, channels)
}

// getTime gets a time from a string in one of three formats,
// yyyy-mm-dd{:hh:mm:ss{:timezone}}".  Timezones are listed
// here:  https://en.wikipedia.org/wiki/List_of_tz_database_time_zones.
// Note that three letter abbreviations such as "CET" are deprecated.
func getTime(timeStr string) (time.Time, error) {
	const dateLayout = "2006-01-02"

	if len(dateLayout) == len(timeStr) {
		dateTime, err := time.Parse(dateLayout, timeStr)
		return dateTime, err
	} else {
		dateTime, err := time.Parse(time.RFC3339, timeStr)
		return dateTime, err
	}
}

// openFile opens the given file and returns a Reader connected
// to it.  If the file name is "-" it returns os.Stdin
func openFile(fileName string) (io.Reader, error) {
	if fileName == "-" {
		return os.Stdin, nil
	}

	file, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}

	return file, nil
}

// writeReadableMessages receives the RTCM messages from the channel,
// decodes them to readable form and writes the result to the given
// writer.  If the channel is closed or there is a write error, it
// terminates.  It can be run in a go routine.
func writeReadableMessages(ch chan rtcm.RTCM3Message, rtcmHandler *rtcm.RTCM, writer io.Writer) {

	for {
		message, ok := <-ch
		if !ok {
			return
		}
		// Decode the message.  (The result is very verbose!)
		display := fmt.Sprintf("%s\n", rtcmHandler.DisplayMessage(&message))
		writer.Write([]byte(display))
	}
}
