package mqttx

import (
	"context"
	"errors"
	"fmt"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// ErrTimeout 表示在期限內沒等到 token 完成。
var ErrTimeout = errors.New("timed out waiting for broker")

// Wait 等 token 完成，但最多等 timeout，且 c 被取消時立刻返回。
//
// 斷線或重連中時 token 可能很久（或永遠）不會完成；直接 token.Wait() 會讓程式卡住、對 Ctrl+C 沒反應。
func Wait(c context.Context, tok mqtt.Token, timeout time.Duration) error {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-tok.Done():
		return tok.Error()
	case <-timer.C:
		return ErrTimeout
	case <-c.Done():
		return fmt.Errorf("wait for broker: %w", c.Err())
	}
}
