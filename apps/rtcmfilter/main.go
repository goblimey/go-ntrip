// The rtcmfilter repeatedly attempts to open the files given by the
// JSON configuration, read the contents, convert it to RTCM messages
// and send them to a set of processor functions.  It's designed to
// receive data from a device that emits messages continuously
// over a serial connection and runs until forcibly stopped.
//
// The incoming data is assumed to contain bursts of RTCM3 messages
// interspersed with other data such as NMEA sentences.  All
// data is presented as rtcm.Message objects, each with a message type.
// There si a special message type for non-RTCM3 data.
//
// Each processor function does different things such as: filter out
// any non-RTCM data and send the resulting pure RTCM3 stream to the
// standard output channel; make a verbatim log, a copy of all the
// data for each day; make a log displaying the binary RTCM3 data in
// a readable form.
//
// Any log files produced contain data collected in one day and the
// name of each file contains a datestamp.
//
// The filter can be used to clean up a stream of incoming data by
// filtering out the non-RTCM data and any RTCM messages that are
// corrupted in transit and sending only valid RTCM messages along a
// pipe to software such as an NTRIP client:
//
//	             RTCM and other data               RTCM data
//	GNSS device ---------------------> rtcmfilter -----------> NTRIP client
//	             serial connection                    pipe
//
// Another potential use is to capture the incoming RTCM messages, and
// create daily log files.  These can be converted into RINEX format
// and sent of for Precise Point Positioning (PPP) processing.
//
// When the application starts up it looks for a JSON config file
// ntrip.json in the current directory.  The config settings define
// which processor functions are run, so the results will be different
// depending on the config.  For example:
//
//	{
//	    "input": ["/dev/ttyACM0","/dev/ttyACM1", "/dev/ttyACM2", "/dev/ttyACM3"],
//	    "display_messages": true,
//	    "record_messages": true,
//	    "message_log_directory": "logs",
//		"read_timeout_milliseconds": 100,
//		"sleep_time_after_read_timeout_milliseconds": 100,
//		"wait_time_on_EOF_millis": 200,
//		"timeout_on_EOF_milliseconds": 3000
//	}
//
// The read timeout and sleep_time values relate to retries when attempting
// to open files.  The wait_time_on_EOF_millis and timeout_on_EOF_seconds
// values relate to the handling of end of file while reading.
//
// The input should be a device, usually a serial line connected to a
// GNSS device emitting NTRIP and other messages.  It may be a single
// device such as an RS/232 serial port or a list of devices as above.
// The second case covers devices that send on a serial USB connection.
//
// If the host computer and the GNSS source lose contact briefly (for
// example because the source has lost power) the file connection may
// break and need to be re-opened.
//
// In the connection is a serial USB channel, that process is a  little
// complicated.  A host running MS Windows uses device names com2, com8
// etc to represent the connection.  A Debian Linux machine uses device
// names /dev/ttyACM0, /dev/ttyACM1.  HOWEVER neither system uses one
// device name per physical port.  The device file is created when the
// GNSS device is plugged into one of the host's USB ports.  If the
// connection is lost later, the device file representing it disappears.
// If the connection is then restored on the same port, the system will
// use one of the other device names.
//
// During my testing on a Debian Linux system with four physical USB
// sockets, the system would create one of four devices to handle the
// traffic, /dev/ttyACM[0-4].  The first time I connected the GNSS
// device, the Linux system created /dev/ttyACM0 to handle the traffic.
// I disconnected the device and reconnected it on the same USB socket.
// The host created /dev/ttyACM1.  On each disconnection and
// reconnection, it cycled around the four device names.
//
// Similar experiments with a Windows host machine produced similar
// behaviour - the operating system cycled round a list of device
// names: com1, com8 and so on.  If you use a Windows host you will
// need to experiment to figure out your configuration.  Note also
// that plugging in another USB device later, such as a mouse, may
// disturb the sequence, so set up all your USB devices first and
// leave them that way.
//
// When I ran experiments with a Raspberry Pi as the host and a
// SparkFun GNSS board as the source I found that the connection did
// drop out fairly regularly.  At the time, the Sparkfun board drew
// its power from the Pi via the USB line and my guess was that the
// Pi couldn't always supply sufficient power.  That would cause the
// Sparkfun board to shut down briefly until the Pi restored the
// power.  Now I connect the two via a powered USB hub, which should
// make the setup more stable. Time will tell.
//
// If your host machine has other devices that use a serial USB
// connection it may be difficult or impossible to predict which
// devices to specify.  That's the kind of reason why I recommend
// using a Raspberry Pi as a host - it's cheap enough that you can
// dedicate it to handling your GPS device.
//
// When the input is a serial connection with a live GNSS device at
// the other end, a read operation on the host will receive End of
// File whenever it's read all the available bytes.  A little while
// later the device will write some more bytes and the next read will
// succeed.  When the filter receives an EoF, it just pauses for a
// (configurable) short time and tries again.
//
// If the device is disconnected, the read attempts will time out.
// The filter then runs through its list of devices, trying to connect
// to each in turn.
//
// A GNSS base station should be configured to send a batch of messages
// every second,so there should only be a delay of a fraction of a
// second between each batch.  If there is a longer delay, then the
// connection between the host machine running the filter and the GNSS
// device has probably died.  The filter will close the input channel
// and attempt to reopen it and continue. The timeout and retry values
// in the JSON control this behavior.  You should tune the values to
// suit your equipment.
//
// The incoming stream of data can be a mixture of RTCM and other
// messages.  It's assumed to come from a GNSS device which is
// issuing messages continuously on some noisy channel. (For
// example a Ublox ZED-FP9 sending data on a serial USB, IRC or
// RS/232 connection.  Some of these media are prone to dropping
// or scrambling the occasional character.)  Dropped characters
// will cause the message's CRC check to fail and the message
// will be deemed invalid.

