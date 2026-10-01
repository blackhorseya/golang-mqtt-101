package mqttx

import (
	"fmt"
	"strings"
)

// 所有 device 相關 topic 的第一層。階層設計讓 monitor 能用 wildcard 一次訂閱所有 device：
//
//	devices/{deviceID}/telemetry
//	devices/{deviceID}/status
const devicesRoot = "devices"

// DeviceID 產生模擬 device 的 ID，例如 ("device", 1) → device-001。
func DeviceID(prefix string, n int) string {
	return fmt.Sprintf("%s-%03d", prefix, n)
}

// TelemetryTopic 回傳 device 發送 telemetry 的 topic。
func TelemetryTopic(deviceID string) string {
	return devicesRoot + "/" + deviceID + "/telemetry"
}

// DeviceIDFromTopic 從 devices/{deviceID}/... 取出 deviceID。
// 用 wildcard（devices/+/telemetry）訂閱時，訊息本身不帶是哪個 device，
// 只能從實際收到的 topic 判斷。
func DeviceIDFromTopic(topic string) (string, error) {
	parts := strings.Split(topic, "/")
	if len(parts) < 3 || parts[0] != devicesRoot || parts[1] == "" {
		return "", fmt.Errorf("parse device id from topic %q: want %s/{deviceID}/...", topic, devicesRoot)
	}
	return parts[1], nil
}
