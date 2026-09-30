# golang-mqtt-101

用 Go 學 MQTT 的練習 repo，採 `go.work` 多 module 結構：每個 app 是獨立 module。

## 結構

```
apps/<name>/     每個 app 一個 module（package main）
pkg/mqttx/       共用 module
deployments/     部署設定（docker/ 內含本機 mosquitto）
go.work          把以上 module 串成一個 workspace
```

## 快速開始

```sh
task broker:up      # 啟動本機 mosquitto (localhost:1883)
task build          # 編譯所有 app 到 ./bin/
./bin/hello
```

## 新增一個 app

```sh
mkdir -p apps/<name> && cd apps/<name>
go mod init github.com/blackhorseya/golang-mqtt-101/apps/<name>
cd ../.. && go work use ./apps/<name>
```

要用共用碼時，在該 app 的 `go.mod` 加上
`require github.com/blackhorseya/golang-mqtt-101/pkg/mqttx v0.0.0`，
workspace 會自動解析到本地的 `pkg/mqttx`。Taskfile 會自動偵測 `apps/*`，不必改。
