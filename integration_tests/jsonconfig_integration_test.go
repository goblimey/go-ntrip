package integrationtests

import (
	"io"
	"log"
	"os"
	"testing"
	"time"

	"github.com/goblimey/go-ntrip/jsonconfig"
	"github.com/goblimey/go-tools/switchwriter"
	"github.com/goblimey/go-tools/testsupport"
)

var jsonConfigLogger *log.Logger

func init() {
	writer := switchwriter.New()
	jsonConfigLogger = log.New(writer, "rtcm_test", 0)
}

// TestGetJSONConfigFromFile tests that getJSONConfigFromFile
// reads a config file correctly.
func TestGetJSONConfigFromFile(t *testing.T) {

	workingDirectory, err := testsupport.CreateWorkingDirectory()
	if err != nil {
		t.Errorf("createWorkingDirectory failed - %v", err)
	}
	defer testsupport.RemoveWorkingDirectory(workingDirectory)

	// Create the JSON control file.
	fileContents := `{
		"input": ["a", "b"],
		"record_messages": true,
		"display_messages": true,
		"message_log_directory": "foo",
		"caster_host_name": "caster.example.com",
		"caster_port": 2101,
		"caster_user_name": "user",
		"caster_password": "password",
		"read_timeout_milliseconds": 1,
		"sleep_time_after_failed_open_milliseconds": 2,
		"wait_time_on_EOF_millis": 3,
		"timeout_on_EOF_milliseconds": 34
	}`
	controlFileName := "config.json"

	controlFile, err := os.Create(controlFileName)
	if err != nil {
		t.Fatal(err)
	}

	_, err = controlFile.Write([]byte(fileContents))
	if err != nil {
		t.Fatal(err)
	}

	config, err := jsonconfig.GetJSONConfigFromFile(controlFileName, jsonConfigLogger)
	if err != nil {
		t.Fatal(err)
	}

	if config == nil {
		t.Fatal("parsing json failed - nil")
	}

	numFiles := len(config.Filenames)
	if numFiles != 2 {
		t.Fatalf("parsing json, expected 2 files, got %d", numFiles)
	}

	if config.Filenames[0] != "a" {
		t.Errorf("parsing json, expected file 0 to be a, got %s",
			config.Filenames[0])
	}

	if config.Filenames[1] != "b" {
		t.Errorf("parsing json, expected file 1 to be b, got %s",
			config.Filenames[1])

	}
	if config.CasterUserName != "user" {
		t.Errorf("parsing json, expected caster username to be user, got %s",
			config.CasterUserName)
	}
	if config.CasterPassword != "password" {
		t.Errorf("parsing json, expected caster password to be password, got %s",
			config.CasterPassword)
	}

	if !config.RecordMessages {
		t.Error("parsing json, expected record_messages to be true")
	}

	if !config.DisplayMessages {
		t.Error("parsing json, expected display_messages to be true")
	}

	if config.MessageLogDirectory != "foo" {
		t.Errorf("parsing json, expected message_log_directory to be \"foo\" got \"%s\"",
			config.MessageLogDirectory)
	}
}

// TestWaitAndConnectToInput tests that waitAndConnectToInput returns a
// reader connected to the correct file when the file does not exist
// initially.  Warning:  this test pauses for a significant time.)
func TestWaitAndConnectToInput(t *testing.T) {

	workingDirectory, err := testsupport.CreateWorkingDirectory()
	if err != nil {
		t.Errorf("createWorkingDirectory failed - %v", err)
	}
	defer testsupport.RemoveWorkingDirectory(workingDirectory)

	// The filename list in the config contains "a", "b" and "c"
	filenames := make([]string, 0)
	filenames = append(filenames, "a")
	filenames = append(filenames, "b")
	filenames = append(filenames, "c")
	config := jsonconfig.Config{Filenames: filenames, ReadTimeoutMilliSeconds: 1,
		SleepTimeAfterFailedOpenMilliSeconds: 1}

	// Wait for a short time and then create file "b" with some contents.
	const expectedContents = "Hello world"
	creator := func() {
		time.Sleep(2 * time.Second)
		// To avoid a race while writing, create "t", write to it and
		// then rename it.  The test won't notice it until it's renamed.
		writer, err := os.Create("t")
		if err != nil {
			log.Fatal(err)
		}
		writer.Write([]byte(expectedContents))
		err = os.Rename("t", "b")
		if err != nil {
			log.Fatal(err)
		}
	}

	go creator()

	// File b doesn't exist at first when this is called.  It should spin
	// and, once file "b" appears, open it for reading.
	reader := config.WaitAndConnectToInput()
	if reader == nil {
		log.Fatalf("findInputDevice returns nil, should open \"b\" for reading")
	}
	contents, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}

	// The contents read should match the expectedContents that was written.
	if expectedContents != string(contents) {
		t.Fatalf("expected %s, got %s", expectedContents, string(contents))
	}
}
