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

	"github.com/goblimey/go-ntrip/apps/rtcmlogger/config"
	"github.com/goblimey/go-tools/dailylogger"
)

const bufferLength = 8096

// eventLogger writes to the daily event log.
var eventLogger *slog.Logger

// These flags are used to restrict the number of error messages in
// the event log (assuming that events are being logged at all).  A
// stream of failed attempts to do something should only produce one
// error message.
var reportingReadErrors = true
var reportingEventLogWriteErrors = true
var reportingLogWriteErrors = true

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

	// The directory in which to record RTCM messages is defined in the
	// JSON config file, or "." by default.
	if len(cfg.MessageLogDirectory) == 0 {
		cfg.MessageLogDirectory = "."
	}

	start(cfg)
}

// start kicks off the RTCM record and (if configured) the event logger.
func start(cfg *config.Config) {

	if cfg.LogEvents {
		// Create the event logger.  It uses structured logging and
		// switches to a new file each day with a datestamped name.
		dailyEventLogger := dailylogger.New(cfg.EventLogDirectory, "rtcmlogger.", ".log")
		eventLogger = slog.New(slog.NewTextHandler(dailyEventLogger, nil))
	}

	// The recorder logs RTCM messages and runs until cfg.RecorderChannel
	// is closed  The defer ensures that this happens as this function
	// ends, which is does if and when the input is exhausted.  If the
	// input is from a live GNSS device, the function will run until
	// the device stops sending or this process is killed.
	recorderChannel := make(chan []byte)
	defer close(recorderChannel)
	dailyRecorder := newLogWriter(cfg)
	go recorder(recorderChannel, dailyRecorder, cfg)

	readAndWrite(recorderChannel, cfg)

	// Done.  The defer above closes the recorder channel, which stops
	// the recorder goroutine.
}

// readAndWrite runs until the input is exhausted (which may never
// happen) or the process is forcibly stopped.  It reads from stdin,
// writes the result to stdout and copies it to the recorder channel.
func readAndWrite(recorderChannel chan []byte, cfg *config.Config) {

	readBuffer := make([]byte, bufferLength)

	// Loop reading from stdin, writing to stdout and
	// recording the data.

	for {

		// Read a block from stdin, write it to stdout and
		// send a copy to the recorder.

		n, errRead := os.Stdin.Read(readBuffer)
		if errRead == io.EOF {
			// This is expected behaviour when the source is
			// a pre-recorded file.  If the source is a live
			// GNSS device it only happens if the device dies.
			break
		}
		if cfg.LogEvents {
			if errRead != nil {
				if reportingReadErrors {
					eventLogger.Error(errRead.Error())
					// Only report repeated failures once.
					reportingReadErrors = false
				}
			} else {
				// After a successful read, re-enable reporting.
				if !reportingReadErrors {
					eventLogger.Info("successful read after failure\n")
				}
				reportingReadErrors = true
			}
		}

		if n == 0 {
			continue
		}

		// Write the data to stdout
		_, errWrite := os.Stdout.Write(readBuffer[:n])
		if cfg.LogEvents {
			if errWrite != nil {
				// Only report repeated failures once.
				reportingEventLogWriteErrors = false
				em := fmt.Sprintf("write failed - %v\n", errWrite)
				eventLogger.Error(em)
			} else {
				// After a successful write, re-enable reporting.
				if !reportingEventLogWriteErrors {
					eventLogger.Info("Successful write after failure\n")
				}
				reportingEventLogWriteErrors = true
			}
		}

		// Send a copy of the input to the recorder.
		copyBuffer := make([]byte, n)
		copy(copyBuffer, readBuffer[:n])
		recorderChannel <- copyBuffer
	}

	if cfg.LogEvents {
		eventLogger.Info("end of file")
	}
}

// recorder loops, reading buffers from the channel and writing them
// to the daily log file.
func recorder(recorderChannel chan []byte, writer io.Writer, cfg *config.Config) {

	if writer == nil {
		if cfg.LogEvents {
			eventLogger.Error("internal error - writer is nil")
		}
		return
	}

	if recorderChannel == nil {
		if cfg.LogEvents {
			eventLogger.Error("internal error - recorder channel is nil")
		}
		return
	}

	// Receive and write messages until recorderChannel is closed.
	for {
		buffer, ok := <-recorderChannel
		if !ok {
			// We're done!
			break
		}

		writeRTCMLog(&buffer, writer, cfg)
	}
}

func newEventWriter(cfg *config.Config) *slog.Logger {
	if cfg.LogEvents {
		// Create the event logger.  It uses structured logging and
		// switches to a new file each day with a datestamped name.
		dailyEventLogger := dailylogger.New(cfg.EventLogDirectory, "rtcmlogger.", ".log")
		eventLogger = slog.New(slog.NewTextHandler(dailyEventLogger, nil))
		return eventLogger
	}

	return nil
}

// newLogWriter creates an RTCM log writer and returns it.  It's separated out to
// support integration testing.
func newLogWriter(cfg *config.Config) io.Writer {
	dailyRecorder := dailylogger.New(cfg.MessageLogDirectory, "rtcmlogger.", ".rtcm")
	return dailyRecorder
}

// writeRTCMLog writes the buffer to the given writer.  It's separated
// out to support unit testing.
func writeRTCMLog(buffer *[]byte, writer io.Writer, cfg *config.Config) {

	n, err := writer.Write(*buffer)

	if cfg.LogEvents {
		if err != nil {
			if reportingEventLogWriteErrors {
				em := fmt.Sprintf("write to daily RTCM log failed - %s", err.Error())
				eventLogger.Error(em)
				// Only report repeated failures once.
				reportingEventLogWriteErrors = false
			} else {
				// After a successful attempt, re-enable reporting.
				reportingEventLogWriteErrors = true
			}
		}

		if n != len(*buffer) {
			if reportingLogWriteErrors {
				// Only report the first of a stream of errors.
				reportingLogWriteErrors = false
				em := fmt.Sprintf(
					"warning: only wrote %d of %d bytes to the daily RTCM log",
					n, len(*buffer))
				eventLogger.Error(em)
			}

		} else {
			// After a successful write, re-enable reporting.
			if !reportingLogWriteErrors {
				eventLogger.Info("Successful write to RTCM log after failure\n")
			}
			reportingLogWriteErrors = true
		}
	}
}
