package mqttx

import (
	"encoding/json"
	"fmt"
	"time"
)

// Telemetry 是 device 定期發送的量測資料，以 JSON 作為 MQTT payload。
// MQTT 本身不管 payload 格式，對 broker 來說它只是 bytes。
//
// Seq 是每個 device 從 1 開始、每筆 +1 的序號；Run 標示 device 這次 process（啟動時間），
// device 重啟後 Seq 重新從 1 開始、Run 也會不同，subscriber 據此判斷是新的序列。
type Telemetry struct {
	DeviceID  string    `json:"device_id"`
	Run       int64     `json:"run"`
	Seq       uint64    `json:"seq"`
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
