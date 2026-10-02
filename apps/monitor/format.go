package main

import (
	"fmt"
	"time"

	"github.com/blackhorseya/golang-mqtt-101/pkg/mqttx"
)

// formatTelemetry 把一筆 telemetry 排成一行，附上序號、實際送達的 QoS、端到端延遲與序號判斷，例如：
//
//	device-001  temp=28.4  humidity=61  battery=82  seq=101  qos=1  lat=1.2ms  accepted
//	device-001  temp=28.4  humidity=61  battery=82  seq=104  qos=1  lat=1.3ms  gap(missing=2)
//
// dup 為 true 時（MQTT 的 DUP flag：broker 重送）在判斷前加上 dup。
func formatTelemetry(t mqttx.Telemetry, qos byte, dup bool, latency time.Duration, v verdict, missing uint64) string {
	line := fmt.Sprintf("%s  temp=%.1f  humidity=%d  battery=%d  seq=%d  qos=%d  lat=%s",
		t.DeviceID, t.Temp, t.Humidity, t.Battery, t.Seq, qos, latency.Round(100*time.Microsecond))
	if dup {
		line += "  dup"
	}
	if v == gap {
		return line + fmt.Sprintf("  gap(missing=%d)", missing)
	}
	return line + "  " + v.String()
}
