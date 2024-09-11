package logger

import (
	"os"
	"sync"
	"testing"
	"time"

	"github.com/goblimey/go-tools/clock"
	"github.com/goblimey/go-tools/testsupport"

	"github.com/goblimey/go-ntrip/apps/rtcmlogger/config"
	"github.com/goblimey/go-ntrip/rtcm/utils"
)

// TestGetStartOfDay tests the getStartOfDay method.
func TestGetStartOfDay(t *testing.T) {
	// This replicates the logic of the function under test, so it's
	// really just a round trip test.
	now := time.Date(2020, time.February, 14, 22, 12, 0, 0, utils.LocationUTC)
	sod := getStartOfDay(now)
	if sod.Year() != 2020 {
		t.Fatalf("expected year to be 2020 actually %d", sod.Year())
	}

	if int(sod.Month()) != 2 {
		t.Errorf("expected month to be February actually %v", sod.Month())
	}

	if sod.Day() != 14 {
		t.Errorf("expected day to be 14 actually %d", sod.Day())
	}

	if sod.Hour() != startOfDayHour {
		t.Errorf("expected hour to be %d actually %d", startOfDayHour, sod.Hour())
	}

	if sod.Minute() != startOfDayMinute {
		t.Errorf("expected minute to be %d actually %d", startOfDayMinute, sod.Minute())
	}

	if sod.Second() != startOfDaySecond {
		t.Errorf("expected second to be %d actually %d", startOfDaySecond, sod.Second())
	}

	if sod.Nanosecond() != 0 {
		t.Errorf("expected nanoseconds to be 0 actually %d", sod.Nanosecond())
	}
}

// TestGetEndOfDay tests the getEndOfDay method.
func TestGetEndOfDay(t *testing.T) {
	// This replicates the logic of the function under test, so it's
	// really just a round trip test.
	now := time.Date(2020, time.February, 14, 22, 12, 0, 0, utils.LocationUTC)
	eod := getEndOfDay(now)
	if eod.Year() != 2020 {
		t.Errorf("expected year to be 2020 actually %d", eod.Year())
	}

	if int(eod.Month()) != 2 {
		t.Errorf("expected month to be February actually %v", eod.Month())
	}

	if eod.Day() != 14 {
		t.Errorf("expected day to be 14 actually %d", eod.Day())
	}

	if eod.Hour() != endOfDayHour {
		t.Errorf("expected hour to be %d actually %d", endOfDayHour, eod.Hour())
	}

	if eod.Minute() != endOfDayMinute {
		t.Errorf("expected minute to be %d actually %d", endOfDayMinute, eod.Minute())
	}

	if eod.Second() != endOfDaySecond {
		t.Errorf("expected second to be %d actually %d", endOfDaySecond, eod.Second())
	}

	if eod.Nanosecond() != 0 {
		t.Errorf("expected nanoseconds to be 0 actually %d", eod.Nanosecond())
	}
}

// TestShouldBeLogging tests that shouldBeLogging works when logging and
// when not logging.
func TestShouldBeLogging(t *testing.T) {
	now := time.Date(2020, time.February, 14, 22, 59, 0, 0, utils.LocationUTC)
	if !shouldBeLogging(now) {
		t.Errorf("shouldBeLogging() returns false during the day - at %v", now)
	}

	// Logging should be off at exactly end of day, exactly start of day and at
	// all times between.
	now = time.Date(2020, time.February, 14,
		endOfDayHour, endOfDayMinute, endOfDaySecond-1, 999999, utils.LocationUTC)
	if !shouldBeLogging(now) {
		t.Errorf("shouldBeLogging() returns false just before start of day - at %v", now)
	}

	now = time.Date(2020, time.February, 14,
		startOfDayHour, startOfDayMinute, startOfDaySecond, 0, utils.LocationUTC)
	if shouldBeLogging(now) {
		t.Errorf("shouldBeLogging() returns true at exactly start of day - at %v", now)
	}

	now = time.Date(2020, time.February, 14, 0, 0, 0, 0, utils.LocationUTC)
	if shouldBeLogging(now) {
		t.Errorf("shouldBeLogging() returns true at midnight - at %v", now)
	}

	now = time.Date(2020, time.February, 14,
		endOfDayHour, endOfDayMinute, endOfDaySecond, 0, utils.LocationUTC)
	if shouldBeLogging(now) {
		t.Errorf("shouldBeLogging() returns true at exactly end of day - at %v", now)
	}

	now = time.Date(2020, time.February, 14,
		startOfDayHour, startOfDayMinute, startOfDaySecond, 1, utils.LocationUTC)
	if !shouldBeLogging(now) {
		t.Errorf("shouldBeLogging() returns false just after start of day - at %v", now)
	}

	// Do the same again but with the UK timezone.
	now = time.Date(2020, time.April, 1,
		endOfDayHour, endOfDayMinute, endOfDaySecond-1, 999999, utils.LocationLondon)
	if !shouldBeLogging(now) {
		t.Errorf("shouldBeLogging() returns false just before start of day - at %v", now)
	}

	now = time.Date(2020, time.April, 1,
		startOfDayHour, startOfDayMinute, startOfDaySecond, 0, utils.LocationLondon)
	if shouldBeLogging(now) {
		t.Errorf("shouldBeLogging() returns true at exactly start of day - at %v", now)
	}

	now = time.Date(2020, time.April, 1, 0, 0, 0, 0, utils.LocationLondon)
	if shouldBeLogging(now) {
		t.Errorf("shouldBeLogging() returns true at midnight UTC- at %v", now)
	}

	now = time.Date(2020, time.April, 1,
		endOfDayHour, endOfDayMinute, endOfDaySecond, 0, utils.LocationLondon)
	if shouldBeLogging(now) {
		t.Errorf("shouldBeLogging() returns true at exactly end of day - at %v", now)
	}

	now = time.Date(2020, time.April, 1,
		startOfDayHour, startOfDayMinute, startOfDaySecond, 1, utils.LocationLondon)
	if !shouldBeLogging(now) {
		t.Errorf("shouldBeLogging() returns false just after start of day - at %v", now)
	}
}

