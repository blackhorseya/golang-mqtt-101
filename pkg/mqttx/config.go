// Package mqttx 放各 app 共用的 MQTT 相關設定與工具。
package mqttx

import (
	"os"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// Config 描述連線到 broker 所需的基本資訊。
type Config struct {
	BrokerURL string
	ClientID  string
}

// ConfigFromEnv 從環境變數讀取設定，未設定時使用本機 mosquitto 的預設值。
func ConfigFromEnv(defaultClientID string) Config {
	return Config{
		BrokerURL: getenv("MQTT_BROKER_URL", "tcp://localhost:1883"),
		ClientID:  getenv("MQTT_CLIENT_ID", defaultClientID),
	}
}

func getenv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

// ClientOptions 以此設定建立 paho 的連線選項；呼叫端可再串接其他 Set* 設定。
// 同一個 broker 上 ClientID 必須唯一：相同 ID 的新連線會把舊連線踢掉。
func (x Config) ClientOptions() *mqtt.ClientOptions {
	return mqtt.NewClientOptions().
		AddBroker(x.BrokerURL).
		SetClientID(x.ClientID)
}
