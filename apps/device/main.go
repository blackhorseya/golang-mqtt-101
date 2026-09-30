// device 模擬多個 IoT device：每個 device 是一個獨立的 MQTT client，定期發送 telemetry。
//
//	device --count 10 --interval 1s
package main

import (
	"context"
	"flag"
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

// Phase 1 固定用 QoS 0（at most once）；Phase 4 才開放設定。
const telemetryQoS = 0

func main() {
	cfg := mqttx.ConfigFromEnv("")
	count := flag.Int("count", 1, "要模擬的 device 數量")
	interval := flag.Duration("interval", 2*time.Second, "每個 device 發送 telemetry 的間隔")
	flag.StringVar(&cfg.BrokerURL, "broker", cfg.BrokerURL, "broker URL（預設讀 MQTT_BROKER_URL）")
	flag.Parse()

	c, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var wg sync.WaitGroup
	for n := 1; n <= *count; n++ {
		devCfg := cfg
		devCfg.ClientID = mqttx.DeviceID(n)
		wg.Go(func() { runDevice(c, devCfg, *interval) })
	}
	wg.Wait()
}

// runDevice 連上 broker 後每隔 interval 發一次 telemetry，直到 c 被取消才正常斷線。
func runDevice(c context.Context, cfg mqttx.Config, interval time.Duration) {
	id := cfg.ClientID
	client := mqtt.NewClient(cfg.ClientOptions().
		SetConnectRetry(true).
		SetConnectRetryInterval(time.Second))
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
			// 正常斷線：送出 DISCONNECT 封包，broker 知道這是主動離開
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
			if tok := client.Publish(topic, telemetryQoS, false, payload); tok.Wait() && tok.Error() != nil {
				log.Printf("%s: publish %s: %v", id, topic, tok.Error())
				continue
			}
			published++
		}
	}
}
