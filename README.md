# golang-mqtt-101

用 Go 學 MQTT 的練習 repo：用 Mosquitto 當 broker，Go 寫的 device simulator 與 monitor 觀察 MQTT 在連線、斷線、重送時的實際行為。

## 結構

```
apps/device/     device simulator：每個 device 是獨立的 MQTT client
apps/monitor/    用 wildcard 訂閱 device 訊息並印到 terminal
apps/hello/      go.work 範例 app
pkg/mqttx/       共用 module：設定、topic、telemetry payload
deployments/     部署設定（docker/ 內含本機 mosquitto，只綁 127.0.0.1）
go.work          把以上 module 串成一個 workspace
```

## Topic 設計

```
devices/{deviceID}/telemetry     device → monitor，量測資料（JSON）
```

後續 phase 會加入 `devices/{deviceID}/status` 與 `commands/{deviceID}/{command}`。

## 快速開始

開三個 terminal：

```sh
# 1. broker
task broker:up                  # 本機 mosquitto（127.0.0.1:1883）

# 2. monitor
task build && ./bin/monitor

# 3. devices
./bin/device --count 3 --interval 1s
```

monitor 會印出：

```
device-001  temp=28.4  humidity=61  battery=82
device-002  temp=27.1  humidity=58  battery=91
device-003  temp=29.2  humidity=64  battery=73
```

`Ctrl+C` 停止；用完 `task broker:down`。

### 設定

| 參數 | flag | 環境變數 | 預設 |
|------|------|----------|------|
| broker | `--broker` | `MQTT_BROKER_URL` | `tcp://localhost:1883` |
| device 數量 | `--count` | | `1` |
| 發送間隔 | `--interval` | | `2s` |
| monitor 訂閱 | `--topic` | | `devices/+/telemetry` |

## Phase 1 練習：Pub/Sub 與 wildcard

**練習 1：一個 publisher、多個 subscriber**

同時開兩個 `./bin/monitor`，再啟動 device。兩個 monitor 都收到每一筆 telemetry：
broker 會把訊息複製給每個符合的訂閱，publisher 不知道也不在乎有幾個 subscriber。

**練習 2：`+` 與 `#`**

```sh
./bin/monitor --topic 'devices/+/telemetry'   # + 只匹配一層：任一 device 的 telemetry
./bin/monitor --topic 'devices/device-001/#'  # # 匹配其下所有層：device-001 的所有訊息
./bin/monitor --topic 'devices/#'             # 所有 device 的所有訊息
./bin/monitor --topic '+/+/telemetry'         # 想想看：這會收到什麼？
```

`#` 只能放在最後一層；`+` 不會匹配 `/`。（shell 裡記得加引號，`#` 會被當成註解）

**練習 3：沒有 subscriber 時的訊息**

先啟動 device，過幾秒再啟動 monitor。monitor 收不到啟動前發的 telemetry：
QoS 0 且非 retained 的訊息，broker 轉發給當下的訂閱者後就丟棄了。

## 測試

```sh
task test               # 單元測試，不需要 broker
task test:integration   # 啟動 mosquitto → 跑整合測試 → 關閉 broker
```

## 新增一個 app

```sh
mkdir -p apps/<name> && cd apps/<name>
go mod init github.com/blackhorseya/golang-mqtt-101/apps/<name>
cd ../.. && go work use ./apps/<name>
```

要用共用碼時，在該 app 的 `go.mod` 加上：

```
require github.com/blackhorseya/golang-mqtt-101/pkg/mqttx v0.0.0

replace github.com/blackhorseya/golang-mqtt-101/pkg/mqttx => ../../pkg/mqttx
```

`v0.0.0` 是佔位版本，`replace` 讓它指向本地目錄，所以在 app 目錄裡跑 `go get` / `go mod tidy` 也不會去網路抓。
Taskfile 會自動偵測 `apps/*`，不必改。
