// Package clitest 提供對 app binary 做端到端測試的共用工具。
package clitest

import (
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// Build 把目前目錄的 main package 編譯到暫存目錄並回傳 binary 路徑。
func Build(t *testing.T, name string) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), name)
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("go build %s: %v\n%s", name, err, out)
	}
	return bin
}

// UnreachableBroker 回傳一個目前沒有人在 listen 的本機 broker URL。
func UnreachableBroker(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	l.Close()
	return "tcp://" + addr
}

// ExitsOnInterrupt 啟動 binary，等 startup 後送 SIGINT，並要求它在 within 內結束。
func ExitsOnInterrupt(t *testing.T, bin string, startup, within time.Duration, args ...string) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	out := &lockedBuffer{}
	cmd.Stdout, cmd.Stderr = out, out
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	time.Sleep(startup)
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatalf("send SIGINT: %v", err)
	}
	select {
	case <-done:
	case <-time.After(within):
		cmd.Process.Kill()
		<-done
		t.Fatalf("%s %v still running %s after SIGINT\noutput:\n%s", filepath.Base(bin), args, within, out.String())
	}
}
