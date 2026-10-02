package main

import (
	"fmt"
	"time"

	"github.com/blackhorseya/golang-mqtt-101/pkg/mqttx"
)

// formatTelemetry 把一筆 telemetry 排成一行，附上實際送達的 QoS 與端到端延遲，例如：
//
//	device-001  temp=28.4  humidity=61  battery=82  qos=1  lat=1.2ms
//
// dup 為 true 時（broker 重送、上一次可能已經送到）行尾加上 dup。
func formatTelemetry(t mqttx.Telemetry, qos byte, dup bool, latency time.Duration) string {
	line := fmt.Sprintf("%s  temp=%.1f  humidity=%d  battery=%d  qos=%d  lat=%s",
		t.DeviceID, t.Temp, t.Humidity, t.Battery, qos, latency.Round(100*time.Microsecond))
	if dup {
		line += "  dup"
	}
	return line
}
