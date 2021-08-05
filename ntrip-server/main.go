// THIS SOFTWARE IS WORK IN PROGRESS AND NOT READY FOR USE.
//
// The Networked Transport of RTCM via Internet Protocol (NTRIP) carries data
// about observations of GNSS satellites from a fixed base station to one or more
// client devices, often known as rovers (because they can move).  A rover takes
// its own observations of satellites and combines them with the base station's
// observations of the same satellites to find its location more accurately.
// Witout assistance a typical rover can find its position to within about 2m.
// With suitable equipment, a rover within 10 Km of a base station and connected
// to the Internet can find its location to within 2cm.  GNSS rovers are used for all
// sorts of purposes including mapping, control of drones and accurate navigation for
// ships and land vehicles.
//
// (GNSS stands for Global Navigation Satellite System, the first of which was the 
// Global Positioning System (GPS).  There are now four constellations of satellites
// providing worldwide GNSS services - the American GPS, the Russian Glonass, the
// European Galileo and the Chinese Beidou.  There are some other services that 
// only work across limited regions.)
//
// The data from a base station is made available to client rovers in a
// publisher/subscriber relationship via an intermediate NTRIP caster (broadcaster),
// a piece of software running on a computer on the Internet. The caster offers a
// set of named mountpoints, one per base station.  The base station constantly scans
// the signals from the GNSS satellites that it can see and posts them to a mountpoint
// on the caster.  A rover connects to the mountpoint representing the base station
// which is nearest to it and gets the base station's observations.
//
// The advantage of having a caster in between the server and the clients is that
// only the caster needs a fixed published IP address and a DNS name.  The server can
// be behind an ordinary domestic broadband service and the clients can connect via a
// WiFi connection to an Internet-enabled mobile phone or a mobile modem.
//
// The base station is composed of a GNSS receiver and a host computer running some
// NTRIP server software.  In my case, the receiver is a UBLOX-F9P in a circuit board
// from Sparkfun and the host computer is a Raspberry Pi running this software.  
//
//     -----------------                          ----------------
//     | Ublox ZED-F9P | -----------------------> | Raspberry Pi |
//     -----------------    IRC or serial USB     ----------------
//
// This equipment runs in my garden shed with an antenna on the shed roof.  The 
// Raspberry Pi connects to the broadband modem in the house using a network connection 
// that runs over the mains electrical supply (Internet over Mains).  My caster is 
// currently some free software from the International GNSS Service (IGS) which I 
// have reworked to run under Docker: https://github.com/goblimey/ntripcaster.  I 
// run it on a Digital Ocean droplet.  My rover is an Emlid Reach M+.  All this 
// hardware costs about $1,000 and the running costs are less than $100 per year.
//
// My GPS device scans the satellites once every second for signals
// and issues one MSM7 message for each constellation.  It also
// sends a type 1005 (base position) message every five seconds.
// Here in the UK it can see four constellations, GPS, Glonass,
// Galileo and Beidou.  An MSM7 message lists the signals sent
// by each satellite that's in view.  My device is dual-band, so I get
// up to 2 signals from each satellite.  The result
// is a batch of four MSM7 messages every second, each 500 to 1,000
// bytes long, plus a type 1005 message every five seconds. The total
// is about 2,000 to 4,000 bytes per second, well within the capacity
// of a home broadband system.
//
// I can only test the handling of messages that my device can see,
// so I've only tested types 1005, 1077, 1087, 1097 and 1227.
// In particular, I can't test the handling of timestamps in other
// MSM7 messages.
//
// The NTRIP Server
//
// The NTRIP server reads bytes from its input channel, filters out
// anything that's not an rtcm message and sends the remaining data
// to an NTRIP caster using the NTRIP protocol.  It's meant to be
// used to clean up a stream of data from a device such as a .
// That device can be configured to send NTRIP and other messages.  Also
// the communication may be done via a lossy serial line, so there
// may be some corrupt NTRIP messages.  Using this server ensures that
// the caster only rceives valid RTCM messages.
//
// The server can produce a log containing a verbatim copy of the NTRIP
// message stream and/or another log containing a readable version of
// them. The verbatim logs can be converted to RINEX format and sent off
// for post-processing to produce data such as a more accurate location
// for the server's antenna.
//
// Configuration
//
// The Raspberry Pi has four USB ports and four device files /dev/ttyACM0,
// /dev/ttyACM1, and so on.  These only exist when a serial USB device is
// connected to a physical USB port.  When the GPS device is plugged into
// the Pi, the system creates one of those device files to handle the traffic.
// It can use any one. If the device disconnects and reconnects, the system 
// may choose a different device file.  So, if the server detects that the
// device has stopped sending, it needs to watch for one of those device files
// to appear, and then connect to it. 
//
// The server looks for a text file rtcmserver.json in the current directory
// when it starts up.  This contains a list of devices to try on startup and
// on reconnection.  If the control file doesn't exist, the server reads on
// stdin, but then there can be no recovery if the other end stops sending.
// (Reading on stdin is useful for integration testing)
//
// The configuration file also contains the connection details for the caster:
// the host name, the port (typically 2101), the mountpoint to connect to and
// the user name and password to use when connecting.
//
// Logging
//
// The server writes up to three log files:  a log of error messages, a verbatim
// copy of the RTCM3 messages that were sent to the caster and a human-readable
// breakdown of the messages.  The file names are datestamped and the logs files
// roll over each day.  
//
// Left to themselves these logs will eventually fill the host's file store,
// so you need to arrange some scheme for culling old files.  The human readable 
// breakdown is especially large.  You may wish to turn this logging on for a while 
// to make sure that the data is flowing correctly, and then turn it off.
//
// Displaying Messages
//
// NTRIP messages contain a numeric timestamp that rolls over
// every week, so to produce a readable log the server needs to know the
// approximate start time (within a week) to make sense of the data.  The
// optional -t argument to the program specifies the start time.
// The format can be "yyyy-mm-dd", meaning midnight UTC at the start of that
// day, or RFC 3339 format, for example "2020-11-13T09:10:11Z", which is a
// date and time in UTC, or "2020-11-13T09:10:11+01:00" which is a date
// and time with an offset from UTC (in that example, one hour ahead).
//
// Timezone formats using three letter timezone abbreviations such as
// "CET" are NOT supported.  This is because there is no common agreement
// on them - "CET" refers to different timezones in different parts of the
// world.
//
// If there is no -d argument, the server uses the current time.
//
// In most cases the timestamp in the messages is the number of milliseconds since 
// the constellation's epoch.  The epoch starts each weekend, when the timestamps
// roll over to zero and start again.  The exception is the Russian GLONASS 
// constellation.  The GLONASS timestamp is composed of a day of the week and a 
// millisecond offset from the start of that day.
//
// The timestamps for different constellatons roll over at different times.  For
// example, the GPS timestamp rolls over to zero a few
// seconds after midnight UTC at the start of Sunday.  The GLONASS
// timestamp rolls over to day zero, millisecond zero at midnight at
// the start of Sunday in the Moscow timezone, which is 21:00 on
// Saturday in UTC.  So, if the server is processing a stream of
// messages which started at 20:45 on a Saturday in UTC, the GLONASS
// timestamp value will be quite large, and about to roll over.  At 21:00
// UTC the GLONASS epoch rolls over and its timestamps start again at
// (zero, zero).  Meanwhile the GPS timestamps will also be large and
// they will roll over to zero a few seconds after midnight UTC.
//
// If the server is logging a readable version of the messages, it converts
// all these timestamps to times in UTC, so given a stream of observations
// advancing in time, it needs to know which week the first ones are in.  It
// can cope with gaps in the data stream, but not if they extend for more
// than a week.
//
// All of the timestamps roll over at the weekend, so if the server is
// started on a weekday, it just needs a start time in same week as the
// first observation.  If it's started close to any rollover, it needs a
// more accurate start time.
//
// If the server is run without supplying a start time, it uses the system time
// when it starts up, so the system clock needs to be correct.  For
// example, if you start the server near midnight at the start of Sunday
// UTC and your system clock is out by a few seconds, the server might
// assume the wrong week for GPS messages.
//
// Format of RTCM Messages
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
// where the message is:
//
//     {message type}{the rest of the message}
//
// Each message type has its own structure.
//
// The CRC is created using an algorithm published by Qalcomm.  The
// integrity of the message can be checked by taking the frame up to
// but not including the CRC, calculating its CRC value and comparing
// that with the given value.
//
// The incoming stream of data can be a mixture of RTCM and other
// messages.  It's assumed to come from a GNSS device which is
// issuing messages continuously on some noisy channel. (In my
// case I can connect the GNSS device to the Raspberry Pi using a
// USB or and IRC connection.  USB is fairly reliable but consumes a lot of power,
// which can cause the PI to restart.  IRC uses les power but is
// prone to dropping or scrambling the occasional character, thus
// corrupting the message and causing the CRC check to fail.)
//
// The embedded rtcm message is binary so it may contain 0xd3
// values by simple coincidence.  If you read this byte, you
// can't be certain that it's the start of a message frame.  You
// need to read the next two bytes to get what you assume is the
// header, check that bits 8-13 are zero, get the message length,
// read the whole message frame and check the CRC.
//
// When the server starts up, it picks up the message stream at some
// arbitrary point and scans, looking for a 0xd3 byte.  The first one it
// finds  may or may not be the start of a valid message.  It attempts
// to read the whole message and check the CRC.  That sequence may fail
// because (a) the 0xd3 byte was not the start of a message or (b) the
// message was scrambled in transit or (c) the GPS device has lost power
// and is resetting itself or (d) the GPS device has failed permanently
// for some reason.  The server attempts to recover from (a), (b) and (c).
// If it's (d), there is no recovery possible.
//
// If the message is valid, the server sends it to the caster and
// scans for the next message.  Intervening text and message frames that
// fail the CRC check are discarded silently.
//
// The caster is a separate process, probably running on a separate machine
// across the Internet.  If that stops responding, the server continues to
// run, trying to connect to the caster, waiting for it to come back online.
// It logs the problem, and the design attempts to avoid excessive logging.
// Sitting and waiting for somebody to fix the caster makes sense, especially
// if the server is producing logs that will be analysed later.
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
// A GNSS base station should be configured to send a batch of
// messages every second,so there should only be a short delay between
// each batch.  If there is, then the host machine running the server
// has probably lost contact with the GNSS device.  The server should
// close the input channel, reopen it and continue.
//

