package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"testing"
)

// This is an integration test for the rtcmlogger to check that it uses the log writer
// in the logger package correctly.  It must be run when logging is enabled, not close
// to midnight.

// TestWriteToLog writes to today's log, finds the log, reads the contents back and
// checks that the write worked.
//
func TestWriteToLog(t *testing.T) {

	// NOTE:  this test uses the filestore.

	const expectedFileContents = "hello world\n"

	workingDirectory, err := createWorkingDirectory()
	if err != nil {
		t.Errorf("createWorkingDirectory failed - %v", err)
	}
	defer removeWorkingDirectory(workingDirectory)

	logDirectory := workingDirectory + "/log"

	// Create an RTCM log writer.  Behind the scenes that will create a log file
	// with a datestamp that we can't easily predict.  However, there should only
	// be one logfile so we can just look for it.
	logWriter := newLogWriter(logDirectory)

	buffer := []byte(expectedFileContents)

	writeBuffer(&buffer, logWriter)

	// Find the log file.
	fileInfoList, err := ioutil.ReadDir(logDirectory)
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
	if length != len(expectedFileContents) {
		t.Fatalf("logfile %s contains %d bytes - expected %d",
			fileInfo.Name(), length, len(buffer))
	}

	contents := string(b[:length])

	if contents != expectedFileContents {
		t.Fatalf("logfile contains \"%s\" - expected \"%s\"",
			contents, expectedFileContents)
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
		log.Fatalf("createWorkingDirectory: " + directoryName + " already exists")
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
