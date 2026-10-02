package main

// verdict 是 monitor 依序號對一筆 telemetry 的判斷。
type verdict int

const (
	accepted   verdict = iota // 下一個序號：處理
	duplicate                 // 和上一個處理過的一樣：不處理
	gap                       // 跳號：中間有遺失，這筆仍然處理
	outOfOrder                // 比上一個處理過的還舊：不處理
)

func (v verdict) String() string {
	switch v {
	case accepted:
		return "accepted"
	case duplicate:
		return "duplicate"
	case gap:
		return "gap"
	case outOfOrder:
		return "out-of-order"
	default:
		return "unknown"
	}
}

// classifySeq 比較「上一個處理過的序號」last 與「新收到的序號」seq，回傳判斷；
// 判為 gap 時另外回傳中間遺失了幾筆。
//
// 只記得 last 一個數字是刻意的簡化：一則很晚才到的舊訊息，和一則重複的舊訊息，
// 在這裡都會被判成 out-of-order，分不出來。
func classifySeq(last, seq uint64) (v verdict, missing uint64) {
	switch {
	case seq == last+1:
		return accepted, 0
	case seq == last:
		return duplicate, 0
	case seq > last:
		// 已確定 seq > last+1，減法不會 underflow
		return gap, seq - last - 1
	default:
		return outOfOrder, 0
	}
}

// seqState 是 monitor 對單一 device 記住的狀態。
type seqState struct {
	run  int64  // device 這次 process 的識別
	last uint64 // 上一個處理過的序號
}

// seqTracker 為每個 device 追蹤上一個處理過的序號。
type seqTracker struct {
	devices map[string]seqState
}

func newSeqTracker() *seqTracker {
	return &seqTracker{devices: map[string]seqState{}}
}

// observe 判斷 deviceID 的這筆 telemetry，並在「會被處理」時（accepted、gap）推進 last。
func (x *seqTracker) observe(deviceID string, run int64, seq uint64) (verdict, uint64) {
	st, seen := x.devices[deviceID]
	// 第一次看到這個 device（例如 monitor 晚啟動），或 device 重啟換了 run：
	// 從這筆開始一條新的序列，不算 gap
	if !seen || st.run != run {
		x.devices[deviceID] = seqState{run: run, last: seq}
		return accepted, 0
	}
	v, missing := classifySeq(st.last, seq)
	if v == accepted || v == gap {
		st.last = seq
		x.devices[deviceID] = st
	}
	return v, missing
}
