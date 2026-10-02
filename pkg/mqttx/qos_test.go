package mqttx

import "testing"

func TestParseQoS(t *testing.T) {
	for _, q := range []int{0, 1, 2} {
		got, err := ParseQoS(q)
		if err != nil || int(got) != q {
			t.Errorf("ParseQoS(%d) = %d, %v; want %d, nil", q, got, err, q)
		}
	}
	for _, q := range []int{-1, 3} {
		if _, err := ParseQoS(q); err == nil {
			t.Errorf("ParseQoS(%d) = nil error, want error", q)
		}
	}
}
