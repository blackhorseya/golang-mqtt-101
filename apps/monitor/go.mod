module github.com/blackhorseya/golang-mqtt-101/apps/monitor

go 1.27.1

require github.com/blackhorseya/golang-mqtt-101/pkg/mqttx v0.0.0

require (
	github.com/eclipse/paho.mqtt.golang v1.5.1 // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
	golang.org/x/net v0.44.0 // indirect
	golang.org/x/sync v0.17.0 // indirect
)

// 本地共用 module；讓此 module 在 GOWORK=off（go mod tidy / go get）時也能解析
replace github.com/blackhorseya/golang-mqtt-101/pkg/mqttx => ../../pkg/mqttx
