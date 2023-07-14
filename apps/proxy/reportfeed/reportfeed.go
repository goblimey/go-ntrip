package reportfeed

import (
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	circularQueue "github.com/goblimey/go-ntrip/apps/proxy/circular_queue"
	"github.com/goblimey/go-tools/dailylogger"
	"github.com/goblimey/go-tools/statusreporter"
)

// Buffer contains an input or output buffer.
type Buffer struct {
	Timestamp     time.Time
	Source        uint64
	Content       *[]byte
	ContentLength int
}

// ReportFeed satisfies the status-reporter ReportFeedT interface.
type ReportFeed struct {
	logger           *dailylogger.Writer
	lastClientBuffer *Buffer
	lastServerBuffer *Buffer

	// RecentMessages contains the messages recently sent.
	RecentMessages *circularQueue.CircularQueue

	*sync.Mutex
}

// This is a compile-time check that ReportFeed implements the statusreporter.ReportFeedT interface.
var _ statusreporter.ReportFeedT = (*ReportFeed)(nil)

// New creates and returns a new ReportFeed object.
// The ReportFeed object contains a pointer to a mutex so always use this
// method to create one.
func New(lgr *dailylogger.Writer, queue *circularQueue.CircularQueue) *ReportFeed {
	var mu sync.Mutex
	reportFeed := ReportFeed{logger: lgr, RecentMessages: queue, Mutex: &mu}

	return &reportFeed
}

// SetLogLevel satisfies the ReportFeedT interface.
func (rf *ReportFeed) SetLogLevel(level uint8) {
	if level == 0 {
		rf.logger.DisableLogging()
	} else {
		rf.logger.EnableLogging()
	}
}

// Status satisfies the ReportFeedT interface.
func (rf *ReportFeed) Status() []byte {
	clientLeader := "no input buffer"
	clientHexDump := ""
	serverLeader := "no output buffer"
	serverHexDump := ""
	rf.Lock()
	defer rf.Unlock()
	if rf.lastClientBuffer != nil && rf.lastClientBuffer.Content != nil {
		fmt.Fprintf(os.Stderr, "client buffer")
		clientLeader = fmt.Sprintf("From Client [%d]:\n%s\n", rf.lastClientBuffer.Source,
			rf.lastClientBuffer.Timestamp.Format("Mon Jan _2 15:04:05 2006"))

		clientHexDump =
			Sanitise(hex.Dump((*rf.lastClientBuffer.Content)[:rf.lastClientBuffer.ContentLength]))
	}
	if rf.lastServerBuffer != nil && rf.lastServerBuffer.Content != nil {
		fmt.Fprintf(os.Stderr, "server buffer")
		serverLeader = fmt.Sprintf("To Server [%d]:\n%s\n", rf.lastServerBuffer.Source,
			rf.lastServerBuffer.Timestamp.Format("Mon Jan _2 15:04:05 2006"))
		serverHexDump =
			Sanitise(hex.Dump((*rf.lastServerBuffer.Content)[:rf.lastServerBuffer.ContentLength]))
	}

	// Get the recent messages from the queue.  (Note that the
	// queue only contains whole messages.  If the client buffer
	// starts or ends with part of a message, they will be
	// displayed as non-RTCM messages.)
	messageDisplay := "\nMessages\n\n"
	for _, message := range rf.RecentMessages.GetMessages() {
		messageDisplay += message.String() + "\n"
	}

	reportBody := fmt.Sprintf(reportFormat,
		clientLeader,
		clientHexDump,
		serverLeader,
		serverHexDump,
		messageDisplay,
	)

	return []byte(reportBody)
}

// SetLogger sets the logger.
func (rf *ReportFeed) SetLogger(logger *dailylogger.Writer) {
	rf.logger = logger
}

// RecordClientBuffer takes a timestamped copy of a client buffer.
func (rf *ReportFeed) RecordClientBuffer(buffer *[]byte, source uint64, length int) {
	rf.Lock()
	defer rf.Unlock()
	rf.lastClientBuffer = &Buffer{time.Now(), uint64(source), buffer, length}
}

// RecordServerBuffer takes a timestamped copy of a server buffer.
func (rf *ReportFeed) RecordServerBuffer(buffer *[]byte, source uint64, length int) {
	rf.Lock()
	defer rf.Unlock()
	rf.lastServerBuffer = &Buffer{time.Now(), uint64(source), buffer, length}
}

// Sanitise edits a string, replacing some dangerous HTML characters.
func Sanitise(s string) string {
	s = strings.Replace(s, "<", "&lt;", -1)
	s = strings.Replace(s, ">", "&gt;", -1)
	return s
}
