package logger

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"time"

	"github.com/goblimey/go-tools/clock"
	"github.com/goblimey/go-tools/dailylogger"
)

// Writer satisfies the io.Writer interface and writes data (which are presumed to
// be RTCM messages) to a log file.  It uses the daily logger so there is a
// separate log file produced each day with a datestamped name, for example for the
// 31st January 2020 the log file is "data.2020-01-31.rtcm3".  Some of the
// organisations that process RTCM data insist that each file runs over no more
// than 24 hours so this practice suits them.
//
// On the first write of the day the logger scans the log directory and pushes any
// files produced on other days into the "data.ready" subdirectory, creating it if
// necessary.  Typically files produced before yesterday will already have been
// dealt with so it will only need to push yesterday's log.
//
// The data arrives into the system in blocks, each containing many RTCM messages.
// A block that arrives just after midnight could contain messages from yesterday
// and today and that could bust the requirement to have only 24 hours worth of
// messages in each file. Also, the host machine's clock may have drifted a little
// so it may disagree with the device sending the RTCM messages precisely when
// midnight occurs.  To avoid all these problems the writer avoids logging for a
// few seconds around midnight.
//
// Dates and times are in local time.
//
// It's assumed that some process watches the "data.ready" subdirectory and does
// sensible things when a new file appears there, for example converts it to RINEX
// format.
//
type Writer struct {
	clock        clock.Clock         // This clock may be a fake during testing.
	logWriter    *dailylogger.Writer // The daily log writer.
	pushing      bool                // true if we should check for old logs to push at end of day.
	logDirectory string              // The directory in which to create the logs
}

// Start logging at 00:00:05
const startOfDayHour = 0
const startOfDayMinute = 0
const startOfDaySecond = 5

// Stop logging at 23:59:55.
const endOfDayHour = 23
const endOfDayMinute = 59
const endOfDaySecond = 55

const subDirectoryForOldLogs = "data.ready"

// This is a compile-time check that Writer implements the io.Writer interface.
var _ io.Writer = (*Writer)(nil)

// New creates a Writer with a system clock and returns it as an io.Writer.
//
func New(logDirectory string) io.Writer {
	clock := clock.NewSystemClock() // The real system clock.
	writer := NewRTCMWriter(clock, logDirectory)
	go writer.logControl()
	go writer.logPusher()

	return writer
}

// NewRTCMWriter creates a Writer and returns it as a log.Writer.  It's called by New
// and can be called explicitely by tests.
//
func NewRTCMWriter(clock clock.Clock, logDirectory string) *Writer {
	logWriter := dailylogger.New(logDirectory, "data.", ".rtcm3")
	writer := Writer{clock: clock, logWriter: logWriter, pushing: true}
	writer.logDirectory = logDirectory
	return &writer
}

// Write writes the buffer to the daily log file, creating the
// file at the start of each day.
//
func (lw *Writer) Write(buffer []byte) (n int, err error) {

	if shouldBeLogging(lw.clock.Now()) {
		// Push old logs once at the end of the day.
		lw.pushing = true
		// Write to the log.
		n, err = lw.logWriter.Write(buffer)
		return n, err
	} else {
		if lw.pushing {
			// Push old logs once.

			lw.pushing = false
		}
		// We don't log anything but we return the buffer length so that
		// caller doesn't think there has been an error.
		return len(buffer), err
	}

}

// logControl disables logging at the end of each day and enables it at
// the start of the next day.  It should be rumn in a goroutine.
func (lw *Writer) logControl() {

	// This should be run in a goroutine.
	//
	// As it runs forever it can't be unit tested.

	for {
		now := time.Now()
		if shouldBeLogging(now) {
			// The time is between start of day and end of day.
			// Wait until end of day.
			endOfDay := getEndOfDay(now)
			sleepTime := time.Until(endOfDay)
			secondsToGo := (sleepTime / time.Second) % 60
			minutesToGo := sleepTime / time.Minute % 60
			hoursToGo := sleepTime / time.Hour % 24
			log.Printf("logControl: logging disabled.  Sleeping for %02d:%02d:%02d until %v\n",
				hoursToGo, minutesToGo, secondsToGo, endOfDay)
			time.Sleep(sleepTime)
			// It's end of day.  Turn off logging.
			lw.logWriter.DisableLogging()
		} else {
			// The time is between end of day today and start
			// of day tomorrow.  Wait until start of day and
			// then turn on logging.
			//
			// AddDate is the recommended way to add days to a
			// time value.)
			startOfDayTomorrow := now.AddDate(0, 0, 1)
			sleepTime := time.Until(startOfDayTomorrow)
			secondsToGo := (sleepTime / time.Second) % 60
			minutesToGo := sleepTime / time.Minute % 60
			hoursToGo := sleepTime / time.Hour % 24
			log.Printf("logControl: logging enabled.  Sleeping for %02d:%02d:%02d until %v\n",
				hoursToGo, minutesToGo, secondsToGo, startOfDayTomorrow)

			time.Sleep(sleepTime)
			// It's the next day and time to start logging.
			lw.logWriter.EnableLogging()
		}
	}
}

