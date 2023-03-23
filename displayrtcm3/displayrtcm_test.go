package main

import (
	"testing"
	"time"
	
	"github.com/goblimey/go-ntrip/rtcm/utils"
)

// TestGetTime tests getTime
func TestGetTime(t *testing.T) {
	const dateTimeLayout = "2006-01-02 15:04:05.000000000 MST"

	expectedTime1 := time.Date(2020, time.November, 13, 0, 0, 0, 0, utils.LocationUTC)
	time1, err1 := getTime("2020-11-13")
	if err1 != nil {
		t.Error(err1)
	}
	if !expectedTime1.Equal(time1) {
		t.Errorf("expected %s got %s",
			expectedTime1.Format(dateTimeLayout),
			time1.Format(dateTimeLayout))
	}

	expectedTime2 := time.Date(2020, time.November, 13, 9, 10, 11, 0, utils.LocationUTC)
	time2, err2 := getTime("2020-11-13T09:10:11Z")
	if err2 != nil {
		t.Error(err2)
	}
	if !expectedTime2.Equal(time2) {
		t.Errorf("expected %s got %s",
			expectedTime2.Format(dateTimeLayout),
			time2.Format(dateTimeLayout))
	}

	expectedTime3 := time.Date(2020, time.November, 13, 9, 10, 11, 0, utils.LocationParis)
	time3, err3 := getTime("2020-11-13T09:10:11+01:00")
	if err3 != nil {
		t.Error(err3)
	}
	if !expectedTime3.Equal(time3) {
		t.Errorf("expected %s got %s",
			expectedTime3.Format(dateTimeLayout),
			time3.Format(dateTimeLayout))
	}

	expectedTime4 := time.Date(2020, time.November, 13, 9, 10, 11, 0, utils.LocationUTC)
	time4, err4 := getTime("2020-11-13T09:10:11Z")
	if err4 != nil {
		t.Error(err4)
	}
	if !expectedTime4.Equal(time4) {
		t.Errorf("expected %s got %s",
			expectedTime4.Format(dateTimeLayout),
			time4.Format(dateTimeLayout))
	}

	// Test time values that should fail.

	junk1 := "2020-11-13T09:10:11+junk"
	_, err5 := getTime(junk1)
	if err5 == nil {
		t.Errorf("timestring %s parsed but it should have failed", junk1)
	}

	junk2 := "2020-11-13T09:10:11+junk"
	_, err6 := getTime(junk2)
	if err6 == nil {
		t.Errorf("timestring %s parsed but it should have failed", junk2)
	}
}
