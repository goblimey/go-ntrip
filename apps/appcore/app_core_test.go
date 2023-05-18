package appcore

import (
	"bufio"
	"os"
	"testing"
	"time"

	"github.com/goblimey/go-ntrip/jsonconfig"
	rtcm "github.com/goblimey/go-ntrip/rtcm/handler"
	"github.com/goblimey/go-ntrip/rtcm/testdata"
	"github.com/goblimey/go-ntrip/rtcm/utils"
	"github.com/goblimey/go-tools/testsupport"
)

func TestProcessMessages(t *testing.T) {

	// Create a temporary directory with a file containing the test data.
	testDirName, createDirectoryError := CreateWorkingDirectory()

	if createDirectoryError != nil {
		t.Error(createDirectoryError)
	}

	// Ensure that th test files ar tidied away at the end.
	defer testsupport.RemoveWorkingDirectory(testDirName)

	testFile := testDirName + "/t"

	writer, fileCreateError := os.Create(testFile)
	if fileCreateError != nil {
		t.Error(fileCreateError)
	}

	_, writeError := writer.Write(testdata.MessageBatchWithJunk)
	if writeError != nil {
		t.Error(writeError)
	}

	// wantMessageTypesRTCM is the list of message types of the RTCM3
	// messages in the test data.
	//
	wantMessageTypesRTCM := []int{1077, 1087, 1097, 1127}
	wantMessageTypesAll := []int{1077, -1, 1087, -1, 1097, 1127, -1}

	r, fileOpenError := os.Open(testFile)
	if fileOpenError != nil {
		t.Error(fileOpenError)
	}

	reader := bufio.NewReader(r)

	// Create a config giving the single file name and setting the
	// retries and timeouts to 0, so that the handler stops when
	// it reaches end of file.
	fileNames := []string{testFile}
	config := jsonconfig.Config{Filenames: fileNames}

	// Create the channels with the goroutines receiving from them.
	rtcmMessages := make([]rtcm.Message, 0)
	rtcmChan := make(chan rtcm.Message, 20)
	go eatRTCM(rtcmChan, &rtcmMessages)

	allMessages := make([]rtcm.Message, 0)
	allChan := make(chan rtcm.Message, 20)
	go eatAll(allChan, &allMessages)

	channels := make([]chan rtcm.Message, 0)
	channels = append(channels, rtcmChan)
	channels = append(channels, nil)
	channels = append(channels, allChan)
	channels = append(channels, nil)

	appCore := New(&config, channels)
	// Run the handler.  It should read the messages from the reader and stop
	appCore.HandleMessagesUntilEOF(time.Now(), reader)

	// Pause to allow the channels to drain.
	time.Sleep(time.Second)

	// Check that the result slices contain the right number of messages in
	// the right order.

	if len(wantMessageTypesRTCM) != len(rtcmMessages) {
		t.Errorf("want %d got %d", len(wantMessageTypesRTCM), len(rtcmMessages))
		return
	}

	for i := range wantMessageTypesRTCM {
		if wantMessageTypesRTCM[i] != rtcmMessages[i].MessageType {
			t.Errorf("%d want %d got %d",
				i, wantMessageTypesRTCM[i], rtcmMessages[i].MessageType)
			return
		}
	}

	if len(wantMessageTypesAll) != len(allMessages) {
		t.Errorf("want %d got %d", len(wantMessageTypesAll), len(allMessages))
	}

	for i := range wantMessageTypesAll {
		if wantMessageTypesAll[i] != allMessages[i].MessageType {
			t.Errorf("%d want %d got %d",
				i, wantMessageTypesAll[i], allMessages[i].MessageType)
		}
	}
}

// eatRTCM receives RTCM messages from the channel and returns them in a slice.
func eatRTCM(messages chan rtcm.Message, buffer *[]rtcm.Message) *[]rtcm.Message {
	// We are updating the slice data of the buffer so we need to
	// faff about with pointers and dereferencing.
	for {
		message, ok := <-messages
		if !ok {
			return buffer
		}
		if message.MessageType == utils.NonRTCMMessage {
			// Ignore non-RTCM messages.
			continue
		}
		*buffer = append(*buffer, message)
	}
}

// eatAll receives all messages from the channel and returns them in a slice.
func eatAll(messages chan rtcm.Message, buffer *[]rtcm.Message) *[]rtcm.Message {
	// We are updating the slice data of the buffer so we need to
	// faff about with pointers and dereferencing.
	for {
		message, ok := <-messages
		if !ok {
			return buffer
		}

		*buffer = append(*buffer, message)
	}
}

// CreateWorkingDirectory creates a directory with a unique name.
// It's good practice to remove the directory when finished:
//
//	    workingDirectory, err := testsupport.CreateWorkingDirectory()
//		   if err != nil {
//			    t.Errorf("createWorkingDirectory failed - %v", err)
//		    }
//	  	defer testsupport.RemoveWorkingDirectory(workingDirectory)
//
// The directory is created in /tmp.  Its name is derived from the
// current date and time.
func CreateWorkingDirectory() (string, error) {
	const limit = 10
	var createError error
	for i := 1; i <= limit; i++ {
		// Get the time and create the name with nanosecond
		// precision.
		directoryName := "/tmp/" + getNameFromTime(time.Now())
		// It's possible to something else in another core might do
		// this at exactly the same time.  In that case the mkdir
		// will fail and we try again.  After a few times we have
		// to give up, but that's very very unlikely.
		createError = os.Mkdir(directoryName, os.ModePerm)

		if createError == nil {
			return directoryName, nil
		}
	}

	// If we get to here, we failed too many times and we have to give up.
	return "", createError
}

// getNameFromTime creates a filename using the given time.
func getNameFromTime(date time.Time) string {

	return date.Format(time.RFC3339Nano)

}