// logPusher runs forever.  At midnight at the start of each day it runs
// the log pusher in a goroutine.  It should itself be run in a goroutine.
func (lw Writer) logPusher() {
	for {
		now := time.Now()
		midnight := getNextMidnight(now)
		sleepTime := time.Until(midnight)
		secondsToGo := (sleepTime / time.Second) % 60
		minutesToGo := sleepTime / time.Minute % 60
		hoursToGo := sleepTime / time.Hour % 24
		log.Printf("logPusher: sleeping for %02d:%02d:%02d until %v\n",
			hoursToGo, minutesToGo, secondsToGo, midnight)
		time.Sleep(sleepTime)
		// It's the next morning.  Push any old logs.
		go pushOldLogs(lw.logDirectory, time.Now())

		// Paranoia.  In case this woke up very slightly early
		// and it's not quite midnight, pause for a short time.
		time.Sleep(1 * time.Second)
	}
}

// getEndOfDay gets the time that we stop logging today, in the local
// timezone.
//
func getEndOfDay(now time.Time) time.Time {
	location := now.Location()
	return time.Date(now.Year(), now.Month(), now.Day(),
		endOfDayHour, endOfDayMinute, endOfDaySecond, 0, location)
}

// getStartOfDay gets the time that we start logging today, in the local
// timezone.
//
func getStartOfDay(now time.Time) time.Time {
	location := now.Location()
	return time.Date(now.Year(), now.Month(), now.Day(),
		startOfDayHour, startOfDayMinute, startOfDaySecond, 0, location)
}

// getNextMidnight gets midnight at the start of the day after the given
// time.
func getNextMidnight(now time.Time) time.Time {
	// AddDate is the recommended way to add days to a
	// date/time value.)
	nextDay := now.AddDate(0, 0, 1)
	return time.Date(nextDay.Year(), nextDay.Month(), nextDay.Day(),
		0, 0, 0, 0, now.Location())
}

// shouldBeLogging returns true if the given time is between the start
// of day and the end of day in the same timezone, false otherwise.
// (Note: at exactly start of day and at exactly end of day it returns
// false.)
//
func shouldBeLogging(now time.Time) bool {
	return getStartOfDay(now).Before(now) &&
		getEndOfDay(now).After(now)
}

// getTodaysLogFilename gets the name of today's logfile, for example
// "data.2020-02-14.rtcm3".
//
func getTodaysLogFilename(now time.Time) string {
	return fmt.Sprintf("data.%04d-%02d-%02d.rtcm3",
		now.Year(), int(now.Month()), now.Day())
}

// pushOldLogs searches the logging directory and pushes all plain files
// except for today's log file into the subdirectory for old logs.
func pushOldLogs(logDirectory string, now time.Time) {
	logFilename := getTodaysLogFilename(now)
	files, err := ioutil.ReadDir(logDirectory)
	if err != nil {
		log.Fatal("pushOldLogs: cannot open logging directory " +
			logDirectory + " - " + err.Error())
	}

	for _, fileInfo := range files {
		if fileInfo.Name() == logFilename {
			// Ignore today's log.
			continue
		}
		if fileInfo.IsDir() {
			// Ignore any directories (including the subdirectory for old logs).
			continue
		}

		pushLogfile(logDirectory, fileInfo.Name())
	}
}

// pushLogfile takes the logFilename and pushes it the subdirectory for
// old log files.
//
func pushLogfile(logDirectory, logFilename string) {
	// Ensure that the destination directory exists.
	subdirectory := logDirectory + "/" + subDirectoryForOldLogs
	err := os.MkdirAll(subdirectory, os.ModePerm)
	if err != nil {
		log.Fatal("pushLogFile: cannot create directory " +
			logDirectory + " - " + err.Error())
	}
	logFilePath := logDirectory + "/" + logFilename
	newLogFilePath := subdirectory + "/" + logFilename
	err = os.Rename(logFilePath, newLogFilePath)
	if err != nil {
		log.Printf("pushLogfile - warning - failed to move logfile %s to directory %s - %v\n",
			logFilename, newLogFilePath, err)
	}
}
