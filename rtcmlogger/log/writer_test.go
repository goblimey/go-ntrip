package log

import (
	"crypto/rand"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/goblimey/go-ntrip/rtcmlogger/clock"
)

// TestGetDurationToEndOfDay tests the getDurationToEndOfDay method.
//
func TestGetDurationToMidnight(t *testing.T) {
	locationUTC, _ := time.LoadLocation("UTC")
	start := time.Date(2020, time.February, 14, 22, 59, 0, 0, locationUTC)
	expectedDurationNanoseconds := time.Hour
	duration := getDurationToEndOfDay(start)
	if duration.Nanoseconds() != int64(expectedDurationNanoseconds) {
		t.Errorf("expected duration to be \"%d\" actually \"%d\"", expectedDurationNanoseconds, duration.Nanoseconds())
	}

	start = time.Date(2020, time.February, 14, 0, 29, 3, 4, locationUTC)
	expectedDurationNanoseconds = 23*time.Hour + 30*time.Minute - (3*time.Second + 4*time.Nanosecond)
	duration = getDurationToEndOfDay(start)
	if duration.Nanoseconds() != int64(expectedDurationNanoseconds) {
		t.Errorf("expected duration to be \"%d\" actually \"%d\"", expectedDurationNanoseconds, duration.Nanoseconds())
	}

	// Test using a time that's not in UTC.  (Paris is one hour ahead in winter.)
	locationParis, _ := time.LoadLocation("Europe/Paris")
	start = time.Date(2020, time.February, 14, 23, 59, 0, 0, locationParis)
	expectedDurationNanoseconds = time.Hour
	duration = getDurationToEndOfDay(start)
	if duration.Nanoseconds() != int64(expectedDurationNanoseconds) {
		t.Errorf("expected duration to be \"%d\" actually \"%d\"", expectedDurationNanoseconds, duration.Nanoseconds())
	}
}

// TestFilenameWhenLogging checks that getFilename returns a filename containing today's
// timestamp when logging is enabled.
//
func TestFilenameWhenLogging(t *testing.T) {
	yymmdd := "200214"
	const expectedFilename = "data.200214.rtcm3"
	filename := getFilename(yymmdd)

	if filename != expectedFilename {
		t.Errorf("expected day to be \"%s\" actually \"%s\"", expectedFilename, filename)
	}
}

// TestFilenameWhenNotLogging checks that getFilename returns /dev/null when
//logging is disabled.
//
func TestFilenameWhenNotLogging(t *testing.T) {
	yymmdd := ""
	const expectedFilename = "/dev/null"

	filename := getFilename(yymmdd)

	if filename != expectedFilename {
		t.Errorf("expected day to be \"%s\" actually \"%s\"", expectedFilename, filename)
	}
}

// TestLoggingDisabled tests that logging is disabled just before and just after midnight UTC.
//
func TestLoggingDisabled(t *testing.T) {
	locationUTC, err := time.LoadLocation("UTC")
	if err != nil {
		t.Errorf("error while loading UTC timezone - %v", err)
	}

	// Test some middle cases.
	stoppedClock := clock.NewStoppedClock(2020, time.February, 14, 0, 0, 30, 0, locationUTC)
	writer := newRtcmWriterWithClock(stoppedClock)

	allowed := writer.loggingAllowed()

	if allowed {
		t.Errorf("expected allowed to be false actually %v", allowed)
	}

	stoppedClock = clock.NewStoppedClock(2020, time.February, 14, 23, 59, 30, 0, locationUTC)
	writer = newRtcmWriterWithClock(stoppedClock)

	allowed = writer.loggingAllowed()

	if allowed {
		t.Errorf("expected allowed to be false actually %v", allowed)
	}

	// Test the edge cases.

	// 23:59:00 - just after disabling.
	stoppedClock = clock.NewStoppedClock(2020, time.February, 14, 23, 59, 0, 0, locationUTC)
	writer = newRtcmWriterWithClock(stoppedClock)

	allowed = writer.loggingAllowed()

	if allowed {
		t.Errorf("expected allowed to be false actually %v", allowed)
	}

	// 00:00:59.999999999 - just before enabling.
	stoppedClock = clock.NewStoppedClock(2020, time.February, 14, 0, 0, 59, 999999999, locationUTC)
	writer = newRtcmWriterWithClock(stoppedClock)

	allowed = writer.loggingAllowed()

	if allowed {
		t.Errorf("expected allowed to be false actually %v", allowed)
	}
}

