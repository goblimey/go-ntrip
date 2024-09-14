package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"go.bug.st/serial"
)

// The config is an array of entries, for example:
type Config struct {

	// These values are used to set the mode struct for serial.Open.
	// It contains Speed, Parity, DataBits and StopBits.

	// Speed is the line speed in bits per second.
	Speed int `json:"speed"`

	// Parity is party of the incoming bytes - no_parity (default)
	// odd_parity, even_parity mark_parity, space_parity.
	Parity string `json:"parity"`

	// DataBits is the number of data bits in the byte: 5-8.
	DataBits int `json:"data_bits"`

	// StopBits is the number of stop bits 1, 1.5 or 2.
	StopBits float32 `json:"stop_bits"`

	// Initial status bits should contain zero to two values.
	// These are used to set RTS status (ReadyToSend) and/or
	// DTR status (DataTerminalReady).  For example, a value of
	// "dtr" sets DTR true.  The default is both true.
	InitialStatusBits []string `json:"initial_status_bits"`

	mode serial.Mode

	// These values control the handling of connections that dry up
	// or get closed.

	// ReadTimeoutSeconds defines the input timeout in milliseconds.
	ReadTimeoutMilliSeconds int `json:"read_timeout_milliseconds"`

	// SleepTimeAfterFailedOpenMilliSeconds defines the time to sleep
	// after a failed attempt to find and open a port before retrying.
	SleepTimeAfterFailedOpenMilliSeconds int `json:"sleep_time_after_failed_open_milliseconds"`

	// SleepTimeOnEOFMilliseconds specifies in milliseconds how long
	// to sleep after encountering end of file before trying to
	// reopen the connection.
	SleepTimeOnEOFMilliseconds int `json:"sleep_time_on_EOF_millis"`

	// Filenames is a list of potential device names of serial USB port,
	// for example "/dev/ttyACM0", "/dev/ttyACM1".  For Windows "COM4",
	//"COM5" etc.
	Filenames []string `json:"filenames"`
}

var logger *slog.Logger

func main() {

	// Log to stderr.
	logger = slog.New(slog.NewTextHandler(os.Stderr, nil))

	// Get the name of the config file (mandatory).
	var configFileName string
	flag.StringVar(&configFileName, "c", "", "JSON config file")
	flag.StringVar(&configFileName, "config", "", "JSON config file")

	flag.Parse()

	if len(configFileName) == 0 {
		logger.Error("missing config file: -c or --config")
	}

	// Get the config.
	config, errConfig := getConfig(configFileName)

	if errConfig != nil {
		logger.Error(errConfig.Error())
		os.Exit(-1)
	}

	GrabFromPorts(config, logger)
}

// GrabFromPorts loops until forcibly stopped.  It gets
// the list of open serial ports and compares that with
// with the given list of filenames.  On the first match
// it opens that file as a serial USB port, reads from it
// writes those data to stdout until they are exhausted
// and the read times out.  Then it repeats all that.
func GrabFromPorts(config *Config, logger *slog.Logger) {

	// atStart is a guard that controls the handling of
	// the problem that no serial ports are found.  If this
	// happens at the very start, the program logs an error
	// and dies.  If it happens later, the program waits
	// silently until ports appear.
	var atStart = true

	for {

		knownSerialPorts, errGetPorts := GetSerialPortList()
		if atStart {
			// On the first trip only, insist on at least one
			// active port.
			if errGetPorts != nil {
				logger.Error("error getting active serial ports - " + errGetPorts.Error())
				os.Exit(-1)
			}

			if len(knownSerialPorts) == 0 {
				logger.Error("No active serial ports found!")
				os.Exit(-1)
			}

			// If we get to here, we've seen some serial ports on
			// the first trip.
			atStart = false
		}

		// On trips apart from the very first, if we find no
		// active ports, sleep for a short time and retry.
		if len(knownSerialPorts) == 0 {
			sleepTime := time.Millisecond *
				time.Duration(config.SleepTimeAfterFailedOpenMilliSeconds)
			time.Sleep(sleepTime)
			continue
		}

		port, errConn := GetConnection(config, knownSerialPorts)
		if errConn != nil {
			sleepTime := time.Millisecond *
				time.Duration(config.SleepTimeOnEOFMilliseconds)
			time.Sleep(sleepTime)
			continue
		}

		errGrab := GrabFromPort(port)
		if errGrab != nil {
			logger.Error(errGrab.Error())
		}

		// If we get to here, the supply from the port has dried
		// up.  Wait for a short time and then continue.
		port.Close()
		sleepTime := time.Millisecond *
			time.Duration(config.SleepTimeOnEOFMilliseconds)
		time.Sleep(sleepTime)
	}
}

