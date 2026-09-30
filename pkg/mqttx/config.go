// Package mqttx 放各 app 共用的 MQTT 相關設定與工具。
package mqttx

import "os"

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
