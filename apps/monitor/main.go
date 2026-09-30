// monitor 用 wildcard 訂閱所有 device 的訊息並印到 terminal。
//
//	monitor                       # 預設訂閱 devices/+/telemetry 與 devices/+/status
//	monitor --topic 'devices/#'   # 試試多層 wildcard
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
	flag.StringVar(&cfg.BrokerURL, "broker", cfg.BrokerURL, "broker URL（預設讀 MQTT_BROKER_URL）")
	flag.Parse()

	filters := map[string]byte{}
	for _, f := range parseFilters(*topics) {
		filters[f] = 0
	}

	c, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	h := &handler{presence: presence{}}
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

	<-c.Done()
	client.Disconnect(250)
}

// handler 處理收到的訊息並維護 device 在線狀態。
type handler struct {
	mu       sync.Mutex
	presence presence
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
		fmt.Println(formatTelemetry(t))
	case strings.HasSuffix(m.Topic(), "/status"):
		id, err := mqttx.DeviceIDFromTopic(m.Topic())
		if err != nil {
			log.Printf("%v", err)
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
		fmt.Println(formatStatus(id, s, online))
	default:
		fmt.Printf("%s  %s\n", m.Topic(), m.Payload())
	}
}
