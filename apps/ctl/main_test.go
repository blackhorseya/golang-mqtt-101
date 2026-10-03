package main

import (
	"strings"
	"testing"
	"time"

	"github.com/blackhorseya/golang-mqtt-101/pkg/mqttx/clitest"
)

// 參數錯誤要在連線前就失敗，並說明正確用法。
func TestInvalidArgsExit(t *testing.T) {
	bin := clitest.Build(t, "ctl")
	cases := []struct {
		args []string
		want string
	}{
		{args: nil, want: "usage"},                                // 沒給 command
		{args: []string{"config", "interval"}, want: "key=value"}, // 參數格式錯
		{args: []string{"--qos", "3", "reboot"}, want: "must be 0, 1 or 2"},
	}
	for _, tc := range cases {
		p := clitest.Start(t, bin, append([]string{"--broker", clitest.UnreachableBroker(t)}, tc.args...)...)
		p.WaitExit(3 * time.Second)
		if code := p.ExitCode(); code == 0 {
			t.Errorf("%v: exit code = 0, want non-zero", tc.args)
		}
		if out := p.Output(); !strings.Contains(out, tc.want) {
			t.Errorf("%v: output does not contain %q:\n%s", tc.args, tc.want, out)
		}
	}
}

func TestParseArgs(t *testing.T) {
	got, err := parseArgs([]string{"interval=500ms", "note=a=b"})
	if err != nil || len(got) != 2 || got["interval"] != "500ms" || got["note"] != "a=b" {
		t.Errorf("parseArgs = %v, %v", got, err)
	}
	for _, args := range [][]string{{"interval"}, {"=1s"}} {
		if _, err := parseArgs(args); err == nil {
			t.Errorf("parseArgs(%v) = nil error, want error", args)
		}
	}
}
