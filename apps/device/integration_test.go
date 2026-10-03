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

// subscribeTelemetry 以指定 QoS 訂閱這組 device 的 telemetry，回傳每則訊息實際送達的 QoS。
func (x *fleet) subscribeTelemetry(name string, qos byte) <-chan byte {
	x.t.Helper()
	got := make(chan byte, 64)
	cfg := mqttx.ConfigFromEnv(x.prefix + "-" + name)
	client := mqtt.NewClient(cfg.ClientOptions().SetConnectRetry(true).SetConnectRetryInterval(200 * time.Millisecond))
	if tok := client.Connect(); !tok.WaitTimeout(10*time.Second) || tok.Error() != nil {
		x.t.Fatalf("connect %s: %v", name, tok.Error())
	}
	x.t.Cleanup(func() { client.Disconnect(250) })
	tok := client.Subscribe("devices/+/telemetry", qos, func(_ mqtt.Client, m mqtt.Message) {
		if id, err := mqttx.DeviceIDFromTopic(m.Topic()); err == nil && strings.HasPrefix(id, x.prefix+"-") {
			got <- m.Qos()
		}
	})
	if !tok.WaitTimeout(5*time.Second) || tok.Error() != nil {
		x.t.Fatalf("subscribe %s: %v", name, tok.Error())
	}
	return got
}

// 實際送達的 QoS = min(發布的 QoS, 訂閱的 QoS)：訂閱的 QoS 是上限，不會把訊息升級。
func TestIntegrationEffectiveQoS(t *testing.T) {
	cases := []struct {
		publish, subscribe, want byte
	}{
		{publish: 0, subscribe: 2, want: 0},
		{publish: 1, subscribe: 2, want: 1},
		{publish: 2, subscribe: 2, want: 2},
		{publish: 2, subscribe: 1, want: 1},
		{publish: 2, subscribe: 0, want: 0},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("publish%d_subscribe%d", tc.publish, tc.subscribe), func(t *testing.T) {
			f := newFleet(t, 1)
			got := f.subscribeTelemetry("tel", tc.subscribe)
			p := f.start("--qos", fmt.Sprint(tc.publish))

			select {
			case qos := <-got:
				if qos != tc.want {
					t.Errorf("delivered QoS = %d, want %d", qos, tc.want)
				}
			case <-time.After(10 * time.Second):
				t.Fatal("timeout waiting for telemetry")
			}
			p.Signal(os.Interrupt)
			p.WaitExit(5 * time.Second)
		})
	}
}

// startMonitor 啟動 monitor binary，只訂閱這組 device 的 telemetry；args 附加在預設參數之後。
func (x *fleet) startMonitor(args ...string) *clitest.Process {
	x.t.Helper()
	bin := clitest.BuildDir(x.t, "monitor", "../monitor")
	base := []string{"--topic", "devices/" + x.devices[0] + "/telemetry", "--report", "0"}
	p := clitest.Start(x.t, bin, append(base, args...)...)
	p.WaitOutput("subscribed to", 10*time.Second)
	return p
}

// 應用層重送（--dup-rate 1：每筆 telemetry 都再發一次）是新的 MQTT 訊息，
// 即使用 QoS 2，broker 也照樣轉發兩次 —— 只有 monitor 依序號判斷才擋得住。
func TestIntegrationDuplicateSurvivesQoS2(t *testing.T) {
	f := newFleet(t, 1)
	m := f.startMonitor()

	p := f.start("--qos", "2", "--dup-rate", "1")
	// 重送是在原本那則確認之後才發，所以要等第二份到了才能中斷 device
	m.WaitCount("seq=1  qos=2", 2, 10*time.Second)
	m.WaitCount("seq=2  qos=2", 2, 10*time.Second)
	p.Signal(os.Interrupt)
	p.WaitExit(5 * time.Second)

	// 每個 seq 的兩份裡，第一份被接受、第二份被判為重複
	out := m.Output()
	for _, seq := range []string{"seq=1  qos=2", "seq=2  qos=2"} {
		var verdicts []string
		for line := range strings.SplitSeq(out, "\n") {
			if strings.Contains(line, seq) {
				verdicts = append(verdicts, line[strings.LastIndex(line, "  ")+2:])
			}
		}
		if len(verdicts) < 2 || verdicts[0] != "accepted" || verdicts[1] != "duplicate" {
			t.Errorf("%s verdicts = %q, want [accepted duplicate ...]\n%s", seq, verdicts, out)
		}
	}
}

