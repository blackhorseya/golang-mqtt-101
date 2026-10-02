package main

import (
	"fmt"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"github.com/blackhorseya/golang-mqtt-101/pkg/mqttx"
	"github.com/blackhorseya/golang-mqtt-101/pkg/mqttx/clitest"
)

// 整合測試共用同一個 broker，而 retained status 會留在 broker 上。
// 每個測試用自己的 device ID 前綴，並在結束時清掉留下的 retained message，彼此才不會互相干擾。

type statusEvent struct {
	deviceID string
	status   mqttx.Status
	retained bool // true 表示因為訂閱而收到的 retained message，而非即時轉發
	cleared  bool // 空 payload：retained message 被清除
}

var (
	online   = mqttx.Status{State: mqttx.StateOnline, Reason: mqttx.ReasonConnected}
	graceful = mqttx.Status{State: mqttx.StateOffline, Reason: mqttx.ReasonGraceful}
	lwt      = mqttx.Status{State: mqttx.StateOffline, Reason: mqttx.ReasonLWT}
)

func requireIntegration(t *testing.T) {
	t.Helper()
	if os.Getenv("MQTT_INTEGRATION") != "1" {
		t.Skip("set MQTT_INTEGRATION=1 to run against a real broker")
	}
}

// fleet 是某個測試專屬的一組 device。
type fleet struct {
	t       *testing.T
	bin     string
	prefix  string
	devices []string
}

// newFleet 編譯 device binary、配一個唯一前綴，並在測試結束時清掉這組 device 的 retained status。
func newFleet(t *testing.T, count int) *fleet {
	t.Helper()
	requireIntegration(t)
	f := &fleet{t: t, bin: clitest.Build(t, "device"), prefix: fmt.Sprintf("it%d", time.Now().UnixNano())}
	for n := 1; n <= count; n++ {
		f.devices = append(f.devices, mqttx.DeviceID(f.prefix, n))
	}
	t.Cleanup(f.clearRetained)
	return f
}

// start 啟動這組 device。
func (x *fleet) start(args ...string) *clitest.Process {
	x.t.Helper()
	base := []string{"--prefix", x.prefix, "--count", fmt.Sprint(len(x.devices)), "--keepalive", "2s", "--interval", "200ms"}
	return clitest.Start(x.t, x.bin, append(base, args...)...)
}

// clearRetained 執行 device --clear-retained，清除這組 device 的 retained status。
func (x *fleet) clearRetained() {
	x.t.Helper()
	p := clitest.Start(x.t, x.bin, "--prefix", x.prefix, "--count", fmt.Sprint(len(x.devices)), "--clear-retained")
	p.WaitExit(10 * time.Second)
}

// subscribe 以新的 client 訂閱 devices/+/status，只回傳這組 device 的事件。
func (x *fleet) subscribe(name string) <-chan statusEvent {
	x.t.Helper()
	events := make(chan statusEvent, 64)
	cfg := mqttx.ConfigFromEnv(x.prefix + "-" + name)
	client := mqtt.NewClient(cfg.ClientOptions().SetConnectRetry(true).SetConnectRetryInterval(200 * time.Millisecond))
	if tok := client.Connect(); !tok.WaitTimeout(10*time.Second) || tok.Error() != nil {
		x.t.Fatalf("connect %s: %v", name, tok.Error())
	}
	x.t.Cleanup(func() { client.Disconnect(250) })
	tok := client.Subscribe("devices/+/status", 0, func(_ mqtt.Client, m mqtt.Message) {
		id, err := mqttx.DeviceIDFromTopic(m.Topic())
		if err != nil || !strings.HasPrefix(id, x.prefix+"-") {
			return
		}
		e := statusEvent{deviceID: id, retained: m.Retained(), cleared: len(m.Payload()) == 0}
		if !e.cleared {
			if e.status, err = mqttx.DecodeStatus(m.Payload()); err != nil {
				x.t.Errorf("%s: %v", m.Topic(), err)
				return
			}
		}
		events <- e
	})
	if !tok.WaitTimeout(5*time.Second) || tok.Error() != nil {
		x.t.Fatalf("subscribe %s: %v", name, tok.Error())
	}
	return events
}

// expect 等到每個 device 都收到符合 match 的事件；收到其他事件就失敗。
func (x *fleet) expect(events <-chan statusEvent, desc string, match func(statusEvent) bool, timeout time.Duration) {
	x.t.Helper()
	pending := map[string]bool{}
	for _, id := range x.devices {
		pending[id] = true
	}
	deadline := time.After(timeout)
	for len(pending) > 0 {
		select {
		case e := <-events:
			if !match(e) {
				x.t.Fatalf("%s: got %+v while waiting for %s", e.deviceID, e, desc)
			}
			delete(pending, e.deviceID)
		case <-deadline:
			x.t.Fatalf("timeout waiting for %s from %v", desc, pending)
		}
	}
}

