package main

import (
	"testing"
	"time"
)

func TestParseInterval(t *testing.T) {
	got, err := parseInterval(map[string]string{"interval": "500ms"})
	if err != nil || got != 500*time.Millisecond {
		t.Errorf("parseInterval = %s, %v, want 500ms", got, err)
	}
	for _, args := range []map[string]string{
		{},                             // 缺 interval
		{"interval": "fast"},           // 不是 duration
		{"interval": "0s"},             // 必須大於 0
		{"interval": "1s", "qos": "2"}, // 不支援的參數
	} {
		if _, err := parseInterval(args); err == nil {
			t.Errorf("parseInterval(%v) = nil error, want error", args)
		}
	}
}

func TestParseDowntime(t *testing.T) {
	if got, err := parseDowntime(map[string]string{}); err != nil || got != defaultRebootDowntime {
		t.Errorf("parseDowntime(no args) = %s, %v, want %s", got, err, defaultRebootDowntime)
	}
	if got, err := parseDowntime(map[string]string{"downtime": "1s"}); err != nil || got != time.Second {
		t.Errorf("parseDowntime(1s) = %s, %v, want 1s", got, err)
	}
	for _, args := range []map[string]string{{"downtime": "-1s"}, {"downtime": "soon"}, {"delay": "1s"}} {
		if _, err := parseDowntime(args); err == nil {
			t.Errorf("parseDowntime(%v) = nil error, want error", args)
		}
	}
}
