package main

import (
	"testing"

	circularQueue "github.com/goblimey/go-ntrip/apps/proxy/circular_queue"
	rtcm "github.com/goblimey/go-ntrip/rtcm/handler"
)

func TestCircularBuffer(t *testing.T) {
	m1 := rtcm.Message{MessageType: 1074}
	m2 := rtcm.Message{MessageType: 1077}

	buf := circularQueue.NewCircularQueue(3)

	// Add 1 item, fetch it back and check it.
	buf.Add(m1)

	got1 := buf.GetMessages()

	if len(got1) != 1 {
		t.Errorf("want 1 item got %d", len(got1))
	}

	if got1[0].MessageType != m1.MessageType {
		t.Error("wanted m1")
	}

	// Add a second item, get both back and check.
	buf.Add(m2)

	got2 := buf.GetMessages()

	if len(got2) != 2 {
		t.Errorf("want 2 items got %d", len(got2))
	}

	if got2[0].MessageType != m1.MessageType {
		t.Error("wanted m1")
	}

	if got2[1].MessageType != m2.MessageType {
		t.Error("wanted m2")
	}
}

// TestCircularBufferEmpty checks that the buffer works correctly
// when empty.
func TestCircularBufferEmpty(t *testing.T) {
	buf := circularQueue.NewCircularQueue(1)

	got := buf.GetMessages()

	if len(got) != 0 {
		t.Errorf("want empty slice, got %d items", len(got))
	}
}

// TestCircularBufferFull checks that the buffer works correctly
// when it has already been filled.
func TestCircularBufferFull(t *testing.T) {
	m1 := rtcm.Message{MessageType: 1074}
	m2 := rtcm.Message{MessageType: 1077}
	m3 := rtcm.Message{MessageType: 1084}
	m4 := rtcm.Message{MessageType: 1087}

	buf := circularQueue.NewCircularQueue(2)

	// Add 3 messages, which will remove the first message.

	buf.Add(m1)
	buf.Add(m2)
	// Adding the third message will remove the first.
	buf.Add(m3)

	got1 := buf.GetMessages()

	if len(got1) != 2 {
		t.Errorf("want 2 items got %d", len(got1))
	}

	if got1[0].MessageType != m2.MessageType {
		t.Error("wanted m2")
	}

	if got1[1].MessageType != m3.MessageType {
		t.Error("wanted m3")
	}

	// Add another message, which will remove the second message.
	buf.Add(m4)

	got2 := buf.GetMessages()

	if len(got2) != 2 {
		t.Errorf("want 2 items got %d", len(got2))
	}

	if got2[0].MessageType != m3.MessageType {
		t.Error("wanted m3")
	}

	if got2[1].MessageType != m4.MessageType {
		t.Error("wanted m4")
	}
}
