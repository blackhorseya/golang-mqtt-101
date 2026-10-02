package mqttx

import "fmt"

// ParseQoS 檢查 QoS 是否為 MQTT 定義的 0、1、2。
//
//	0：at most once —— 送出就不管，沒有確認
//	1：at least once —— broker 回 PUBACK；沒收到確認就重送，所以可能重複
//	2：exactly once —— PUBREC / PUBREL / PUBCOMP 四次交握，只保證這一段（client ↔ broker）不重複
func ParseQoS(q int) (byte, error) {
	if q < 0 || q > 2 {
		return 0, fmt.Errorf("parse qos %d: must be 0, 1 or 2", q)
	}
	return byte(q), nil
}
