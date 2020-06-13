package clock

import (
	"time"
)

// StoppedClock is a Clock that implements unchanging time.
//
type StoppedClock struct {
	time time.Time
}

var _ Clock = (*StoppedClock)(nil) // Ensure that StoppedClock implements Clock.

// NewStoppedClock creates a StoppedClock.
//
func NewStoppedClock(year int, month time.Month, day, hour, minute, second, nanosecond int, location *time.Location) Clock {
	time := time.Date(year, month, day, hour, minute, second, nanosecond, location)
	return &StoppedClock{time: time}
}

// SetTime sets a new unchanging time.
//
func (c *StoppedClock) SetTime(time time.Time) {
	c.time = time
}

// Now always returns the same time.
//
func (c *StoppedClock) Now() time.Time {
	return c.time
}
