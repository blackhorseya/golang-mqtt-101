package clitest

import (
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

// Proxy 是介於 app 與 broker 之間的 TCP proxy，Close 後等同 broker 從網路上消失。
type Proxy struct {
	t         *testing.T
	listener  net.Listener
	upstream  string
	connected chan struct{}
	once      sync.Once

	mu    sync.Mutex
	conns []net.Conn
}

// NewProxy 在本機隨機 port 開一個轉發到 brokerURL（tcp://host:port）的 proxy。
func NewProxy(t *testing.T, brokerURL string) *Proxy {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	x := &Proxy{t: t, listener: l, upstream: strings.TrimPrefix(brokerURL, "tcp://"), connected: make(chan struct{})}
	go x.serve()
	t.Cleanup(x.Close)
	return x
}

// URL 回傳 app 應該連的 broker URL。
func (x *Proxy) URL() string {
	return "tcp://" + x.listener.Addr().String()
}

// WaitForConnection 等到第一條連線建立。
func (x *Proxy) WaitForConnection(timeout time.Duration) {
	x.t.Helper()
	select {
	case <-x.connected:
	case <-time.After(timeout):
		x.t.Fatalf("no client connected to proxy within %s", timeout)
	}
}

// Close 停止接受新連線並切斷所有現有連線。
func (x *Proxy) Close() {
	x.listener.Close()
	x.mu.Lock()
	defer x.mu.Unlock()
	for _, conn := range x.conns {
		conn.Close()
	}
	x.conns = nil
}

func (x *Proxy) serve() {
	for {
		client, err := x.listener.Accept()
		if err != nil {
			return
		}
		broker, err := net.Dial("tcp", x.upstream)
		if err != nil {
			client.Close()
			continue
		}
		x.mu.Lock()
		x.conns = append(x.conns, client, broker)
		x.mu.Unlock()
		x.once.Do(func() { close(x.connected) })
		go pipe(client, broker)
		go pipe(broker, client)
	}
}

func pipe(dst, src net.Conn) {
	io.Copy(dst, src)
	dst.Close()
	src.Close()
}
