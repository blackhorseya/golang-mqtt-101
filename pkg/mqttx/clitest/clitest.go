// Package clitest 提供對 app binary 做端到端測試的共用工具。
package clitest

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Build 把目前目錄的 main package 編譯到暫存目錄並回傳 binary 路徑。
func Build(t *testing.T, name string) string {
	t.Helper()
	return BuildDir(t, name, ".")
}

// BuildDir 把 dir 的 main package 編譯到暫存目錄並回傳 binary 路徑。
func BuildDir(t *testing.T, name, dir string) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), name)
	if out, err := exec.Command("go", "build", "-o", bin, dir).CombinedOutput(); err != nil {
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

// Process 是一個由測試啟動的 app process。
type Process struct {
	t    *testing.T
	name string
	cmd  *exec.Cmd
	out  *lockedBuffer
	done chan struct{} // process 結束時 close，可重複讀取
}

// Start 啟動 binary；測試結束時若還在跑會被 kill。
func Start(t *testing.T, bin string, args ...string) *Process {
	t.Helper()
	cmd := exec.Command(bin, args...)
	out := &lockedBuffer{}
	cmd.Stdout, cmd.Stderr = out, out
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	p := &Process{t: t, name: fmt.Sprintf("%s %v", filepath.Base(bin), args), cmd: cmd, out: out, done: make(chan struct{})}
	go func() {
		cmd.Wait()
		close(p.done)
	}()
	t.Cleanup(func() {
		select {
		case <-p.done:
		default:
			cmd.Process.Kill()
			<-p.done
		}
	})
	return p
}

// Signal 送出 signal（例如 os.Interrupt、syscall.SIGKILL）。
func (x *Process) Signal(sig os.Signal) {
	x.t.Helper()
	if err := x.cmd.Process.Signal(sig); err != nil {
		x.t.Fatalf("signal %v to %s: %v", sig, x.name, err)
	}
}

// WaitExit 要求 process 在 within 內結束。
func (x *Process) WaitExit(within time.Duration) {
	x.t.Helper()
	select {
	case <-x.done:
	case <-time.After(within):
		x.t.Fatalf("%s still running after %s\noutput:\n%s", x.name, within, x.out.String())
	}
}

// Output 回傳目前為止 stdout + stderr 的內容。
func (x *Process) Output() string {
	return x.out.String()
}

// ExitsOnInterrupt 啟動 binary，等 startup 後送 SIGINT，並要求它在 within 內結束。
func ExitsOnInterrupt(t *testing.T, bin string, startup, within time.Duration, args ...string) {
	t.Helper()
	p := Start(t, bin, args...)
	time.Sleep(startup)
	p.Signal(os.Interrupt)
	p.WaitExit(within)
}

// ExitCode 回傳已結束 process 的 exit code；須先呼叫 WaitExit。
func (x *Process) ExitCode() int {
	x.t.Helper()
	select {
	case <-x.done:
		return x.cmd.ProcessState.ExitCode()
	default:
		x.t.Fatalf("%s has not exited", x.name)
		return -1
	}
}

// WaitOutput 等到輸出中出現 substr，timeout 就失敗。
func (x *Process) WaitOutput(substr string, timeout time.Duration) {
	x.t.Helper()
	x.WaitCount(substr, 1, timeout)
}

// WaitCount 等到輸出中 substr 出現至少 n 次，timeout 就失敗。
func (x *Process) WaitCount(substr string, n int, timeout time.Duration) {
	x.t.Helper()
	deadline := time.Now().Add(timeout)
	for strings.Count(x.out.String(), substr) < n {
		if time.Now().After(deadline) {
			x.t.Fatalf("%s: %q appeared %d times (want %d) after %s\noutput:\n%s",
				x.name, substr, strings.Count(x.out.String(), substr), n, timeout, x.out.String())
		}
		time.Sleep(50 * time.Millisecond)
	}
}
