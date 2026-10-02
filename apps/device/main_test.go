package main

import (
	"strings"
	"testing"
	"time"

	"github.com/blackhorseya/golang-mqtt-101/pkg/mqttx/clitest"
)

// broker 連不上時 device 會持續重試連線；Ctrl+C 仍必須讓它結束。
func TestInterruptWhileBrokerUnavailable(t *testing.T) {
	bin := clitest.Build(t, "device")
	clitest.ExitsOnInterrupt(t, bin, time.Second, 3*time.Second,
		"--broker", clitest.UnreachableBroker(t), "--count", "2")
}

// QoS 只有 0、1、2；不合法的值要在連線前就失敗。
func TestInvalidQoSExits(t *testing.T) {
	p := clitest.Start(t, clitest.Build(t, "device"), "--qos", "3", "--broker", clitest.UnreachableBroker(t))
	p.WaitExit(3 * time.Second)
	if code := p.ExitCode(); code == 0 {
		t.Errorf("exit code = 0, want non-zero\noutput:\n%s", p.Output())
	}
	// 必須是 QoS 檢查擋下的，而不是例如「flag 未定義」之類的其他錯誤
	if out := p.Output(); !strings.Contains(out, "must be 0, 1 or 2") {
		t.Errorf("output does not explain the valid QoS values:\n%s", out)
	}
}

// --dup-rate / --skip-rate 是機率，必須在 [0, 1]。
func TestInvalidRateExits(t *testing.T) {
	bin := clitest.Build(t, "device")
	for _, args := range [][]string{{"--dup-rate", "1.5"}, {"--skip-rate", "-0.1"}} {
		p := clitest.Start(t, bin, append(args, "--broker", clitest.UnreachableBroker(t))...)
		p.WaitExit(3 * time.Second)
		if code := p.ExitCode(); code == 0 {
			t.Errorf("%v: exit code = 0, want non-zero", args)
		}
		if out := p.Output(); !strings.Contains(out, "must be between 0 and 1") {
			t.Errorf("%v: output does not explain the valid range:\n%s", args, out)
		}
	}
}
