// The rtcmfilter reads a stream of bytes from stdin, filters out
// anything that's not an rtcm message and writes the remaining
// data to stdout.   It uses github.com/goblimey/go-ntrip/rtcm to
// analyse the incoming messages.
//
// The filter is meant to be used to clean up a stream of
// incoming data that contains valid NTRIP messages interspersed
// with other data - corrupted NTRIP messages, messages in other
// formats and so on.  Only valid NTRIP messages are passed on to
// stdout.
//
// It can be convenient to configure the GNSS device to send out
// messages in all sorts of formats, but sending them all on to an
// NTRIP caster is a waste of Internet bandwidth.  Also some casters
// and rovers only expect to receive RTCM messages.  Sending
// anything else can cause problems.
//
// RTCM is a binary format.  Tools exist to convert it to another
// format called RINEX which can be used for Precise Point
// Positioning (ppp).  The program can write a verbatim copy of the
// valid messages to a daily log file which can be converted into
// RINEX format and analysed.  It can also produce a separate log
// file containing the messages in a readable form, which is useful
// for fault finding when setting up equipment.  (The readable format
// is very verbose, so you shouldn't leave the filter running in
// that mode for too long).  Both log files have datestamped names
// and they roll over at the end of each day.
//
// The behaviour is controlled by a JSON file "ntrip.json" in the
// current directory.  For example:
//
// {
//		"input": ["/dev/ttyACM0", "/dev/ttyACM1"],
//		"stop_on_eof": false,
//		"record_messages": true,
//		"message_log_directory": "someDirectory",
//		"display_messages": true,
//		"timeout": 1,
//		"sleeptime": 2
//	}
//
// If the two boards lose contact briefly (for example because the
// GNSS device has lost power) the file connection may break and need
// to be re-opened.  In the case of a USB connection, that process is a
// little complicated.  A Windows machine uses device names com1, com2
// etc to represent the connection.  An Ubuntu Linux machine uses device
// names /dev/ttyACM0, /dev/ttyACM1 etc for serial USB connections.
// HOWEVER neither system uses one device name per physical port.  The
// device is only created when the plug is inserted.  If the connection
// is lost, the device representing it disappears.  If the connection is
// restored later, the system may use one of the other device names.
// (During my testing on an Ubuntu system, the first connection used
// /dev/ttyACM0 until the device was disconnected, then /dev/ttyACM1 when
// it was reconnected on the same USB socket, and so on.
//
// The filter looks for a file rtcmfilter.json in the current
// directory when it starts up.  This contains a list of devices
// to try on startup and on reconnection.
//
// The StopOnEOF flag in the JSON controls the handling of an EOF
// condition.  If the input is a serial connection with a live GPS
// device on the other end, an EOF wll be received whenever all the
// available bytes have been read.  A little while later the device
// will write some more bytes and the next read by the filter will
// succeed.  StopOnEOF should be set false and the filter will run
// until it's forcibly shut down.  On the other hand, if the input
// is a text file on the disk, the filter should stop when it
// encounters the first EOF.
//
// A GNSS base station should be configured to send a batch of
// messages every second,so there should only be a short delay between
// each batch.  If there is, then the host machine running the filter
// has probably lost contact with the GNSS device.  The filter should
// close the input channel, reopen it and continue.  The timeout and
// retry values in the JSON control this behavior.  The device name
// may change each time this happens, so the filter scans through the
// list of input devices in the JSON.
//
// The filter needs a start time to make sense of the data (see
// this repository's README for why).  The first argument is optional
// and specifies the start time, if supplied.  The format is
// "yyyy-mm-dd", meaning midnight UTC at the start of that day, or
// RFC 3339 format, for example "2020-11-13T09:10:11Z", which is a
// date and time in UTC, or "2020-11-13T09:10:11+01:00" which is a
// date and time in a timezone one hour ahead of UTC.
//
// (Formats using three letter timezone abbreviations such as
// "CET" are NOT supported.  This is because there is no common
// agreement on them - "CET" refers to different timezones in
// different parts of the world.)
//
// If there is no argument, the filter uses the current time.
// Tis is sensible if it's receiving input from a live GNSS
// device rather than processing a file that was produced some
// time before.
//
// The incoming stream of data can be a mixture of RTCM and other
// messages.  It's assumed to come from a GNSS device which is
// issuing messages continuously on some noisy channel. (For
// example a Ublox ZED-FP9 sending data on a serial USB or IRC
// connection.  These media are both prone to dropping or scrambling
// the occasional character.)  Dropped characters will cause the
// message to fail its CRC check and be deemed invalid.
//
// If the message is valid, it's written to standard output and
// the filter scans for the next one.  Intervening text and message
// frames that fail the CRC check are discarded.
//
package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/goblimey/go-ntrip/jsonconfig"
	"github.com/goblimey/go-ntrip/rtcm"
	"github.com/goblimey/go-tools/dailylogger"
)

// controlFileName is the name of the JSON control file that defines
// the names of the potential input files.
const controlFileName = "./ntrip.json"

// logger writes to the daily event log.
var logger *log.Logger

func main() {

	logger := getDailyLogger()

	config, err := jsonconfig.GetJSONConfigFromFile(controlFileName, logger)

	if err != nil {
		// There is no JSON config file.  We can't continue.
		logger.Fatalf("cannot find config %s", controlFileName)
		os.Exit(1)
	}

	// If StopOnEOF is true, run process just once, which is
	// correct when the input is a plain file.  If it's false, the
	// loop runs forever, which is correct when the input is
	// something like a serial USB connection.
	if config.StopOnEOF {
		// The input should be a single plain file.  Process it and die.
		processMessages(config)
	} else {
		// The input should be one or more files that will produce an
		// endless stream of data.  An EOF indicates that there is nothing
		// to read right now, but there may be something later.
		// ProcessMessages will terminate if it receives any other error
		// and it will be run again to attempt to reconnect.
		for {
			processMessages(config)
		}
	}
}

