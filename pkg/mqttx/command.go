package mqttx

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// command 走另一個方向（server → device），所以放在另一個根底下：
//
//	commands/{deviceID}/{command}   server → device，device 訂閱 commands/{deviceID}/#
//	devices/{deviceID}/ack          device → server，執行結果
//
// ack 不能放在 commands/{deviceID}/ 底下，否則 device 會收到自己發的 ack。
const commandsRoot = "commands"

// CommandFilter 回傳 device 要訂閱的 topic filter：這台 device 的所有 command。
func CommandFilter(deviceID string) string {
	return commandsRoot + "/" + deviceID + "/#"
}

// CommandTopic 回傳發給 device 某個 command 的 topic。
func CommandTopic(deviceID, name string) string {
	return commandsRoot + "/" + deviceID + "/" + name
}

// CommandNameFromTopic 從 commands/{deviceID}/{command} 取出 command 名稱。
func CommandNameFromTopic(topic string) (string, error) {
	parts := strings.SplitN(topic, "/", 3)
	if len(parts) < 3 || parts[0] != commandsRoot || parts[1] == "" || parts[2] == "" {
		return "", fmt.Errorf("parse command from topic %q: want %s/{deviceID}/{command}", topic, commandsRoot)
	}
	return parts[2], nil
}

// AckTopic 回傳 device 回報 command 執行結果的 topic。
func AckTopic(deviceID string) string {
	return devicesRoot + "/" + deviceID + "/ack"
}

// Command 是發到 commands/{deviceID}/{command} 的 payload。
//
// MQTT 3.1.1 沒有「回應」的概念：publish 出去就結束了，不知道誰收到、結果如何。
// 要做 request/response，得自己在 payload 放一個 ID，device 在 ack 裡帶回來，發送端靠它找到對應的回應。
// （MQTT 5 才在協定裡加入 response topic 與 correlation data。）
type Command struct {
	ID   string            `json:"id"`
	Args map[string]string `json:"args,omitempty"`
}

// Ack 是 device 執行 command 後發到 devices/{deviceID}/ack 的 payload。
type Ack struct {
	ID      string `json:"id"` // 對應的 Command.ID
	Command string `json:"command"`
	OK      bool   `json:"ok"`
	Error   string `json:"error,omitempty"`
}

// EncodeCommand 把 Command 編成 payload。
func EncodeCommand(cmd Command) ([]byte, error) {
	b, err := json.Marshal(cmd)
	if err != nil {
		return nil, fmt.Errorf("encode command %s: %w", cmd.ID, err)
	}
	return b, nil
}

// DecodeCommand 從 payload 解出 Command；沒有 ID 的 command 無法回應，視為無效。
func DecodeCommand(payload []byte) (Command, error) {
	var cmd Command
	if err := json.Unmarshal(payload, &cmd); err != nil {
		return Command{}, fmt.Errorf("decode command: %w", err)
	}
	if cmd.ID == "" {
		return Command{}, errors.New("decode command: missing id")
	}
	return cmd, nil
}

// EncodeAck 把 Ack 編成 payload。
func EncodeAck(ack Ack) ([]byte, error) {
	b, err := json.Marshal(ack)
	if err != nil {
		return nil, fmt.Errorf("encode ack %s: %w", ack.ID, err)
	}
	return b, nil
}

// DecodeAck 從 payload 解出 Ack。
func DecodeAck(payload []byte) (Ack, error) {
	var ack Ack
	if err := json.Unmarshal(payload, &ack); err != nil {
		return Ack{}, fmt.Errorf("decode ack: %w", err)
	}
	return ack, nil
}
