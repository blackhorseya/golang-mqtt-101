package main

import (
	"os"
	"slices"
	"syscall"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"github.com/blackhorseya/golang-mqtt-101/pkg/mqttx"
	"github.com/blackhorseya/golang-mqtt-101/pkg/mqttx/clitest"
)

type statusEvent struct {
	deviceID string
	status   mqttx.Status
}

// subscribeStatus 訂閱 devices/+/status，只回傳 devices 裡的 device 的事件
// （同一個 broker 上可能還有其他測試或 device 在發 status）。
func subscribeStatus(t *testing.T, devices []string) <-chan statusEvent {
	t.Helper()
	if os.Getenv("MQTT_INTEGRATION") != "1" {
		t.Skip("set MQTT_INTEGRATION=1 to run against a real broker")
	}
	events := make(chan statusEvent, 64)
	cfg := mqttx.ConfigFromEnv("it-status-watcher")
	client := mqtt.NewClient(cfg.ClientOptions().SetConnectRetry(true).SetConnectRetryInterval(200 * time.Millisecond))
	if tok := client.Connect(); !tok.WaitTimeout(10*time.Second) || tok.Error() != nil {
		t.Fatalf("connect: %v", tok.Error())
	}
	t.Cleanup(func() { client.Disconnect(250) })
	tok := client.Subscribe("devices/+/status", 0, func(_ mqtt.Client, m mqtt.Message) {
		id, err := mqttx.DeviceIDFromTopic(m.Topic())
		if err != nil || !slices.Contains(devices, id) {
			return
		}
		s, err := mqttx.DecodeStatus(m.Payload())
		if err != nil {
			t.Errorf("%s: %v", m.Topic(), err)
			return
		}
		events <- statusEvent{deviceID: id, status: s}
	})
	if !tok.WaitTimeout(5*time.Second) || tok.Error() != nil {
		t.Fatalf("subscribe: %v", tok.Error())
	}
	return events
}

// expectStatus 等到 devices 裡每個 device 都收到 want，timeout 就失敗。
func expectStatus(t *testing.T, events <-chan statusEvent, devices []string, want mqttx.Status, timeout time.Duration) {
	t.Helper()
	pending := slices.Clone(devices)
	deadline := time.After(timeout)
	for len(pending) > 0 {
		select {
		case e := <-events:
			if e.status != want {
				t.Fatalf("%s: got %+v while waiting for %+v", e.deviceID, e.status, want)
			}
			pending = slices.DeleteFunc(pending, func(id string) bool { return id == e.deviceID })
		case <-deadline:
			t.Fatalf("timeout waiting for %+v from %v", want, pending)
		}
	}
}

var (
	online   = mqttx.Status{State: mqttx.StateOnline, Reason: mqttx.ReasonConnected}
	graceful = mqttx.Status{State: mqttx.StateOffline, Reason: mqttx.ReasonGraceful}
	lwt      = mqttx.Status{State: mqttx.StateOffline, Reason: mqttx.ReasonLWT}
)

// kill -9：device 沒機會送 DISCONNECT，broker 代為發布 LWT。
func TestIntegrationKilledDevicePublishesLWT(t *testing.T) {
	devices := []string{"device-001", "device-002"}
	events := subscribeStatus(t, devices)

	p := clitest.Start(t, clitest.Build(t, "device"), "--count", "2", "--keepalive", "2s", "--interval", "200ms")
	expectStatus(t, events, devices, online, 10*time.Second)

	p.Signal(syscall.SIGKILL)
	p.WaitExit(3 * time.Second)
	expectStatus(t, events, devices, lwt, 10*time.Second)
}

// Ctrl+C：device 自己送 offline 再 DISCONNECT，broker 丟棄 LWT。
func TestIntegrationGracefulShutdownSkipsLWT(t *testing.T) {
	devices := []string{"device-001", "device-002"}
	events := subscribeStatus(t, devices)

	p := clitest.Start(t, clitest.Build(t, "device"), "--count", "2", "--keepalive", "2s", "--interval", "200ms")
	expectStatus(t, events, devices, online, 10*time.Second)

	p.Signal(os.Interrupt)
	p.WaitExit(3 * time.Second)
	expectStatus(t, events, devices, graceful, 10*time.Second)

	// 超過 keepalive 的 1.5 倍仍不該出現 LWT
	select {
	case e := <-events:
		t.Fatalf("unexpected status after graceful shutdown: %s %+v", e.deviceID, e.status)
	case <-time.After(4 * time.Second):
	}
}

// broker 中途消失後按 Ctrl+C：送 offline 不可能成功，但 device 仍要能結束。
func TestIntegrationInterruptAfterBrokerLost(t *testing.T) {
	if os.Getenv("MQTT_INTEGRATION") != "1" {
		t.Skip("set MQTT_INTEGRATION=1 to run against a real broker")
	}
	// 用一個會被我們關掉的 TCP proxy 模擬 broker 消失，不影響真正的 mosquitto
	proxy := clitest.NewProxy(t, mqttx.ConfigFromEnv("").BrokerURL)
	p := clitest.Start(t, clitest.Build(t, "device"), "--broker", proxy.URL(), "--keepalive", "2s", "--interval", "200ms")
	proxy.WaitForConnection(10 * time.Second)

	proxy.Close()
	time.Sleep(500 * time.Millisecond)
	p.Signal(os.Interrupt)
	p.WaitExit(5 * time.Second)
}
