package main

import (
	"testing"

	"github.com/blackhorseya/golang-mqtt-101/pkg/mqttx"
)

func TestFormatStatus(t *testing.T) {
	cases := []struct {
		status   mqttx.Status
		online   int
		retained bool
		want     string
	}{
		{mqttx.Status{State: mqttx.StateOnline, Reason: mqttx.ReasonConnected}, 3, false, "device-001  status=online  reason=connected  online=3"},
		{mqttx.Status{State: mqttx.StateOffline, Reason: mqttx.ReasonGraceful}, 2, false, "device-001  status=offline  reason=graceful  online=2"},
		{mqttx.Status{State: mqttx.StateOffline, Reason: mqttx.ReasonLWT}, 0, false, "device-001  status=offline  reason=lwt  online=0"},
		// 因為訂閱而收到的 retained message 要標示出來，和即時轉發的訊息區分
		{mqttx.Status{State: mqttx.StateOnline, Reason: mqttx.ReasonConnected}, 1, true, "device-001  status=online  reason=connected  online=1  (retained)"},
	}
	for _, tc := range cases {
		if got := formatStatus("device-001", tc.status, tc.online, tc.retained); got != tc.want {
			t.Errorf("formatStatus(%+v, %d, %v) = %q, want %q", tc.status, tc.online, tc.retained, got, tc.want)
		}
	}
}

// 空 payload 的 retained publish 會清掉 broker 保留的訊息，monitor 會即時收到這則空訊息。
func TestFormatCleared(t *testing.T) {
	if got, want := formatCleared("device-001", 2), "device-001  status=cleared  online=2"; got != want {
		t.Errorf("formatCleared = %q, want %q", got, want)
	}
}

func TestPresence(t *testing.T) {
	p := presence{}
	on := mqttx.Status{State: mqttx.StateOnline, Reason: mqttx.ReasonConnected}
	off := mqttx.Status{State: mqttx.StateOffline, Reason: mqttx.ReasonLWT}

	steps := []struct {
		deviceID string
		status   mqttx.Status
		want     int
	}{
		{"device-001", on, 1},
		{"device-002", on, 2},
		{"device-001", on, 2}, // 重連後再次 online 不重複計算
		{"device-001", off, 1},
		{"device-003", off, 1}, // 沒看過的 device 下線
	}
	for i, s := range steps {
		if got := p.apply(s.deviceID, s.status); got != s.want {
			t.Errorf("step %d: apply(%s, %s) = %d, want %d", i, s.deviceID, s.status.State, got, s.want)
		}
	}
}

func TestPresenceRemove(t *testing.T) {
	p := presence{}
	on := mqttx.Status{State: mqttx.StateOnline, Reason: mqttx.ReasonConnected}
	p.apply("device-001", on)
	p.apply("device-002", on)
	if got := p.remove("device-001"); got != 1 {
		t.Errorf("remove(device-001) = %d, want 1", got)
	}
	if got := p.remove("device-999"); got != 1 {
		t.Errorf("remove(unknown) = %d, want 1", got)
	}
}

func TestParseFilters(t *testing.T) {
	got := parseFilters(" devices/+/telemetry, devices/+/status ,")
	want := []string{"devices/+/telemetry", "devices/+/status"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("parseFilters = %q, want %q", got, want)
	}
}
