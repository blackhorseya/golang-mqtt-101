package mqttx

import (
	"errors"
	"net"
	"net/url"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// pipeDialer 回傳 net.Pipe 的一端當作連線，另一端留給測試觀察。
func pipeDialer(peers chan<- net.Conn) mqtt.OpenConnectionFunc {
	return func(*url.URL, mqtt.ClientOptions) (net.Conn, error) {
		client, server := net.Pipe()
		peers <- server
		return client, nil
	}
}

// Cut 關掉目前的連線，並在中斷期間拒絕重新撥號；時間過了就恢復。
func TestOutageCutClosesConnectionsAndRefusesDial(t *testing.T) {
	var outage Outage
	peers := make(chan net.Conn, 2)
	dial := outage.OpenConnection(pipeDialer(peers))
	uri, _ := url.Parse("tcp://broker:1883")

	conn, err := dial(uri, mqtt.ClientOptions{})
	if err != nil {
		t.Fatalf("dial before outage: %v", err)
	}
	peer := <-peers

	if n := outage.Cut(300 * time.Millisecond); n != 1 {
		t.Errorf("Cut() closed %d connections, want 1", n)
	}
	// 被關掉的是我們這一端；對端讀到 EOF，就像 broker 看到 TCP 斷線
	peer.SetReadDeadline(time.Now().Add(time.Second))
	if _, err := peer.Read(make([]byte, 1)); err == nil {
		t.Error("peer read succeeded after Cut, want error")
	}
	if _, err := conn.Write([]byte("x")); err == nil {
		t.Error("write on cut connection succeeded, want error")
	}

	if _, err := dial(uri, mqtt.ClientOptions{}); !errors.Is(err, ErrOutage) {
		t.Errorf("dial during outage error = %v, want ErrOutage", err)
	}

	time.Sleep(400 * time.Millisecond)
	conn, err = dial(uri, mqtt.ClientOptions{})
	if err != nil {
		t.Fatalf("dial after outage: %v", err)
	}
	defer conn.Close()
	<-peers
}

// 已經自己關閉的連線不再被追蹤，Cut 不會重複計算。
func TestOutageForgetsClosedConnections(t *testing.T) {
	var outage Outage
	peers := make(chan net.Conn, 1)
	conn, err := outage.OpenConnection(pipeDialer(peers))(&url.URL{Scheme: "tcp"}, mqtt.ClientOptions{})
	if err != nil {
		t.Fatal(err)
	}
	<-peers
	conn.Close()

	if n := outage.Cut(time.Millisecond); n != 0 {
		t.Errorf("Cut() closed %d connections, want 0", n)
	}
}

func TestParseOutage(t *testing.T) {
	cases := []struct {
		every, length time.Duration
		ok            bool
	}{
		{every: 0, length: 3 * time.Second, ok: true}, // 關閉
		{every: 10 * time.Second, length: 3 * time.Second, ok: true},
		{every: -time.Second, length: time.Second, ok: false},
		{every: 10 * time.Second, length: 0, ok: false},
		{every: 10 * time.Second, length: 10 * time.Second, ok: false}, // 永遠連不上
	}
	for _, tc := range cases {
		err := ParseOutage(tc.every, tc.length)
		if (err == nil) != tc.ok {
			t.Errorf("ParseOutage(%s, %s) error = %v, want ok=%t", tc.every, tc.length, err, tc.ok)
		}
	}
}
