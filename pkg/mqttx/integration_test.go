package mqttx

import (
	"fmt"
	"os"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// 整合測試需要真的 broker：MQTT_INTEGRATION=1 才跑（見 task test:integration）。
func requireBroker(t *testing.T) Config {
	t.Helper()
	if os.Getenv("MQTT_INTEGRATION") != "1" {
		t.Skip("set MQTT_INTEGRATION=1 to run against a real broker")
	}
	return ConfigFromEnv("")
}

func connect(t *testing.T, cfg Config, clientID string) mqtt.Client {
	t.Helper()
	cfg.ClientID = clientID
	opts := cfg.ClientOptions().SetConnectRetry(true).SetConnectRetryInterval(200 * time.Millisecond)
	client := mqtt.NewClient(opts)
	if tok := client.Connect(); !tok.WaitTimeout(10*time.Second) || tok.Error() != nil {
		t.Fatalf("connect %s: %v", clientID, tok.Error())
	}
	t.Cleanup(func() { client.Disconnect(250) })
	return client
}

// 驗證 `+` 只匹配單一層級、`#` 匹配其下所有層級：
// 同一個 device 先發 status 再發 telemetry，
// devices/+/telemetry 只該收到 telemetry；devices/# 兩則都該收到。
func TestIntegrationWildcards(t *testing.T) {
	cfg := requireBroker(t)
	deviceID := fmt.Sprintf("it-%d", time.Now().UnixNano())

	cases := []struct {
		filter string
		want   []string
	}{
		{filter: "devices/+/telemetry", want: []string{TelemetryTopic(deviceID)}},
		{filter: "devices/#", want: []string{StatusTopic(deviceID), TelemetryTopic(deviceID)}},
	}
	for _, tc := range cases {
		t.Run(tc.filter, func(t *testing.T) {
			got := make(chan mqtt.Message, 16)
			sub := connect(t, cfg, deviceID+"-sub")
			tok := sub.Subscribe(tc.filter, 0, func(_ mqtt.Client, m mqtt.Message) {
				// 只看本次測試的 device，避免同時有其他 device 在發
				if id, err := DeviceIDFromTopic(m.Topic()); err == nil && id == deviceID {
					got <- m
				}
			})
			if !tok.WaitTimeout(5*time.Second) || tok.Error() != nil {
				t.Fatalf("subscribe %s: %v", tc.filter, tok.Error())
			}

			pub := connect(t, cfg, deviceID+"-pub")
			in := Telemetry{DeviceID: deviceID, Temp: 25.5, Humidity: 50, Battery: 99, Timestamp: time.Now().UTC().Truncate(time.Millisecond)}
			payload, err := EncodeTelemetry(in)
			if err != nil {
				t.Fatal(err)
			}
			status, err := EncodeStatus(Status{State: StateOnline, Reason: ReasonConnected})
			if err != nil {
				t.Fatal(err)
			}
			pub.Publish(StatusTopic(deviceID), 0, false, status).WaitTimeout(5 * time.Second)
			pub.Publish(TelemetryTopic(deviceID), 0, false, payload).WaitTimeout(5 * time.Second)

			// telemetry 一定最後到（同一 client 的訊息依序轉發），收到它就代表不會再有 status
			var topics []string
			for {
				select {
				case m := <-got:
					topics = append(topics, m.Topic())
					if m.Topic() != TelemetryTopic(deviceID) {
						continue
					}
					out, err := DecodeTelemetry(m.Payload())
					if err != nil || out != in {
						t.Fatalf("telemetry = %+v, %v; want %+v", out, err, in)
					}
				case <-time.After(10 * time.Second):
					t.Fatalf("timeout; received topics %v, want %v", topics, tc.want)
				}
				if topics[len(topics)-1] == TelemetryTopic(deviceID) {
					break
				}
			}
			if fmt.Sprint(topics) != fmt.Sprint(tc.want) {
				t.Errorf("received topics %v, want %v", topics, tc.want)
			}
		})
	}
}
