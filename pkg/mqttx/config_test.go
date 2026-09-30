package mqttx

import "testing"

func TestClientOptions(t *testing.T) {
	cfg := Config{BrokerURL: "tcp://127.0.0.1:1883", ClientID: "device-007"}
	opts := cfg.ClientOptions()
	if len(opts.Servers) != 1 || opts.Servers[0].String() != cfg.BrokerURL {
		t.Errorf("Servers = %v, want [%s]", opts.Servers, cfg.BrokerURL)
	}
	if opts.ClientID != cfg.ClientID {
		t.Errorf("ClientID = %q, want %q", opts.ClientID, cfg.ClientID)
	}
}