// TestLoggingEnabled tests that logging is enabled apart from just before and just
// after midnight UTC.
//
func TestLoggingEnabled(t *testing.T) {
	locationUTC, err := time.LoadLocation("UTC")
	if err != nil {
		t.Errorf("error while loading UTC timezone - %v", err)
	}

	// Test a middle case.
	const exectedDatestamp = "20200214"
	stoppedClock := clock.NewStoppedClock(2020, time.February, 14, 12, 0, 0, 0, locationUTC)
	writer := newRtcmWriterWithClock(stoppedClock)
	day := writer.todayYYYYMMDD()
	if day != exectedDatestamp {
		t.Errorf("expected day to be %s actually %s", exectedDatestamp, day)
	}

	// Test the edge cases.

	// 00:01:00 - just after enabling.
	stoppedClock = clock.NewStoppedClock(2020, time.February, 14, 0, 1, 0, 0, locationUTC)
	writer = newRtcmWriterWithClock(stoppedClock)
	day = writer.todayYYYYMMDD()
	if day != exectedDatestamp {
		t.Errorf("expected day to be %s actually %s", exectedDatestamp, day)
	}
	// 00:58:59.999999999 - just before disabling.
	stoppedClock = clock.NewStoppedClock(2020, time.February, 14, 0, 58, 59, 999999999, locationUTC)
	writer = newRtcmWriterWithClock(stoppedClock)

	day = writer.todayYYYYMMDD()

	if day != exectedDatestamp {
		t.Errorf("expected day to be %s actually %s", exectedDatestamp, day)
	}
}

// TestOtherTimezone tests that logging is disabled correctly when the timezone
// is other than UTC.
//
func TestOtherTimezoneLoggingDisabled(t *testing.T) {
	locationParis, err := time.LoadLocation("Europe/Paris")
	if err != nil {
		t.Errorf("error while loading UTC timezone - %v", err)
	}

	// Test some middle cases.
	stoppedClock := clock.NewStoppedClock(2020, time.February, 14, 1, 0, 30, 0, locationParis)
	writer := newRtcmWriterWithClock(stoppedClock)

	allowed := writer.loggingAllowed()

	if allowed {
		t.Errorf("expected loggin to be not llowed actually allowed")
	}

	stoppedClock = clock.NewStoppedClock(2020, time.February, 14, 0, 59, 30, 0, locationParis)
	writer = newRtcmWriterWithClock(stoppedClock)

	allowed = writer.loggingAllowed()

	if allowed {
		t.Errorf("expected loggin to be not llowed actually allowed")
	}

	// Test the edge cases.

	// 14th 00:59:00 CET is 13th 23:59:00 - just after disabling.
	stoppedClock = clock.NewStoppedClock(2020, time.February, 14, 0, 59, 0, 0, locationParis)
	writer = newRtcmWriterWithClock(stoppedClock)

	allowed = writer.loggingAllowed()

	if allowed {
		t.Errorf("expected loggin to be not llowed actually allowed")
	}

	// 14th 01:00:59.99999999 CET is 14th 00:00:59 UTC - just before enabling.
	stoppedClock = clock.NewStoppedClock(2020, time.February, 14, 1, 0, 59, 999999999, locationParis)
	writer = newRtcmWriterWithClock(stoppedClock)
	allowed = writer.loggingAllowed()

	if allowed {
		t.Errorf("expected loggin to be not llowed actually allowed")
	}
}

