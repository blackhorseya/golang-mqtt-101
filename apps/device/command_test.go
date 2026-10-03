package main

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"github.com/blackhorseya/golang-mqtt-101/pkg/mqttx"
)

// doneToken 是已完成的 token，Error 回傳 err；result 是 SUBACK 的結果（和 paho 的 *SubscribeToken 一樣）。
type doneToken struct {
	err    error
	result map[string]byte
}

func (doneToken) Wait() bool                     { return true }
func (doneToken) WaitTimeout(time.Duration) bool { return true }
func (doneToken) Done() <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}
func (x doneToken) Error() error            { return x.err }
func (x doneToken) Result() map[string]byte { return x.result }

// subscribeClient 是只實作 Subscribe 與 IsConnectionOpen 的假 client：前 failures 次訂閱失敗
// （suback 為 true 時是 broker 回 SUBACK 0x80 拒絕，否則是 token 帶錯誤）；open 為 false 時表示連線已經斷了。
type subscribeClient struct {
	mqtt.Client
	failures int
	suback   bool
	calls    int
	open     bool
}

func (x *subscribeClient) Subscribe(filter string, _ byte, _ mqtt.MessageHandler) mqtt.Token {
	x.calls++
	switch {
	case x.calls > x.failures:
		return doneToken{result: map[string]byte{filter: 1}}
	case x.suback:
		return doneToken{result: map[string]byte{filter: 0x80}}
	default:
		return doneToken{err: errors.New("connection lost")}
	}
}

func (x *subscribeClient) IsConnectionOpen() bool { return x.open }

// connectWith 以 withCommands 包過的 OnConnect 模擬一次連上，回傳原本的 OnConnect（宣告 online）被呼叫幾次。
func connectWith(client mqtt.Client) int32 {
	var announced atomic.Int32
	opts := mqttx.Config{ClientID: "device-001"}.ClientOptions().
		SetOnConnectHandler(func(mqtt.Client) { announced.Add(1) })
	opts = withCommands(opts, "device-001", make(chan received, 1))
	opts.OnConnect(client)
	return announced.Load()
}

// 訂閱 command 失敗時不能宣告 online：看到 online 就發 command 的人會發到一個沒人訂閱的 topic。
// 連線還在就重試訂閱，成功後才宣告 online。
func TestWithCommandsRetriesSubscribeBeforeOnline(t *testing.T) {
	subscribeRetryInterval = time.Millisecond
	t.Cleanup(func() { subscribeRetryInterval = time.Second })
	client := &subscribeClient{failures: 2, open: true}
	if n := connectWith(client); n != 1 {
		t.Errorf("announced online %d times, want 1", n)
	}
	if client.calls != 3 {
		t.Errorf("subscribe called %d times, want 3", client.calls)
	}
}

// broker 以 SUBACK 0x80 拒絕訂閱時，paho 的 token 沒有錯誤；也必須當成失敗重試，不能宣告 online。
func TestWithCommandsRetriesRefusedSubscription(t *testing.T) {
	subscribeRetryInterval = time.Millisecond
	t.Cleanup(func() { subscribeRetryInterval = time.Second })
	client := &subscribeClient{failures: 2, suback: true, open: true}
	if n := connectWith(client); n != 1 {
		t.Errorf("announced online %d times, want 1", n)
	}
	if client.calls != 3 {
		t.Errorf("subscribe called %d times, want 3", client.calls)
	}
}

// 連線已經斷了就不再重試，也不宣告 online；自動重連後 OnConnect 會再被呼叫一次。
func TestWithCommandsGivesUpWhenConnectionLost(t *testing.T) {
	client := &subscribeClient{failures: 1, open: false}
	if n := connectWith(client); n != 0 {
		t.Errorf("announced online %d times, want 0", n)
	}
}

func TestParseInterval(t *testing.T) {
	got, err := parseInterval(map[string]string{"interval": "500ms"})
	if err != nil || got != 500*time.Millisecond {
		t.Errorf("parseInterval = %s, %v, want 500ms", got, err)
	}
	for _, args := range []map[string]string{
		{},                             // 缺 interval
		{"interval": "fast"},           // 不是 duration
		{"interval": "0s"},             // 必須大於 0
		{"interval": "1s", "qos": "2"}, // 不支援的參數
	} {
		if _, err := parseInterval(args); err == nil {
			t.Errorf("parseInterval(%v) = nil error, want error", args)
		}
	}
}

func TestParseDowntime(t *testing.T) {
	if got, err := parseDowntime(map[string]string{}); err != nil || got != defaultRebootDowntime {
		t.Errorf("parseDowntime(no args) = %s, %v, want %s", got, err, defaultRebootDowntime)
	}
	if got, err := parseDowntime(map[string]string{"downtime": "1s"}); err != nil || got != time.Second {
		t.Errorf("parseDowntime(1s) = %s, %v, want 1s", got, err)
	}
	for _, args := range []map[string]string{{"downtime": "-1s"}, {"downtime": "soon"}, {"delay": "1s"}} {
		if _, err := parseDowntime(args); err == nil {
			t.Errorf("parseDowntime(%v) = nil error, want error", args)
		}
	}
}
