package mqttx

import (
	"encoding/json"
	"fmt"
)

// State 是 device 的在線狀態。
type State string

const (
	StateOnline  State = "online"
	StateOffline State = "offline"
)

// Reason 說明狀態是怎麼來的。重點是區分兩種 offline：
// graceful 是 device 自己發的；lwt 是 device 異常斷線後 broker 代發的 Last Will。
type Reason string

const (
	ReasonConnected Reason = "connected"
	ReasonGraceful  Reason = "graceful"
	ReasonLWT       Reason = "lwt"
)

// Status 是發到 devices/{deviceID}/status 的 payload。
type Status struct {
	State  State  `json:"state"`
	Reason Reason `json:"reason"`
}

// StatusTopic 回傳 device 發送在線狀態的 topic。
func StatusTopic(deviceID string) string {
	return devicesRoot + "/" + deviceID + "/status"
}

// EncodeStatus 把 Status 編成 payload。
func EncodeStatus(s Status) ([]byte, error) {
	b, err := json.Marshal(s)
	if err != nil {
		return nil, fmt.Errorf("encode status %s: %w", s.State, err)
	}
	return b, nil
}

// DecodeStatus 從 payload 解出 Status。
func DecodeStatus(payload []byte) (Status, error) {
	var s Status
	if err := json.Unmarshal(payload, &s); err != nil {
		return Status{}, fmt.Errorf("decode status: %w", err)
	}
	return s, nil
}
