package main

import (
	"testing"
	"time"

	"github.com/blackhorseya/golang-mqtt-101/pkg/mqttx"
)

// LWT 是 device 連線時先交給 broker 保管的訊息，payload 必須和正常下線不同，monitor 才分得出來。
func TestDeviceOptionsWill(t *testing.T) {
	opts := deviceOptions(mqttx.Config{BrokerURL: "tcp://127.0.0.1:1883", ClientID: "device-001"}, 5*time.Second, true)

	if !opts.WillEnabled {
		t.Fatal("WillEnabled = false, want true")
	}
	if got, want := opts.WillTopic, "devices/device-001/status"; got != want {
		t.Errorf("WillTopic = %q, want %q", got, want)
	}
	will, err := mqttx.DecodeStatus(opts.WillPayload)
	if err != nil {
		t.Fatalf("decode will payload %q: %v", opts.WillPayload, err)
	}
	if want := (mqttx.Status{State: mqttx.StateOffline, Reason: mqttx.ReasonLWT}); will != want {
		t.Errorf("will = %+v, want %+v", will, want)
	}
	// LWT 要和 status 一樣 retained，否則 broker 保留的仍是最後一則 online，晚到的訂閱者會以為 device 還在線
	if !opts.WillRetained {
		t.Error("WillRetained = false, want true")
	}
	if opts.WillQos != 0 {
		t.Errorf("WillQos = %d; QoS belongs to phase 4", opts.WillQos)
	}
	if got := opts.KeepAlive; got != 5 {
		t.Errorf("KeepAlive = %ds, want 5s", got)
	}
}

// --retain=false 時 status 與 LWT 都不保留，重現 Phase 2 的行為以便比較。
func TestDeviceOptionsWillNotRetained(t *testing.T) {
	opts := deviceOptions(mqttx.Config{BrokerURL: "tcp://127.0.0.1:1883", ClientID: "device-001"}, 5*time.Second, false)
	if opts.WillRetained {
		t.Error("WillRetained = true with retain disabled, want false")
	}
}
