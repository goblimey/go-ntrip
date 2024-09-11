package jsonconfig

// The jsonconfig package provides support for reading and using a JSON configuration
// file in a standard format for various NTRIP applications.
//
// An example config file:
//
// {
//		"input": ["/dev/ttyACM0", "/dev/ttyACM1", "/dev/ttyACM2", "/dev/ttyACM3"],
//      "sendToCaster": true,
//		"writeoutputLog": false,
//		"writereadableLog": false,
//      "casterhostname": "caster.example.com",
//      "casterport": 2101,
//      "casterUserName": "user",
//		"casterPassword": "password",
//		"timeout": 1,
//		"sleeptime": 2
//	}
//
// This example suits the NTRIP server running on a Raspberry Pi and reading RTCM
// messages from a GPS device over a serial USB connection and sending them to a set
// of output channels for processing.  (For example, the goroutine at the other end of
// the channel might send the messages to an NTRIP caster, or it might log them in a
// file.)  The config specifies the list of Linux devices that may be used to
// represent the USB connection, flags that determine which output channels should be
// enabled, the details needed to  connect to an NTRIP caster and some controls for
// handling timeouts and retries if the incoming message stream dies.
//
// Other applications such as the RTCM Filter use the same format but don't use all
// the fields.
//
// The package contains functions to read a configuration from a file, connect to the
// incoming data stream and to attempt to reconnect if the stream then dies.

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"time"
)

// Config contains the values from the JSON config file and a ready-made writer
// that writes to the system log.  To support unit testing, functions that
// write to the log should use this writer - we don't want to force unit tests
// to write to a real log file.)
type Config struct {
	// Filenames is a list of filenames to try to open - first one wins.
	Filenames []string `json:"input"`

	// RecordMessages says whether to record a verbatim copy of RTCM messages in a file.
	RecordMessages bool `json:"record_messages"`

	// MessageLogDirectory specifies the directory in which the file of RTCM
	// messages is stored
	MessageLogDirectory string `json:"message_log_directory"`

	// DisplayMessages says whether to write a readable display of the incoming messages.
	// Note: turning this on will produce a lot of output.
	DisplayMessages bool `json:"display_messages"`

	// CasterHostName is host name of the NTRIP (broad)caster.
	CasterHostName string `json:"caster_host_name"`

	// CasterPort is port on which the (broad)caster is listening for NTRIP traffic.
	CasterPort uint `json:"caster_port"`

	// CasterUsername is the user name to connect to the NTRIP (broad)caster.
	CasterUserName string `json:"caster_user_name"`

	// CasterPassword is password to connect to the NTRIP (broad)caster.
	CasterPassword string `json:"caster_password"`

	// ReadTimeoutSeconds defines the input timeout in milliseconds.  This is
	// bound into the reader by getInputFile.  The function ReadTimeout
	// returns this as a duration.
	ReadTimeoutMilliSeconds uint `json:"read_timeout_milliseconds"`

	// SleepTimeAfterFailedOpenMilliSeconds defines the time that a caller of
	// getInputFile should pause before retrying if it fails to find any of
	// the files listed in the config.  The function SleepTimeAfterFailedOpen returns this
	// as a duration.
	SleepTimeAfterFailedOpenMilliSeconds uint `json:"sleep_time_after_failed_open_milliseconds"`

	// WaitTimeOnEOFMilliseconds specifies in milliseconds how long a caller
	// should wait before retrying if a read operation on a serial connection
	// returns EOF.  The function WaitTimeOnEOF returns this as a duration.
	WaitTimeOnEOFMilliseconds uint `json:"wait_time_on_EOF_millis"`

	// TimeoutOnEOFMilliSeconds returns the duration of the timeout that a caller
	// should apply when a series of read operations return EOF.  A value of 0 means
	// that the caller should give up on the first EOF.
	//
	// The function TimeoutOnEOF returns this as a duration.
	TimeoutOnEOFMilliSeconds uint `json:"timeout_on_EOF_milliseconds"`

	// SystemLog is the Writer used for the daily activity log (as opposed to
	// the log of incoming RTCM messages) and can be nil.  It's not supplied
	// in the JSON.  The application should call GetJSONConfigFromFile and, if
	// there is a log writer, supply it as a parameter.
	SystemLog *log.Logger
}

