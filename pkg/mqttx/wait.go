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

// ErrSubscribeRefused 表示 broker 在 SUBACK 回傳失敗（0x80）拒絕了訂閱。
var ErrSubscribeRefused = errors.New("subscription refused by broker")

// subackFailure 是 MQTT 3.1.1 SUBACK 的失敗代碼；成功時回傳的是 broker 給的 QoS（0、1、2）。
const subackFailure = 0x80

// WaitSubscribe 等 Subscribe 的 token 完成，並檢查 SUBACK 結果。
//
// broker 拒絕訂閱（例如 ACL 不允許）時，paho 照樣把 token 標為完成、Error() 是 nil，
// 失敗只記在 SubscribeToken.Result() 裡；只看 Error() 會以為訂閱成功。
func WaitSubscribe(c context.Context, tok mqtt.Token, timeout time.Duration) error {
	if err := Wait(c, tok, timeout); err != nil {
		return err
	}
	sub, ok := tok.(interface{ Result() map[string]byte })
	if !ok {
		return nil
	}
	for filter, code := range sub.Result() {
		if code == subackFailure {
			return fmt.Errorf("subscribe %s: %w", filter, ErrSubscribeRefused)
		}
	}
	return nil
}
