package main

import (
	"os"
	"testing"

	"github.com/goblimey/go-tools/testsupport"
	"go.bug.st/serial"
)

func TestParseConfig(t *testing.T) {

	json := []byte(`
		{
			"speed": 1,
			"parity":"even_parity",
			"data_bits": 5,
			"stop_bits": 1.5,
			"initial_status_bits": ["dtr"],
			"read_timeout_milliseconds": 5,
			"sleep_time_after_failed_open_milliseconds": 6,
			"sleep_time_on_EOF_millis": 7,
			"filenames": ["a", "b"]
			
		}
	`)

	config, err := parseConfigFromBytes(json)

	if err != nil {
		t.Error(err)
		return
	}

	if config.Speed != 1 {
		t.Errorf("want 1, got %d", config.Speed)
	}

	if config.Parity != "even_parity" {
		t.Errorf("want even_parity , got %s", config.Parity)
	}

	if config.DataBits != 5 {
		t.Errorf("want 5, got %d", config.DataBits)
	}

	if config.StopBits != 1.5 {
		t.Errorf("want 1.5, got %f", config.StopBits)
	}

	if len(config.InitialStatusBits) != 1 {
		t.Errorf("want 1 status bit, got %d", len(config.InitialStatusBits))
	}

	if config.InitialStatusBits[0] != "dtr" {
		t.Errorf("want ctr got %s", config.InitialStatusBits[0])
	}

	if config.mode.BaudRate != 1 {
		t.Errorf("want 1, got %d", config.mode.BaudRate)
	}

	if config.mode.Parity != serial.EvenParity {
		t.Errorf("want EvenParity (2), got %v", config.mode.Parity)
	}

	if config.mode.DataBits != 5 {
		t.Errorf("want 5, got %d", config.mode.DataBits)
	}

	if config.mode.StopBits != serial.OnePointFiveStopBits {
		t.Errorf("want 1.5 stop bits, got %v", config.mode.StopBits)
	}

	if !config.mode.InitialStatusBits.DTR {
		t.Errorf("want DTR true")
	}

	if config.mode.InitialStatusBits.RTS {
		t.Errorf("want RTS false")
	}

	if config.ReadTimeoutMilliSeconds != 5 {
		t.Errorf("want 5, got %d",
			config.ReadTimeoutMilliSeconds)
	}

	if config.SleepTimeAfterFailedOpenMilliSeconds != 6 {
		t.Errorf("want 6, got %d",
			config.SleepTimeAfterFailedOpenMilliSeconds)
	}

	if config.SleepTimeOnEOFMilliseconds != 7 {
		t.Errorf("want 7, got %d",
			config.SleepTimeOnEOFMilliseconds)
	}

	if len(config.Filenames) == 0 {
		t.Error("no file names")
		return
	}
	if len(config.Filenames) != 2 {
		t.Errorf("want two filenames, got %d", len(config.Filenames))
		return
	}

	if config.Filenames[0] != "a" {
		t.Errorf("want a, got %s", config.Filenames[0])
	}

	if config.Filenames[1] != "b" {
		t.Errorf("want b, got %s", config.Filenames[1])
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
			"speed": 115200,
			"parity": "no_parity",
			"data_bits": 7,
			"stop_bits": 2,
			"initial_status_bits": ["dtr"],
			"read_timeout_milliseconds": 3000,
			"sleep_time_after_failed_open_milliseconds": 4000,
			"sleep_time_on_EOF_millis": 5000,
			"filenames": [
				"/dev/ttyACM0", "/dev/ttyACM1"
			]
		}
	`
	_, writeError := writer.Write([]byte(json))
	if writeError != nil {
		t.Error(writeError)
		return
	}

	config, errConfig := getConfig("./config.json")
	if errConfig != nil {
		t.Error(errConfig)
		return
	}

	if config.Speed != 115200 {
		t.Errorf("want 115200, got %d", config.Speed)
	}

	if config.Parity != "no_parity" {
		t.Errorf("want no_parity, got %s", config.Parity)
	}

	if config.DataBits != 7 {
		t.Errorf("want 7, got %d", config.DataBits)
	}

	if config.StopBits != 2 {
		t.Errorf("want 2, got %f", config.StopBits)
	}

	if len(config.InitialStatusBits) != 1 {
		t.Errorf("want 1 status bit, got %d", len(config.InitialStatusBits))
	}

	if config.InitialStatusBits[0] != "dtr" {
		t.Error("want dtr, got " + config.InitialStatusBits[0])
	}

	if config.ReadTimeoutMilliSeconds != 3000 {
		t.Errorf("want 3000, got %d",
			config.ReadTimeoutMilliSeconds)
	}

	if config.SleepTimeAfterFailedOpenMilliSeconds != 4000 {
		t.Errorf("want 4000, got %d",
			config.SleepTimeAfterFailedOpenMilliSeconds)
	}

	if config.SleepTimeOnEOFMilliseconds != 5000 {
		t.Errorf("want 5000, got %d",
			config.SleepTimeOnEOFMilliseconds)
	}

	if len(config.Filenames) == 0 {
		t.Error("no file names")
		return
	}
	if len(config.Filenames) != 2 {
		t.Errorf("want two filenames, got %d", len(config.Filenames))
		return
	}

	if config.Filenames[0] != "/dev/ttyACM0" {
		t.Errorf("want /dev/ttyACM0, got %s", config.Filenames[0])
	}

	if config.Filenames[1] != "/dev/ttyACM1" {
		t.Errorf("want /dev/ttyACM1, got %s", config.Filenames[1])
	}
}
