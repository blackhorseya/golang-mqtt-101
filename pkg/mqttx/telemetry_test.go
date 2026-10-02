package mqttx

import (
	"testing"
	"time"
)

func TestTelemetryRoundTrip(t *testing.T) {
	in := Telemetry{
		DeviceID:  "device-001",
		Run:       1790900000000000000,
		Seq:       101,
		Temp:      28.4,
		Humidity:  61,
		Battery:   82,
		Timestamp: time.Date(2026, 10, 1, 7, 30, 0, 0, time.UTC),
	}
	b, err := EncodeTelemetry(in)
	if err != nil {
		t.Fatalf("EncodeTelemetry: %v", err)
	}
	out, err := DecodeTelemetry(b)
	if err != nil {
		t.Fatalf("DecodeTelemetry: %v", err)
	}
	if out != in {
		t.Errorf("round trip = %+v, want %+v", out, in)
	}
}

func TestDecodeTelemetryInvalid(t *testing.T) {
	if _, err := DecodeTelemetry([]byte("not json")); err == nil {
		t.Error("DecodeTelemetry(invalid) = nil error, want error")
	}
}
