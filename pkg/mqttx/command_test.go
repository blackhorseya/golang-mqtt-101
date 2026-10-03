package mqttx

import (
	"reflect"
	"testing"
)

func TestCommandTopics(t *testing.T) {
	cases := []struct{ got, want string }{
		{CommandFilter("device-001"), "commands/device-001/#"},
		{CommandTopic("device-001", "reboot"), "commands/device-001/reboot"},
		// ack 不能放在 commands/{id}/ 底下：device 訂閱 commands/{id}/#，會收到自己發的 ack
		{AckTopic("device-001"), "devices/device-001/ack"},
	}
	for _, tc := range cases {
		if tc.got != tc.want {
			t.Errorf("got %q, want %q", tc.got, tc.want)
		}
	}
}

func TestCommandNameFromTopic(t *testing.T) {
	name, err := CommandNameFromTopic("commands/device-001/config")
	if err != nil || name != "config" {
		t.Errorf("CommandNameFromTopic = %q, %v, want config", name, err)
	}
	for _, topic := range []string{"commands/device-001", "commands/device-001/", "devices/device-001/config"} {
		if _, err := CommandNameFromTopic(topic); err == nil {
			t.Errorf("CommandNameFromTopic(%q) = nil error, want error", topic)
		}
	}
}

func TestCommandRoundTrip(t *testing.T) {
	in := Command{ID: "abc", Args: map[string]string{"interval": "500ms"}}
	b, err := EncodeCommand(in)
	if err != nil {
		t.Fatal(err)
	}
	out, err := DecodeCommand(b)
	if err != nil || !reflect.DeepEqual(out, in) {
		t.Errorf("round trip %+v = %+v, %v", in, out, err)
	}
	if _, err := DecodeCommand([]byte("reboot")); err == nil {
		t.Error("DecodeCommand(non-JSON) = nil error, want error")
	}
	// 沒有 id 就無法對應回應，視為無效
	if _, err := DecodeCommand([]byte(`{"args":{}}`)); err == nil {
		t.Error("DecodeCommand(without id) = nil error, want error")
	}
}

func TestAckRoundTrip(t *testing.T) {
	for _, in := range []Ack{
		{ID: "abc", Command: "reboot", OK: true},
		{ID: "def", Command: "config", OK: false, Error: "bad interval"},
	} {
		b, err := EncodeAck(in)
		if err != nil {
			t.Fatal(err)
		}
		out, err := DecodeAck(b)
		if err != nil || out != in {
			t.Errorf("round trip %+v = %+v, %v", in, out, err)
		}
	}
	if _, err := DecodeAck([]byte("ok")); err == nil {
		t.Error("DecodeAck(non-JSON) = nil error, want error")
	}
}
