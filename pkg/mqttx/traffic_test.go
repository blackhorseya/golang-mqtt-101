package mqttx

import (
	"io"
	"net"
	"testing"
)

// Traffic 計算經過連線的位元組數，用來比較不同 QoS 的協定開銷。
func TestTrafficCountsBytes(t *testing.T) {
	var traffic Traffic
	client, server := net.Pipe()
	defer server.Close()
	conn := traffic.Wrap(client)
	defer conn.Close()

	go func() {
		buf := make([]byte, 5)
		io.ReadFull(server, buf)
		server.Write([]byte("abc"))
	}()

	if _, err := conn.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 3)
	if _, err := io.ReadFull(conn, buf); err != nil {
		t.Fatal(err)
	}

	if got := traffic.Sent(); got != 5 {
		t.Errorf("Sent() = %d, want 5", got)
	}
	if got := traffic.Received(); got != 3 {
		t.Errorf("Received() = %d, want 3", got)
	}
}