// TestOtherTimezone tests that logging is enabled correctly when the timezone is
// other than UTC.
//
func TestOtherTimezoneLoggingEnabled(t *testing.T) {

	locationParis, _ := time.LoadLocation("Europe/Paris")

	// Test a middle case.
	const datestampToday = "20200214"
	const datestampYesterday = "20200213"
	stoppedClock := clock.NewStoppedClock(2020, time.February, 14, 12, 0, 0, 0, locationParis)
	writer := newRtcmWriterWithClock(stoppedClock)

	day := writer.todayYYYYMMDD()

	if day != datestampToday {
		t.Errorf("expected day to be %s actually %s", datestampToday, day)
	}

	// Test the case when it's still yesterday in UTC and logging is enabled.
	stoppedClock = clock.NewStoppedClock(2020, time.February, 14, 0, 0, 59, 0, locationParis)
	writer = newRtcmWriterWithClock(stoppedClock)

	day = writer.todayYYYYMMDD()

	if day != datestampYesterday {
		t.Errorf("expected day to be %s actually %s", datestampYesterday, day)
	}

	// Test the edge cases.

	// 14th 00:58:59.999999999 CET is 13th 23:58:59.999999999 - just before disabling.
	stoppedClock = clock.NewStoppedClock(2020, time.February, 14, 0, 58, 59, 999999999, locationParis)
	writer = newRtcmWriterWithClock(stoppedClock)

	day = writer.todayYYYYMMDD()

	if day != datestampYesterday {
		t.Errorf("expected day to be %s actually %s", datestampYesterday, day)
	}
	// 14th 01:01:00 CET is 14th 00:01:00 UTC - just after enabling.
	stoppedClock = clock.NewStoppedClock(2020, time.February, 14, 1, 1, 0, 0, locationParis)
	writer = newRtcmWriterWithClock(stoppedClock)

	day = writer.todayYYYYMMDD()
	if day != datestampToday {
		t.Errorf("expected day to be %s actually %s", datestampToday, day)
	}
}

// TestFileNotCreatedWhenLoggingDisabled checks that the Writer does not create a file
// when logging is disabled.
//
func TestFileNotCreatedWhenLoggingDisabled(t *testing.T) {

	// NOTE:  this test uses the filestore.

	directoryName, err := createWorkingDirectory()
	if err != nil {
		t.Errorf("createWorkingDirectory failed - %v", err)
	}
	defer removeWorkingDirectory(directoryName)

	// Set the time close to midnight and write some text to the logger.  It should not
	// create a log file or write anything.
	locationUTC, err := time.LoadLocation("UTC")
	if err != nil {
		t.Errorf("error while loading UTC timezone - %v", err)
	}
	stoppedClock := clock.NewStoppedClock(2020, time.February, 14, 0, 0, 30, 0, locationUTC)
	writer := newRtcmWriterWithClock(stoppedClock)

	buffer := []byte("hello")
	n, err := writer.Write(buffer)

	if err != nil {
		t.Errorf("Write failed - %v", err)
	}

	if n != len(buffer) {
		t.Errorf("Write returned %d - expected %d", n, len(buffer))
	}

	// Check that no log file was created.
	var files []string
	err = filepath.Walk(directoryName, func(path string, info os.FileInfo, err error) error {
		files = append(files, path)
		return nil
	})
	if err != nil {
		t.Errorf("filepath.walk failed - %v", err)
	}

	// files contains the name of the directory and the names of any files in it.
	if len(files) > 1 {
		t.Errorf("directory %s contains %d files including %s.  Should be empty.", directoryName, len(files)-1, files[1])
	}
}

// TestFileCreatedWhenLoggingEnabled checks that GetWriter creates a file of the
// right name when logging is enabled.
//
func TestFileCreatedWhenLoggingEnabled(t *testing.T) {

	// NOTE: this test uses the filestore.  It creates a directory in /tmp,
	// then a plain file in there called data.yymmdd.rtcm3 where yymmdd is today's
	// date.  At the end it attempts to remove everything it created.

	directoryName, err := createWorkingDirectory()
	if err != nil {
		t.Errorf("createWorkingDirectory failed - %v", err)
	}
	defer removeWorkingDirectory(directoryName)

	// Set the time to 00:01:00 and call GetWriter().  It should create
	// a log file.
	locationUTC, err := time.LoadLocation("UTC")
	if err != nil {
		t.Errorf("error while loading UTC timezone - %v", err)
	}

	stoppedClock := clock.NewStoppedClock(2020, time.February, 14, 0, 1, 0, 0, locationUTC)
	expectedFilename := directoryName + "/" + "data.20200214.rtcm3"
	writer := newRtcmWriterWithClock(stoppedClock)
	buffer := []byte("hello")

	n, err := writer.Write(buffer)

	if err != nil {
		t.Errorf("Write failed - %v", err)
	}

	if n != len(buffer) {
		t.Errorf("Write returned %d - expected %d", n, len(buffer))
	}

	// Check that one log file was created and contains the expected contents.
	var files []string
	err = filepath.Walk(directoryName, func(path string, info os.FileInfo, err error) error {
		files = append(files, path)
		return nil
	})
	if err != nil {
		t.Errorf("filepath.walk failed - %v", err)
	}

	// files contains the name of the directory and the names of any files in it.  The log file
	// will be called something like "/tmp/fb0a4b2f-19fb-1d9d-dc33-8ec1aa5addb3/data.200214.rtcm3"
	if len(files) != 2 {
		t.Errorf("directory %s contains %d files.  Should contain exactly one.", directoryName, len(files)-1)
	}

	if files[1] != expectedFilename {
		t.Errorf("directory %s contains file \"%s\", expected \"%s\".", directoryName, files[1], expectedFilename)
	}

	// Check the contents.
	inputFile, err := os.OpenFile(expectedFilename, os.O_RDONLY, 0644)
	defer inputFile.Close()
	b := make([]byte, 8096)
	length, err := inputFile.Read(b)
	if err != nil {
		t.Errorf("error reading logfile back - %v", err)
	}
	if length != len(buffer) {
		t.Errorf("logfile contains %d bytes - expected %d", length, len(buffer))
	}
}

