package main

import (
	"fmt"
	"sync"
	"time"
)

// publishStats 統計所有 device 的 telemetry 發布結果，可同時被多個 device goroutine 更新。
//
// publisher 只知道 broker 有沒有確認：
//   - confirmed：token 在期限內完成。QoS 1/2 代表 broker 已回 ack；QoS 0 沒有 ack，只代表交給了網路層
//   - unconfirmed：期限內等不到。QoS 1/2 的訊息仍在 client 裡，重連後可能還會送出，所以不等於遺失
//   - retries：應用層重送（同一筆 telemetry、同一個 seq 再發一次），包含在 published 裡
//   - skipped：序號已用掉但沒有發出去（模擬送出前就遺失），subscriber 會看到 gap
//
// subscriber 到底收到幾則，publisher 永遠不知道。
type publishStats struct {
	mu        sync.Mutex
	published int
	retries   int
	skipped   int
	confirmed int
	ackTotal  time.Duration
}

// record 記錄一次發布；ack 是從 Publish 到 token 完成的時間，retry 表示這是應用層重送。
func (x *publishStats) record(ack time.Duration, err error, retry bool) {
	x.mu.Lock()
	defer x.mu.Unlock()
	x.published++
	if retry {
		x.retries++
	}
	if err == nil {
		x.confirmed++
		x.ackTotal += ack
	}
}

// recordSkip 記錄一筆跳過不發的 telemetry。
func (x *publishStats) recordSkip() {
	x.mu.Lock()
	defer x.mu.Unlock()
	x.skipped++
}

// summary 產生一行統計，sent / received 為連線上實際傳輸的位元組數。
func (x *publishStats) summary(sent, received int64) string {
	x.mu.Lock()
	defer x.mu.Unlock()
	var avg time.Duration
	if x.confirmed > 0 {
		avg = x.ackTotal / time.Duration(x.confirmed)
	}
	return fmt.Sprintf("published=%d  retries=%d  skipped=%d  confirmed=%d  unconfirmed=%d  avg_ack=%s  sent=%dB  received=%dB",
		x.published, x.retries, x.skipped, x.confirmed, x.published-x.confirmed, avg.Round(10*time.Microsecond), sent, received)
}
