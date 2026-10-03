module github.com/blackhorseya/golang-mqtt-101/apps/ctl

go 1.27.1

require github.com/blackhorseya/golang-mqtt-101/pkg/mqttx v0.0.0

// 本地共用 module；讓此 module 在 GOWORK=off（go mod tidy / go get）時也能解析
replace github.com/blackhorseya/golang-mqtt-101/pkg/mqttx => ../../pkg/mqttx