// TestRollover checks that the Writer creates a new file each day.
//
func TestRollover(t *testing.T) {

	// NOTE:  this test uses the filestore.

	directoryName, err := createWorkingDirectory()
	if err != nil {
		t.Errorf("createWorkingDirectory failed - %v", err)
	}
	defer removeWorkingDirectory(directoryName)

	locationUTC, err := time.LoadLocation("UTC")
	if err != nil {
		t.Errorf("error while loading UTC timezone - %v", err)
	}

	// Set the time to when logging is enabled and write some text to the logger.
	// That should create a file for today.
	stoppedClock := clock.NewStoppedClock(2020, time.February, 14, 0, 1, 30, 0, locationUTC)
	writer := newRtcmWriterWithClock(stoppedClock)
	buffer1 := []byte("goodbye")
	n, err := writer.Write(buffer1)
	if err != nil {
		t.Errorf("Write failed - %v", err)
	}

	if n != len(buffer1) {
		t.Errorf("Write returns %d - expected %d", n, len(buffer1))
	}

	// Set the time close to midnight and write some text.  This should do nothing.
	stoppedClock = clock.NewStoppedClock(2020, time.February, 14, 23, 59, 30, 0, locationUTC)
	writer = newRtcmWriterWithClock(stoppedClock)
	buffer2 := []byte("hello")

	n, err = writer.Write(buffer2)
	if err != nil {
		t.Errorf("Write failed - %v", err)
	}

	if n != len(buffer2) {
		t.Errorf("Write returns %d - expected %d", n, len(buffer2))
	}

	// Run endOfDay to roll over the log.  That creates a directory "data.ready" and moves
	// data.200214.rtcm3 into it.

	// writer.endOfDay()

	// time.Sleep(5 * time.Second)

	// Set the time to a point during the next day when logging is enabled.  That
	// should create a new logfile for that day.
	stoppedClock = clock.NewStoppedClock(2020, time.February, 15, 0, 1, 30, 0, locationUTC)
	writer = newRtcmWriterWithClock(stoppedClock)
	buffer3 := []byte("cruel world")

	n, err = writer.Write(buffer3)
	if err != nil {
		t.Errorf("Write failed - %v", err)
	}

	if n != len(buffer3) {
		t.Errorf("Write returns %d - expected %d", n, len(buffer3))
	}

	// The current directory should contain a file "data.200215.rtcm3 and a directory
	// "data.ready" containg a file "data.200214.rtcm3".
	var files []string
	err = filepath.Walk(directoryName, func(path string, info os.FileInfo, err error) error {
		files = append(files, path)
		return nil
	})
	if err != nil {
		t.Errorf("filepath.walk failed - %v", err)
	}

	// files contains the name of the directory and the names of any files in it.
	if len(files) != 4 {
		for _, file := range files {
			fmt.Println("file " + file)
		}
		t.Errorf("directory %s contains %d files.  Should be three.", directoryName, len(files)-1)
	}

	// Check the contents.
	expectedFilename1 := directoryName + "/data.ready/data.200214.rtcm3"
	inputFile, err := os.OpenFile(expectedFilename1, os.O_RDONLY, 0644)
	defer inputFile.Close()
	b := make([]byte, 8096)
	length, err := inputFile.Read(b)
	if err != nil {
		t.Errorf("error reading logfile back - %v", err)
	}
	if length != len(buffer1) {
		t.Errorf("logfile contains %d bytes - expected %d", length, len(buffer1))
	}

	expectedFilename2 := directoryName + "/data.200215.rtcm3"
	inputFile, err = os.OpenFile(expectedFilename2, os.O_RDONLY, 0644)
	defer inputFile.Close()
	length, err = inputFile.Read(b)
	if err != nil {
		t.Errorf("error reading logfile back - %v", err)
	}
	if length != len(buffer3) {
		t.Errorf("logfile contains %d bytes - expected %d", length, len(buffer3))
	}
}

