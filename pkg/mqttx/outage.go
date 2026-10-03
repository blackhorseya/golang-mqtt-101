package mqttx

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// ErrOutage 是模擬中斷期間撥號得到的錯誤。
var ErrOutage = errors.New("simulated network outage")

// Outage 模擬網路中斷：Cut 直接關掉底層的 TCP 連線（不送 DISCONNECT），並在一段時間內拒絕重新撥號。
// 對 broker 來說這和網路線被拔掉一樣，所以會發布 LWT；對 paho 來說則是連線中斷，會進入自動重連。
// 可同時給多條連線使用：Cut 會一次斷掉全部。零值即可使用。
type Outage struct {
	mu    sync.Mutex
	conns map[*trackedConn]struct{}
	until time.Time
}

// OpenConnection 包住 next：中斷期間撥號直接失敗，成功的連線則記下來讓 Cut 可以關掉。
func (x *Outage) OpenConnection(next mqtt.OpenConnectionFunc) mqtt.OpenConnectionFunc {
	return func(uri *url.URL, options mqtt.ClientOptions) (net.Conn, error) {
		if x.down() {
			return nil, fmt.Errorf("dial %s: %w", uri.Host, ErrOutage)
		}
		conn, err := next(uri, options)
		if err != nil {
			return nil, err
		}
		tracked := &trackedConn{Conn: conn, outage: x}
		x.mu.Lock()
		defer x.mu.Unlock()
		// 撥號期間可能已經 Cut 過：那次 Cut 看不到這條連線，所以在同一把鎖下再檢查一次
		if time.Now().Before(x.until) {
			conn.Close()
			return nil, fmt.Errorf("dial %s: %w", uri.Host, ErrOutage)
		}
		if x.conns == nil {
			x.conns = map[*trackedConn]struct{}{}
		}
		x.conns[tracked] = struct{}{}
		return tracked, nil
	}
}

// Cut 關掉目前所有連線，並在 d 之內拒絕撥號；回傳關掉的連線數。
func (x *Outage) Cut(d time.Duration) int {
	x.mu.Lock()
	x.until = time.Now().Add(d)
	conns := make([]*trackedConn, 0, len(x.conns))
	for conn := range x.conns {
		conns = append(conns, conn)
	}
	x.mu.Unlock()

	for _, conn := range conns {
		conn.cut()
	}
	return len(conns)
}

// Every 每隔 every 中斷一次、每次持續 length，直到 c 被取消。onCut 在每次中斷後被呼叫，可用來印 log。
func (x *Outage) Every(c context.Context, every, length time.Duration, onCut func(closed int)) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		select {
		case <-c.Done():
			return
		case <-ticker.C:
			onCut(x.Cut(length))
		}
	}
}

func (x *Outage) down() bool {
	x.mu.Lock()
	defer x.mu.Unlock()
	return time.Now().Before(x.until)
}

func (x *Outage) forget(conn *trackedConn) {
	x.mu.Lock()
	defer x.mu.Unlock()
	delete(x.conns, conn)
}

// ParseOutage 檢查 --outage-every / --outage-for：every 為 0 表示不中斷；
// 否則 length 必須大於 0 且比 every 短，不然連線永遠回不來。
func ParseOutage(every, length time.Duration) error {
	switch {
	case every < 0:
		return fmt.Errorf("parse outage-every %s: must not be negative", every)
	case every == 0:
		return nil
	case length <= 0:
		return fmt.Errorf("parse outage-for %s: must be positive", length)
	case length >= every:
		return fmt.Errorf("parse outage-for %s: must be shorter than outage-every %s", length, every)
	}
	return nil
}

type trackedConn struct {
	net.Conn
	outage  *Outage
	severed atomic.Bool
}

// cut 關掉底層連線，之後的讀寫都回傳 ErrOutage。
//
// 不能只是 Close：paho 讀寫時若遇到 "use of closed network connection"，會當成是自己關的而不回報，
// 結果不會觸發 connection lost 與自動重連。真的斷網時得到的是 EOF、connection reset 之類的錯誤，
// 所以這裡換成另一個錯誤，讓 paho 照斷線處理。
func (i *trackedConn) cut() {
	i.severed.Store(true)
	i.Close()
}

func (i *trackedConn) Read(p []byte) (int, error) {
	n, err := i.Conn.Read(p)
	if err != nil && i.severed.Load() {
		return n, fmt.Errorf("read: %w", ErrOutage)
	}
	return n, err
}

func (i *trackedConn) Write(p []byte) (int, error) {
	n, err := i.Conn.Write(p)
	if err != nil && i.severed.Load() {
		return n, fmt.Errorf("write: %w", ErrOutage)
	}
	return n, err
}

func (i *trackedConn) Close() error {
	i.outage.forget(i)
	return i.Conn.Close()
}