func (x *fleet) expectStatus(events <-chan statusEvent, want mqttx.Status, retained bool, timeout time.Duration) {
	x.t.Helper()
	desc := fmt.Sprintf("%+v retained=%v", want, retained)
	x.expect(events, desc, func(e statusEvent) bool {
		return !e.cleared && e.status == want && e.retained == retained
	}, timeout)
}

// expectNothing 要求 d 內不再收到任何事件。
func expectNothing(t *testing.T, events <-chan statusEvent, d time.Duration) {
	t.Helper()
	select {
	case e := <-events:
		t.Fatalf("unexpected status event: %+v", e)
	case <-time.After(d):
	}
}

// kill -9：device 沒機會送 DISCONNECT，broker 代為發布 LWT；LWT 是 retained，之後才訂閱的 client 也看得到。
func TestIntegrationKilledDevicePublishesLWT(t *testing.T) {
	f := newFleet(t, 2)
	live := f.subscribe("live")

	p := f.start()
	f.expectStatus(live, online, false, 10*time.Second)

	p.Signal(syscall.SIGKILL)
	p.WaitExit(3 * time.Second)
	f.expectStatus(live, lwt, false, 10*time.Second)

	// broker 只保留最新的一則：晚到的訂閱者只收到 LWT，不會收到之前的 online
	late := f.subscribe("late")
	f.expectStatus(late, lwt, true, 5*time.Second)
	expectNothing(t, late, time.Second)
}

// Ctrl+C：device 自己送 offline 再 DISCONNECT，broker 丟棄 LWT；保留的是 graceful offline。
func TestIntegrationGracefulShutdownSkipsLWT(t *testing.T) {
	f := newFleet(t, 2)
	live := f.subscribe("live")

	p := f.start()
	f.expectStatus(live, online, false, 10*time.Second)

	p.Signal(os.Interrupt)
	p.WaitExit(3 * time.Second)
	f.expectStatus(live, graceful, false, 10*time.Second)

	// 超過 keepalive 的 1.5 倍仍不該出現 LWT
	expectNothing(t, live, 4*time.Second)

	late := f.subscribe("late")
	f.expectStatus(late, graceful, true, 5*time.Second)
	expectNothing(t, late, time.Second)
}

// device 在線時才開始訂閱的 client，一訂閱就從 broker 拿到 retained online。
func TestIntegrationLateSubscriberSeesRetainedOnline(t *testing.T) {
	f := newFleet(t, 2)
	live := f.subscribe("live")

	f.start()
	f.expectStatus(live, online, false, 10*time.Second)

	late := f.subscribe("late")
	f.expectStatus(late, online, true, 5*time.Second)
}

// --retain=false：重現 Phase 2，晚到的訂閱者收不到任何 status。
func TestIntegrationRetainDisabled(t *testing.T) {
	f := newFleet(t, 2)
	live := f.subscribe("live")

	f.start("--retain=false")
	f.expectStatus(live, online, false, 10*time.Second)

	late := f.subscribe("late")
	expectNothing(t, late, 2*time.Second)
}

// --clear-retained 發布空的 retained payload：現有訂閱者收到清除通知，之後訂閱的 client 什麼都收不到。
func TestIntegrationClearRetained(t *testing.T) {
	f := newFleet(t, 2)
	live := f.subscribe("live")

	p := f.start()
	f.expectStatus(live, online, false, 10*time.Second)
	p.Signal(os.Interrupt)
	p.WaitExit(3 * time.Second)
	f.expectStatus(live, graceful, false, 10*time.Second)

	f.clearRetained()
	f.expect(live, "cleared", func(e statusEvent) bool { return e.cleared }, 5*time.Second)

	late := f.subscribe("late")
	expectNothing(t, late, 2*time.Second)
}

// broker 中途消失後按 Ctrl+C：送 offline 不可能成功，但 device 仍要能結束。
func TestIntegrationInterruptAfterBrokerLost(t *testing.T) {
	f := newFleet(t, 1)
	// 用一個會被我們關掉的 TCP proxy 模擬 broker 消失，不影響真正的 mosquitto
	proxy := clitest.NewProxy(t, mqttx.ConfigFromEnv("").BrokerURL)
	p := f.start("--broker", proxy.URL())
	proxy.WaitForConnection(10 * time.Second)

	proxy.Close()
	time.Sleep(500 * time.Millisecond)
	p.Signal(os.Interrupt)
	p.WaitExit(5 * time.Second)
}
