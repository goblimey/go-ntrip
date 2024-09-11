package config

import (
	"os"
	"testing"

	"github.com/goblimey/go-tools/testsupport"
)

func TestParseConfig(t *testing.T) {

	json := []byte(`
		{
			"log_events": true,
			"message_log_directory": "l",
			"directory_for_old_message_logs": "l/m",
			"event_log_directory": "events"
		}
	`)

	config, err := parseConfigFromBytes(json)

	if err != nil {
		t.Error(err)
		return
	}

	if !config.LogEvents {
		t.Error("want LogEvents true")
	}

	if config.MessageLogDirectory != "l" {
		t.Errorf("want l, got %s", config.MessageLogDirectory)
	}

	if config.DirectoryForOldMessageLogs != "l/m" {
		t.Errorf("want l/m, got %s", config.DirectoryForOldMessageLogs)
	}

	if config.EventLogDirectory != "events" {
		t.Errorf("want events, got %s", config.EventLogDirectory)
	}
}

func TestParseConfigWithError(t *testing.T) {

	jsonData := []byte("{junk}")

	_, err := parseConfigFromBytes(jsonData)

	if err == nil {
		t.Error("expected an error")
	}
}

// TestGetConfig checks that getConfig correctly reads a config file.
func TestGetConfig(t *testing.T) {

	// Create a temporary directory with a file containing the config.
	testDirName, createDirectoryError := testsupport.CreateWorkingDirectory()

	if createDirectoryError != nil {
		t.Error(createDirectoryError)
		return
	}

	// Ensure that the test files are tidied away at the end.
	defer testsupport.RemoveWorkingDirectory(testDirName)

	configFile := "config.json"

	writer, fileCreateError := os.Create(configFile)
	if fileCreateError != nil {
		t.Error(fileCreateError)
		return
	}

	json := `
		{
			"log_events": true,
			"message_log_directory": "log",
			"directory_for_old_message_logs": "log/log",
			"event_log_directory": "eventlog"
		}
	`
	_, writeError := writer.Write([]byte(json))
	if writeError != nil {
		t.Error(writeError)
		return
	}

	config, errConfig := GetConfig("./config.json")
	if errConfig != nil {
		t.Error(errConfig)
		return
	}

	if !config.LogEvents {
		t.Error("want LogEvents true")
	}

	if config.MessageLogDirectory != "log" {
		t.Errorf("want log, got %s", config.MessageLogDirectory)
	}

	if config.DirectoryForOldMessageLogs != "log/log" {
		t.Errorf("want log/log, got %s", config.DirectoryForOldMessageLogs)
	}

	if config.EventLogDirectory != "eventlog" {
		t.Errorf("want eventlog, got %s", config.EventLogDirectory)
	}
}