func GrabFromPort(port serial.Port) error {

	const bufferSize = 1 // 10240

	buffer := make([]byte, bufferSize)

	for {

		n, errRead := port.Read(buffer)
		if errRead != nil {
			return errRead
		}

		if n == 0 {
			// This probably indicates that the Read has timed out.
			return errors.New("timeout")

		} else {
			// We read some data.  Write it out.
			os.Stdout.Write(buffer[:n])
		}
	}
}

func GetConnection(config *Config, knownSerialPorts []string) (serial.Port, error) {
	for _, portName := range knownSerialPorts {
		for i := range config.Filenames {
			if config.Filenames[i] == portName {
				port, errOpen := OpenPort(config, config.Filenames[i])
				if errOpen != nil {
					return nil, errOpen
				}

				timeout := time.Duration(config.ReadTimeoutMilliSeconds) * time.Millisecond
				port.SetReadTimeout(timeout)
				return port, nil
			}
		}
	}

	return nil, errors.New("no matching serial ports found")
}

func OpenPort(config *Config, fileName string) (serial.Port, error) {

	port, err := serial.Open(fileName, &config.mode)
	if err != nil {
		return nil, err
	}
	return port, nil
}

func GetSerialPortList() ([]string, error) {
	ports, err := serial.GetPortsList()
	if err != nil {
		return nil, err
	}

	return ports, nil
}

// getConfig gets the config from the given file.
func getConfig(configFile string) (*Config, error) {
	file, err := os.Open(configFile)
	if err != nil {
		em := fmt.Sprintf("[-] Cannot open config file: %s\n", err.Error())
		slog.Error(em)
		os.Exit(1)
	}

	config, errParse := getConfigFromReader(file)

	if errParse != nil {
		return nil, errParse
	}

	return config, nil
}

// GetConfigFrom Reader gets the config from the given reader.
func getConfigFromReader(configReader io.Reader) (*Config, error) {

	data := make([]byte, 4096)
	n, errRead := configReader.Read(data)
	if errRead != nil {
		em := fmt.Sprintf("[-] Error reading config file: %s\n", errRead.Error())
		slog.Error(em)
		return nil, errRead
	}

	config, parseError := parseConfigFromBytes(data[:n])
	if parseError != nil {
		em := fmt.Sprintf("[-] Not a valid config file: %s\n", parseError.Error())
		slog.Error(em)
		return nil, parseError
	}

	return config, nil
}

func parseConfigFromBytes(data []byte) (*Config, error) {
	var config Config
	config.Filenames = make([]string, 0)
	err := json.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	config.mode.BaudRate = 9600
	if config.Speed != 0 {
		config.mode.BaudRate = config.Speed
	}

	if len(config.Parity) > 0 {
		switch config.Parity {
		case "no_parity":
			config.mode.Parity = serial.NoParity
		case "odd_parity":
			config.mode.Parity = serial.OddParity
		case "even_parity":
			config.mode.Parity = serial.EvenParity
		case "mark_parity":
			config.mode.Parity = serial.MarkParity
		case "space_parity":
			config.mode.Parity = serial.SpaceParity
		default:
			return nil, errors.New("config: illegal parity value " + config.Parity)
		}
	}

	// Must be 5-8.
	if config.DataBits > 0 {
		if !(config.DataBits >= 5 && config.DataBits <= 8) {
			em := fmt.Sprintf("config: data bits must be 5-8, got %d", config.DataBits)
			return nil, errors.New(em)
		}
		config.mode.DataBits = config.DataBits
	}

	if config.StopBits > 0 {
		switch config.StopBits {
		case 0:
			config.mode.StopBits = serial.OneStopBit
		case 1:
			config.mode.StopBits = serial.OneStopBit
		case 1.5:
			config.mode.StopBits = serial.OnePointFiveStopBits
		case 2:
			config.mode.StopBits = serial.TwoStopBits
		default:
			em := fmt.Sprintf("config: stop bit value must be 1, 1.5 or 2.  Got %f",
				config.StopBits)
			return nil, errors.New(em)
		}
	}

	if len(config.InitialStatusBits) > 0 {
		var bits serial.ModemOutputBits
		config.mode.InitialStatusBits = &bits
		for _, b := range config.InitialStatusBits {
			switch strings.ToLower(b) {
			case "dtr":
				config.mode.InitialStatusBits.DTR = true
			case "rts":
				config.mode.InitialStatusBits.RTS = true
			default:
				return nil, errors.New("config: illegal initial status bit value " + b)
			}
		}
	}

	return &config, nil
}