// TestAppendOnRestart checks that if the program creates a log file for the day,
// then crashes and restarts, the Writer appends to the existing file rather than
// overwriting it.
//
func TestAppendOnRestart(t *testing.T) {

	// NOTE:  this test uses the filestore.

	directoryName, err := createWorkingDirectory()
	if err != nil {
		t.Errorf("createWorkingDirectory failed - %v", err)
	}
	defer removeWorkingDirectory(directoryName)

	locationUTC, err := time.LoadLocation("UTC")
	if err != nil {
		t.Errorf("error while loading UTC timezone - %v", err)
	}

	// Set the time to when logging is enabled and write some text to the logger.
	// That should create a file for today.
	stoppedClock := clock.NewStoppedClock(2020, time.February, 14, 0, 1, 30, 0, locationUTC)
	writer1 := newRtcmWriterWithClock(stoppedClock)
	outputBuffer := []byte("hello ")
	n, err := writer1.Write(outputBuffer)
	if err != nil {
		t.Errorf("Write failed - %v", err)
	}
	if n != len(outputBuffer) {
		t.Errorf("Write returns %d - expected %d", n, len(outputBuffer))
	}

	const expectedFilename = "data.200214.rtcm3"
	const expectedFirstContents = "hello "
	inputFile, err := os.OpenFile(expectedFilename, os.O_RDONLY, 0644)
	defer inputFile.Close()
	inputBuffer := make([]byte, 8096)
	n, err = inputFile.Read(inputBuffer)
	if err != nil {
		t.Errorf("error reading logfile back - %v", err)
	}
	if string(inputBuffer[:n]) != expectedFirstContents {
		t.Errorf("logfile contains \"%s\" - expected \"%s\"", string(inputBuffer[:n]), expectedFirstContents)
	}

	// Create a new writer.  On the first call it will behave as on system startup
	// and append to the existing daily log.
	stoppedClock = clock.NewStoppedClock(2020, time.February, 14, 0, 2, 30, 0, locationUTC)
	writer2 := newRtcmWriterWithClock(stoppedClock)
	outputBuffer = []byte("world")
	n, err = writer2.Write(outputBuffer)
	if err != nil {
		t.Errorf("Write failed - %v", err)
	}
	if n != len(outputBuffer) {
		t.Errorf("Write returns %d - expected %d", n, len(outputBuffer))
	}

	// Check that only one log file was created and check the contents.
	var files []string
	err = filepath.Walk(directoryName, func(path string, info os.FileInfo, err error) error {
		files = append(files, path)
		return nil
	})
	if err != nil {
		t.Errorf("filepath.walk failed - %v", err)
	}

	// files contains the name of the directory and the names of any files in it.
	if len(files) != 2 {
		t.Errorf("directory %s contains %d files.  Should be 1.", directoryName, len(files)-1)
	}

	expectedPathname := directoryName + "/data.200214.rtcm3"
	if files[1] != expectedPathname {
		t.Errorf("directory %s contains file %s - expected %s.", directoryName, files[1], expectedPathname)
	}

	// Check the contents.  It should be the result of the two Write calls.

	const expectedFinalContents = "hello world"
	inputFile, err = os.OpenFile(expectedFilename, os.O_RDONLY, 0644)
	defer inputFile.Close()
	inputBuffer = make([]byte, 8096)
	n, err = inputFile.Read(inputBuffer)
	if err != nil {
		t.Errorf("error reading logfile back - %v", err)
	}
	if string(inputBuffer[:n]) != expectedFinalContents {
		t.Errorf("logfile contains \"%s\" - expected \"%s\"", string(inputBuffer[:n]), expectedFinalContents)
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
	err := os.Mkdir(directoryName, 0777)
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
