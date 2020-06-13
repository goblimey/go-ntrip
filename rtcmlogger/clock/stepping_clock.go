package clock

import (
	"sync"
	"time"
)

// SteppingClock is a Clock that returns a given series of time values,
// one at a time.  It's ueful in a test case that make a series of calls to
// get the current time.
//
// NewSteppingClock takes a slice of time values.  Each call of Now()
// returns the next one of these.  Once Now() has returned all the values,
// any subsequent call returns the last value.
//
type SteppingClock struct {
	mutex    sync.Mutex
	nextTime int         // The next time to be returned.
	times    []time.Time // The list of times to be returned.
}

// This is a compile-time check that SteppingClock implements Clock.
var _ Clock = (*SteppingClock)(nil)

// NewSteppingClock creates a SteppingClock.
//
func NewSteppingClock(timeList *[]time.Time) Clock {
	return &SteppingClock{times: *timeList}
}

// SetTimes sets the array of times to return.
//
func (c *SteppingClock) SetTimes(time *[]time.Time) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.times = *time
}

// Now returns the next time value from the given array.  If
// previous calls have reached the end of the array, it returns
// the last time value again. If the array has not been set, it
// returns the UNIX Epoch.
//
func (c *SteppingClock) Now() time.Time {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if len(c.times) == 0 {
		locationUTC, _ := time.LoadLocation("UTC")
		return time.Date(1970, time.January, 1, 0, 0, 0, 0, locationUTC)
	}
	if c.nextTime == len(c.times) {
		// We have reached the end of the list.
		return c.times[len(c.times)-1]
	}

	// Return the next time value.
	result := c.times[c.nextTime]
	c.nextTime++
	return result

}
