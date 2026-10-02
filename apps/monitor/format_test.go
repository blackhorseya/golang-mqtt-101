package main

import (
	"testing"
	"time"

	"github.com/blackhorseya/golang-mqtt-101/pkg/mqttx"
)

func TestFormatTelemetry(t *testing.T) {
	tel := mqttx.Telemetry{DeviceID: "device-001", Seq: 101, Temp: 28.4, Humidity: 61, Battery: 82}
	cases := []struct {
		qos     byte
		dup     bool
		latency time.Duration
		verdict verdict
		missing uint64
		want    string
	}{
		{0, false, 1500 * time.Microsecond, accepted, 0, "device-001  temp=28.4  humidity=61  battery=82  seq=101  qos=0  lat=1.5ms  accepted"},
		{2, false, 812 * time.Microsecond, duplicate, 0, "device-001  temp=28.4  humidity=61  battery=82  seq=101  qos=2  lat=800µs  duplicate"},
		// DUP flag：broker 重送、且上一次可能已經送到過
		{1, true, 2 * time.Millisecond, gap, 2, "device-001  temp=28.4  humidity=61  battery=82  seq=101  qos=1  lat=2ms  dup  gap(missing=2)"},
		{1, false, 2 * time.Millisecond, outOfOrder, 0, "device-001  temp=28.4  humidity=61  battery=82  seq=101  qos=1  lat=2ms  out-of-order"},
	}
	for _, tc := range cases {
		if got := formatTelemetry(tel, tc.qos, tc.dup, tc.latency, tc.verdict, tc.missing); got != tc.want {
			t.Errorf("formatTelemetry(qos=%d, dup=%v, %s, %v) = %q, want %q", tc.qos, tc.dup, tc.latency, tc.verdict, got, tc.want)
		}
	}
}