// GetJSONConfigFromFile gets the config from the file given by configName.
func GetJSONConfigFromFile(configFileName string, systemLog *log.Logger) (*Config, error) {
	jsonReader, fileErr := os.Open(configFileName)
	if fileErr != nil {
		if systemLog != nil {
			systemLog.Printf("cannot open config file %s: %v", configFileName, fileErr)
		} else {
			log.Printf("cannot open config file %s: %v", configFileName, fileErr)
		}
		return nil, fileErr
	}

	// There is a JSON control file.  Read and unmarshall it.
	config, jsonError := getJSONConfig(jsonReader, systemLog)
	if jsonError != nil {
		return nil, jsonError
	}

	return config, nil
}

// getJSONConfig reads from the given source and returns the config.
func getJSONConfig(jsonSource io.Reader, systemLog *log.Logger) (*Config, error) {

	jsonBytes, jsonReadError := ioutil.ReadAll(jsonSource)
	if jsonReadError != nil {
		// We can't read the control file - permissions?
		errorMessage1 := fmt.Sprintf("cannot read the JSON control file - %v\n", jsonReadError)
		if systemLog != nil {
			systemLog.Println(errorMessage1)
		} else {
			log.Println(errorMessage1)
		}
		return nil, jsonReadError
	}

	var config Config
	// Parse the JSON control file
	jsonParseError := json.Unmarshal(jsonBytes, &config)
	if jsonParseError != nil {
		errorMessage2 := fmt.Sprintf("cannot parse the JSON control file - %v\n", jsonParseError)
		if systemLog != nil {
			systemLog.Println(errorMessage2)
		} else {
			log.Println(errorMessage2)
		}
		return nil, jsonParseError
	}

	// Set the fields that are not set by the JSON.

	config.SystemLog = systemLog

	return &config, nil
}

// ReadTimeout gets the read timeout as a duration.
func (config *Config) ReadTimeout() time.Duration {
	return time.Duration(config.ReadTimeoutMilliSeconds) * time.Millisecond
}

// SleepTimeAfterFailedOpen returns the time that a caller of getInputFile should
// pause before retrying if it fails to find any of the files listed in the config.
func (config *Config) SleepTimeAfterFailedOpen() time.Duration {
	return time.Duration(config.SleepTimeAfterFailedOpenMilliSeconds) * time.Millisecond
}

// WaitTimeOnEOF returns the time that a caller should pause before retrying
// after a read operation has failed.
func (config *Config) WaitTimeOnEOF() time.Duration {
	return time.Duration(config.WaitTimeOnEOFMilliseconds) * time.Millisecond
}

// TimeoutOnEOF returns the duration of the timeout that should be applied
// when a series of read operations return EOF.  A value of 0 means that the
// caller should give up on the first EOF.
func (config *Config) TimeoutOnEOF() time.Duration {
	return time.Duration(config.TimeoutOnEOFMilliSeconds) * time.Millisecond
}

// connectionFailureLogged controls when a connection failure is
// logged.
var connectionFailureLogged = false

// WaitAndConnectToInput tries repeatedly (potentially indefinitely)
// to connect to one of the input files whose names are given.
func (config *Config) WaitAndConnectToInput() io.Reader {
	for {
		reader := config.getInputFile()
		if reader != nil {
			logEntry1 := "waitAndConnectToInput: connected to GNSS source"
			if config.SystemLog != nil {
				config.SystemLog.Println(logEntry1)
			} else {
				log.Println(logEntry1)
			}
			// Log the next connection failure.
			connectionFailureLogged = false
			return reader // Success!
		}

		if !connectionFailureLogged {
			// Log only the first of a series of connection failures.
			logEntry2 := "waitAndConnectToInput: failed to connect to GNSS source.  Retrying"
			if config.SystemLog != nil {
				config.SystemLog.Println(logEntry2)
			} else {
				log.Println(logEntry2)
			}
			connectionFailureLogged = true
		}
		// Pause and try again.
		time.Sleep(config.SleepTimeAfterFailedOpen())
	}
}

// getInputFile returns a connection to the first file in the given list
// that it can open for reading or nil if it can't open any file.  The
// connection returned has a read deadline set given by the configuration.
func (config *Config) getInputFile() io.Reader {
	for _, name := range config.Filenames {

		// Use https://github.com/tarm/serial

		file, err := os.Open(name)
		if err == nil {
			logEntry := fmt.Sprintf("getInputFile: found %s", name)
			if config.SystemLog != nil {
				config.SystemLog.Println(logEntry)
			} else {
				log.Println(logEntry)
			}
			// The file exists and we've just opened it for reading.
			// Set the read deadline using the value given in the config.
			deadline := time.Now().Add(config.ReadTimeout())
			file.SetReadDeadline(deadline)
			// Return the file as a reader.
			return file
		}

		// The open failed.  Try the next file.
	}

	// The attempt to open every file in the list failed.
	return nil
}
