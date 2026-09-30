package main

import (
	"testing"

	"github.com/blackhorseya/golang-mqtt-101/pkg/mqttx"
)

func TestFormatStatus(t *testing.T) {
	cases := []struct {
		status mqttx.Status
		online int
		want   string
	}{
		{mqttx.Status{State: mqttx.StateOnline, Reason: mqttx.ReasonConnected}, 3, "device-001  status=online  reason=connected  online=3"},
		{mqttx.Status{State: mqttx.StateOffline, Reason: mqttx.ReasonGraceful}, 2, "device-001  status=offline  reason=graceful  online=2"},
		{mqttx.Status{State: mqttx.StateOffline, Reason: mqttx.ReasonLWT}, 0, "device-001  status=offline  reason=lwt  online=0"},
	}
	for _, tc := range cases {
		if got := formatStatus("device-001", tc.status, tc.online); got != tc.want {
			t.Errorf("formatStatus(%+v, %d) = %q, want %q", tc.status, tc.online, got, tc.want)
		}
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

func TestParseFilters(t *testing.T) {
	got := parseFilters(" devices/+/telemetry, devices/+/status ,")
	want := []string{"devices/+/telemetry", "devices/+/status"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("parseFilters = %q, want %q", got, want)
	}
}
