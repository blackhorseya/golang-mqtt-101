package mqttx

import (
	"log"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// Session 是斷線與重連相關的設定。
type Session struct {
	// Clean 為 true 時（MQTT 預設）broker 不保留這個 client 的 session：斷線期間的訊息不會替它排隊，
	// 重連後訂閱也要重新建立。為 false 時 broker 依 ClientID 保留訂閱，並替它排隊 QoS 1/2 訊息。
	Clean bool

	// MaxReconnectInterval 是自動重連 backoff 的上限。paho 斷線後先等約 1 秒再重連，失敗就把等待時間加倍，
	// 最多到這個值（paho 預設 10 分鐘）。注意 SetConnectRetryInterval 只管「第一次」連線的重試間隔。
	MaxReconnectInterval time.Duration
}

// Apply 把 session 設定與斷線、重連的 log 加到 opts；name 是 log 的前綴。
func (x Session) Apply(opts *mqtt.ClientOptions, name string) *mqtt.ClientOptions {
	return opts.
		SetCleanSession(x.Clean).
		SetAutoReconnect(true).
		SetMaxReconnectInterval(x.MaxReconnectInterval).
		SetConnectionLostHandler(func(_ mqtt.Client, err error) {
			log.Printf("%s: connection lost: %v", name, err)
		}).
		SetReconnectingHandler(func(mqtt.Client, *mqtt.ClientOptions) {
			log.Printf("%s: reconnecting", name)
		})
}
