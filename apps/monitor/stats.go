package main

import (
	"fmt"
	"sync"
	"time"
)

// receiveStats 統計 monitor 收到的 telemetry：序號判斷結果、各 QoS 的數量、DUP flag 次數與平均端到端延遲。
//
// duplicate 是應用層看到的重複（序號相同），dup 是 MQTT 協定層的 DUP flag，兩者不同。
type receiveStats struct {
	mu         sync.Mutex
	received   int
	verdicts   [4]int // 依 verdict 計數
	missing    uint64 // gap 中遺失的筆數總和
	byQoS      [3]int
	dup        int
	latencySum time.Duration
}

// record 記錄一則 telemetry；qos 是實際送達的 QoS（= min(發布 QoS, 訂閱 QoS)）。
func (x *receiveStats) record(qos byte, dup bool, latency time.Duration, v verdict, missing uint64) {
	x.mu.Lock()
	defer x.mu.Unlock()
	x.received++
	if int(v) < len(x.verdicts) {
		x.verdicts[v]++
	}
	x.missing += missing
	if int(qos) < len(x.byQoS) {
		x.byQoS[qos]++
	}
	if dup {
		x.dup++
	}
	x.latencySum += latency
}

func (x *receiveStats) summary() string {
	x.mu.Lock()
	defer x.mu.Unlock()
	var avg time.Duration
	if x.received > 0 {
		avg = x.latencySum / time.Duration(x.received)
	}
	return fmt.Sprintf("received=%d  accepted=%d  duplicate=%d  gap=%d  missing=%d  out_of_order=%d  qos0=%d  qos1=%d  qos2=%d  dup=%d  avg_lat=%s",
		x.received, x.verdicts[accepted], x.verdicts[duplicate], x.verdicts[gap], x.missing, x.verdicts[outOfOrder],
		x.byQoS[0], x.byQoS[1], x.byQoS[2], x.dup, avg.Round(10*time.Microsecond))
}
