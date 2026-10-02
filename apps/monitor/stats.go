package main

import (
	"fmt"
	"sync"
	"time"
)

// receiveStats 統計 monitor 收到的 telemetry：各 QoS 的數量、DUP flag 次數與平均端到端延遲。
type receiveStats struct {
	mu         sync.Mutex
	received   int
	byQoS      [3]int
	dup        int
	latencySum time.Duration
}

// record 記錄一則 telemetry；qos 是實際送達的 QoS（= min(發布 QoS, 訂閱 QoS)）。
func (x *receiveStats) record(qos byte, dup bool, latency time.Duration) {
	x.mu.Lock()
	defer x.mu.Unlock()
	x.received++
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
	return fmt.Sprintf("received=%d  qos0=%d  qos1=%d  qos2=%d  dup=%d  avg_lat=%s",
		x.received, x.byQoS[0], x.byQoS[1], x.byQoS[2], x.dup, avg.Round(10*time.Microsecond))
}
