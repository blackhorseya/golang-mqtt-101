package main

import (
	"testing"

	"github.com/blackhorseya/golang-mqtt-101/pkg/mqttx"
)

func TestFormatTelemetry(t *testing.T) {
	got := formatTelemetry(mqttx.Telemetry{DeviceID: "device-001", Temp: 28.4, Humidity: 61, Battery: 82})
	if want := "device-001  temp=28.4  humidity=61  battery=82"; got != want {
		t.Errorf("formatTelemetry = %q, want %q", got, want)
	}
}
