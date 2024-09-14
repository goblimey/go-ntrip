package config

import (
	"os"
	"testing"

	"github.com/goblimey/go-tools/testsupport"
)

func TestParseConfig(t *testing.T) {

	json := []byte(`
		{
			"display_messages": true,
			"record_messages": true,
			"log_directory": "l"
		}
	`)

	config, err := parseConfigFromBytes(json)

	if err != nil {
		t.Error(err)
		return
	}

	if !config.DisplayMessages {
		t.Error("want DisplayMessages true")
	}

	if !config.RecordMessages {
		t.Error("want RecordMessages true")
	}

	if config.LogDirectory != "l" {
		t.Errorf("want l, got %s", config.LogDirectory)
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
			"display_messages": true,
			"record_messages": false,
			"log_directory": "log"
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

	if !config.DisplayMessages {
		t.Error("want DisplayMessages true")
	}

	if config.RecordMessages {
		t.Error("want RecordMessages false")
	}

	if config.LogDirectory != "log" {
		t.Errorf("want log, got %s", config.LogDirectory)
	}
}
