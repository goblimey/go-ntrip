package clock

import (
	"time"
)

// SystemClock satisfies the Clock interface by supplying the system time.
type SystemClock struct {
}

// NewSystemClock creates a system clock and returns it as a Clock.
func NewSystemClock() Clock {
	var systemClock SystemClock
	return &systemClock
}

// Now returns the system time.
func (c SystemClock) Now() time.Time {
	return time.Now()
}