//
package main

import (
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"os"
	"time"

	"github.com/dolmen-go/contextio"
	"github.com/goblimey/go-ntrip/rtcm"
	"github.com/goblimey/go-tools/dailylogger"
)

// InputFiles contains a list of possible input files.
// this is useful, for example, when the input is a USB
// device and can have one of a number of names.
type InputFiles struct {
	Files []InputFile `json:"input"`
}

// InputFile contains just a filename, for example "/dev/ttyACM0"
type InputFile struct {
	Name string `json:"name"`
}

// lostInputConnectionTimeout defines the input timeout.
const lostInputConnectionTimeout = 3 * time.Second

// This control prevents excessive complaints when a device disconnects.
var reportConnectionError = true

// This is the time to sleep between connection attempts.
const lostConnectionSleepTime = 1 * time.Second

// controlFileName is the name of the JSON control file that defines
// the names of the potential input files.
const controlFileName = "./rtcmserver.json"

// verbatimLog contains a verbatim copy of the valid RTCM messages sent to the caster.
// It's rolled over every day.
//
var verbatimLog io.Writer

// systemLog is for errors, warnings etc.  It's rolled over every day.
//
var systemLog io.Writer

var startDateArg string

func init() {
	verbatimLog = dailylogger.New("", "rtcm3")
	systemLog = dailylogger.New("ntripserver", "log")

	startDateArg := flag.String("t", "", "start time - yyyy-mm-dd or yyyy-mm-ddThh:mm:ss+hh:mm")

	flag.Parse()
}

