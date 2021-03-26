// go-ntrip reads bytes from stdin, filters out anything that's not
// rtcm and writes the remaining RTCM to stdout.  It writes a
// readable form to stderr, which requires a reference date.  This
// is because some RTCM messages contain a rolling time value
// (milliseconds since midnight at the start of Sunday in some time
// zone) so it needs to know the date of the first timestamped
// message.
//
// The program takes one argument which should be in the format
// yyyy-mm-dd.  The RTCM input should begin on that day in UTC.
//
package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/goblimey/go-ntrip/rtcm"
)

func main() {

	var startDate time.Time
	var err error
	if len(os.Args) > 1 {
		// The format of arg[1] should be "yyyy-mm-dd".
		const dateLayout = "2006-01-02"
		startDate, err = time.Parse(dateLayout, os.Args[1])
		if err != nil {
			log.Fatal("usage: go-ntrip yyyy-mm-dd")
		}
	}

	messageCount := make(map[uint]uint)

	rtcm := rtcm.New(startDate)
	for {
		message, err := rtcm.ReadNextMessage(os.Stdin)
		if err != nil {
			break
		}

		messageCount[message.MessageType]++
	}

	for c := range messageCount {
		fmt.Printf("message type %4d: %6d\n", c, messageCount[c])
	}
}
