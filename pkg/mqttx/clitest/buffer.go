package clitest

import (
	"bytes"
	"sync"
)

// lockedBuffer 讓 stdout/stderr 可以同時寫入同一個 buffer。
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (x *lockedBuffer) Write(p []byte) (int, error) {
	x.mu.Lock()
	defer x.mu.Unlock()
	return x.buf.Write(p)
}

func (x *lockedBuffer) String() string {
	x.mu.Lock()
	defer x.mu.Unlock()
	return x.buf.String()
}
