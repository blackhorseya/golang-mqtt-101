package main

import (
	"math/rand/v2"
	"testing"
	"time"
)

func TestSensorStaysInRange(t *testing.T) {
	s := newSensor("device-001", 42, rand.New(rand.NewPCG(1, 2)))
	prevBattery := 101
	now := time.Now()
	for i := range 1000 {
		r := s.next(now)
		if r.DeviceID != "device-001" {
			t.Fatalf("step %d: DeviceID = %q", i, r.DeviceID)
		}
		if r.Humidity < 0 || r.Humidity > 100 {
			t.Fatalf("step %d: humidity %d out of [0,100]", i, r.Humidity)
		}
		if r.Battery < 0 || r.Battery > prevBattery {
			t.Fatalf("step %d: battery %d (prev %d) must be non-negative and non-increasing", i, r.Battery, prevBattery)
		}
		prevBattery = r.Battery
		if r.Temp < -20 || r.Temp > 60 {
			t.Fatalf("step %d: temp %.1f out of [-20,60]", i, r.Temp)
		}
	}
}

// 每個 device 的序號從 1 開始、每筆 +1；run 標示這次 process，device 重啟後 run 會不同。
func TestSensorSequence(t *testing.T) {
	s := newSensor("device-001", 42, rand.New(rand.NewPCG(1, 2)))
	now := time.Now()
	for want := uint64(1); want <= 3; want++ {
		r := s.next(now)
		if r.Seq != want || r.Run != 42 {
			t.Fatalf("next() seq=%d run=%d, want seq=%d run=42", r.Seq, r.Run, want)
		}
	}
}
