// The rtcmlogger reads RTCM messages on standard input and writes them to standard
// output.  If the JSON config files sets "record_messages" true then it also
// creates a file containing a copy of the messages which can be converted into RINEX
// format for Precise Point Positioning (PPP).  If record_messages is false then the
// program just acts as a pass-through,  copying stdin to stdout.  The program is
// intended to run in a pipeline between a device which produces RTCM messages and an
// NTRIP server which sends the messages to an NTRIP caster.
//
// The GNSS community use the word "log" for a copy of the RTCM messages stored
// in a file.  This program also maintains an event log, where runtime problems
// are reported.  The names of both contain a datestamp and both are rolled over
// each day.  The log of RTCM messages is called "data.{date}.log" and the event
// log is called "rtcmlogger.{date}.log".  If the program dies during the day and
// is restarted, it picks up the existing log file and appends to it, so the
// existing log data is preserved.
//
// The logger reads and writes the data in blocks.  Around midnight the input block may
// contain some messages from yesterday and some from today, so those blocks are ignored -
// logging is disabled just before midnight and a new datestamped log file is created
// just after midnight the next day.  (For the precise timings, see the logger module.)
// THs also helps RINEX processors which assume that all the data in a RINEX file was
// collected within a 24-hour period.  This only applies to the messages written to the
// logfile, not the ones written to the stdout.  All incoming messages are written to
// stdout regardless of the time of day.
package main

import (
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/goblimey/go-ntrip/rtcmlogger/config"
	rtcmLogger "github.com/goblimey/go-ntrip/rtcmlogger/logger"
	"github.com/goblimey/go-tools/dailylogger"
)

const bufferLength = 8096

var ch chan []byte

// logDirectory is the directory for RTCM logs.  The default is ".".
var logDirectory string

// eventLogger writes to the daily event log.
var eventLogger *slog.Logger

// These control the logging of failures.  To avoid flooding the file system,
// a stream of failures with no successes is only logged once.
var reportingWriteErrors = true
var reportingChannelErrors = true
var reportingWriteFailures = true

func main() {

	// First get the config.  This defines the name and location of the
	// system log (if any) so until we have it, we log any errors to stderr.

	// Get the name of the config file (mandatory).
	var configFileName string
	flag.StringVar(&configFileName, "c", "", "JSON config file")
	flag.StringVar(&configFileName, "config", "", "JSON config file")

	flag.Parse()

	if len(configFileName) == 0 {
		os.Stderr.Write([]byte("missing config file: -c or --config"))
		os.Exit(-1)
	}

	// Get the config.
	cfg, errConfig := config.GetConfig(configFileName)

	if errConfig != nil {
		os.Stderr.Write([]byte((errConfig.Error())))
		os.Exit(-1)
	}

	// The directory in which to record messages is defined in the
	// JSON config file, or "." by default.
	if len(cfg.MessageLogDirectory) == 0 {
		cfg.MessageLogDirectory = "."
	}

	if cfg.LogEvents {
		// Create the event logger.  It uses structured logging and
		// switches to a new file each day with a datestamped name.
		dailyEventLogger := dailylogger.New(cfg.MessageLogDirectory, "rtcmlogger.", ".log")
		eventLogger = slog.New(slog.NewTextHandler(dailyEventLogger, nil))
	}

	if cfg.RecordMessages {
		cfg.RecorderChannel = make(chan []byte)
		defer close(cfg.RecorderChannel)
		dailyRecorder := dailylogger.New(".", "rtcmlogger.", ".rtcm")
		// Run the recorder.  It runs until cfg.RecorderChannel is closed,
		// which happens as this function returns.
		go recorder(dailyRecorder, cfg)
	}

	// readAndWrite runs until the input is exhausted (which may never
	// happen).  If it returns, the defer above closes the recorder
	// channel which stops the recorder goroutine.
	readAndWrite(cfg)
}

func readAndWrite(cfg *config.Config) {

	readBuffer := make([]byte, bufferLength)

	// Loop reading from stdin, writing to stdout and, if
	// directed by the config, recording the data.

	for {

		// Read a block from stdin, and write it to stdout.  If
		// configured, send a copy to the recorder.

		n, errRead := os.Stdin.Read(readBuffer)
		if errRead == io.EOF {
			break
		}
		if errRead != nil {
			if reportingWriteErrors {
				eventLogger.Error(errRead.Error())
				// Only report repeated failures once.
				reportingWriteErrors = false
			}
		} else {
			// After a successful read, re-enable reporting.
			reportingWriteErrors = true
		}

		// Write the data to stdout
		_, errRead = os.Stdout.Write(readBuffer[:n])

		if errRead != nil {
			if cfg.LogEvents && reportingChannelErrors {
				em := fmt.Sprintf("read failed - %v\n", errRead)
				eventLogger.Error(em)
				// Only report repeated failures once.
				reportingChannelErrors = false
			}
		} else {
			// After a successful fetch attempt, re-enable reporting.
			reportingChannelErrors = true
		}

		if cfg.RecorderChannel != nil {
			// Recording is on.  Copy the data and send it to the
			// recorder channel.
			copyBuffer := make([]byte, n)
			copy(copyBuffer, readBuffer[:n])
			cfg.RecorderChannel <- copyBuffer
		}
	}

	if cfg.LogEvents {
		eventLogger.Info("end of file")
	}
}

// recorder loops, reading buffers from the channel and writing them
// to the daily log file.
func recorder(writer io.Writer, cfg *config.Config) {

	if writer == nil {
		return
	}

	if cfg.RecorderChannel == nil {
		return
	}

	// Receive and write messages until cfg.RecorderChannel
	// is closed.
	for {
		buffer, ok := <-cfg.RecorderChannel
		if !ok {
			// We're done!
			break
		}

		writeBuffer(&buffer, writer, cfg)
	}
}

// newLogWriter creates an RTCM log writer and returns it.  It's separated out to
// support integration testing.
func newLogWriter(logDirectory string) io.Writer {
	return rtcmLogger.New(logDirectory)
}

// writeBuffer writes the buffer to the writer and, if configured, to
// the log file.  It's separated out to support unit testing.
func writeBuffer(buffer *[]byte, writer io.Writer, cfg *config.Config) {

	n, err := writer.Write(*buffer)

	if err != nil {
		if cfg.LogEvents && reportingWriteFailures {
			em := fmt.Sprintf("write to daily RTCM log failed - %s", err.Error())
			eventLogger.Error(em)
			// Only report repeated failures once.
			reportingWriteFailures = false
		} else {
			// After a successful attempt, re-enable reporting.
			reportingWriteFailures = true
		}
	}

	if n != len(*buffer) {
		if cfg.LogEvents && reportingWriteFailures {
			em := fmt.Sprintf(
				"warning: only wrote %d of %d bytes to the daily RTCM log",
				n, len(*buffer))
			eventLogger.Error(em)
		}
	}
}
