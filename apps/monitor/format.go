package main

import (
	"fmt"

	"github.com/blackhorseya/golang-mqtt-101/pkg/mqttx"
)

// formatTelemetry 把一筆 telemetry 排成一行，例如：
//
//	device-001  temp=28.4  humidity=61  battery=82
func formatTelemetry(t mqttx.Telemetry) string {
	return fmt.Sprintf("%s  temp=%.1f  humidity=%d  battery=%d", t.DeviceID, t.Temp, t.Humidity, t.Battery)
}
