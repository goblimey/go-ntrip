// This is the core of a number of applications.  It contains functionality
// to read from an input file (typically either a text file or a serial line
// connected to a device which is sending NTRIP messages.
package appcore

import (
	"bufio"
	"time"

	fileHandler "github.com/goblimey/go-ntrip/file_handler"
	"github.com/goblimey/go-ntrip/jsonconfig"
	rtcm "github.com/goblimey/go-ntrip/rtcm/handler"
	"github.com/goblimey/go-ntrip/rtcm/utils"
)

type AppCore struct {
	Conf     *jsonconfig.Config
	Channels []chan rtcm.Message
}

func New(conf *jsonconfig.Config, channels []chan rtcm.Message) *AppCore {
	appCore := AppCore{Conf: conf, Channels: channels}
	return &appCore
}

// HandleMessages repeatedly searches for and reads the input file(s)
// specified in the config, converts the data to RTCM messages and sends them
// to the message channel.  If input is provided indefinitely, it will run
// until the calling application is shut down.
//
// It's assumed that the input files are the device names of a device that
// is sending data on a serial connection and will do so indefinitely.  If
// the device is connecting on a serial USB connection and connectivity is
// lost and then restored, the device name this time will be different from
// the one use last time.  The config should specify all the possible device
// file names.  If the device is connected using an RS/232 serial line then
// the config just needs to specify one name, the name of that device.
func (appCore *AppCore) HandleMessages() {

	// Loop forever:  find and consume input files, read the data from them,
	// convert them to messages and send the messages to the given channel.
	// When a data file is exhausted (which may or may not happen), search
	// for the next one and open it.
	//
	// This setup copes well with a GNSS device that occasionally drops out
	// of service and then comes back.  The function simply waits until
	// messages start arriving again.  However, if the GNSS device fails hard,
	// this could hang and require human intervention to stop it.
	for {
		// Find the input file and get a buffered reader.
		r := appCore.Conf.WaitAndConnectToInput()
		reader := bufio.NewReader(r)

		continueFlag := appCore.HandleMessagesUntilEOF(reader)

		if continueFlag == 1 {
			// Stop processing.  This is to allow a unit test to run this function.
			// It should never happen in production.
			break
		}
	}
}

// HandleMessagesUntilEOF takes the given reader, creates a file handler and
// runs it.
//
// Whenever it receives a message from the handler, it sends a copy to each
// of the AppCore's channels.  It's assumed that something is listening to
// each channel and doing something with the messages, for example writing
// them to a log file.
//
// In production the value returned is always 0 (continue).  In test the
// returned value may be 1 (stop).  If a caller that would normally run
// indefinitely receives a stop return, it should stop.  This is to allow
// unit testing of processes that would normally never terminate.
func (appCore *AppCore) HandleMessagesUntilEOF(reader *bufio.Reader) int {

	// Create a message channel.
	messageChan := make(chan rtcm.Message)

	// Create a file handler.
	waitTimeOnEOF := appCore.Conf.WaitTimeOnEOF()
	timeoutOnEOF := appCore.Conf.TimeoutOnEOF()
	fh := fileHandler.New(messageChan, waitTimeOnEOF, timeoutOnEOF)

	// Start the file handler and feed the input into it.  Messages will come out
	// of the message channel.  The file handler will die when it
	// encounters EOF and then the EOF timeout expires.  On the way out it will
	// close the message channel, which is how we know that it's finished.
	//
	// Significant time may have elapsed in WaitAndConnectToInput, maybe
	// days so the time we got on the previous trip may be stale.  Reset the
	// handler's time and therefore the meaning of any MSM timestamps.
	go fh.Handle(time.Now(), reader)

	// Fetch the messages and send them to the processing channels.
	for {
		message, more := <-messageChan
		if !more {
			break
		}

		if message.MessageType == utils.MessageTypeStop {
			// We've received the stop message (which should only
			// happen in testing).  Tell the caller to stop.
			return 1
		}
		for i := range appCore.Channels {
			if appCore.Channels[i] != nil {
				appCore.Channels[i] <- message
			}
		}
	}

	return 0
}
