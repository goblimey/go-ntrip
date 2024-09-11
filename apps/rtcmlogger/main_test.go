package main

import (
	"bytes"
	"fmt"
	"log"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/goblimey/go-ntrip/apps/rtcmlogger/config"

	"github.com/goblimey/go-tools/testsupport"
)

func TestRecorder(t *testing.T) {

	s1 := "hello"
	s2 := " world"
	want := s1 + s2
	b := make([]byte, 0, len(want))
	output := bytes.NewBuffer(b)
	ch := make(chan []byte)
	var cfg config.Config
	go recorder(ch, output, &cfg)

	// Send text to the channel.  It should be written to the buffer.
	ch <- []byte(s1)
	ch <- []byte(s2)
	close(ch) // Stop the recorder.
	// Give the recorder time to write the text.
	time.Sleep(time.Second)

	got := output.String()
	if got != want {
		em := fmt.Sprintf("want %s got %s", want, got)
		t.Error(em)
	}
}

// This is an integration test for the rtcmlogger to check that it uses the log writer
// in the logger package correctly.  It must be run when logging is enabled, not close
// to midnight.

// TestWriteToLog writes to today's log, finds the log, reads the contents back and
// checks that the write worked.
func TestWriteToLog(t *testing.T) {

	// NOTE:  this test uses the filestore.

	const wantFileContents = "hello world\n"

	workingDirectory, err := testsupport.CreateWorkingDirectory()
	if err != nil {
		t.Errorf("createWorkingDirectory failed - %v", err)
	}
	defer testsupport.RemoveWorkingDirectory(workingDirectory)

	logDirectory := workingDirectory + "/log"

	// Create an RTCM log writer.  Behind the scenes that will create a log file
	// with a datestamp that we can't easily predict.  However, there should only
	// be one logfile so we can just look for it.
	cfg := config.Config{MessageLogDirectory: logDirectory}
	logWriter := newLogWriter(&cfg)

	buffer := []byte(wantFileContents)

	// ch := make(chan []byte)
	// cfg := config.Config{RecordMessages: true, RecorderChannel: ch}

	writeRTCMLog(&buffer, logWriter, &cfg)

	// Find the log file.
	fileInfoList, err := os.ReadDir(logDirectory)
	if err != nil {
		t.Fatalf("Cannot scan directory %s - %v", logDirectory, err)
	}

	// fileInfoList should show exactly one file.
	if len(fileInfoList) == 0 {
		t.Errorf("directory %s is empty.  Should contain one log file.",
			logDirectory)
	}
	if len(fileInfoList) > 1 {
		t.Errorf("directory %s contains %d files.  Should be just one.",
			logDirectory, len(fileInfoList))
		for _, fileInfo := range fileInfoList {
			t.Errorf("found file %s", fileInfo.Name())
		}
		t.Fatalf("terminating")
		return
	}

	fileInfo := fileInfoList[0]
	pathName := logDirectory + "/" + fileInfo.Name()

	file, err := os.Open(pathName)
	if err != nil {
		t.Fatalf("Cannot open log file %s - %v", fileInfo.Name(), err)
	}
	defer file.Close()

	b := make([]byte, 8096)
	length, err := file.Read(b)
	if err != nil {
		t.Fatalf("error reading logfile %s back - %v", fileInfo.Name(), err)
	}
	if length != len(wantFileContents) {
		t.Fatalf("logfile %s contains %d bytes - expected %d",
			fileInfo.Name(), length, len(buffer))
	}

	gotContents := string(b[:length])

	if gotContents != wantFileContents {
		t.Fatalf("logfile contains \"%s\" - expected \"%s\"",
			gotContents, wantFileContents)
	}
}

// makeUUID creates a UUID.  See https://yourbasic.org/golang/generate-uuid-guid/.
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
func createWorkingDirectory() (string, error) {
	directoryName := "/tmp/" + makeUUID()
	err := os.Mkdir(directoryName, os.ModePerm)
	if err != nil {
		log.Fatalf("createWorkingDirectory: " + directoryName + " already exists")
	}
	err = os.Chdir(directoryName)
	if err != nil {
		return "", err
	}
	return directoryName, nil
}

// removeWorkingDirectory removes the working directory and any files in it.
func removeWorkingDirectory(directoryName string) error {
	err := os.RemoveAll(directoryName)
	if err != nil {
		return err
	}
	return nil
}