// device 重啟後序號從 1 重來；run 不同，monitor 要當成新的序列，而不是一路判成 out-of-order。
func TestIntegrationRestartedDeviceStartsNewSequence(t *testing.T) {
	f := newFleet(t, 1)
	m := f.startMonitor()

	first := f.start()
	m.WaitOutput("seq=3", 10*time.Second)
	first.Signal(syscall.SIGKILL)
	first.WaitExit(3 * time.Second)

	second := f.start()
	// 第一次執行與重啟後各出現一次 seq=2
	m.WaitCount("seq=2  ", 2, 10*time.Second)
	second.Signal(os.Interrupt)
	second.WaitExit(5 * time.Second)

	if out := m.Output(); strings.Contains(out, "out-of-order") || strings.Contains(out, "duplicate") {
		t.Errorf("restart was misclassified\n%s", out)
	}
}

// outageArgs 讓連線在啟動 1.5 秒後斷 1 秒（只斷一次：測試在下一次之前就結束），
// 並把重連的 backoff 上限壓低，不然 paho 預設的 backoff 會拖長測試。
var outageArgs = []string{"--outage-every", "1500ms", "--outage-for", "1s", "--max-reconnect-interval", "500ms"}

// stopAfterReconnect 等 p 的輸出出現 n 次 marker（第 2 次以後代表重連成功），再讓資料多跑一下後中斷 device。
func stopAfterReconnect(device, waitOn *clitest.Process, marker string) {
	waitOn.WaitCount(marker, 2, 15*time.Second)
	time.Sleep(time.Second)
	device.Signal(os.Interrupt)
	device.WaitExit(5 * time.Second)
}

// device 斷線期間 paho 會直接丟掉 QoS 0 的 publish（token 仍回報成功），monitor 依序號看到 gap。
func TestIntegrationOutageDropsQoS0(t *testing.T) {
	f := newFleet(t, 1)
	m := f.startMonitor()

	p := f.start(append([]string{"--qos", "0"}, outageArgs...)...)
	p.WaitOutput("connection lost", 10*time.Second)
	stopAfterReconnect(p, p, "status online")

	if out := m.Output(); !strings.Contains(out, "gap(missing=") {
		t.Errorf("QoS 0 messages published during the outage should be lost\n%s", out)
	}
}

// QoS 1 的 publish 在斷線期間存在 client 端，重連後補送：monitor 不會看到 gap（可能看到 duplicate）。
func TestIntegrationOutageResendsQoS1(t *testing.T) {
	f := newFleet(t, 1)
	m := f.startMonitor("--qos", "1")

	p := f.start(append([]string{"--qos", "1"}, outageArgs...)...)
	p.WaitOutput("connection lost", 10*time.Second)
	stopAfterReconnect(p, p, "status online")

	out := m.Output()
	if strings.Contains(out, "gap(missing=") {
		t.Errorf("QoS 1 messages should be resent after reconnect, not lost\n%s", out)
	}
	if !strings.Contains(out, "seq=") {
		t.Errorf("monitor received no telemetry\n%s", out)
	}
}

// subscriber 斷線時：clean session 下 broker 不替它保留訊息，重連後出現 gap；
// persistent session（--clean-session=false）下 broker 替它排隊 QoS 1 訊息，重連後一次補齊。
func TestIntegrationMonitorSession(t *testing.T) {
	cases := []struct {
		cleanSession string
		wantGap      bool
	}{
		{cleanSession: "true", wantGap: true},
		{cleanSession: "false", wantGap: false},
	}
	for _, tc := range cases {
		t.Run("clean_session_"+tc.cleanSession, func(t *testing.T) {
			f := newFleet(t, 1)
			m := f.startMonitor(append([]string{"--qos", "1", "--clean-session=" + tc.cleanSession}, outageArgs...)...)

			p := f.start("--qos", "1")
			m.WaitOutput("connection lost", 10*time.Second)
			stopAfterReconnect(p, m, "subscribed to")

			out := m.Output()
			if got := strings.Contains(out, "gap(missing="); got != tc.wantGap {
				t.Errorf("gap = %t, want %t\n%s", got, tc.wantGap, out)
			}
		})
	}
}
