package log

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/goblimey/go-ntrip/rtcmlogger/clock"
	"github.com/goblimey/go-tools/switchWriter"
	"github.com/robfig/cron"
)

// Writer satisfies the io.Writer interface and writes data (which are presumed to
// be RTCM messages) to a log file.  Behind the scenes it applies some rules to
// the data written.  These fulfill two goals (1) the Writer produces a daily log
// file with a datestamped name, for example "data.200131.rtcm3", and (2) the data
// in each file should contain no more than 24 hours worth of messages.  That
// requirement is imposed by several of the organisations that process the log data.
//
// The data arrives in blocks each containing many messages and a block that
// arrives just after midnight could contain messages from yesterday and today.
// In any case, the host machine's clock may have drifted a little.  To avoid all
// these problems and to give time for the log file to be rolled over, the Writer
// avoids logging around midnight.
//
// Calls to Write() within one minute before or after midnight are ignored -
// nothing is written.  A cron-style "job" runs at one minute before midnight
// that saves the day's log and pushes it into a subdirectory to signal that it's
// ready for processing.  The first call of Write after 00:01 am creates a new
// log file for that day.  Subsequent calls during the day write to the same file.
//
// It's assumed that some process watches the subdirectory and does sensible things
// when a new file appears there, for example, converts it to RINEX format.
//
// Using the Writer to create the day's log file depends on it being called, and
// that only happens when a buffer arrives containing RTCM messages.  Normally, one
// arrives about every second.  If they stop arriving for some reason, no new
// log file will be created.
//
type Writer struct {
	logMutex        sync.Mutex
	clock           clock.Clock
	currentYYYYMMDD string               // The timestamp string.
	logFile         *os.File             // The current log file (nil if not logging).
	switchWriter    *switchWriter.Writer // The connection to the file.
	cronjob         *cron.Cron
}

const endOfDayHour = 23
const endOfDayMinute = 59
const endOfDaySecond = 0

// This is a compile-time check that Writer implements the io.Writer interface.
var _ io.Writer = (*Writer)(nil)

// New creates a Writer and returns it as an io.Writer.
//
func New() io.Writer {

	// Check at midnight each night that the log has been tidied away.
	cr := cron.New()
	writer := &Writer{clock: clock.NewSystemClock(), switchWriter: switchWriter.New(),
		cronjob: cr}
	cr.AddFunc("0, 0, *, *, *", writer.endOfDay)
	cr.Start()
	return writer
}

// newRtcmWriterWithClock creates a Writer with a supplied clock and returns a pointer to it.
// (This is used for testing.)
//
func newRtcmWriterWithClock(clock clock.Clock) *Writer {
	return &Writer{clock: clock, switchWriter: switchWriter.New(), cronjob: nil}
}

// Write writes the buffer to the daily log file, creating the file at the start of each day.
//
func (lw *Writer) Write(buffer []byte) (n int, err error) {

	// Avoid a race with endOfDay.
	lw.logMutex.Lock()
	defer lw.logMutex.Unlock()

	if !lw.loggingAllowed() {
		// We have reached end of day and should not be logging.
		if lw.logFile != nil {
			// On the first call after end of day, close the log file.
			lw.switchWriter.SwitchTo(nil)
			lw.closeLog()
		}
		// We don't log anything but we return the buffer length so that
		// caller doesn't think there has been an error.
		return len(buffer), err
	}

	// Logging is enabled.
	yyyymmdd := lw.todayYYYYMMDD()
	if lw.logFile == nil || yyyymmdd != lw.currentYYYYMMDD {
		// We have just started up or the day has rolled over.  Create today's
		// log file and set the SwitchWriter writing to it.
		fmt.Println("start of day")
		file, err := openFile(getFilename(yyyymmdd))
		if err != nil {
			log.Fatal(err)
		}

		lw.currentYYYYMMDD = yyyymmdd
		lw.logFile = file // Hang onto the file so that we can close it later.
		lw.switchWriter.SwitchTo(file)
	}

	// Write to the log.
	n, err = lw.switchWriter.Write(buffer)

	return n, err
}

// SetCronjob sets the Writer's cronjob.
//
func (lw *Writer) SetCronjob(cronjob *cron.Cron) {
	lw.cronjob = cronjob
}

