// device 模擬多個 IoT device：每個 device 是一個獨立的 MQTT client，
// 連上後宣告 online（retained）、定期發送 telemetry，並在連線時設定 LWT。
//
//	device --count 10 --interval 1s --keepalive 10s --qos 1
//	device --qos 2 --dup-rate 0.2        # 20% 的 telemetry 由應用程式再發一次
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
	// status 與 LWT 固定 QoS 0；telemetry 的 QoS 由 --qos 設定。
	statusQoS = 0

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
	qosFlag := flag.Int("qos", 0, "telemetry 的 QoS（0、1、2）")
	report := flag.Duration("report", 5*time.Second, "每隔多久印一次發布統計（0 表示只在結束時印）")
	dupFlag := flag.Float64("dup-rate", 0, "每筆 telemetry 由應用程式再發一次的機率（0–1），模擬應用層重送")
	skipFlag := flag.Float64("skip-rate", 0, "每筆 telemetry 用掉序號但不發出的機率（0–1），模擬送出前遺失")
	flag.StringVar(&cfg.BrokerURL, "broker", cfg.BrokerURL, "broker URL（預設讀 MQTT_BROKER_URL）")
	flag.Parse()

	qos, err := mqttx.ParseQoS(*qosFlag)
	if err != nil {
		log.Fatal(err)
	}
	dupRate, err := parseRate("dup-rate", *dupFlag)
	if err != nil {
		log.Fatal(err)
	}
	skipRate, err := parseRate("skip-rate", *skipFlag)
	if err != nil {
		log.Fatal(err)
	}

	c, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if *clearOnly {
		if err := clearRetained(c, cfg, *prefix, *count); err != nil {
			log.Fatalf("clear retained: %v", err)
		}
		return
	}

	sim := &simulator{
		interval: *interval,
		qos:      qos,
		retain:   *retain,
		dupRate:  dupRate,
		skipRate: skipRate,
		// 這次 process 的識別；device 重啟後序號從 1 重來，subscriber 靠 run 不同分辨
		run:     time.Now().UnixNano(),
		stats:   &publishStats{},
		traffic: &mqttx.Traffic{},
	}
	stopReport := sim.reportEvery(*report)

	var wg sync.WaitGroup
	for n := 1; n <= *count; n++ {
		devCfg := cfg
		devCfg.ClientID = mqttx.DeviceID(*prefix, n)
		opts := deviceOptions(devCfg, *keepAlive, *retain).SetCustomOpenConnectionFn(sim.traffic.OpenConnection())
		wg.Go(func() { sim.runDevice(c, opts, devCfg) })
	}
	wg.Wait()
	stopReport()
	log.Printf("stats (qos %d): %s", qos, sim.summary())
}

// parseRate 檢查機率是否在 [0, 1]。
func parseRate(name string, v float64) (float64, error) {
	if v < 0 || v > 1 {
		return 0, fmt.Errorf("parse %s %g: must be between 0 and 1", name, v)
	}
	return v, nil
}

// simulator 是所有 device 共用的設定與統計。
type simulator struct {
	interval time.Duration
	qos      byte
	retain   bool
	dupRate  float64
	skipRate float64
	run      int64
	stats    *publishStats
	traffic  *mqttx.Traffic
}

func (s *simulator) summary() string {
	return s.stats.summary(s.traffic.Sent(), s.traffic.Received())
}

// reportEvery 每隔 d 印一次統計；回傳的函式停止回報。d 為 0 時不回報。
func (s *simulator) reportEvery(d time.Duration) (stop func()) {
	if d <= 0 {
		return func() {}
	}
	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Go(func() {
		ticker := time.NewTicker(d)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				log.Printf("stats (qos %d): %s", s.qos, s.summary())
			}
		}
	})
	return func() {
		close(done)
		wg.Wait()
	}
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
func (s *simulator) runDevice(c context.Context, opts *mqtt.ClientOptions, cfg mqttx.Config) {
	id := cfg.ClientID
	client := mqtt.NewClient(opts)
	if err := mqttx.Connect(c, client); err != nil {
		log.Printf("%s: %s: %v", id, cfg.BrokerURL, err)
		return
	}
	log.Printf("%s: connected to %s", id, cfg.BrokerURL)

	sensor := newSensor(id, s.run, rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64())))
	rng := rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
	topic := mqttx.TelemetryTopic(id)
	published := 0

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-c.Done():
			// 正常下線：先自己宣告 offline，再送 DISCONNECT（broker 因此不會發布 LWT）。
			// 此時 c 已取消，所以用新的 context；broker 不在時最多等 brokerTimeout。
			publishStatus(context.Background(), client, id, mqttx.StateOffline, mqttx.ReasonGraceful, s.retain)
			client.Disconnect(250)
			log.Printf("%s: disconnected, published %d telemetry", id, published)
			return
		case now := <-ticker.C:
			tel := sensor.next(now)
			if rng.Float64() < s.skipRate {
				// 序號已經用掉，但這筆從來沒送出：subscriber 會看到 gap
				s.stats.recordSkip()
				continue
			}
			payload, err := mqttx.EncodeTelemetry(tel)
			if err != nil {
				log.Printf("%s: %v", id, err)
				continue
			}
			if s.publish(c, client, topic, payload, false) {
				published++
			}
			if rng.Float64() < s.dupRate {
				// 應用層重送：同一筆 payload（同一個 seq）再發一次。對 MQTT 來說這是一則全新的訊息，
				// 即使 QoS 2 也擋不住 —— 只有 subscriber 依序號判斷才能發現
				if s.publish(c, client, topic, payload, true) {
					published++
				}
			}
		}
	}
}

// publish 發布一則 telemetry 並記錄統計，回傳是否在期限內得到確認。
func (s *simulator) publish(c context.Context, client mqtt.Client, topic string, payload []byte, retry bool) bool {
	// token 完成的時機取決於 QoS：0 是寫進網路層、1 是收到 PUBACK、2 是收到 PUBCOMP
	start := time.Now()
	err := mqttx.Wait(c, client.Publish(topic, s.qos, false, payload), brokerTimeout)
	s.stats.record(time.Since(start), err, retry)
	if err != nil {
		log.Printf("%s: publish: %v", topic, err)
		return false
	}
	return true
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
