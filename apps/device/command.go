package main

import (
	"context"
	"fmt"
	"log"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"github.com/blackhorseya/golang-mqtt-101/pkg/mqttx"
)

const (
	// command 與 ack 都用 QoS 1：request/response 需要送達的保證；重複的可能性留給 command ID 處理。
	commandQoS = 1

	// reboot 沒指定 downtime 時，斷線多久再重連。
	defaultRebootDowntime = 3 * time.Second
)

// received 是收到、等待 device 主迴圈處理的 command。
type received struct {
	name string
	cmd  mqttx.Command
}

// withCommands 讓 device 每次連上都先訂閱自己的 command，再執行原本的 OnConnect（宣告 online）。
// 先訂閱再宣告 online：看到 online 就發 command 的人，才不會在訂閱生效前送出而遺失。
//
// message handler 只把 command 丟進 channel，不在裡面執行：paho 預設依序呼叫 handler，
// handler 卡住會卡住整個收訊；在 handler 裡等 publish 的確認或呼叫 Disconnect 都可能 deadlock。
func withCommands(opts *mqtt.ClientOptions, id string, commands chan<- received) *mqtt.ClientOptions {
	announce := opts.OnConnect
	handle := func(_ mqtt.Client, m mqtt.Message) {
		name, err := mqttx.CommandNameFromTopic(m.Topic())
		if err != nil {
			log.Printf("%s: %v", id, err)
			return
		}
		cmd, err := mqttx.DecodeCommand(m.Payload())
		if err != nil {
			// 沒有 ID 就無從回應，只能記下來
			log.Printf("%s: command %s: %v", id, name, err)
			return
		}
		select {
		case commands <- received{name: name, cmd: cmd}:
		default:
			log.Printf("%s: command %s %s dropped: too many pending commands", id, name, cmd.ID)
		}
	}
	return opts.SetOnConnectHandler(func(client mqtt.Client) {
		tok := client.Subscribe(mqttx.CommandFilter(id), commandQoS, handle)
		if err := mqttx.Wait(context.Background(), tok, brokerTimeout); err != nil {
			log.Printf("%s: subscribe commands: %v", id, err)
		} else {
			log.Printf("%s: subscribed to %s", id, mqttx.CommandFilter(id))
		}
		if announce != nil {
			announce(client)
		}
	})
}

// publishAck 回報 command 的執行結果；err 為 nil 表示成功。
func publishAck(c context.Context, client mqtt.Client, id string, r received, err error) {
	ack := mqttx.Ack{ID: r.cmd.ID, Command: r.name, OK: err == nil}
	if err != nil {
		ack.Error = err.Error()
	}
	// Ack 只有 string 與 bool 欄位，json.Marshal 不會失敗
	payload, _ := mqttx.EncodeAck(ack)
	if err := mqttx.Wait(c, client.Publish(mqttx.AckTopic(id), commandQoS, false, payload), brokerTimeout); err != nil {
		log.Printf("%s: publish ack %s: %v", id, r.cmd.ID, err)
		return
	}
	log.Printf("%s: ack %s %s ok=%t", id, r.name, r.cmd.ID, ack.OK)
}

// parseInterval 解析 config command 的參數：只支援 interval，且必須大於 0。
func parseInterval(args map[string]string) (time.Duration, error) {
	if err := onlyArgs(args, "interval"); err != nil {
		return 0, err
	}
	v, ok := args["interval"]
	if !ok {
		return 0, fmt.Errorf("parse config: missing interval")
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("parse interval %q: %w", v, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("parse interval %q: must be positive", v)
	}
	return d, nil
}

// parseDowntime 解析 reboot command 的參數：downtime 可省略，不可為負。
func parseDowntime(args map[string]string) (time.Duration, error) {
	if err := onlyArgs(args, "downtime"); err != nil {
		return 0, err
	}
	v, ok := args["downtime"]
	if !ok {
		return defaultRebootDowntime, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("parse downtime %q: %w", v, err)
	}
	if d < 0 {
		return 0, fmt.Errorf("parse downtime %q: must not be negative", v)
	}
	return d, nil
}

func onlyArgs(args map[string]string, allowed string) error {
	for k := range args {
		if k != allowed {
			return fmt.Errorf("unsupported argument %q (want %s)", k, allowed)
		}
	}
	return nil
}
