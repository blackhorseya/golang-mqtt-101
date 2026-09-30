package mqttx

import (
	"encoding/json"
	"fmt"
	"time"
)

// Telemetry 是 device 定期發送的量測資料，以 JSON 作為 MQTT payload。
// MQTT 本身不管 payload 格式，對 broker 來說它只是 bytes。
type Telemetry struct {
	DeviceID  string    `json:"device_id"`
	Temp      float64   `json:"temp"`
	Humidity  int       `json:"humidity"`
	Battery   int       `json:"battery"`
	Timestamp time.Time `json:"ts"`
}

// EncodeTelemetry 把 Telemetry 編成 payload。
func EncodeTelemetry(t Telemetry) ([]byte, error) {
	b, err := json.Marshal(t)
	if err != nil {
		return nil, fmt.Errorf("encode telemetry %s: %w", t.DeviceID, err)
	}
	return b, nil
}

// DecodeTelemetry 從 payload 解出 Telemetry。
func DecodeTelemetry(payload []byte) (Telemetry, error) {
	var t Telemetry
	if err := json.Unmarshal(payload, &t); err != nil {
		return Telemetry{}, fmt.Errorf("decode telemetry: %w", err)
	}
	return t, nil
}