// processMessages runs
// HandleMessages repeatedly until the server is forcibly shut down.  The
// gnss device connects via a serial USB channel.  When it's connected, the
// connection is represented at this end by one of four device files
// (/dev/ttyACM0 etc) specified in the config object.  The device can lose power
// briefly and then come back. This time the connection will be represented by
// another of the four device files.
//
// The function creates the goroutines that process incoming messages and then
// loops forever.   In the loop, waitAndConnectToInput scans the given device
// files, waiting for one to appear.   When that happens, the gnss device is
// powered up and sending messages.  Then HandleMessages reads the messages and
// processes them until they stop coming. That probably means that the device has
// lost power.  Hopefully that's temporary and the device will come back, but it
// will use one of the other four devices to represent the connection.  The next
// trip round the loop scans for that and then consumes the messages, ad infinitum.
//
// This setup copes well with a GNSS device that occasionally drops out
// of service and then comes back.  The server simply waits until messages
// start arriving again.  However, if the GNSS device fails hard, the server
// will hang and human intervention is required to stop it.
//
// The function runs until it's forcibly shut down, connecting to the same set
// of files over and over, so it's not amenable to being run by a unit test.
// The logic that can be unit tested is factored out into HandleMessages.
//
func processMessages(config *jsonconfig.Config) {

	// Log files are created in the directory given by the config
	// (default ".").
	logDir := "."
	if len(config.MessageLogDirectory) > 0 {
		logDir = config.MessageLogDirectory
	}

	logger = getDailyLogger()

	reader := config.WaitAndConnectToInput()

	rtcmHandler := rtcm.New(time.Now(), logger)
	if config.StopOnEOF {
		rtcmHandler.StopOnEOF = true
	}

	const channelCap = 100
	var channels []chan rtcm.RTCM3Message
	stdoutChannel := make(chan rtcm.RTCM3Message, channelCap)
	defer close(stdoutChannel)
	channels = append(channels, stdoutChannel)
	go writeMessagesToStdout(stdoutChannel)

	// Make a log of all messages.
	verbatimChannel := make(chan rtcm.RTCM3Message, channelCap)
	defer close(verbatimChannel)
	channels = append(channels, verbatimChannel)
	verbatimLog := dailylogger.New(logDir, "verbatim.", ".log")
	go writeAllMessages(verbatimChannel, verbatimLog)

	if config.RecordMessages {
		// Make a log of the RTCM messages.
		rtcmChannel := make(chan rtcm.RTCM3Message, channelCap)
		defer close(rtcmChannel)
		channels = append(channels, rtcmChannel)
		rtcmLog := dailylogger.New(logDir, "data.", ".rtcm")
		go writeRTCMMessages(rtcmChannel, rtcmLog)
	}

	if config.DisplayMessages {
		// Make a log of the RTCM messages in readable form
		// (which is VERY verbose).
		displayChannel := make(chan rtcm.RTCM3Message, channelCap)
		defer close(displayChannel)
		channels = append(channels, displayChannel)
		displayLog := dailylogger.New(logDir, "rtcmfilter.display.", ".log")
		go writeReadableMessages(displayChannel, rtcmHandler, displayLog)
	}

	logger.Printf("processMessages: %d channels", len(channels))

	handleMessages(rtcmHandler, reader, channels)
}

func handleMessages(rtcmHandler *rtcm.RTCM, reader io.Reader, channels []chan rtcm.RTCM3Message) {

	// Read and process messages until the connection dies.
	rtcmHandler.HandleMessages(reader, channels)
}

// writeMessagesToStdout receives the messages from the channel and writes them
// to the given writer.  If the channel is closed or there is an error while
// writing, it terminates.  It can be run in a go routine.
func writeMessagesToStdout(ch chan rtcm.RTCM3Message) {
	for {
		message, ok := <-ch
		if !ok {
			return
		}

		// We only want valid RTCM messages.
		if message.MessageType == rtcm.NonRTCMMessage {
			continue
		}

		n, err := os.Stdout.Write(message.RawData)
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
func writeAllMessages(ch chan rtcm.RTCM3Message, writer io.Writer) {
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

// writeRTCMMessages receives the messages from the channel and writes them
// to the given writer.  If the channel is closed or there is an error while
// writing, it terminates.  It can be run in a go routine.
func writeRTCMMessages(ch chan rtcm.RTCM3Message, writer io.Writer) {
	for {
		message, ok := <-ch
		if !ok {
			return
		}

		// We only want valid messages.
		if message.MessageType == rtcm.NonRTCMMessage {
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

// writeReadableMessages receives the RTCM messages from the channel,
// decodes them to readable form and writes the result to the given log file.
// If the channel is closed or there is a write error, it terminates.
// It can be run in a go routine.
func writeReadableMessages(ch chan rtcm.RTCM3Message, rtcmHandler *rtcm.RTCM, writer io.Writer) {

	for {
		message, ok := <-ch
		if !ok {
			return
		}
		// Decode the message.  (The result is very verbose!)
		display := fmt.Sprintf("%s\n", rtcmHandler.DisplayMessage(&message))
		writer.Write([]byte(display))
	}
}

// getDailyLogger gets a daily log file which can be written to as a logger
// (each lin decorated with filename, date, time, etc).
func getDailyLogger() *log.Logger {
	dailyLog := dailylogger.New("logs", "rtcmfilter.", ".log")
	logFlags := log.LstdFlags | log.Lshortfile | log.Lmicroseconds
	return log.New(dailyLog, "rtcmfilter", logFlags)
}
