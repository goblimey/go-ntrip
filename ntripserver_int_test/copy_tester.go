package main

import (
	"log"
	"os"
	"time"

	"github.com/goblimey/go-ntrip/rtcm"
	"github.com/goblimey/go-ntrip/rtcm/utils"
	"github.com/goblimey/go-tools/dailylogger"
)

// Integration test for  the NTRIP server.  This just reads RTCM messages
// from stdin and copies them to stdout.  If the input file contains only
// complete valid RTCM messages, the result should be a copy of it.
//
// The application also produces a log file with a date-stamped name.
//
func main() {

	writer := dailylogger.New(".", "ntripserver.int.test.", ".log")
	logFlags := log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile | log.LUTC | log.Lmsgprefix
	systemLog := log.New(writer, "ntripserver_int_test: ", logFlags)

	outputLogChan := make(chan []byte, 1000)
	// This goroutine sends to stdout any messages sent to the channel
	go rtcm.WriteMessagesToLog(outputLogChan, os.Stdout)

	var channels []chan []byte
	channels = append(channels, outputLogChan)

	if err != nil {
		log.Fatal(err)
	}
	startTime := time.Date(2020, time.November, 13, 0, 0, 0, 0, utils.LocationUTC)
	rtcmHandler := rtcm.New(startTime, systemLog)
	rtcm.HandleMessages(rtcmHandler, os.Stdin, channels)
}
