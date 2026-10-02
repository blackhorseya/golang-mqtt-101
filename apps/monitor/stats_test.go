package main

import (
	"testing"
	"time"
)

func TestReceiveStatsSummary(t *testing.T) {
	var s receiveStats
	s.record(0, false, time.Millisecond, accepted, 0)
	s.record(1, true, 2*time.Millisecond, duplicate, 0)
	s.record(2, false, 3*time.Millisecond, gap, 2)
	s.record(2, false, 2*time.Millisecond, outOfOrder, 0)

	want := "received=4  accepted=1  duplicate=1  gap=1  missing=2  out_of_order=1  qos0=1  qos1=1  qos2=2  dup=1  avg_lat=2ms"
	if got := s.summary(); got != want {
		t.Errorf("summary = %q, want %q", got, want)
	}
}

func TestReceiveStatsSummaryEmpty(t *testing.T) {
	var s receiveStats
	want := "received=0  accepted=0  duplicate=0  gap=0  missing=0  out_of_order=0  qos0=0  qos1=0  qos2=0  dup=0  avg_lat=0s"
	if got := s.summary(); got != want {
		t.Errorf("summary = %q, want %q", got, want)
	}
}
