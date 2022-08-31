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
//
package main

import (
	"io"
	"log"
	"os"

	"github.com/goblimey/go-ntrip/jsonconfig"
	rtcmLogger "github.com/goblimey/go-ntrip/rtcmlogger/logger"
	"github.com/goblimey/go-tools/dailylogger"
)

const bufferLength = 8096

var ch chan []byte

// controlFileName is the name of the JSON control file that defines
// the names of the potential input files.
const controlFileName = "./ntrip.json"

// logDirectory is the directory for RTCM logs.  The default is ".".
var logDirectory string

// eventLogger writes to the daily event log.
var eventLog *log.Logger

// These control the logging of failures - a stream of failures is only
// logged once.
var reportingWriteErrors = true
var reportingChannelErrors = true
var reportingLogWriteFailures = true

func init() {
	// Create the event log.
	eventLogger := dailylogger.New(".", "rtcmlogger.", ".log")
	eventLog = log.New(eventLogger, "rtcmlogger",
		log.LstdFlags|log.Lshortfile)
}

func main() {

	jsonConfig, err := jsonconfig.GetJSONConfigFromFile(controlFileName, eventLog)

	if err != nil {
		// There is no JSON config file.  We can't continue.
		eventLog.Fatalf("cannot find config %s", controlFileName)
		os.Exit(1)
	}

	// The directory in which to record messages is defined in the JSON control
	// file, or "." by default.
	if len(jsonConfig.MessageLogDirectory) == 0 {
		jsonConfig.MessageLogDirectory = "."
	}

	ch = make(chan []byte)
	defer close(ch)

	go recorder(jsonConfig.MessageLogDirectory)

	buffer := make([]byte, bufferLength)

	if jsonConfig.RecordMessages {
		ch = make(chan []byte)
	defer close(ch)
	go recorder(jsonConfig.MessageLogDirectory)
	}

	for {

		// Read a block from stdin, send a copy to the recorder and
		// write the block to stdout.

		n, err := os.Stdin.Read(buffer)
		if err == io.EOF {
			break
		}
		if err != nil {
			if reportingWriteErrors {
				reportingWriteErrors = false // stop logging write failures
				eventLog.Println(err)
			}
		} else {
			reportingWriteErrors = true // start logging write failures
		}

		// Write the data to stdout
		_, err = os.Stdout.Write(buffer[:n])

		if jsonConfig.RecordMessages {
			// Copy the data and send it to the logging channel.
			copyBuffer := make([]byte, n)
			copy(copyBuffer, buffer[:n])
			ch <- copyBuffer
			if err != nil {
				if reportingChannelErrors {
					reportingChannelErrors = false // Stop logging channel failures.
					eventLog.Printf("write failed %v\n", err)
				}
			} else {
				reportingChannelErrors = true // Start logging channel failures.
			}
		}
	}
}

// recorder loops, reading buffers from the channel and writing them to the daily log file.
func recorder(logDirectory string) {
	// The logWriter handles the timing and naming rules for logging.  It creates a new log
	// file each morning and over the day it appends to that file.  When logging is disabled
	// it discards anything written to it.
	logWriter := newLogWriter(logDirectory)

	// Receive and write messages the program exits.
	for {
		buffer, ok := <-ch
		if ok {
			writeBuffer(&buffer, logWriter)
		}
	}
}

// newLogWriter creates an RTCM log writer and returns it.  It's separated out to
// support integration testing.
func newLogWriter(logDirectory string) io.Writer {
	return rtcmLogger.New(logDirectory)
}

// writeBuffer writes the buffer to the RTCM log file.  It's separated out to
// support integration testing.
//
func writeBuffer(buffer *[]byte, writer io.Writer) {
	n, err := writer.Write(*buffer)
	if err != nil {
		reportingLogWriteFailures = false // Stop logging failures.
		log.Printf("write to daily RTCM log failed - %s\n", err.Error())
	} else {
		reportingLogWriteFailures = true // Start logging failures.
	}

	if n != len(*buffer) {
		log.Printf("warning: only wrote %d of %d bytes to the daily RTCM log\n",
			n, len(*buffer))
	}
}