// todayYYMMDD returns today's date in the UTC timezone in yyyymmdd format.
//
func (lw *Writer) todayYYYYMMDD() string {
	nowUTC := lw.clock.Now().In(time.UTC)
	year := nowUTC.Year()
	return fmt.Sprintf("%04d%02d%02d", year, nowUTC.Month(), nowUTC.Day())
}

// tomorrowYYMMDD returns tomorrow's date in the UTC timezone in yyyymmdd format.
//
func (lw *Writer) tomorrowYYYYMMDD() string {
	tomorrowUTC := lw.clock.Now().In(time.UTC).Add(24 * time.Hour)
	year := tomorrowUTC.Year()
	return fmt.Sprintf("%04d%02d%02d", year, tomorrowUTC.Month(), tomorrowUTC.Day())
}

// loggingAllowed return true if logging should not be enabled, false otherwise.
// Logging is enabled all day except for one minute either side of midnight UTC.
//
func (lw *Writer) loggingAllowed() bool {
	nowUTC := lw.clock.Now().In(time.UTC)
	if nowUTC.Hour() == 0 && nowUTC.Minute() == 0 {
		return false
	}
	if nowUTC.Hour() == 23 && nowUTC.Minute() == 59 {
		return false
	}
	return true
}

// endOfDay saves the day's log.  It should be run soon after logging is
// disabled at the end of the day.  In that case it will not clash with
// a call of Writer trying to write to the log.  If endOfDay is delayed
// for some reason, a clash could occur, but the mutex will prevent a
// race.  In the worst case, the log will not be rolled over and will
// contain material from more than one day.
//
func (lw *Writer) endOfDay() {
	// Avoid a race with Write.
	lw.logMutex.Lock()
	defer lw.logMutex.Unlock()

	fmt.Println("end of day")

	// endOfDay should only be run when logging is disabled.
	if lw.loggingAllowed() {
		fmt.Fprintf(os.Stderr, "warning - endOfDay called when logging is allowed\n")
		return
	}

	lw.closeLog()
}

// closeLog closes any open log file and runs a goroutine to deal with it.
// It does not set the mutex, so it should ONLY be called by another method
// which does.
//
func (lw *Writer) closeLog() {
	fmt.Println("closeLog")
	if lw.logFile != nil {
		fmt.Println("saving " + lw.logFile.Name())
		oldLogName := lw.logFile.Name()
		err := lw.logFile.Close()
		if err != nil {
			fmt.Fprintf(os.Stderr,
				"logger: warning - error while closing logfile. %v.  Continuing", err)
		}

		go saveLog(oldLogName /*getFilename(lw.currentYYYYMMDD)*/)
		lw.logFile = nil
	}
}

// saveLog takes the data in logFilename and saves it in the subdirectory "data.ready".
//
func saveLog(logFilename string) {

	fmt.Println("saveLog")
	// Ensure that the directory exists.
	command := exec.Command("mkdir", "-p", "data.ready")
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	err := command.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "rtcmlogger: error while creating directory \"data.ready\" - %v\n", err)
	}
	// Move the file.
	command = exec.Command("mv", logFilename, "data.ready")
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	err = command.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "rtcmlogger: failed to move logfile %s to data.ready directory - %v\n", logFilename, err)
	}
}

// getFilename returns today's log filename, for example "data.200119.rtcm3".
//
func getFilename(yyyymmdd string) string {
	if yyyymmdd == "" {
		// When logging is disabled return the null file.  (This is
		// a backstop measure.  It's better for the caller to detect
		// this situation and avoid opening a log file at all.)
		return "/dev/null"
	}

	return "data." + yyyymmdd + ".rtcm3"
}

//openFile either creates and opens the file or, if it already exists, opens it
// in append mode.
//
func openFile(name string) (*os.File, error) {
	file, err := os.OpenFile(name, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}
	_, err = file.Seek(0, 2)
	if err != nil {
		log.Fatal(err)
	}
	return file, nil
}

// getDurationToEndOfDay gets the duration between start and the end of day (which is in UTC).
//
func getDurationToEndOfDay(start time.Time) time.Duration {
	startInUTC := start.In(time.UTC)
	locationUTC, _ := time.LoadLocation("UTC")
	endOfDay :=
		time.Date(startInUTC.Year(), startInUTC.Month(), startInUTC.Day(),
			endOfDayHour, endOfDayMinute, endOfDaySecond, 0, locationUTC)
	return endOfDay.Sub(startInUTC)
}
