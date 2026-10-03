package mqttx

import (
	"context"
	"errors"
	"testing"
	"time"
)

// subackToken 是已完成、帶 SUBACK 結果的 token（和 paho 的 *SubscribeToken 一樣有 Result）。
type subackToken struct {
	err    error
	result map[string]byte
}

func (subackToken) Wait() bool                     { return true }
func (subackToken) WaitTimeout(time.Duration) bool { return true }
func (subackToken) Done() <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}
func (x subackToken) Error() error            { return x.err }
func (x subackToken) Result() map[string]byte { return x.result }

// broker 拒絕訂閱時回 SUBACK 0x80；paho 照樣把 token 標為完成、Error() 為 nil，失敗只記在 Result 裡。
func TestWaitSubscribe(t *testing.T) {
	c := context.Background()
	granted := subackToken{result: map[string]byte{"commands/device-001/#": 1}}
	if err := WaitSubscribe(c, granted, time.Second); err != nil {
		t.Errorf("granted subscription error = %v, want nil", err)
	}

	refused := subackToken{result: map[string]byte{"commands/device-001/#": 0x80}}
	if err := WaitSubscribe(c, refused, time.Second); !errors.Is(err, ErrSubscribeRefused) {
		t.Errorf("refused subscription error = %v, want ErrSubscribeRefused", err)
	}

	broken := subackToken{err: errors.New("connection lost")}
	if err := WaitSubscribe(c, broken, time.Second); err == nil {
		t.Error("token error = nil, want error")
	}
}
