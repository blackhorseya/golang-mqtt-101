// ctl 對一台 device 發送 command，並等待它的 ack。
//
//	ctl --device device-001 reboot
//	ctl --device device-001 reboot downtime=5s
//	ctl --device device-001 config interval=500ms
//
// 流程是 MQTT 上的 request/response：先訂閱 devices/{deviceID}/ack，再把帶 ID 的 command
// 發到 commands/{deviceID}/{command}，等 ID 相同的 ack 回來。收到 ok=false 或逾時都以非零結束。
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"github.com/blackhorseya/golang-mqtt-101/pkg/mqttx"
)

// 對 broker 的單次操作（訂閱、發布）最多等多久。
const brokerTimeout = 5 * time.Second

func main() {
	cfg := mqttx.ConfigFromEnv(fmt.Sprintf("ctl-%d", os.Getpid()))
	device := flag.String("device", mqttx.DeviceID("device", 1), "目標 device ID")
	timeout := flag.Duration("timeout", 10*time.Second, "等待 ack 的時間上限（reboot 要大於它的 downtime）")
	qosFlag := flag.Int("qos", 1, "command 的 QoS（0、1、2）")
	flag.StringVar(&cfg.BrokerURL, "broker", cfg.BrokerURL, "broker URL（預設讀 MQTT_BROKER_URL）")
	flag.Usage = func() {
		fmt.Fprintln(flag.CommandLine.Output(), "usage: ctl [flags] <command> [key=value ...]")
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(2)
	}
	name := flag.Arg(0)
	args, err := parseArgs(flag.Args()[1:])
	if err != nil {
		log.Fatal(err)
	}
	qos, err := mqttx.ParseQoS(*qosFlag)
	if err != nil {
		log.Fatal(err)
	}

	c, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cmd := mqttx.Command{ID: newID(), Args: args}
	ack, rtt, err := request(c, cfg, *device, name, cmd, qos, *timeout)
	if err != nil {
		log.Fatalf("%s %s %s: %v", *device, name, cmd.ID, err)
	}
	line := fmt.Sprintf("%s  ack  command=%s  id=%s  ok=%t  rtt=%s", *device, ack.Command, ack.ID, ack.OK, rtt.Round(100*time.Microsecond))
	if !ack.OK {
		fmt.Println(line + "  error=" + ack.Error)
		os.Exit(1)
	}
	fmt.Println(line)
}

// request 發出 command 並等待 ID 相同的 ack，回傳 ack 與來回時間。
func request(c context.Context, cfg mqttx.Config, device, name string, cmd mqttx.Command, qos byte, timeout time.Duration) (mqttx.Ack, time.Duration, error) {
	client := mqtt.NewClient(cfg.ClientOptions())
	if err := mqttx.Connect(c, client); err != nil {
		return mqttx.Ack{}, 0, fmt.Errorf("%s: %w", cfg.BrokerURL, err)
	}
	defer client.Disconnect(250)

	// 必須先訂閱、等到 SUBACK 才發 command：device 可能立刻回 ack，
	// 訂閱還沒生效時到達的 ack 不會被保留（不是 retained），就永遠收不到了。
	acks := make(chan mqttx.Ack, 8)
	tok := client.Subscribe(mqttx.AckTopic(device), 1, func(_ mqtt.Client, m mqtt.Message) {
		ack, err := mqttx.DecodeAck(m.Payload())
		if err != nil {
			log.Printf("%s: %v", m.Topic(), err)
			return
		}
		select {
		case acks <- ack:
		default:
		}
	})
	if err := mqttx.Wait(c, tok, brokerTimeout); err != nil {
		return mqttx.Ack{}, 0, fmt.Errorf("subscribe %s: %w", mqttx.AckTopic(device), err)
	}

	payload, err := mqttx.EncodeCommand(cmd)
	if err != nil {
		return mqttx.Ack{}, 0, err
	}
	start := time.Now()
	// command 不可以 retained：broker 會保留它，device 每次重連（例如 reboot 之後）都會再收到、再執行一次
	if err := mqttx.Wait(c, client.Publish(mqttx.CommandTopic(device, name), qos, false, payload), brokerTimeout); err != nil {
		return mqttx.Ack{}, 0, fmt.Errorf("publish %s: %w", mqttx.CommandTopic(device, name), err)
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case <-c.Done():
			return mqttx.Ack{}, 0, c.Err()
		case <-timer.C:
			// 沒有回應不代表沒執行：可能 device 不在線、command 遺失，也可能執行了但 ack 遺失
			return mqttx.Ack{}, 0, fmt.Errorf("no ack after %s: %w", timeout, errTimeout)
		case ack := <-acks:
			// 同一個 ack topic 上也有其他人的 command 的回應，只認自己的 ID
			if ack.ID == cmd.ID {
				return ack, time.Since(start), nil
			}
		}
	}
}

var errTimeout = errors.New("timeout")

// parseArgs 把 key=value 參數轉成 map；value 可以含 =。
func parseArgs(args []string) (map[string]string, error) {
	m := map[string]string{}
	for _, a := range args {
		k, v, ok := strings.Cut(a, "=")
		if !ok || k == "" {
			return nil, fmt.Errorf("parse argument %q: want key=value", a)
		}
		m[k] = v
	}
	return m, nil
}

// newID 產生 command ID，用來把 ack 對應回這次的 request。
func newID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}
