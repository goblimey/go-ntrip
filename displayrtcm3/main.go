// displayrtcm3 reads bytes from stdin, ignores anything that's not
// in RTCM version 3 format and writes a readable form of the RTCM to
// stdout.  (Raw RTCM3 is a tightly compressed binary format.)  There
// are many different types of RTCM message, all in different formats.
// The most important for accurate GNSS systems are message type 1005,
// which gives the position of the base station, and Multiple Signal
// Messages, either type 4 (MSM4) or type 7 (MSM7).  These both contain
// the base station's observations of satellite signals.  Type 4 messages
// are of sufficient precision for 2-cm accurate positioning.  Type 7
// messages are of even high precision.  A GNSS device needs only emit
// either MSM4 or MSM7.
//
// Usage:
//		displayrtcm3 file date
//
// Example:
//		displayrtcm3 testdata.rtcm 2020-11-13
//
// The tool handles MSM data for GPS, Galileo, GLONASS and BeiDou
// satellites.  These can be observed from anywhere in the world given
// an open view of the sky.
//
// The RTCM data may contain other messages and these are displayed in
// "od" format - hex values and readable text.  They are mostly ASCII
// strings, for example NMEA messages, so they should be fairly readable.
//
// The program takes one argument which should be in the format
// "yyyy-mm-dd".  This is turned into a date/time at midnight UTC on
// that day.  This is used to figure out the start of the various
// GNSS weeks - the GPS week and so on.
//
// Each Multiple Signal Message (MSM) contains a timestamp.  The
// timestamps in the input data should relate to the current GNSS weeks.
// The timestamp is represented as milliseconds since some start date, in
// most cases the start of the week, which is midnight at the start of
// Sunday in some time zone.  The timestamp rolls over to zero at the
// start of the next period, so the tool needs to know which period it's
// handling.
//
// If the tool runs for a long time, the week will roll over into the
// next, and the next, and so on.
//
// The GPS timestamp rolls over a few seconds BEFORE midnight at the
// start of Sunday in UTC (in 2021, 18 seconds before midnight).
// Galileo uses the same time as GPS.  The Beidou timestamp rolls
// over 14 seconds AFTER midnight on Sunday.  The GLONASS timestamp
// contains two fields, day and milliseconds since the start of the day
// in the Moscow time zone.  Day 0 is Sunday which starts at 21:00 on
// Saturday evening in UTC.  Day 7 is an illegal value and should
// never occur.
//
package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	handler "github.com/goblimey/go-ntrip/file_handler"
	rtcm "github.com/goblimey/go-ntrip/rtcm/handler"
)

func main() {

	var startDate time.Time
	var reader *bufio.Reader
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
		ioReader, openError := openFile(fileName)
		if openError != nil {
			log.Fatalf("%s: cannot open %s - %v", appName, fileName, openError)
		}

		reader = bufio.NewReader(ioReader)
	}

	messageChan := make(chan rtcm.Message, 100)
	fileHandler := handler.New(startDate, messageChan)

	// The output is always to stdout.

	// Write the heading.
	fmt.Printf("RTCM data\n")
	fmt.Printf("\nNote: times are in UTC.  RINEX format uses GPS time, which is currently (Jan 2021)\n")
	fmt.Printf("18 seconds ahead of UTC\n\n")

	// Fetch the messages.
	go fileHandler.Handle(reader)

	// Display the messages.
	for {
		message, ok := <-messageChan
		if !ok {
			// No more message.  We're done.
			os.Exit(0)
		}

		_, writeError := fmt.Println(message.String())
		if writeError != nil {
			os.Stderr.Write([]byte(writeError.Error()))
		}
	}
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
func writeReadableMessages(ch chan rtcm.Message, rtcmHandler *rtcm.Handler, writer io.Writer) {

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
