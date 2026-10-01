package mqttx

import "testing"

func TestStatusTopic(t *testing.T) {
	if got, want := StatusTopic("device-001"), "devices/device-001/status"; got != want {
		t.Errorf("StatusTopic = %q, want %q", got, want)
	}
}

func TestStatusRoundTrip(t *testing.T) {
	for _, in := range []Status{
		{State: StateOnline, Reason: ReasonConnected},
		{State: StateOffline, Reason: ReasonGraceful},
		{State: StateOffline, Reason: ReasonLWT},
	} {
		b, err := EncodeStatus(in)
		if err != nil {
			t.Fatalf("EncodeStatus(%+v): %v", in, err)
		}
		out, err := DecodeStatus(b)
		if err != nil || out != in {
			t.Errorf("round trip %+v = %+v, %v", in, out, err)
		}
	}
}

func TestDecodeStatusInvalid(t *testing.T) {
	if _, err := DecodeStatus([]byte("online")); err == nil {
		t.Error("DecodeStatus(non-JSON) = nil error, want error")
	}
}
