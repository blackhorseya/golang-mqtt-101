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
devices/{deviceID}/status        device 在線狀態：{"state":"online|offline","reason":"connected|graceful|lwt"}
```

後續 phase 會加入 `commands/{deviceID}/{command}`。

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
device-001  status=online  reason=connected  online=1
device-002  status=online  reason=connected  online=2
device-003  status=online  reason=connected  online=3
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
| keep alive | `--keepalive` | | `10s` |
| monitor 訂閱（逗號分隔） | `--topic` | | `devices/+/telemetry,devices/+/status` |

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

## Phase 2 練習：Device 狀態與 LWT

device 連上時發 `online`；正常結束前自己發 `offline reason=graceful` 再送 DISCONNECT。
連線時還會先把一則 **Last Will**（`offline reason=lwt`）交給 broker 保管：
device 沒送 DISCONNECT 就消失時，由 broker 代發。monitor 從 `reason` 看得出是哪一種。

先開 monitor，再用 `./bin/device --count 1 --keepalive 5s` 啟動一個 device，然後分別試：

**練習 4：正常下線（Ctrl+C）**

```
device-001  status=offline  reason=graceful  online=0
```

之後不會再出現 `reason=lwt`：broker 收到 DISCONNECT 就把 Last Will 丟掉了。

**練習 5：異常斷線（kill -9）**

```sh
kill -9 $(pgrep -x device)
```

```
device-001  status=offline  reason=lwt  online=0
```

process 來不及送任何東西，但 OS 關閉了 TCP 連線，broker 立刻發現並發布 Last Will。

**練習 6：Keep Alive（kill -STOP）**

```sh
kill -STOP $(pgrep -x device)   # 暫停 process：連線還在，但不再有任何封包
# 等約 7.5 秒（keepalive 5s × 1.5）
kill -CONT $(pgrep -x device)   # 恢復
```

暫停期間 TCP 連線沒斷，broker 只能靠 keep alive 判斷：超過 1.5 倍 keepalive 沒收到封包才判定斷線並發布 `reason=lwt`。
恢復後 device 自動重連，又出現 `status=online`。把 `--keepalive` 改成 30s 再試一次，比較 LWT 出現的時間。
這就是 keep alive 存在的原因：網路靜默中斷（拔網路線、NAT 逾時）時，沒有人會通知 broker。

**練習 7：晚到的 monitor**

先啟動 device，再啟動 monitor。monitor 只看得到 telemetry，看不到任何 status，`online=` 計數也對不上：
status 訊息只送給當下的訂閱者。Phase 3 會用 retained message 解決這個問題。

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
