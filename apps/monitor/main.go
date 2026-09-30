// monitor 用 wildcard 訂閱所有 device 的訊息並印到 terminal。
//
//	monitor                       # 預設訂閱 devices/+/telemetry
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
	"syscall"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"github.com/blackhorseya/golang-mqtt-101/pkg/mqttx"
)

func main() {
	// 預設 ClientID 帶 pid，讓多個 monitor 可以同時跑而不互踢
	cfg := mqttx.ConfigFromEnv(fmt.Sprintf("monitor-%d", os.Getpid()))
	filter := flag.String("topic", "devices/+/telemetry", "訂閱的 topic filter（可用 + 與 #）")
	flag.StringVar(&cfg.BrokerURL, "broker", cfg.BrokerURL, "broker URL（預設讀 MQTT_BROKER_URL）")
	flag.Parse()

	c, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	opts := cfg.ClientOptions().
		SetConnectRetry(true).
		SetConnectRetryInterval(time.Second).
		// 在 OnConnect 裡訂閱：clean session 下斷線重連後 broker 不會記得舊訂閱，每次連上都要重訂
		SetOnConnectHandler(func(client mqtt.Client) {
			tok := client.Subscribe(*filter, 0, handleMessage)
			if tok.Wait() && tok.Error() != nil {
				log.Printf("subscribe %s: %v", *filter, tok.Error())
				return
			}
			log.Printf("subscribed to %s", *filter)
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

// handleMessage 印出收到的訊息；telemetry 以固定格式呈現，其他 topic（例如用 devices/# 訂閱時）印原始內容。
func handleMessage(_ mqtt.Client, m mqtt.Message) {
	if !strings.HasSuffix(m.Topic(), "/telemetry") {
		fmt.Printf("%s  %s\n", m.Topic(), m.Payload())
		return
	}
	t, err := mqttx.DecodeTelemetry(m.Payload())
	if err != nil {
		log.Printf("%s: %v", m.Topic(), err)
		return
	}
	fmt.Println(formatTelemetry(t))
}
