// device 模擬多個 IoT device：每個 device 是一個獨立的 MQTT client，
// 連上後宣告 online（retained）、定期發送 telemetry，並在連線時設定 LWT。
//
//	device --count 10 --interval 1s --keepalive 10s
//	device --count 10 --clear-retained   # 清除這些 device 留在 broker 上的 retained status
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand/v2"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"github.com/blackhorseya/golang-mqtt-101/pkg/mqttx"
)

const (
	// QoS 目前固定為 0（at most once）；Phase 4 才開放設定。
	telemetryQoS = 0
	statusQoS    = 0

	// 對 broker 的單次操作最多等多久；斷線時不能無限等下去。
	brokerTimeout = 2 * time.Second
)

func main() {
	cfg := mqttx.ConfigFromEnv("")
	count := flag.Int("count", 1, "要模擬的 device 數量")
	interval := flag.Duration("interval", 2*time.Second, "每個 device 發送 telemetry 的間隔")
	keepAlive := flag.Duration("keepalive", 10*time.Second, "MQTT keep alive；broker 超過 1.5 倍時間沒收到封包就判定斷線並發布 LWT")
	prefix := flag.String("prefix", "device", "device ID 前綴，ID 為 {prefix}-001、{prefix}-002…")
	retain := flag.Bool("retain", true, "status 與 LWT 以 retained message 發布（--retain=false 重現 Phase 2 行為）")
	clearOnly := flag.Bool("clear-retained", false, "不啟動 device，改為清除這些 device 的 retained status 後結束")
	flag.StringVar(&cfg.BrokerURL, "broker", cfg.BrokerURL, "broker URL（預設讀 MQTT_BROKER_URL）")
	flag.Parse()

	c, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if *clearOnly {
		if err := clearRetained(c, cfg, *prefix, *count); err != nil {
			log.Fatalf("clear retained: %v", err)
		}
		return
	}

	var wg sync.WaitGroup
	for n := 1; n <= *count; n++ {
		devCfg := cfg
		devCfg.ClientID = mqttx.DeviceID(*prefix, n)
		wg.Go(func() { runDevice(c, deviceOptions(devCfg, *keepAlive, *retain), devCfg, *interval, *retain) })
	}
	wg.Wait()
}

// deviceOptions 建立 device 的連線選項。
//
// LWT（Last Will and Testament）在 CONNECT 時就交給 broker 保管：
// 若 device 沒送 DISCONNECT 就斷線（process 被 kill、網路中斷、keep alive 逾時），
// broker 會代為發布這則訊息；正常 DISCONNECT 時 broker 則丟棄它。
//
// retain 為 true 時 status 與 LWT 都是 retained：broker 對每個 topic 保留最新一則，
// 之後才訂閱的 client 一訂閱就會收到。LWT 也必須 retained，否則 device 當掉後 broker 保留的仍是 online。
func deviceOptions(cfg mqttx.Config, keepAlive time.Duration, retain bool) *mqtt.ClientOptions {
	id := cfg.ClientID
	return cfg.ClientOptions().
		SetKeepAlive(keepAlive).
		SetBinaryWill(mqttx.StatusTopic(id), statusPayload(mqttx.StateOffline, mqttx.ReasonLWT), statusQoS, retain).
		SetConnectRetry(true).
		SetConnectRetryInterval(time.Second).
		// 每次連上（包含自動重連）都重新宣告 online
		SetOnConnectHandler(func(client mqtt.Client) {
			publishStatus(context.Background(), client, id, mqttx.StateOnline, mqttx.ReasonConnected, retain)
		})
}

// runDevice 連上 broker 後每隔 interval 發一次 telemetry，直到 c 被取消才正常下線。
func runDevice(c context.Context, opts *mqtt.ClientOptions, cfg mqttx.Config, interval time.Duration, retain bool) {
	id := cfg.ClientID
	client := mqtt.NewClient(opts)
	if err := mqttx.Connect(c, client); err != nil {
		log.Printf("%s: %s: %v", id, cfg.BrokerURL, err)
		return
	}
	log.Printf("%s: connected to %s", id, cfg.BrokerURL)

	s := newSensor(id, rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64())))
	topic := mqttx.TelemetryTopic(id)
	published := 0

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-c.Done():
			// 正常下線：先自己宣告 offline，再送 DISCONNECT（broker 因此不會發布 LWT）。
			// 此時 c 已取消，所以用新的 context；broker 不在時最多等 brokerTimeout。
			publishStatus(context.Background(), client, id, mqttx.StateOffline, mqttx.ReasonGraceful, retain)
			client.Disconnect(250)
			log.Printf("%s: disconnected, published %d telemetry", id, published)
			return
		case now := <-ticker.C:
			payload, err := mqttx.EncodeTelemetry(s.next(now))
			if err != nil {
				log.Printf("%s: %v", id, err)
				continue
			}
			// QoS 0 的 token 在送進網路層後就完成，不代表 broker 或任何 subscriber 收到了
			if err := mqttx.Wait(c, client.Publish(topic, telemetryQoS, false, payload), brokerTimeout); err != nil {
				log.Printf("%s: publish %s: %v", id, topic, err)
				continue
			}
			published++
		}
	}
}

func publishStatus(c context.Context, client mqtt.Client, id string, state mqttx.State, reason mqttx.Reason, retain bool) {
	tok := client.Publish(mqttx.StatusTopic(id), statusQoS, retain, statusPayload(state, reason))
	if err := mqttx.Wait(c, tok, brokerTimeout); err != nil {
		log.Printf("%s: publish status %s: %v", id, state, err)
		return
	}
	log.Printf("%s: status %s (%s)", id, state, reason)
}

func statusPayload(state mqttx.State, reason mqttx.Reason) []byte {
	// Status 只有兩個 string 欄位，json.Marshal 不會失敗
	b, _ := mqttx.EncodeStatus(mqttx.Status{State: state, Reason: reason})
	return b
}

// clearRetained 對每個 device 的 status topic 發布空的 retained payload：
// broker 收到後刪除該 topic 保留的訊息（並照常轉發給當下的訂閱者）。
func clearRetained(c context.Context, cfg mqttx.Config, prefix string, count int) error {
	cfg.ClientID = prefix + "-clear"
	client := mqtt.NewClient(cfg.ClientOptions())
	if err := mqttx.Connect(c, client); err != nil {
		return fmt.Errorf("%s: %w", cfg.BrokerURL, err)
	}
	defer client.Disconnect(250)

	for n := 1; n <= count; n++ {
		topic := mqttx.StatusTopic(mqttx.DeviceID(prefix, n))
		if err := mqttx.Wait(c, client.Publish(topic, statusQoS, true, []byte{}), brokerTimeout); err != nil {
			return fmt.Errorf("clear %s: %w", topic, err)
		}
		log.Printf("cleared retained %s", topic)
	}
	return nil
}
