package main

import (
	"testing"

	circularQueue "github.com/goblimey/go-ntrip/apps/proxy/circular_queue"
	rtcm "github.com/goblimey/go-ntrip/rtcm/handler"
)

func TestParseConfig(t *testing.T) {

	j := `{
		"remote_host": "remote:1001",
		"local_host": "local",
		"local_port": 42,
		"control_port": 43,
		"tls": {
			"country": ["GB"],
			"org": ["some org"],
			"common_name": "*.domain.com"
			},
		"cert_file": "certFile",
		"record_messages": true,
    	"message_log_directory": "./logs"
	}`

	jsonData := []byte(j)

	var config Config

	err := parseConfig(jsonData, &config)

	if err != nil {
		t.Error(err)
		return
	}

	if config.Remotehost != "remote:1001" {
		t.Errorf("want remote:1001 got %s", config.Remotehost)
	}

	if config.Localhost != "local" {
		t.Errorf("want local got %s", config.Localhost)
	}

	if config.Localport != 42 {
		t.Errorf("want 42 got %d", config.Localport)
	}

	if config.ControlPort != 43 {
		t.Errorf("want 43 got %d", config.ControlPort)
	}

	if len(config.TLS.Country) != 1 {
		t.Errorf("want 1 country, got %d", len(config.TLS.Country))
		return
	}

	if config.TLS.Country[0] != "GB" {
		t.Errorf("want GB got %s", config.TLS.Country[0])
	}

	if len(config.TLS.Org) != 1 {
		t.Errorf("want 1 organisation, got %d", len(config.TLS.Org))
		return
	}

	if config.TLS.Org[0] != "some org" {
		t.Errorf("want some org got %s", config.TLS.Org[0])
	}

	if config.TLS.CommonName != "*.domain.com" {
		t.Errorf("want *.domain.com got %s", config.TLS.CommonName)
	}

	if config.CertFile != "certFile" {
		t.Errorf("want certFile got %s", config.CertFile)
	}

	if !config.RecordMessages {
		t.Errorf("RecordMessages should be true")
	}

	if config.MessageLogDirectory != "./logs" {
		t.Errorf("want ./logs got %s", config.MessageLogDirectory)
	}
}

func TestParseConfigWithError(t *testing.T) {

	j := "{junk}"

	jsonData := []byte(j)

	var config Config

	err := parseConfig(jsonData, &config)

	if err == nil {
		t.Error("expected an error")
	}
}

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