// TestGetTodaysLogFilename checks that getTodaysLogFilename returns a filename
// containing today's timestamp.
func TestGetTodaysLogFilename(t *testing.T) {
	now := time.Date(2020, time.February, 14, 22, 59, 0, 0, utils.LocationUTC)
	const expectedFilename = "data.2020-02-14.rtcm3"

	filename := getTodaysLogFilename(now)

	if filename != expectedFilename {
		t.Errorf("expected filename to be \"%s\" actually \"%s\"", expectedFilename, filename)
	}
}

// TestWriteWhenLoggingEnabled checks that the Writer does not write
// to the log file when logging is disabled.
func TestWriteWhenLoggingEnabled(t *testing.T) {

	// NOTE:  this test uses the filestore.

	const loggingDirectory = "foo"
	const expectedFileContents = "hello world\n"

	wd, err := testsupport.CreateWorkingDirectory()
	if err != nil {
		t.Errorf("createWorkingDirectory failed - %v", err)
	}
	defer testsupport.RemoveWorkingDirectory(wd)

	times := []time.Time{
		time.Date(2020, 2, 14, 0, 0, 5, 1, utils.LocationUTC),
		time.Date(2020, 2, 14, 12, 0, 0, 0, utils.LocationUTC),
		time.Date(2020, 2, 14, 23, 59, 54, 999999, utils.LocationUTC),
	}

	clock := clock.NewSteppingClock(&times)

	// Create an RTCM Writer using the supplied clock.  Behind the scenes that will
	// create a daily logger with a real clock, so that will create a log file with
	// a datestamp that we can't easily predict.  However, there should only be one
	// logfile so we can just look for it.
	var m sync.Mutex
	writer := NewRTCMWriter(clock, loggingDirectory, &m)
	// This is only testing the write capability, so turn off the log
	// pusher.
	writer.YearOfLastWrite = time.Now().Year()
	writer.MonthOfLastWrite = time.Now().Month()
	writer.DayOfLastWrite = time.Now().Day()

	buffer := []byte("hello ")

	n, err := writer.Write(buffer)
	if err != nil {
		t.Errorf("Write failed - %v", err)
	}
	if n != len(buffer) {
		t.Errorf("Write returned %d - expected %d", n, len(buffer))
	}

	buffer = []byte("world")
	n, err = writer.Write(buffer)
	if err != nil {
		t.Errorf("Write failed - %v", err)
	}
	if n != len(buffer) {
		t.Errorf("Write returned %d - expected %d", n, len(buffer))
	}

	buffer = []byte("\n")
	n, err = writer.Write(buffer)
	if err != nil {
		t.Errorf("Write failed - %v", err)
	}
	if n != len(buffer) {
		t.Errorf("Write returned %d - expected %d", n, len(buffer))
	}

	// Find the log file.
	logDirectoryPathName := wd + "/" + loggingDirectory
	fileInfoList, err := os.ReadDir(logDirectoryPathName)
	if err != nil {
		t.Fatalf("Cannot scan directory %s - %v", logDirectoryPathName, err)
	}

	// fileInfoList should show exactly one file.
	if len(fileInfoList) == 0 {
		t.Errorf("directory %s is empty.  Should contain one log file.",
			logDirectoryPathName)
	}
	if len(fileInfoList) > 1 {
		t.Errorf("directory %s contains %d files.  Should be just one.",
			logDirectoryPathName, len(fileInfoList))
		for _, fileInfo := range fileInfoList {
			t.Errorf("found file %s", fileInfo.Name())
		}
		return
	}

	fileInfo := fileInfoList[0]

	filePathName := logDirectoryPathName + "/" + fileInfo.Name()
	file, err := os.Open(filePathName)
	if err != nil {
		t.Fatalf("Cannot open log file %s - %v", filePathName, err)
	}
	defer file.Close()

	b := make([]byte, 8096)
	length, err := file.Read(b)
	if err != nil {
		t.Fatalf("error reading logfile back - %v", err)
	}
	if length != len(expectedFileContents) {
		t.Fatalf("logfile %s contains %d bytes - expected %d",
			filePathName, length, len(buffer))
	}

	contents := string(b[:length])

	if contents != expectedFileContents {
		t.Fatalf("logfile %s contains \"%s\" - expected \"%s\"",
			filePathName, contents, expectedFileContents)
	}
}

