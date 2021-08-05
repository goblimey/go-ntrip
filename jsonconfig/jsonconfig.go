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
	"io"
	"io/ioutil"
	"log"
	"os"
	"time"
)

// Config contains the values from the JSON config file and a
// pointer to the system log.  To support unit testing, functions
// that need to write to the log should get it from the config
// or from an argument.
type Config struct {
	Filenames        []string `json:"input"`
	SendToCaster     bool     `json:"sendtocaster"`
	WriteOutputLog   bool     `json:"writeoutputLog"`
	WriteReadableLog bool     `json:"writereadableLog"`
	CasterHostName   string   `json:"casterhostname"`
	CasterPort       uint     `json:"casterport"`
	CasterUserName   string   `json:"casterUserName"`
	CasterPassword   string   `json:"casterPassword"`
	// LostInputConnectionTimeout defines the input timeout.
	LostInputConnectionTimeout uint `json:"timeout"`
	// LostConnectionSleepTime is the time to sleep between connection attempts.
	LostInputConnectionSleepTime uint `json:"sleeptime"`

	// SystemLog is the Writer used for logging and can be nil.  It's not
	// supplied in the JSON.  The application should call GetJSONConfigFromFile
	// and, if there is a log writer, supply it as a parameter.
	SystemLog *log.Logger

	// logging indicates that logging should be done.
	logging bool
}

// GetJSONConfigFromFile gets the config from the file given by configName.
func GetJSONConfigFromFile(configFileName string, systemLog *log.Logger) (*Config, error) {

	jsonReader, fileErr := os.Open(configFileName)
	if fileErr != nil {
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
		systemLog.Printf("cannot read the JSON control file - %s\n", jsonReadError.Error())
		return nil, jsonReadError
	}

	var config Config
	// Parse the JSON control file
	jsonParseError := json.Unmarshal(jsonBytes, &config)
	if jsonParseError != nil {
		systemLog.Printf("cannot parse the JSON control file - %s\n", jsonParseError.Error())
		return nil, jsonParseError
	}

	// Set the fields that are not set by the JSON.
	config.SystemLog = systemLog
	config.logging = true

	return &config, nil
}

// WaitAndConnectToInput tries repeatedly (potentially indefinitely)
// to connect to one of the input files whose names are given.
func WaitAndConnectToInput(config *Config) io.Reader {
	for {
		reader := findInputDevice(config)
		if reader != nil {
			if config.logging {
				config.SystemLog.Println(
					"waitAndConnect: connected to GNSS source")
			}
			return reader // Success!
		}
		if config.logging {
			config.SystemLog.Println(
				"waitAndConnectToInput: failed to connect to GNSS source.  Retrying")
		}
		sleeptime := time.Duration(config.LostInputConnectionSleepTime) * time.Second
		time.Sleep(sleeptime)
	}
}

// findInputdevice searches the given list of InputFiles.If one of the named
// files exists and can be opened for reading, it returns a Reader connected
// to it.  The Reader responds to the supplied Context (which may, for example,
// contain a read timeout).
func findInputDevice(config *Config) io.Reader {
	// Note:  The device names "/dev/ttyACM0" etc on a Raspberry Pi
	// DO NOT relate to the physical USB sockets on the circuit board. They
	// are used in turn. After the Pi boots, the first connection uses
	// "/dev/ttyACM0".  If the GNSS device loses power briefly, then when it
	// comes back, the connection is represented by "/dev/ttyACM1", and so on,
	// even though the USB plu is connected to the same port. So, whenever
	// software running on the Pi needs to establish a connection with a serial
	// USB device, it needs to do this search.

	file := getInputFile(config)
	if file == nil {
		// None of the input file are present. Return nil.
		return nil
	}

	// The file exists and is open.  Return it.
	return file
}

// getInputFile returns a connection to the first file in the given list
// that it can open for reading or nil if it can't open any file.  The
// connection returned has a read deadline set given by the configuration.
//
func getInputFile(config *Config) *os.File {
	for _, name := range config.Filenames {
		file, err := os.Open(name)
		if err == nil {
			if config.logging {
				config.SystemLog.Printf("getInputFile: found %s", name)
				// Turn off logging after the first successful scan.
				config.logging = false
			}
			// The file exists and we've just opened it for reading.
			return file
		}
		durationToDeadline := time.Duration(config.LostInputConnectionTimeout) *
			time.Second
		deadline := time.Now().Add(durationToDeadline)
		file.SetReadDeadline(deadline)
	}

	return nil
}
