package main

import (
	"math"
	"math/rand/v2"
	"time"

	"github.com/blackhorseya/golang-mqtt-101/pkg/mqttx"
)

// sensor 以 random walk 產生看起來合理的量測值；數值本身不重要，重點是讓每個 device 有東西可發。
type sensor struct {
	deviceID string
	run      int64
	seq      uint64
	rng      *rand.Rand
	temp     float64
	humidity int
	battery  int
}

func newSensor(deviceID string, run int64, rng *rand.Rand) *sensor {
	return &sensor{
		deviceID: deviceID,
		run:      run,
		rng:      rng,
		temp:     20 + rng.Float64()*10,
		humidity: 40 + rng.IntN(30),
		battery:  70 + rng.IntN(31),
	}
}

// next 往前走一步並回傳這次的量測值，序號 +1。電量只會遞減。
func (x *sensor) next(now time.Time) mqttx.Telemetry {
	x.seq++
	x.temp = math.Round(min(max(x.temp+x.rng.Float64()-0.5, -20), 60)*10) / 10
	x.humidity = min(max(x.humidity+x.rng.IntN(3)-1, 0), 100)
	if x.battery > 0 && x.rng.IntN(5) == 0 {
		x.battery--
	}
	return mqttx.Telemetry{
		DeviceID:  x.deviceID,
		Run:       x.run,
		Seq:       x.seq,
		Temp:      x.temp,
		Humidity:  x.humidity,
		Battery:   x.battery,
		Timestamp: now.UTC(),
	}
}
