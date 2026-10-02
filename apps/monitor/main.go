// monitor 用 wildcard 訂閱所有 device 的訊息並印到 terminal。
//
//	monitor                       # 預設訂閱 devices/+/telemetry 與 devices/+/status
//	monitor --topic 'devices/#'   # 試試多層 wildcard
//	monitor --qos 0               # 以 QoS 0 訂閱：訊息最高只會以 QoS 0 送達
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"github.com/blackhorseya/golang-mqtt-101/pkg/mqttx"
)

func main() {
	// 預設 ClientID 帶 pid，讓多個 monitor 可以同時跑而不互踢
	cfg := mqttx.ConfigFromEnv(fmt.Sprintf("monitor-%d", os.Getpid()))
	topics := flag.String("topic", "devices/+/telemetry,devices/+/status", "訂閱的 topic filter，多個以逗號分隔（可用 + 與 #）")
	qosFlag := flag.Int("qos", 2, "訂閱的 QoS 上限（0、1、2）；訊息實際送達的 QoS = min(發布的 QoS, 此值)")
	report := flag.Duration("report", 5*time.Second, "每隔多久印一次接收統計（0 表示只在結束時印）")
	flag.StringVar(&cfg.BrokerURL, "broker", cfg.BrokerURL, "broker URL（預設讀 MQTT_BROKER_URL）")
	flag.Parse()

	qos, err := mqttx.ParseQoS(*qosFlag)
	if err != nil {
		log.Fatal(err)
	}
	filters := map[string]byte{}
	for _, f := range parseFilters(*topics) {
		filters[f] = qos
	}

	c, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	h := &handler{presence: presence{}, seq: newSeqTracker(), stats: &receiveStats{}}
	opts := cfg.ClientOptions().
		SetConnectRetry(true).
		SetConnectRetryInterval(time.Second).
		// 在 OnConnect 裡訂閱：clean session 下斷線重連後 broker 不會記得舊訂閱，每次連上都要重訂
		SetOnConnectHandler(func(client mqtt.Client) {
			tok := client.SubscribeMultiple(filters, h.handle)
			if tok.Wait() && tok.Error() != nil {
				log.Printf("subscribe %s: %v", *topics, tok.Error())
				return
			}
			log.Printf("subscribed to %s", *topics)
		})
	client := mqtt.NewClient(opts)
	if err := mqttx.Connect(c, client); err != nil {
		log.Printf("%s: %v", cfg.BrokerURL, err)
		return
	}
	log.Printf("connected to %s as %s", cfg.BrokerURL, cfg.ClientID)

	if *report > 0 {
		ticker := time.NewTicker(*report)
		defer ticker.Stop()
	loop:
		for {
			select {
			case <-c.Done():
				break loop
			case <-ticker.C:
				log.Printf("stats: %s", h.stats.summary())
			}
		}
	} else {
		<-c.Done()
	}
	client.Disconnect(250)
	log.Printf("stats: %s", h.stats.summary())
}

// handler 處理收到的訊息，維護 device 在線狀態與序號。
type handler struct {
	mu       sync.Mutex
	presence presence
	seq      *seqTracker
	stats    *receiveStats
}

// handle 依 topic 印出訊息：telemetry 與 status 以固定格式呈現，其他 topic（例如用 devices/# 訂閱時）印原始內容。
func (h *handler) handle(_ mqtt.Client, m mqtt.Message) {
	switch {
	case strings.HasSuffix(m.Topic(), "/telemetry"):
		t, err := mqttx.DecodeTelemetry(m.Payload())
		if err != nil {
			log.Printf("%s: %v", m.Topic(), err)
			return
		}
		// 端到端延遲：device 產生 telemetry 到 monitor 收到（同一台機器，時鐘一致）
		latency := time.Since(t.Timestamp)
		// 應用層 idempotency：依序號決定這筆要不要處理（duplicate、out-of-order 不處理）
		h.mu.Lock()
		v, missing := h.seq.observe(t.DeviceID, t.Run, t.Seq)
		h.mu.Unlock()
		h.stats.record(m.Qos(), m.Duplicate(), latency, v, missing)
		fmt.Println(formatTelemetry(t, m.Qos(), m.Duplicate(), latency, v, missing))
	case strings.HasSuffix(m.Topic(), "/status"):
		id, err := mqttx.DeviceIDFromTopic(m.Topic())
		if err != nil {
			log.Printf("%v", err)
			return
		}
		// 空 payload 的 retained publish 代表清除保留訊息
		if len(m.Payload()) == 0 {
			h.mu.Lock()
			online := h.presence.remove(id)
			h.mu.Unlock()
			fmt.Println(formatCleared(id, online))
			return
		}
		s, err := mqttx.DecodeStatus(m.Payload())
		if err != nil {
			log.Printf("%s: %v", m.Topic(), err)
			return
		}
		h.mu.Lock()
		online := h.presence.apply(id, s)
		h.mu.Unlock()
		fmt.Println(formatStatus(id, s, online, m.Retained()))
	default:
		fmt.Printf("%s  %s\n", m.Topic(), m.Payload())
	}
}
