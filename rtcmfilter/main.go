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
// A typical configuration is a GNSS receiver such as a Sparkfun
// RTK Express device producing RTCM and other messages, connected
// via I2C or serial USB to a host computer running this filter.
// The host could be a Windows machine but something as cheap as
// a Raspberry Pi single board computer is quite adequate:
//
//  -------------          messages          --------------
// | RTK Express | -----------------------> | Raspberry Pi |
//  -------------    IRC or serial USB       --------------
//
// With a bit more free software that can be used to create an NTRIP
//  base station:
//
//
//   messages    ------------   RTCM    --------------   RTCM over NTRIP
// -----------> | rtcmfilter | ------> | NTRIP server | ----------------->
//               ------------   stdout  --------------      Internet
//
// The base station sends RTCM corrections over the Internet using the
// NTRIP protocol to an NTRIP caster and on to moving GNSS rovers:
//
//                                               ---------> rover
//  ------------                     --------   /
// | NTRIP base |  RTCM over NTRIP  | NTRIP  | /
// | station    | ----------------> | caster | \
//  ------------                     --------   \
//                                               ----------> rover
//
// It's convenient to configure the GNSS device to send out messages
// in all sorts of formats, but sending them on to a caster is a waste
// of Internet bandwidth.  Also some casters and rovers only expect to
// receive RTCM messages.
//
// RTCM is a binary format.  Tools exist to convert it to another
// format called RINEX which is commonly used for Precise Point
// Positioning (ppp).  The program can write a verbatim copy of the
// valid messages to a daily log file which can be converted into
// RINEX format and analysed.  It can also produce a separate log
// file containing the messages in a readable form, which is useful
// for fault finding when setting up equipment.  (That latter format
// is very verbose, so you shouldn't leave the filter running in
// that mode for too long).  Both log files have datestamped names
// and they roll over at the end of each day.
//
// The behaviour is controlled by a JSON file "ntrip.json" in the
// current directory.  For example:
//
//
// The filter can be run on a Linux machine Raspberry Pi single-board computer
// connected to a GNSS device
// in a configuration like this:
//

// If the two boards lose contact briefly (for example because the
// GNSS device has lost power) the file connection may break and need
// to be re-opened.  In the case of a USB connection, that process is a
// little complicated.  A Windows machine uses device names com1, com2
// etc to represent the connection.  An Ubuntu Linux machine uses device
// names /dev/ttyACM0, /dev/ttyACM1 etc for serial USB connections.
// HOWEVER neither system uses one device name per  physical port.  The
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
// A GNSS base station should be configured to send a batch of
// messages every second,so there should only be a short delay between
// each batch.  If there is, then the host machine running the filter
// has probably lost contact with the GNSS device.  The filter should
// close the input channel, reopen it and continue.
//
// The filter needs a start time to make sense of the data (see
// below for why).  The first argument is optional and specifies
// the start time, if supplied.  The format is "yyyy-mm-dd",
// meaning midnight UTC at the start of that day, or RFC 3339
// format, for example "2020-11-13T09:10:11Z", which is a date and
// time in UTC, or "2020-11-13T09:10:11+01:00" which is a date and
// time in a timezone one hour ahead of UTC.
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
// The filter needs a start time because MSM7 messages contain a
// timestamp, in most cases milliseconds from the constellation's
// epoch, which rolls over every week.  (The exception is GLONASS
// which uses a two-part timestamp containing a day of the week and
// a millisecond offset from the start of day.)  The filter displays
// all these timestamps as times in UTC, so given a stream of
// observations advancing in time, it needs to know which week the
// first ones are in.

