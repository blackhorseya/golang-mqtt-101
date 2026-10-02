package main

import "testing"

// classifySeq 只看「上一個處理過的序號」與「新收到的序號」。
func TestClassifySeq(t *testing.T) {
	cases := []struct {
		name        string
		last, seq   uint64
		want        verdict
		wantMissing uint64
	}{
		{"next in sequence", 100, 101, accepted, 0},
		{"same seq again", 101, 101, duplicate, 0},
		{"one missing", 101, 103, gap, 1},
		{"several missing", 101, 105, gap, 3},
		{"older than last", 105, 102, outOfOrder, 0},
	}
	for _, tc := range cases {
		got, missing := classifySeq(tc.last, tc.seq)
		if got != tc.want || missing != tc.wantMissing {
			t.Errorf("%s: classifySeq(%d, %d) = %v, %d; want %v, %d", tc.name, tc.last, tc.seq, got, missing, tc.want, tc.wantMissing)
		}
	}
}

// seqTracker 為每個 device 記住 (run, 上一個處理過的序號)。
func TestSeqTrackerObserve(t *testing.T) {
	tr := newSeqTracker()
	steps := []struct {
		deviceID    string
		run         int64
		seq         uint64
		want        verdict
		wantMissing uint64
	}{
		{"device-001", 1, 57, accepted, 0},   // 第一次看到（monitor 晚啟動）：直接接受，不算 gap
		{"device-001", 1, 58, accepted, 0},   //
		{"device-001", 1, 58, duplicate, 0},  // 重複：不處理
		{"device-001", 1, 61, gap, 2},        // 59、60 遺失；61 仍然處理
		{"device-001", 1, 59, outOfOrder, 0}, // 比上一個舊：不處理
		{"device-001", 1, 62, accepted, 0},   // gap 之後從 61 接著算
		{"device-002", 1, 1, accepted, 0},    // 每個 device 各自獨立
		{"device-001", 2, 1, accepted, 0},    // device 重啟（run 不同）：序號重新開始，不是 out-of-order
		{"device-001", 2, 2, accepted, 0},    //
	}
	for i, s := range steps {
		got, missing := tr.observe(s.deviceID, s.run, s.seq)
		if got != s.want || missing != s.wantMissing {
			t.Errorf("step %d: observe(%s, run=%d, seq=%d) = %v, %d; want %v, %d", i, s.deviceID, s.run, s.seq, got, missing, s.want, s.wantMissing)
		}
	}
}