// TestPushOldLogs checks that pushOldLogs moves the given file into the
// subdirectory "data.ready".  This test uses the filestore.
func TestPushOldLogs(t *testing.T) {

	// These are the fields of the date for the test.
	const year = 2020
	const monthNumber = 2
	const dayNumber = 14
	const hour = 12
	const minute = 13
	const second = 14
	const nanosecond = 15

	workingDirectory, err := testsupport.CreateWorkingDirectory()
	if err != nil {
		t.Fatalf("createWorkingDirectory failed - %v", err)
	}
	defer testsupport.RemoveWorkingDirectory(workingDirectory)

	loggingDirectory := workingDirectory + "/logs"

	// Create logging directory - ignore if it already exists.
	err = os.MkdirAll(loggingDirectory, os.ModePerm)
	if err != nil {
		t.Fatalf("TestSaveLog: cannot create logs directory %s - %s",
			loggingDirectory, err.Error())
	}

	// Create files "foo", "bar" and today's logfile.
	pathname1 := loggingDirectory + "/" + "foo"
	createFile(t, pathname1)

	pathname2 := loggingDirectory + "/" + "bar"
	createFile(t, pathname2)

	now := time.Date(
		year, monthNumber, dayNumber, hour, minute, second, nanosecond, utils.LocationUTC,
	)

	todaysLogFileName := getTodaysLogFilename(now)
	pathname3 := loggingDirectory + "/" + todaysLogFileName
	createFile(t, pathname3)

	// PushOldLogs is part of a log writer so we need
	// to create one of those.  That will produce
	// another log file as a side effect with a name
	// based on today's date, which we can't know for
	// sure (because this test may be running around
	// midnight).
	var m sync.Mutex
	systemClock := clock.NewSystemClock()
	writer := NewRTCMWriter(systemClock, loggingDirectory, &m)
	// turn off the log
	// pusher.
	writer.YearOfLastWrite = now.Year()
	writer.MonthOfLastWrite = now.Month()
	writer.DayOfLastWrite = now.Day()
	cfg := config.Config{
		MessageLogDirectory:        loggingDirectory,
		DirectoryForOldMessageLogs: "./old_logs",
	}
	writer.CFG = &cfg
	// Push the non-matching files into the subdirectory.
	// That will include the extra log file that we don't
	// know the name of.
	writer.pushOldLogs(loggingDirectory, now)

	// The logging directory should contain just the
	// logfile for 14th Feb 2020.
	files, err := os.ReadDir(loggingDirectory)
	if err != nil {
		t.Fatalf("cannot scan directory %s - %v", loggingDirectory, err)
	}
	if len(files) != 1 {
		t.Fatalf("expected the current directory to contain just one file, contains  %d",
			len(files))
	}

	// the directory for old logs should contain "foo",
	// "bar" and the log file that we don't know the name
	// of.
	directoryForOldLogs := writer.CFG.DirectoryForOldMessageLogs
	oldLogFiles, err2 := os.ReadDir(directoryForOldLogs)
	if err2 != nil {
		t.Errorf("cannot scan directory %s - %v", directoryForOldLogs, err2)
		return
	}
	if len(oldLogFiles) != 3 {
		t.Errorf("expected the directory %s to contain exactly 3 files, found  %d",
			directoryForOldLogs, len(oldLogFiles))
		return
	}

	// Check for foo and bar.
	foundFoo := false
	foundBar := false

	for _, f := range oldLogFiles {
		if f.Name() == "foo" {
			foundFoo = true
		}
		if f.Name() == "bar" {
			foundBar = true
		}
	}

	if !foundFoo || !foundBar {
		t.Errorf("expected foo, bar and one other, found %s, %s and %s",
			oldLogFiles[0].Name(), oldLogFiles[1].Name(), oldLogFiles[2].Name(),
		)
		return
	}
}

// createFile creates an empty file.
func createFile(t *testing.T, pathname string) {
	file, err := os.Create(pathname)
	if err != nil {
		t.Errorf("TestSaveLog: cannot create %s - %s", pathname, err.Error())
		return
	}
	file.Close()
}
