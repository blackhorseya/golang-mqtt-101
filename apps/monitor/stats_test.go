package main

import (
	"testing"
	"time"
)

func TestReceiveStatsSummary(t *testing.T) {
	var s receiveStats
	s.record(0, false, time.Millisecond)
	s.record(1, true, 2*time.Millisecond)
	s.record(2, false, 3*time.Millisecond)

	want := "received=3  qos0=1  qos1=1  qos2=1  dup=1  avg_lat=2ms"
	if got := s.summary(); got != want {
		t.Errorf("summary = %q, want %q", got, want)
	}
}

func TestReceiveStatsSummaryEmpty(t *testing.T) {
	var s receiveStats
	if got, want := s.summary(), "received=0  qos0=0  qos1=0  qos2=0  dup=0  avg_lat=0s"; got != want {
		t.Errorf("summary = %q, want %q", got, want)
	}
}
