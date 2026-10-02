package main

import (
	"testing"
	"time"

	"github.com/blackhorseya/golang-mqtt-101/pkg/mqttx"
)

// publisher 只知道 broker 有沒有確認（ack），不知道 subscriber 有沒有收到；
// 等不到確認的訊息叫 unconfirmed，不是 lost（QoS 1/2 在重連後可能還會送出）。
func TestPublishStatsSummary(t *testing.T) {
	var s publishStats
	s.record(time.Millisecond, nil)
	s.record(3*time.Millisecond, nil)
	s.record(0, mqttx.ErrTimeout)

	got := s.summary(1200, 300)
	want := "published=3  confirmed=2  unconfirmed=1  avg_ack=2ms  sent=1200B  received=300B"
	if got != want {
		t.Errorf("summary = %q, want %q", got, want)
	}
}

func TestPublishStatsSummaryEmpty(t *testing.T) {
	var s publishStats
	if got, want := s.summary(0, 0), "published=0  confirmed=0  unconfirmed=0  avg_ack=0s  sent=0B  received=0B"; got != want {
		t.Errorf("summary = %q, want %q", got, want)
	}
}
