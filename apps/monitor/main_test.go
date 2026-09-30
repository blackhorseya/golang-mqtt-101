package main

import (
	"testing"
	"time"

	"github.com/blackhorseya/golang-mqtt-101/pkg/mqttx/clitest"
)

// broker 連不上時 monitor 會持續重試連線；Ctrl+C 仍必須讓它結束。
func TestInterruptWhileBrokerUnavailable(t *testing.T) {
	bin := clitest.Build(t, "monitor")
	clitest.ExitsOnInterrupt(t, bin, time.Second, 3*time.Second,
		"--broker", clitest.UnreachableBroker(t))
}