package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"time"

	AppCore "github.com/goblimey/go-ntrip/apps/appcore"
	"github.com/goblimey/go-ntrip/jsonconfig"
	rtcm "github.com/goblimey/go-ntrip/rtcm/handler"
	"github.com/goblimey/go-ntrip/rtcm/utils"

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

	processMessages(config)
}

// processMessages creates a local message channel and runs HandleMessages as
// a goroutine.  They may both run until the source file is exhausted.  That
// may never happen, in which case they run until the program is forcibly shut
// down.  HandleMessages converts the input to a stream of messages on the
// channel.  If and when the input is exhausted it closes the channel and
// ProcessMessages returns.
//
// The function creates the goroutines that process incoming messages and then
// loops forever.   The loop receives messages from the message channel and
// passes them to the processing channels.  It's up to them what they do with
// the messages.
//
// This setup copes well with a GNSS device that occasionally drops out
// of service and then comes back.  The server simply waits until messages
// start arriving again.  However, if the GNSS device fails hard, the server
// will hang and human intervention is required to stop it.
//
// The function runs until it's forcibly shut down, connecting to the same set
// of files over and over, so it's not amenable to being run by a unit test.
func processMessages(config *jsonconfig.Config) {

	// messageChannelCap is the capacity of the message channels.
	const messageChannelCap = 1000

	// Log files are created in the directory given by the config
	// (default ".").
	logDir := "."
	if len(config.MessageLogDirectory) > 0 {
		logDir = config.MessageLogDirectory
	}

	logger = getDailyLogger()

	// Set up the output channels and the goroutines that consume them.  Ensure
	// that the channels are closed on our return.
	var channels []chan rtcm.Message
	stdoutChannel := make(chan rtcm.Message, messageChannelCap)
	defer close(stdoutChannel)
	channels = append(channels, stdoutChannel)
	go writeRTCMMessages(stdoutChannel, os.Stdout)

	// Make a log of all messages.
	verbatimChannel := make(chan rtcm.Message, messageChannelCap)
	defer close(verbatimChannel)
	channels = append(channels, verbatimChannel)
	verbatimLog := dailylogger.New(logDir, "verbatim.", ".log")
	go writeAllMessages(verbatimChannel, verbatimLog)

	if config.RecordMessages {
		// Make a log of the RTCM messages.
		rtcmChannel := make(chan rtcm.Message, messageChannelCap)
		defer close(rtcmChannel)
		channels = append(channels, rtcmChannel)
		rtcmLog := dailylogger.New(logDir, "data.", ".rtcm")
		go writeRTCMMessages(rtcmChannel, rtcmLog)
	}

	if config.DisplayMessages {
		// Make a log of the RTCM messages in readable form
		// (which is VERY verbose).
		displayChannel := make(chan rtcm.Message, messageChannelCap)
		defer close(displayChannel)
		channels = append(channels, displayChannel)
		displayLog := dailylogger.New(logDir, "rtcmfilter.display.", ".log")
		go writeReadableMessages(displayChannel, displayLog)
	}

	// Start a handler.  The resulting messages will emerge from the
	// message channel.
	appCore := AppCore.New(config, channels)
	go appCore.HandleMessages(time.Now())

	// HandleMessages runs until it's forcibly shut down. To keep the goroutines
	// running, loop forever.
	// for {
	// 	time.Sleep(time.Hour)
	// }
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

// writeReadableMessages receives the RTCM messages from the channel,
// decodes them to readable form and writes the result to the given log
// file. It terminates when the channel is closed or there is a write
// error.  It can be run in a go routine.
func writeReadableMessages(ch chan rtcm.Message, writer io.Writer) {

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

// getDailyLogger gets a daily log file which can be written to as a logger
// (each line decorated with filename, date, time, etc).
func getDailyLogger() *log.Logger {
	dailyLog := dailylogger.New("logs", "rtcmfilter.", ".log")
	logFlags := log.LstdFlags | log.Lshortfile | log.Lmicroseconds
	return log.New(dailyLog, "rtcmfilter", logFlags)
}