func main() {

	// If specified, the -t argument specifies the start date/time.
	var startdate time.Time
	var dateError error
	if len(startdateArg) > 1 {
		startDate, dateError = getTime(startdateArg)
		if dateError != nil {
			log.Fatal("usage: go-ntrip -t yyyy-mm-dd{:hh:mm:ss+hh:mm}")
		}
	} else {
		startDate = time.Now()
	}

	var input = os.Stdin
	var inputFiles *inputFiles
	jsonReader, err := os.Open(controlFileName)

	// Failure to open is expected behaviour.  It just means that
	// there is no JSON control file.
	
	if err == nil {
		// There is a JSON control file.  Read and unmarshall it.
		inputFiles = getJSONControl(jsonReader)
	}

	ctx, cancel := context.WithTimeout(
		context.Background(), lostInputConnectionTimeout)
	defer cancel()

	rtcmHandler := rtcm.New(startDate)

	processMessages(rtcmHandler, verbatimLog)
	
}

func processMessages(rtcm *rtcm.RTCM, writer io.Writer) {

	for {

		reader := waitAndConnectToInput(ctx, filenames.Files)

		// Read messages until the connection dies.
		processMessagesUntilEOF(rtcm, reader, writer)

		// A long time may have passed, so find the current time
		// and use that to drive the RTCM.
		rtcmHandler = rtcm.New(time.Now())
	}
}

