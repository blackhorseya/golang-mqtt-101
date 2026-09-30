package mqttx

import (
	"context"
	"fmt"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// Connect 連上 broker 並等待結果，c 被取消時立刻放棄。
//
// 搭配 SetConnectRetry(true) 時，broker 連不上的話 Connect() 的 token 會一直重試不會完成；
// 直接 token.Wait() 會讓程式對 Ctrl+C 沒反應，所以要同時等 c.Done()。
func Connect(c context.Context, client mqtt.Client) error {
	tok := client.Connect()
	select {
	case <-tok.Done():
		if err := tok.Error(); err != nil {
			return fmt.Errorf("connect: %w", err)
		}
		return nil
	case <-c.Done():
		// 讓 paho 停止背景重試
		client.Disconnect(0)
		return fmt.Errorf("connect: %w", c.Err())
	}
}
