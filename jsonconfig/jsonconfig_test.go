package jsonconfig

import (
	"log"
	"strings"
	"testing"

	"github.com/goblimey/go-tools/switchwriter"
)

// TestJSONControl tests that the correct data is produced when the
// text from a JSON control file is unmarshalled.
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
		"read_timeout_milliseconds": 100,
		"sleep_time_after_failed_open_milliseconds": 200,
		"wait_time_on_EOF_millis": 300,
		"timeout_on_EOF_milliseconds":2000
	}`)

	const wantReadTimeout = 100
	const wantSleepTime = 200
	const wantWaitTime = 300
	const wantEOFTimeout = 2000

	writer := switchwriter.New()
	logger := log.New(writer, "jsonconfig_test", 0)

	config, err := getJSONConfig(reader, logger)
	if err != nil {
		t.Error(err)
		return
	}

	if config == nil {
		t.Error("parsing json failed - nil")
		return
	}

	numFiles := len(config.Filenames)
	if numFiles != 2 {
		t.Errorf("parsing json, expected 2 files, got %d", numFiles)
		return
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

	if config.ReadTimeoutMilliSeconds != wantReadTimeout {
		t.Errorf("parsing json, expected timeout to be %d, got %d",
			wantReadTimeout, config.ReadTimeoutMilliSeconds)
	}

	if config.SleepTimeAfterFailedOpenMilliSeconds != wantSleepTime {
		t.Errorf("parsing json, expected sleep time to be %d, got %d",
			wantSleepTime, config.SleepTimeAfterFailedOpenMilliSeconds)
	}

	if config.WaitTimeOnEOFMilliseconds != wantWaitTime {
		t.Errorf("parsing json, expected wait time to be %d, got %d",
			wantWaitTime, config.WaitTimeOnEOFMilliseconds)
	}

	if config.TimeoutOnEOFMilliSeconds != wantEOFTimeout {
		t.Errorf("parsing json, expected wait time to be %d, got %d",
			wantEOFTimeout, config.TimeoutOnEOFMilliSeconds)
	}
}

// TestJSONControl tests that the correct data is produced when the
// text from a JSON control file is unmarshalled.
func TestGetJSONControlWithOneFile(t *testing.T) {
	reader := strings.NewReader(`{
		"input": [
			"a"
		],
		"stop_on_eof": true,
		"record_messages": true,
		"message_log_directory": "someDirectory",
		"display_messages": true,
		"caster_host_name": "caster.example.com",
		"caster_port": 2101,
		"caster_user_name": "user",
		"caster_password": "password",
		"read_timeout_milliseconds": 100,
		"sleep_time_after_failed_open_milliseconds": 200,
		"wait_time_on_EOF_millis": 300,
		"timeout_on_EOF_milliseconds":2000
	}`)

	writer := switchwriter.New()
	logger := log.New(writer, "jsonconfig_test", 0)

	config, err := getJSONConfig(reader, logger)
	if err != nil {
		t.Error(err)
		return
	}

	if config == nil {
		t.Error("parsing json failed - nil")
		return
	}

	numFiles := len(config.Filenames)
	if numFiles != 1 {
		t.Errorf("parsing json, expected 1 file, got %d", numFiles)
		return
	}

	if config.Filenames[0] != "a" {
		t.Errorf("want a, got %s", config.Filenames[0])
	}
}

// TestTimeouts checks that the read timeout is correctly converted to a
// duration.
func TestReadTimeout(t *testing.T) {
	const timeout = 2
	const want = 2000000

	config := Config{ReadTimeoutMilliSeconds: timeout}

	got := config.ReadTimeout()
	if want != got {
		t.Errorf("want %d got %d", want, got)
	}
}

// TestSleepTimeAfterReadTimeout checks that SleepTimeAfterReadTimeout correctly
// converts the value in the config to a duration.
func TestSleepTimeAfterReadTimeout(t *testing.T) {
	const sleepTime = 300
	const want = 300000000

	config := Config{SleepTimeAfterFailedOpenMilliSeconds: sleepTime}

	got := config.SleepTimeAfterFailedOpen()
	if want != got {
		t.Errorf("want %d got %d", want, got)
	}
}