// The timestamps for different constellatons roll over at different
// times.  For example, the GPS timestamp rolls over to zero a few
// seconds after midnight UTC at the start of Sunday.  The GLONASS
// timestamp rolls over to day zero, millisecond zero at midnight at
// the start of Sunday in the Moscow timezone, which is 21:00 on
// Saturday in UTC.  So, if the filter is processing a stream of
// messages which started at 20:45 on a Saturday in UTC, the GLONASS
// timestamp value will be quite large.  At 21:00 the epoch rolls
// over and the timestamps start again at (zero, zero).  Meanwhile
// the GPS timestamps will also be large and they will roll over to
// zero a few seconds after the next midnight UTC.
//
// The filter can keep track of this as long as (a) it knows the time
// of the first observation, and (b) there are no large gaps in the
// observations.  If there was a gap, how long was it and has it taken
// us into a different epoch?
//
// All of the timestamps roll over at the weekend, so if the filter is
// started on a weekday, it just needs a start time in same week as the
// first observation.  If it's started close to any rollover, it needs a
// more accurate start time.
//
// If the filter is run without supplying a start time, it assumes
// that the data is coming from a live source and uses the system time
// when it starts up.  So the system clock needs to be correct.  For
// example, if you start the filter near midnight at the start of Sunday
// UTC and your system clock is out by a few seconds, the filter might
// assume the wrong GPS week.
//
// An rtcm message (strictly a message frame) is a stream of
// bits starting with 0xd3.  That's the start of a 24-bit (3 byte)
// header.  Bit 0 is the top bit of the first byte, so the 0xd3 is
// bits 0 to 7 of the bitstream.  Bits 8-13 are always zero, bits
// 14-23 are a 10-bit unsigned value giving the length in bytes
// of the embedded message.  That message comes next, followed by
// a 24-bit Cyclic Redundancy Check (CRC) value:
//
//     < message frame  >
//     header message CRC
//
// The CRC is created using an algorithm published by Qalcomm.  The
// integrity of the message can be checked by taking the frame up to
// but not including the CRC, calculating its CRC value and comparing
// that with the given value.
//
// The incoming stream of data can be a mixture of RTCM and other
// messages.  It's assumed to come from a GNSS device which is
// issuing messages continuously on some noisy channel. (In my
// case it's a Ublox ZED-FP9 sending data on a serial USB or IRC
// connection.  These are both prone to dropping or scrambling
// the occasional character.)  When the filter starts up, it picks
// up the messages at some arbitrary point and scans for a 0xd3
// byte.  This may or may not be the start of a valid message.
// It reads the next two bytes, assumes that it's the header,
// gets the length and reads the rest of the message.  It checks
// the CRC.  That check may fail if the 0xd3 byte was not the
// start of a message or if the message was scrambled in transit.
//
// The embedded rtcm message is binary so it may contain 0xd3
// values by simple coincidence.  If you read this byte, you
// can't be certain that it's the start of a message frame.  You
// need to read the next two bytes to get what you assume is the
// header, check that bits 8-13 are zero, get the message length,
// read the whole message frame and check the CRC.
//
// If the message is valid, it's written to standard output and
// the filter scans for the next one.  Intervening text and message
// frames that fail the CRC check are discarded.
//
// The first 12 bits of the embedded message (bits 24-35 of the
// message frame) contain the message number (message type) a
// value in the range 0 - 4095.  Each message type starts with the
// type number in that position but apart from that, they all have
// different formats.  For the purposes of finding an accurate
// position, only a few type matter: 1005, which gives the position
// of the base station, and a Multiple Signal Message type 7 (MSM7)
// for each satellite constellation:
//
// Type 1005: base station position
// type 1077: high-resolution GPS signals
// type 1087: high-resolution Glonass signals
// type 1097: high resolution Galileo signals
// tupe 1107: high resolution SBAS signals
// type 1117: high resolution QZSS signals
// type 1127: high resolution Beidou signals
// type 1137: high resolution NavIC/IRNSS signals
//

//

//
// My device scans the satellites once every second for signals
// and issues one MSM7 message for each constellation.  It also
// sends a type 1005 (base position) message every five seconds.
// Here in the UK it can see four constellations, GPS, Glonass,
// Galileo and Beidou.  An MSM7 message lists the signals sent
// by each satellite that's in view.  My receiver is a dual-band
// device, so I get up to 2 signals from each satellite.  The result
// is a batch of four MSM7 messages every second, each 500 to 1,000
// bytes long, plus a type 1005 message very five seconds.
//
// I can only test the handling of messages that my device can see,
// so I've only tested types 1005, 1077, 1087, 1097 and 1227.
// In particular, I can't test the handling of timestamps in other
// MSM7 messages.
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
	var channels []chan rtcm.Message
	stdoutChannel := make(chan rtcm.Message, channelCap)
	defer close(stdoutChannel)
	channels = append(channels, stdoutChannel)
	go writeMessagesToStdout(stdoutChannel)

	// Make a log of all messages.
	verbatimChannel := make(chan rtcm.Message, channelCap)
	defer close(verbatimChannel)
	channels = append(channels, verbatimChannel)
	verbatimLog := dailylogger.New(logDir, "verbatim.", ".log")
	go writeAllMessages(verbatimChannel, verbatimLog)

	if config.RecordMessages {
		// Make a log of the RTCM messages.
		rtcmChannel := make(chan rtcm.Message, channelCap)
		defer close(rtcmChannel)
		channels = append(channels, rtcmChannel)
		rtcmLog := dailylogger.New(logDir, "data.", ".rtcm")
		go writeRTCMMessages(rtcmChannel, rtcmLog)
	}

	if config.DisplayMessages {
		// Make a log of the RTCM messages in readable form
		// (which is VERY verbose).
		displayChannel := make(chan rtcm.Message, channelCap)
		defer close(displayChannel)
		channels = append(channels, displayChannel)
		displayLog := dailylogger.New(logDir, "rtcmfilter.display.", ".log")
		go writeReadableMessages(displayChannel, rtcmHandler, displayLog)
	}

	logger.Printf("processMessages: %d channels", len(channels))

	handleMessages(rtcmHandler, reader, channels)
}

func handleMessages(rtcmHandler *rtcm.RTCM, reader io.Reader, channels []chan rtcm.Message) {

	// Read and process messages until the connection dies.
	rtcmHandler.HandleMessages(reader, channels)
}

// writeMessagesToStdout receives the messages from the channel and writes them
// to the given writer.  If the channel is closed or there is an error while
// writing, it terminates.  It can be run in a go routine.
func writeMessagesToStdout(ch chan rtcm.Message) {
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
func writeAllMessages(ch chan rtcm.Message, writer io.Writer) {
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
func writeRTCMMessages(ch chan rtcm.Message, writer io.Writer) {
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
func writeReadableMessages(ch chan rtcm.Message, rtcmHandler *rtcm.RTCM, writer io.Writer) {

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
