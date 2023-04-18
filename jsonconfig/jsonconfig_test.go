package jsonconfig

import (
	"log"
	"strings"
	"testing"

	"github.com/goblimey/go-tools/switchwriter"
)

// TestJSONControl tests that the correct data is produced when the
// text from a JSON control file is unmarshalled.
//
func TestGetJSONControl(t *testing.T) {
	reader := strings.NewReader(`{
		"input": ["a", "b"],
		"stop_on_eof": true,
		"record_messages": true,
		"message_log_directory": "someDirectory",
		"display_messages": true,
		"caster_host_name": "caster.example.com",
		"caster_port": 2101,
		"caster_user_name": "user",
		"caster_password": "password",
		"timeout": 1,
		"sleep_time": 2,
		"wait_time_on_EOF_millis": 3,
		"timeout_on_EOF_seconds": 4
	}`)

	writer := switchwriter.New()
	logger := log.New(writer, "jsonconfig_test", 0)

	config, err := getJSONConfig(reader, logger)
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

	if config.CasterHostName != "caster.example.com" {
		t.Errorf("parsing json, expected caster host name to be caster.example.com, got %s",
			config.CasterHostName)
	}
	if config.CasterPort != 2101 {
		t.Errorf("parsing json, expected caster port to be 2101, got %d",
			config.CasterPort)
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

	if config.MessageLogDirectory != "someDirectory" {
		t.Errorf("parsing json, expected display_message_directory to be \"someDirectory\", got \"%s\"",
			config.MessageLogDirectory)
	}

	if !config.DisplayMessages {
		t.Error("parsing json, expected display_message to be true")
	}

	if config.LostInputConnectionTimeout != 1 {
		t.Errorf("parsing json, expected timeout to be 1, got %d",
			config.LostInputConnectionTimeout)
	}

	if config.LostInputConnectionSleepTime != 2 {
		t.Errorf("parsing json, expected sleep time to be 2, got %d",
			config.LostInputConnectionSleepTime)
	}

	if config.WaitTimeOnEOF != 3 {
		t.Errorf("parsing json, expected wait time to be 3, got %d",
			config.WaitTimeOnEOF)
	}

	if config.TimeoutOnEOF != 4 {
		t.Errorf("parsing json, expected wait time to be 4, got %d",
			config.TimeoutOnEOF)
	}
}
