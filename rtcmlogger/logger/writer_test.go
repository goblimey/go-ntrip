package logger

import (
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/goblimey/go-tools/clock"
)

// TestGetStartOfDay tests the getStartOfDay method.
func TestGetStartOfday(t *testing.T) {
	// This replicates the logic of the function under test, so it's
	// really just a round trip test.
	locationUTC, _ := time.LoadLocation("UTC")
	now := time.Date(2020, time.February, 14, 22, 12, 0, 0, locationUTC)
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
	locationUTC, _ := time.LoadLocation("UTC")
	now := time.Date(2020, time.February, 14, 22, 12, 0, 0, locationUTC)
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
//
func TestShouldBeLogging(t *testing.T) {
	locationUTC, _ := time.LoadLocation("UTC")
	now := time.Date(2020, time.February, 14, 22, 59, 0, 0, locationUTC)
	if !shouldBeLogging(now) {
		t.Errorf("shouldBeLogging() returns false during the day - at %v", now)
	}

	// Logging should be off at exactly end of day, exactly start of day and at
	// all times between.
	now = time.Date(2020, time.February, 14,
		endOfDayHour, endOfDayMinute, endOfDaySecond-1, 999999, locationUTC)
	if !shouldBeLogging(now) {
		t.Errorf("shouldBeLogging() returns false just before start of day - at %v", now)
	}

	now = time.Date(2020, time.February, 14,
		startOfDayHour, startOfDayMinute, startOfDaySecond, 0, locationUTC)
	if shouldBeLogging(now) {
		t.Errorf("shouldBeLogging() returns true at exactly start of day - at %v", now)
	}

	now = time.Date(2020, time.February, 14, 0, 0, 0, 0, locationUTC)
	if shouldBeLogging(now) {
		t.Errorf("shouldBeLogging() returns true at midnight - at %v", now)
	}

	now = time.Date(2020, time.February, 14,
		endOfDayHour, endOfDayMinute, endOfDaySecond, 0, locationUTC)
	if shouldBeLogging(now) {
		t.Errorf("shouldBeLogging() returns true at exactly end of day - at %v", now)
	}

	now = time.Date(2020, time.February, 14,
		startOfDayHour, startOfDayMinute, startOfDaySecond, 1, locationUTC)
	if !shouldBeLogging(now) {
		t.Errorf("shouldBeLogging() returns false just after start of day - at %v", now)
	}

	// Do the same again but with the UK timezone.

	locationUK, _ := time.LoadLocation("Europe/London")
	now = time.Date(2020, time.April, 1,
		endOfDayHour, endOfDayMinute, endOfDaySecond-1, 999999, locationUK)
	if !shouldBeLogging(now) {
		t.Errorf("shouldBeLogging() returns false just before start of day - at %v", now)
	}

	now = time.Date(2020, time.April, 1,
		startOfDayHour, startOfDayMinute, startOfDaySecond, 0, locationUK)
	if shouldBeLogging(now) {
		t.Errorf("shouldBeLogging() returns true at exactly start of day - at %v", now)
	}

	now = time.Date(2020, time.April, 1, 0, 0, 0, 0, locationUK)
	if shouldBeLogging(now) {
		t.Errorf("shouldBeLogging() returns true at midnight UTC- at %v", now)
	}

	now = time.Date(2020, time.April, 1,
		endOfDayHour, endOfDayMinute, endOfDaySecond, 0, locationUK)
	if shouldBeLogging(now) {
		t.Errorf("shouldBeLogging() returns true at exactly end of day - at %v", now)
	}

	now = time.Date(2020, time.April, 1,
		startOfDayHour, startOfDayMinute, startOfDaySecond, 1, locationUK)
	if !shouldBeLogging(now) {
		t.Errorf("shouldBeLogging() returns false just after start of day - at %v", now)
	}
}

// TestGetTodaysLogFilename checks that getTodaysLogFilename returns a filename
// containing today's timestamp.
//
func TestGetTodaysLogFilename(t *testing.T) {
	locationUTC, _ := time.LoadLocation("UTC")
	now := time.Date(2020, time.February, 14, 22, 59, 0, 0, locationUTC)
	const expectedFilename = "data.2020-02-14.rtcm3"

	filename := getTodaysLogFilename(now)

	if filename != expectedFilename {
		t.Errorf("expected filename to be \"%s\" actually \"%s\"", expectedFilename, filename)
	}
}

// TestWriteWhenLoggingEnabled checks that the Writer does not write
// to the log file when logging is disabled.
//
func TestWriteWhenLoggingEnabled(t *testing.T) {

	// NOTE:  this test uses the filestore.

	const loggingDirectory = "foo"
	const expectedFileContents = "hello world\n"

	wd, err := createWorkingDirectory()
	if err != nil {
		t.Errorf("createWorkingDirectory failed - %v", err)
	}
	defer removeWorkingDirectory(wd)

	// Set times when logging is enabled and write some text to the logger.
	locationUTC, err := time.LoadLocation("UTC")
	if err != nil {
		t.Errorf("error while loading UTC timezone - %v", err)
	}

	times := []time.Time{
		time.Date(2020, 2, 14, 0, 0, 5, 1, locationUTC),
		time.Date(2020, 2, 14, 12, 0, 0, 0, locationUTC),
		time.Date(2020, 2, 14, 23, 59, 54, 999999, locationUTC),
	}

	clock := clock.NewSteppingClock(&times)

	// Create an RTCM Writer using the supplied clock.  Behind the scenes that will
	// create a daily logger with a real clock, so that will create a log file with
	// a datestamp that we can't easily predict.  However, there should only be one
	// logfile so we can just look for it.
	writer := NewRTCMWriter(clock, loggingDirectory)

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
	fileInfoList, err := ioutil.ReadDir(logDirectoryPathName)
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

// TestNoWriteWhenLoggingDisabled checks that the Writer does not write
// to the log file when logging is disabled.
//
func TestNoWriteWhenLoggingDisabled(t *testing.T) {

	// NOTE:  this test uses the filestore.

	const loggingDirectory = "foo"

	wd, err := createWorkingDirectory()
	if err != nil {
		t.Errorf("createWorkingDirectory failed - %v", err)
	}
	defer removeWorkingDirectory(wd)

	// Set times close to midnight and write some text to the logger.  It should not
	// write anything to the log file.
	locationUTC, err := time.LoadLocation("UTC")
	if err != nil {
		t.Errorf("error while loading UTC timezone - %v", err)
	}

	// Create a stepping clock with times around midnight.
	times := []time.Time{
		time.Date(2020, 2, 14, 0, 0, 4, 0, locationUTC),
		time.Date(2020, 2, 14, 0, 0, 5, 0, locationUTC),
		time.Date(2020, 2, 14, 23, 59, 55, 1, locationUTC),
		time.Date(2020, 2, 14, 23, 59, 59, 0, locationUTC)}

	clock := clock.NewSteppingClock(&times)

	// Create an RTCM Writer using the supplied clock.  Behind the scenes that will
	// create a daily logger with a real clock, so that will create a log file with
	// a datestamp that we can't easily predict.  However, there should only be one
	// logfile so we can just look for it.
	writer := NewRTCMWriter(clock, loggingDirectory)

	buffer := []byte("hello")

	n, err := writer.Write(buffer)
	if err != nil {
		t.Errorf("Write failed - %v", err)
	}
	if n != len(buffer) {
		t.Errorf("Write returned %d - expected %d", n, len(buffer))
	}

	n, err = writer.Write(buffer)
	if err != nil {
		t.Errorf("Write failed - %v", err)
	}
	if n != len(buffer) {
		t.Errorf("Write returned %d - expected %d", n, len(buffer))
	}

	n, err = writer.Write(buffer)
	if err != nil {
		t.Errorf("Write failed - %v", err)
	}
	if n != len(buffer) {
		t.Errorf("Write returned %d - expected %d", n, len(buffer))
	}

	n, err = writer.Write(buffer)
	if err != nil {
		t.Errorf("Write failed - %v", err)
	}
	if n != len(buffer) {
		t.Errorf("Write returned %d - expected %d", n, len(buffer))
	}

	// Check that the log file is empty.
	logDirectoryPathName := wd + "/" + loggingDirectory
	fileInfoList, err := ioutil.ReadDir(logDirectoryPathName)

	if err != nil {
		t.Fatalf("Cannot scan directory %s - %v", logDirectoryPathName, err)
	}

	// fileInfoList should contain the fileinfo for one file.
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
	if fileInfo.Size() > 0 {
		t.Fatalf("log file %s contains %d bytes.  Should be empty",
			fileInfo.Name(), fileInfo.Size())
	}
}

// TestPushOldLogs checks that pushOldLogs moves the given file into the
// subdirectory "data.ready".  This test uses the filestore.
func TestPushOldLogs(t *testing.T) {

	workingDirectory, err := createWorkingDirectory()
	if err != nil {
		t.Fatalf("createWorkingDirectory failed - %v", err)
	}
	defer removeWorkingDirectory(workingDirectory)

	loggingDirectory := workingDirectory + "/logs"

	// Create logging directory - ignore if it already exists.
	err = os.MkdirAll(loggingDirectory, os.ModePerm)
	if err != nil {
		t.Fatalf("TestSaveLog: cannot create logs directory %s - %s",
			loggingDirectory, err.Error())
	}

	// Create files "foo", "bar" and today's logfile.
	pathname := loggingDirectory + "/" + "foo"
	file, err := os.Create("./logs/" + "foo")
	if err != nil {
		t.Fatalf("TestSaveLog: cannot create %s - %s", pathname, err.Error())
	}
	file.Close()

	pathname = loggingDirectory + "/" + "bar"
	file, err = os.Create(pathname)
	if err != nil {
		t.Fatalf("TestSaveLog: cannot create %s - %s", pathname, err.Error())
	}
	file.Close()

	locationUTC, err := time.LoadLocation("UTC")
	if err != nil {
		t.Fatalf("error while loading UTC timezone - %v", err)
	}
	now := time.Date(2020, 2, 14, 12, 13, 14, 15, locationUTC)

	todaysLogFileName := getTodaysLogFilename(now)
	pathname = loggingDirectory + "/" + todaysLogFileName
	file, err = os.Create(pathname)
	if err != nil {
		t.Fatalf("TestSaveLog: cannot create %s - %s", pathname, err.Error())

	}
	file.Close()

	// Push the non-matching files into the subdirectory
	pushOldLogs(loggingDirectory, now)

	// The current directory should contain the logfile and the subdirectory
	// for old log files, containing files "foo" and "bar".
	files, err := ioutil.ReadDir(loggingDirectory)
	if err != nil {
		t.Fatalf("cannot scan directory %s - %v", loggingDirectory, err)
	}
	if len(files) != 2 {
		t.Fatalf("expected the current directory to contain just two files, contains  %d",
			len(files))
	}

	if files[0].Name() != subDirectoryForOldLogs &&
		files[0].Name() != todaysLogFileName {

		t.Fatalf("expected file %s or %s, actually found %s",
			subDirectoryForOldLogs, todaysLogFileName, files[0].Name())
	}

	if files[1].Name() != subDirectoryForOldLogs &&
		files[1].Name() != todaysLogFileName {

		t.Fatalf("expected file %s or %s, actually found %s",
			subDirectoryForOldLogs, todaysLogFileName, files[1].Name())
	}

	// Check that the destination contains the file
	pathname = loggingDirectory + "/" + subDirectoryForOldLogs
	files, err = ioutil.ReadDir(pathname)
	if err != nil {
		t.Fatalf("cannot scan directory %s - %v", pathname, err)
	}
	if len(files) != 2 {
		t.Fatalf("expected the directory %s to contain exactly 2 files, found  %d",
			pathname, len(files))
	}

	if files[0].Name() != "foo" && files[0].Name() != "bar" {

		t.Fatalf("expected file foo or bar, actually found %s",
			files[0].Name())
	}

	if files[1].Name() != "foo" && files[1].Name() != "bar" {

		t.Fatalf("expected file foo or bar, actually found %s",
			files[1].Name())
	}
}

// makeUUID creates a UUID.  See https://yourbasic.org/golang/generate-uuid-guid/.
//
func makeUUID() string {
	// Produces something like "9e0825f2-e557-28df-93b7-a01c789f36a8".
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		log.Fatal(err)
	}
	uuid := fmt.Sprintf("%x-%x-%x-%x-%x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
	return uuid
}

// createWorkingDirectory create a working directory and makes it the current
// directory.
//
func createWorkingDirectory() (string, error) {
	directoryName := "/tmp/" + makeUUID()
	err := os.Mkdir(directoryName, os.ModePerm)
	if err != nil {
		return "", err
	}
	err = os.Chdir(directoryName)
	if err != nil {
		return "", err
	}
	return directoryName, nil
}

// removeWorkingDirectory removes the working directory and any files in it.
//
func removeWorkingDirectory(directoryName string) error {
	err := os.RemoveAll(directoryName)
	if err != nil {
		return err
	}
	return nil
}
