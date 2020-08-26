package main

import (
	"flag"
	"io"
	"log"
	"os"

	rtcmLogger "github.com/goblimey/go-ntrip/rtcmlogger/logger"
)

// The logger runs constantly reading RTCM messages in blocks on standard input and writing
// to standard output.  It also creates a daily log of the messages.  Each log file is named
// after the day it was created, for example "data.2020-04-01.rtcm3".  Around midnight the
// input block may contain some messages from yesterday and some from today, so those blocks
// are ignored - logging is disabled just before midnight and a new datestamped log file is
// created just after midnight the next day.  (For the precise timings, see the logger
// module.)
//
// If the application dies during the day and is restarted, the logger picks up the existing
// existing log file and appends to it, so any existing log data is preserved.

const bufferLength = 8096

var ch chan []byte

// The -l option specifies the directory for RTCM logs.  The default is /ntrip/rtcmlog,
// which is what the docker version uses.
var logDirectory = flag.String("l", "/ntrip/rtcmlog", "log directory")

func main() {

	flag.Parse()

	ch = make(chan []byte)
	defer close(ch)

	go recorder()

	buffer := make([]byte, bufferLength)

	loggingErrors := true // log errors only once.

	for {

		// Read a block from stdin, send a copy to the recorder and
		// write the block to stdout.

		n, err := os.Stdin.Read(buffer)
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}

		copyBuffer := make([]byte, n)
		copy(copyBuffer, buffer[:n])
		ch <- copyBuffer

		_, err = os.Stdout.Write(buffer[:n])
		if err != nil {
			if loggingErrors {
				// Log errors only once.
				loggingErrors = false
				log.Printf("write failed %v\n", err)
			}
		}
	}
}

// recorder loops, reading buffers from the channel and writing them to the daily log file.
func recorder() {
	// The logWriter handles the timing and naming rules for logging.  It creates a new log
	// file each morning and over the day it appends to that file.  When logging is disabled
	// it discards anything written to it.
	logWriter := newLogWriter(*logDirectory)
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

// writeBuffer writes the buffer to the log file.  It's separated out to support
// integration testing.
//
func writeBuffer(buffer *[]byte, writer io.Writer) {
	n, err := writer.Write(*buffer)
	if err != nil {
		log.Printf("write to daily RTCM log failed - %s\n", err.Error())
	}

	if n != len(*buffer) {
		log.Printf("warning: only wrote %d of %d bytes to the daily RTCM log\n",
			n, len(*buffer))
	}
}
