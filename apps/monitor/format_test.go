package main

import (
	"testing"
	"time"

	"github.com/blackhorseya/golang-mqtt-101/pkg/mqttx"
)

func TestFormatTelemetry(t *testing.T) {
	tel := mqttx.Telemetry{DeviceID: "device-001", Temp: 28.4, Humidity: 61, Battery: 82}
	cases := []struct {
		qos     byte
		dup     bool
		latency time.Duration
		want    string
	}{
		{0, false, 1500 * time.Microsecond, "device-001  temp=28.4  humidity=61  battery=82  qos=0  lat=1.5ms"},
		{2, false, 812 * time.Microsecond, "device-001  temp=28.4  humidity=61  battery=82  qos=2  lat=800µs"},
		// DUP flag：broker 重送、且上一次可能已經送到過
		{1, true, 2 * time.Millisecond, "device-001  temp=28.4  humidity=61  battery=82  qos=1  lat=2ms  dup"},
	}
	for _, tc := range cases {
		if got := formatTelemetry(tel, tc.qos, tc.dup, tc.latency); got != tc.want {
			t.Errorf("formatTelemetry(qos=%d, dup=%v, %s) = %q, want %q", tc.qos, tc.dup, tc.latency, got, tc.want)
		}
	}
}
