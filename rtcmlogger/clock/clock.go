package clock

import (
	"time"
)

// Clock provides a clock service as an alternative to using the standard
// time package.  The intention is that testing and production code be
// 'plug compatible'.  This supports non-invasive testing of software that
// handles events triggered by the system clock.  In a real application the
// Now() method should yield the system time. In test it can yield suitable
// chosen values.
//
// Known types that respect this interface are:
// github.com/goblimey/go-ntrip/rtcmlogger/SystemClock
//     whose Now() method returns the system time.
// github.com/goblimey/go-ntrip/rtcmlogger/StoppedClock
//     whose Now() method always returns the same time, which can be set.
//
type Clock interface {
	Now() time.Time
}
