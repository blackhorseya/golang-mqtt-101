package main

import (
	"fmt"
	"strings"

	"github.com/blackhorseya/golang-mqtt-101/pkg/mqttx"
)

// formatStatus 把一筆 status 排成一行，並附上目前 monitor 認為在線的 device 數，例如：
//
//	device-001  status=offline  reason=lwt  online=2
//	device-001  status=online  reason=connected  online=1  (retained)
//
// retained 表示這則是 broker 因為我們剛訂閱而送來的保留訊息；即時轉發的訊息不帶這個標記。
func formatStatus(deviceID string, s mqttx.Status, online int, retained bool) string {
	line := fmt.Sprintf("%s  status=%s  reason=%s  online=%d", deviceID, s.State, s.Reason, online)
	if retained {
		line += "  (retained)"
	}
	return line
}

// formatCleared 表示 device 的 retained status 被清除（收到空 payload）。
func formatCleared(deviceID string, online int) string {
	return fmt.Sprintf("%s  status=cleared  online=%d", deviceID, online)
}

// presence 記錄 monitor 從 status 訊息得知的各 device 在線狀態。
// status 是 retained 時，monitor 一訂閱就會收到每個 device 的最新狀態，晚啟動也能算對。
type presence map[string]bool

// apply 更新 deviceID 的狀態並回傳目前在線數。
func (x presence) apply(deviceID string, s mqttx.Status) int {
	x[deviceID] = s.State == mqttx.StateOnline
	return x.online()
}

// remove 忘掉 deviceID（它的 retained status 被清除了）並回傳目前在線數。
func (x presence) remove(deviceID string) int {
	delete(x, deviceID)
	return x.online()
}

func (x presence) online() int {
	online := 0
	for _, on := range x {
		if on {
			online++
		}
	}
	return online
}

// parseFilters 把逗號分隔的 topic filter 拆成清單。
func parseFilters(s string) []string {
	var filters []string
	for f := range strings.SplitSeq(s, ",") {
		if f = strings.TrimSpace(f); f != "" {
			filters = append(filters, f)
		}
	}
	return filters
}

// formatAck 把 device 回報的 command 執行結果排成一行，例如：
//
//	device-001  ack  command=reboot  id=3f2a…  ok=true
//	device-001  ack  command=config  id=9c1d…  ok=false  error=parse interval "fast": …
func formatAck(deviceID string, ack mqttx.Ack) string {
	line := fmt.Sprintf("%s  ack  command=%s  id=%s  ok=%t", deviceID, ack.Command, ack.ID, ack.OK)
	if ack.Error != "" {
		line += "  error=" + ack.Error
	}
	return line
}
