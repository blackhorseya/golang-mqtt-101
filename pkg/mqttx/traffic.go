package mqttx

import (
	"fmt"
	"net"
	"net/url"
	"sync/atomic"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// Traffic 累計經過連線的位元組數，用來比較不同 QoS 的協定開銷（QoS 越高，每則訊息的確認封包越多）。
// 可同時給多條連線使用。
type Traffic struct {
	sent     atomic.Int64
	received atomic.Int64
}

// Sent 回傳送出的位元組數。
func (x *Traffic) Sent() int64 { return x.sent.Load() }

// Received 回傳收到的位元組數。
func (x *Traffic) Received() int64 { return x.received.Load() }

// Wrap 回傳會把讀寫量記到 x 的連線。
func (x *Traffic) Wrap(conn net.Conn) net.Conn {
	return &countingConn{Conn: conn, traffic: x}
}

// OpenConnection 給 ClientOptions.SetCustomOpenConnectionFn 使用：自己撥 TCP 連線並包上計數。
// 每次（重新）連線都會呼叫，所以數字會跨重連累計。只支援 tcp://。
func (x *Traffic) OpenConnection() mqtt.OpenConnectionFunc {
	return func(uri *url.URL, options mqtt.ClientOptions) (net.Conn, error) {
		if uri.Scheme != "tcp" && uri.Scheme != "mqtt" {
			return nil, fmt.Errorf("open connection %s: only tcp:// is supported", uri)
		}
		d := net.Dialer{Timeout: options.ConnectTimeout}
		conn, err := d.Dial("tcp", uri.Host)
		if err != nil {
			return nil, fmt.Errorf("dial %s: %w", uri.Host, err)
		}
		return x.Wrap(conn), nil
	}
}

type countingConn struct {
	net.Conn
	traffic *Traffic
}

func (i *countingConn) Read(p []byte) (int, error) {
	n, err := i.Conn.Read(p)
	i.traffic.received.Add(int64(n))
	return n, err
}

func (i *countingConn) Write(p []byte) (int, error) {
	n, err := i.Conn.Write(p)
	i.traffic.sent.Add(int64(n))
	return n, err
}
