// The rtcmfilter reads a bit stream from stdin, converts it to RTCM
// messages and sends them to a set of processor functions.  It's
// designed to receive data from a device that emits messages
// continuously so it runs until forcibly stopped.  In the real
// world the device is a GNSS receiver transmitting RTCM messages
// over a serial USB connection.  The serial_usb_grabber handles
// the details of the USB connection and transmits messages on
// stdout, so we can connect it to this via a pipe.
//
// When the application starts up it looks for a JSON config file
// ntrip.json in the current directory.  The config settings define
// which processor functions are run, so the results will be different
// depending on the config.  For example:
//
//	{
//	    "display_messages": true,
//      "record_messages": true,
//      "log_directory": "rtcmlog"
//	}
//
// The incoming data is assumed to contain bursts of RTCM3 messages
// interspersed with other data such as NMEA sentences.  All
// data is presented as rtcm.Message objects, each with a message type.
// There is a special message type for non-RTCM3 data.
//
// The incoming stream of data can be a mixture of RTCM and other
// messages.  It's assumed to come from a GNSS device which is
// issuing messages continuously, for example a Ublox ZED-FP9 sending
// data on a serial USB, IRC or RS/232 connection.  Some of these media
// are prone to dropping or scrambling the occasional character.  That
// will cause the message's CRC check to fail and the message will be
// deemed invalid.
//
// The application starts a new log file each day with a datestamped
// name (such as "filter.2024-08-31.rtcm"), so each log file contains
// data collected in one day.
//
// The filter can be used to clean up a stream of incoming data by
// filtering out the non-RTCM data and any RTCM messages that are
// corrupted in transit and sending only valid RTCM messages along a
// pipe to software such as an NTRIP client:
//
//		      RTCM and                                        RTCM
//	 ------   other data                                      data  ------
//	|GNSS  |-------------> serial_usb_grabber ---> rtcmfilter ---> |NTRIP |
//	|device|  serial USB                      pipe            pipe |client|
//	 ------   connection                                            ------
//
// Another potential use is to capture the incoming RTCM messages, and
// record files of RTCM messages.  These can be converted into RINEX
// format for Precise Point Positioning (PPP) processing.  PPP can be
// used to find the correct position of a fixed base station.

package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	AppCore "github.com/goblimey/go-ntrip/apps/appcore"
	"github.com/goblimey/go-ntrip/apps/rtcmfilter/config"
	"github.com/goblimey/go-ntrip/jsonconfig"
	rtcm "github.com/goblimey/go-ntrip/rtcm/handler"
	"github.com/goblimey/go-ntrip/rtcm/utils"
	"github.com/goblimey/go-tools/dailylogger"
)

type MessageChannel chan rtcm.Message

func main() {

	// logger writes to the daily event log.
	logger := utils.GetDailyLogger("rtcmfilter")

	// Get the name of the config file (mandatory).
	var configFileName string
	flag.StringVar(&configFileName, "c", "", "JSON config file")
	flag.StringVar(&configFileName, "config", "", "JSON config file")

	flag.Parse()

	if len(configFileName) == 0 {
		logger.Println("missing config file: -c or --config")
		os.Exit(-1)
	}

	// Get the config.
	config, errConfig := config.GetConfig(configFileName)

	_ = config
	if errConfig != nil {
		logger.Println(errConfig.Error())
		os.Exit(-1)
	}

	jc := jsonconfig.Config{
		RecordMessages:      config.RecordMessages,
		DisplayMessages:     config.DisplayMessages,
		MessageLogDirectory: config.LogDirectory,
	}

	now := time.Now()

	HandleMessages(now, os.Stdin, os.Stdout, &jc)
}

// writeRTCMMessages receives the messages from the channel and writes them
// to the given writer.  If the channel is closed or there is an error while
// writing, it terminates.  It can be run in a go routine.
func writeRTCMMessages(ch MessageChannel, writer io.Writer) {
	for {
		message, ok := <-ch
		if !ok {
			return
		}

		// We only want valid RTCM messages.
		if message.MessageType == utils.NonRTCMMessage {
			continue
		}

		n, err := writer.Write(message.RawData)
		if err != nil {
			// error - run out of disk space or something.
			return
		}
		if n != len(message.RawData) {
			// incomplete write (which indicates some sort of trouble.)
			return
		}
	}
}

// writeAllMessages receives the messages from the channel and writes them
// to the given writer.  If the channel is closed or there is an error while
// writing, it terminates.  It can be run in a go routine.
func writeAllMessages(ch MessageChannel, writer io.Writer) {
	for {
		message, ok := <-ch
		if !ok {
			return
		}

		n, err := writer.Write(message.RawData)
		if err != nil {
			// error - run out of disk space or something.
			return
		}
		if n != len(message.RawData) {
			// incomplete write (which indicates some sort of trouble.)
			return
		}
	}
}

// writeReadableMessages receives the RTCM messages from the channel,
// decodes them to readable form and writes the result to the given log
// file. It terminates when the channel is closed or there is a write
// error.  It can be run in a go routine.
func writeReadableMessages(ch MessageChannel, writer io.Writer) {

	for {
		message, ok := <-ch
		if !ok {
			return
		}
		// Decode the message.  (The result is very verbose!)
		display := fmt.Sprintf("%s\n", message.String())
		writer.Write([]byte(display))
	}
}

func HandleMessages(startTime time.Time, reader io.Reader, writer io.Writer, config *jsonconfig.Config) {

	bufferedReader := bufio.NewReader(reader)

	channels := make([]chan rtcm.Message, 0)

	messageChan := make(chan rtcm.Message)
	go writeRTCMMessages(messageChan, writer)
	channels = append(channels, messageChan)

	if config.DisplayMessages {
		displayLogWriter :=
			dailylogger.New(config.MessageLogDirectory, "rtcm.", ".txt")
		displayChan := make(chan rtcm.Message)
		go writeReadableMessages(displayChan, displayLogWriter)
		channels = append(channels, displayChan)
	}
	if config.RecordMessages {
		messageLogWriter := dailylogger.New(config.MessageLogDirectory, "rtcmfilter.", ".rtcm")
		rtcmChan := make(chan rtcm.Message)
		go writeRTCMMessages(rtcmChan, messageLogWriter)
		channels = append(channels, rtcmChan)
	}

	appCore := AppCore.New(config, channels)
	appCore.HandleMessagesUntilEOF(startTime, bufferedReader)

	// We only get to here if the handler stops.
	close(messageChan)
}
