package main

import (
	"fmt"
	"strings"

	"github.com/blackhorseya/golang-mqtt-101/pkg/mqttx"
)

// formatStatus 把一筆 status 排成一行，並附上目前 monitor 認為在線的 device 數，例如：
//
//	device-001  status=offline  reason=lwt  online=2
func formatStatus(deviceID string, s mqttx.Status, online int) string {
	return fmt.Sprintf("%s  status=%s  reason=%s  online=%d", deviceID, s.State, s.Reason, online)
}

// presence 記錄 monitor 從 status 訊息得知的各 device 在線狀態。
// 它只知道 monitor 啟動後收到的訊息：monitor 晚於 device 啟動時，計數會是錯的（Phase 3 用 retained 解決）。
type presence map[string]bool

// apply 更新 deviceID 的狀態並回傳目前在線數。
func (x presence) apply(deviceID string, s mqttx.Status) int {
	x[deviceID] = s.State == mqttx.StateOnline
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
