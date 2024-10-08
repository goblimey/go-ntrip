package filehandler

import (
	"bufio"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/goblimey/go-ntrip/jsonconfig"
	rtcm "github.com/goblimey/go-ntrip/rtcm/handler"
)

// Handler provides code to handle a text file containing RTCM3 messages, possibly
// interspersed with messages of other formats.  It's assumed that the file is no
// longer being written to.  (To handle a file which is being written to, such as
// a USB connection fed by an NTRIP source, see the serial line handler.)
type Handler struct {
	RTCMHandler        *rtcm.Handler     // Handles RTCM3 messages ...
	MessageChan        chan rtcm.Message // ... and issues them on this channel.
	RetryIntervalOnEOF time.Duration     // The time to wait between retries on EOF.
	EOFTimeout         time.Duration     // Give up retrying after this time has elapsed.
	Config             *jsonconfig.Config
}

// New creates a handler.
func New(messageChan chan rtcm.Message, config *jsonconfig.Config) *Handler {

	handler := Handler{
		MessageChan: messageChan,
		Config:      config,
	}
	return &handler
}

// Handle reads the file and sends the contents to an RTCM handler which extracts
// RTCM messages and sends them to the message channel. If there is a read error
// (typically EOF), it's returned.
func (handler *Handler) Handle(startTime time.Time, reader *bufio.Reader) error {

	// An EOF on a read is not necessarily fatal.  It can just mean that there
	// is no data to read just now, but there may be some in the future.  If the
	// EOFTimeout is nil, we return immediately on EOF.  If it's set, then we
	// retry reads for that duration and then return the error if the timeout
	// elapses.  On any other read error we stop immediately.
	//
	// If the reader is connected to a file that's not being written, the caller
	// should supply a nil timeout value.  Handle will then process the file and
	// die.
	//
	// If the reader is connected to a serial line fed by a live NTRIP source,
	// the bytes should come in indefinitely, for example a burst of messages
	// every second, taking a fraction of a second for each burst to arrive,
	// followed by silence for the rest of the second.  If the timeout is set to
	// a small number of seconds then it will only expire if the host machine
	// loses its connection to the device, so the handler may run for days or
	// weeks.  When a read timeout does expire, the handler closes its message
	// channel and returns.  (If the handler is called in a goroutine, closing
	// the message channel signals to the caller that it's stopped.)  The caller
	// should attempt to reopen the connection to the device, create a new
	// handler and continue.
	//
	// If the timeout is too short then we may return too early, part way
	// through reading an RTCM message.  That could result in data being lost
	// or a message being broken into parts, each part becoming a non-RTCM
	// message on the message channel.

	// timeOfFirstEOF is set when the read has returned EOF one or more times
	// in a row.  It's the time that we saw the first of a stream of EOFs.
	// If the last read was successful, the value is left as nil.
	//
	var timeOfFirstEOF *time.Time

	byteChan := make(chan byte)
	// Ensure that the byte channel is closed on return.
	defer close(byteChan)

	// Set up an RTCM handler connected to the input and output channels
	// and start it running.
	handler.RTCMHandler = rtcm.New(startTime, slog.LevelDebug)
	go handler.RTCMHandler.HandleMessages(byteChan, handler.MessageChan)

	// Read the file and send the data to the byte channel.
	for {
		buf := make([]byte, 1)
		n, err := reader.Read(buf)
		if err != nil {
			// Error of some kind, probably EOF or i/o timeout.  If the latter, the
			// error message is something like: "read /dev/ttyUSB0: i/o timeout"
			if err != io.EOF && !strings.Contains(err.Error(), "i/o timeout") {
				// Some other kind of file handling error.  (This is difficult
				// to provoke during testing without using a mock.)
				if handler.Config.SystemLog != nil {
					handler.Config.SystemLog.Printf("%v", err)
				}
				return err
			}

			// EOF.
			if handler.Config.TimeoutOnEOF() == 0 {
				// No timeout so don't retry.
				if handler.Config.SystemLog != nil {
					handler.Config.SystemLog.Printf("%v\n", err)
				}
				return err
			}

			// EOF may really mean end of file or just that there
			// is currently no data to read.
			// Retry until the timeout elapses and then return.
			if timeOfFirstEOF == nil {
				// The last read was successful, this one produced EOF.
				// Set up the timeout, pause and try again.
				t := time.Now()
				timeOfFirstEOF = &t
				if handler.Config.WaitTimeOnEOF() != 0 {
					time.Sleep(handler.Config.WaitTimeOnEOF())
				}
				if handler.Config.SystemLog != nil {
					handler.Config.SystemLog.Printf("ignoring %v\n", err)
				}
				continue
			}

			// If we get to here, we've seen EOF this time and last time too.
			// Has the timeout elapsed?
			now := time.Now()
			if now.Sub(*timeOfFirstEOF) > handler.Config.TimeoutOnEOF() {
				// The timeout has elapsed.  Give up.
				if handler.Config.SystemLog != nil {
					handler.Config.SystemLog.Printf("giving up on %v\n", err)
				}
				return err
			}

			// The timeout has not elapsed yet.  Pause and try again.
			// em := fmt.Sprintf("sleeping for %d\n", handler.RetryIntervalOnEOF)
			// os.Stderr.Write([]byte(em))
			if handler.Config.TimeoutOnEOF() != 0 {
				time.Sleep(handler.Config.TimeoutOnEOF())
			}
		}

		if n > 0 {
			// We have read a byte.  Reset the timeout mechanism and send the
			// byte to the channel.
			timeOfFirstEOF = nil
			// em := "read one byte\n"
			byteChan <- buf[0]
		}
	}
}
