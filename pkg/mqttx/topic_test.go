package mqttx

import "testing"

func TestDeviceID(t *testing.T) {
	cases := map[int]string{1: "device-001", 42: "device-042", 1000: "device-1000"}
	for n, want := range cases {
		if got := DeviceID(n); got != want {
			t.Errorf("DeviceID(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestTelemetryTopic(t *testing.T) {
	if got, want := TelemetryTopic("device-001"), "devices/device-001/telemetry"; got != want {
		t.Errorf("TelemetryTopic = %q, want %q", got, want)
	}
}

func TestDeviceIDFromTopic(t *testing.T) {
	cases := []struct {
		topic   string
		want    string
		wantErr bool
	}{
		{topic: "devices/device-001/telemetry", want: "device-001"},
		{topic: "devices/device-002/status", want: "device-002"},
		{topic: "commands/device-001/reboot", wantErr: true},
		{topic: "devices//telemetry", wantErr: true},
		{topic: "devices", wantErr: true},
	}
	for _, tc := range cases {
		got, err := DeviceIDFromTopic(tc.topic)
		if tc.wantErr {
			if err == nil {
				t.Errorf("DeviceIDFromTopic(%q) = %q, want error", tc.topic, got)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Errorf("DeviceIDFromTopic(%q) = %q, %v; want %q", tc.topic, got, err, tc.want)
		}
	}
}
