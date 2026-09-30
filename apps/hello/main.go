// hello 是範例 app：示範如何在 go.work 下引用共用的 pkg/mqttx module。
package main

import (
	"fmt"

	"github.com/blackhorseya/golang-mqtt-101/pkg/mqttx"
)

func main() {
	cfg := mqttx.ConfigFromEnv("hello")
	fmt.Printf("broker=%s client_id=%s\n", cfg.BrokerURL, cfg.ClientID)
}