func processMessagesUntilEOF(rtcm *rtcm.RTCM, reader io.Reader, writer io.Writer) {
	for {
		// Get the next message from the reader, discarding any intervening
		// non-RTCM3 material.

		message, err := rtcm.ReadNextMessage(reader)
		if err != nil {
			return
		}

		// Copy the RTCM message to the writer
		verbatimlog.Write(message.RawData)
	}
}

// findInputdevice searches the given list of InputFiles.If one of the named
// files exists and can be opened for reading, it returns a Reader connected
// to it.  The reader is a contextio.Reader, which responds to the timeout
// embedded in the supplied Context.
func findInputDevice(ctx context.Context, filenames []InputFile) io.Reader {
	// By default the client reads from stdin but that can be overridden
	// if a JSON control is present.
	//
	// This is meant to be used in a Raspberry Pi Ubuntu environment with
	// a GNSS device that connects using a serial USB connection.  In that
	// case, one of "/dev/ttyACM0", "/dev/ttyACM1", "/dev/ttyACM2" or
	// "/dev/ttyACM3" will exist, representing the serial connection to the
	// device.  The device is created on connection and removed on
	// disconnection, so if the GNSS device is the only serial device
	// connected, only one of those devices will exist.  The device names
	// DO NOT relate to the physical USB sockets on the circuit board. They
	// are used in turn. After the Pi boots, the first connection uses
	// "/dev/ttyACM0", but if the GNSS device loses power briefly and
	// disconnects, when it reconnects, it appears as "/dev/ttyACM1", and so
	// on.  So, whenever software running on the Pi needs to establish a
	// connection with the GNSS device on a USB port, it needs to do this
	// search.

	reader := contextio.NewReader(ctx, os.Stdin)

	if filenames != nil {
		for _, f := range filenames {
			file, err := os.Open(f.Name)
			if err == nil {
				// The file exists and is open.  Create a Reader with timeout
				// and return it.
				reader = contextio.NewReader(ctx, file)
			}
		}
	}

	return reader
}

// waitAndConnectToInput tries repeatedly (potentially indefinitely)
// to connect to one of the input files whose names are given.
func waitAndConnectToInput(ctx context.Context, filenames []InputFile) io.Reader {
	var result io.Reader 
	for {
		result = findInputDevice(ctx, filenames)
		if result == nil {
			if reportConnectionError {
				log.Printf(
					"waitAndConnect: failed to connect to GNSS source.  Retrying\n")
				reportConnectionError = false // Shut up from now on.
				time.Sleep(lostConnectionSleepTime)
			}
		}
	}

	// Success.
	reportConnectionError = true	// Report the next input connection failure.
		log.Printf("waitAndConnect: connected to GNSS source\n")
		reportConnectionError =  
	return file
}

func getJSONControl(jsonSource io.Reader) *InputFiles {

	// The JSON control file should contain a list of possible input files.
	jsonBytes, jsonReadError := ioutil.ReadAll(jsonSource)
	if jsonReadError != nil {
		// We can't read the control file - permissions?
		log.Fatalf("cannot read the JSON control file - %s", jsonReadError.Error())
	}

	var inputFiles InputFiles
	// Parse the JSON control file
	jsonParseError := json.Unmarshal(jsonBytes, &inputFiles)
	if jsonParseError != nil {
		log.Fatalf("cannot parse JSON control - %s", jsonParseError.Error())
	}

	return &inputFiles
}

// getTime gets a time from a string in one of three formats,
// yyyy-mm-dd{:hh:mm:ss{:timezone}}".  Timezones are listed
// here:  https://en.wikipedia.org/wiki/List_of_tz_database_time_zones.
// Note that three letter abbreviations such as "CET" are deprecated.
func getTime(timeStr string) (time.Time, error) {
	const dateLayout = "2006-01-02"

	if len(dateLayout) == len(timeStr) {
		dateTime, err := time.Parse(dateLayout, timeStr)
		return dateTime, err
	} else {
		dateTime, err := time.Parse(time.RFC3339, timeStr)
		return dateTime, err
	}
}
