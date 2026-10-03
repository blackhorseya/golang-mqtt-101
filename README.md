# golang-mqtt-101

用 Go 學 MQTT 的練習 repo：用 Mosquitto 當 broker，Go 寫的 device simulator 與 monitor 觀察 MQTT 在連線、斷線、重送時的實際行為。

## 結構

```
apps/device/     device simulator：每個 device 是獨立的 MQTT client
apps/monitor/    用 wildcard 訂閱 device 訊息並印到 terminal
apps/ctl/        對 device 發送 command 並等待 ack
apps/hello/      go.work 範例 app
pkg/mqttx/       共用 module：設定、topic、telemetry payload、模擬斷線
deployments/     部署設定（docker/ 內含本機 mosquitto，只綁 127.0.0.1）
go.work          把以上 module 串成一個 workspace
```

## Topic 設計

```
devices/{deviceID}/telemetry     device → monitor，量測資料（JSON）
devices/{deviceID}/status        device 在線狀態：{"state":"online|offline","reason":"connected|graceful|lwt|reboot"}
devices/{deviceID}/ack           device → ctl，command 的執行結果：{"id":"…","command":"reboot","ok":true}
commands/{deviceID}/{command}    ctl → device，command：{"id":"…","args":{"interval":"500ms"}}
```

device 訂閱 `commands/{deviceID}/#`。ack 放在 `devices/` 底下而不是 `commands/{deviceID}/` 底下，否則 device 會收到自己發的 ack。

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
device-001  temp=28.4  humidity=61  battery=82  seq=1  qos=0  lat=0.6ms  accepted
device-002  temp=27.1  humidity=58  battery=91  seq=1  qos=0  lat=0.5ms  accepted
device-003  temp=29.2  humidity=64  battery=73  seq=1  qos=0  lat=0.7ms  accepted
```

`Ctrl+C` 停止；用完 `task broker:down`。

### 設定

| 參數 | flag | 環境變數 | 預設 |
|------|------|----------|------|
| broker | `--broker` | `MQTT_BROKER_URL` | `tcp://localhost:1883` |
| device 數量 | `--count` | | `1` |
| 發送間隔 | `--interval` | | `2s` |
| keep alive | `--keepalive` | | `10s` |
| device ID 前綴 | `--prefix` | | `device` |
| status / LWT 是否 retained | `--retain` | | `true` |
| 清除 retained status 後結束 | `--clear-retained` | | `false` |
| telemetry QoS | `--qos`（device） | | `0` |
| 訂閱 QoS 上限 | `--qos`（monitor） | | `2` |
| 統計回報間隔 | `--report`（兩者皆有） | | `5s` |
| 應用層重送機率 | `--dup-rate`（device） | | `0` |
| 跳號（送出前遺失）機率 | `--skip-rate`（device） | | `0` |
| monitor 訂閱（逗號分隔） | `--topic` | | `devices/+/telemetry,devices/+/status` |
| 每隔多久模擬一次斷線（0 表示不斷） | `--outage-every`（兩者皆有） | | `0` |
| 每次斷線多久 | `--outage-for`（兩者皆有） | | `5s` |
| clean session | `--clean-session`（兩者皆有） | | `true` |
| 自動重連 backoff 上限 | `--max-reconnect-interval`（兩者皆有） | | `10m` |
| monitor 的 ClientID | | `MQTT_CLIENT_ID` | `monitor-{pid}` |
| ctl 的目標 device | `--device`（ctl） | | `device-001` |
| ctl 等待 ack 的上限 | `--timeout`（ctl） | | `10s` |
| command 的 QoS | `--qos`（ctl） | | `1` |

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

用 `./bin/device --retain=false` 先啟動 device，再啟動 monitor。monitor 只看得到 telemetry，看不到任何 status，`online=` 計數也對不上：
一般訊息只送給當下的訂閱者。Phase 3 用 retained message 解決這個問題（現在預設就是 retained）。

## Phase 3 練習：Retained Messages

發布時帶 retain flag 的訊息，broker 會替該 topic **保留最新的一則**（每個 topic 最多一則）。
之後有 client 訂閱到這個 topic，broker 會立刻把保留的那則送給它。
device 的 status 與 LWT 現在預設都是 retained。

monitor 會在因為訂閱而收到的 retained message 後面標上 `(retained)`；
即時轉發給既有訂閱者的訊息，即使發布時帶了 retain flag，收到時也不會有這個標記（MQTT 3.1.1 的規則）。

**練習 8：晚到的 monitor（retained 版）**

先啟動 `./bin/device --count 3`，過幾秒再啟動 monitor：

```
device-001  status=online  reason=connected  online=1  (retained)
device-002  status=online  reason=connected  online=2  (retained)
device-003  status=online  reason=connected  online=3  (retained)
```

和練習 7 比較：monitor 一連上就知道誰在線。

**練習 9：retained offline**

啟動一個 device，`kill -9` 它，然後才啟動 monitor：

```
device-001  status=offline  reason=lwt  online=0  (retained)
```

broker 代發的 LWT 也是 retained，所以後來的 monitor 也看得到 device 是怎麼離線的。
如果 LWT 沒有 retained，broker 保留的會是最後一則 `online`，monitor 會以為 device 還在線。

**練習 10：最新值，不是歷史**

device 依序經歷 online → Ctrl+C（offline graceful），之後啟動 monitor：只會收到一則 `offline reason=graceful (retained)`，
看不到之前的 online。retained message 是「這個 topic 目前的值」，不是訊息紀錄。

再試：昨天跑過 `--count 10`、今天只跑 `--count 3`，monitor 仍會列出 10 個 device，其中 7 個 offline ——
retained message 會一直留著，直到被覆蓋或清除。

**練習 11：清除 retained message**

```sh
./bin/device --count 10 --clear-retained
```

對 topic 發布**空 payload 的 retained message** 就會刪掉 broker 保留的那則。正在跑的 monitor 會即時看到：

```
device-001  status=cleared  online=0
```

之後才啟動的 monitor 則什麼都收不到。

> 本機 mosquitto 沒有開啟 persistence：`task broker:down` 之後 retained message 也會跟著消失。

## Phase 4 練習：QoS

| QoS | 名稱 | publisher → broker 的交握 | 保證 |
|-----|------|---------------------------|------|
| 0 | at most once | PUBLISH | 可能遺失，不會重複 |
| 1 | at least once | PUBLISH → PUBACK | 不會遺失，可能重複（沒收到 PUBACK 就重送） |
| 2 | exactly once | PUBLISH → PUBREC → PUBREL → PUBCOMP | 這一段不遺失、不重複 |

device 與 monitor 都會定期（`--report`）在 stderr 印統計：

```
# device
stats (qos 1): published=490  confirmed=490  unconfirmed=0  avg_ack=2.93ms  sent=69609B  received=2000B
# monitor
stats: received=490  qos0=0  qos1=490  qos2=0  dup=0  avg_lat=2.7ms
```

- `confirmed` / `unconfirmed`：publisher 只知道 broker 有沒有確認。QoS 0 沒有確認，token 一寫進網路層就完成，連「被丟掉」也算 confirmed。
  QoS 1/2 等不到確認時記為 unconfirmed，但訊息還在 client 裡，重連後可能照樣送出 —— **unconfirmed 不等於遺失**，subscriber 到底收到幾則 publisher 永遠不知道
- `avg_ack`：從 Publish 到 token 完成的時間，大致是 QoS 0 ≈ 0、QoS 1 ≈ 1 個來回、QoS 2 ≈ 2 個來回
- `sent` / `received`：連線上實際傳輸的位元組數（含 CONNECT、PINGREQ 等），用來比較協定開銷
- monitor 每行的 `qos=` 是實際送達的 QoS，`lat=` 是 device 產生資料到 monitor 收到的時間，`dup` 是 broker 重送時帶的 DUP flag

**練習 12：同樣的工作量，不同的 QoS**

```sh
./bin/device --count 10 --interval 100ms --qos 0 --report 0   # 跑 10 秒後 Ctrl+C
./bin/device --count 10 --interval 100ms --qos 1 --report 0
./bin/device --count 10 --interval 100ms --qos 2 --report 0
```

比較三次結束時的 `avg_ack` 與 `sent` / `received`。本機實測（各跑 5 秒）：

```
stats (qos 0): published=470  confirmed=470  unconfirmed=0  avg_ack=30µs   sent=65996B  received=40B
stats (qos 1): published=490  confirmed=490  unconfirmed=0  avg_ack=2.93ms sent=69609B  received=2000B
stats (qos 2): published=490  confirmed=490  unconfirmed=0  avg_ack=4.16ms sent=71579B  received=3960B
```

- QoS 0 的 `received` 只有連線時的 CONNACK 等封包；沒有任何確認
- QoS 1 每則多收一個 PUBACK（4 bytes）：490 × 4 ≈ 1960
- QoS 2 每則多收 PUBREC + PUBCOMP（8 bytes）、多送一個 PUBREL：`received` 約為 QoS 1 的兩倍，`sent` 也更多
- `avg_ack` 隨著要等的確認次數增加

**練習 13：訂閱 QoS 是上限**

```sh
./bin/monitor --qos 0
./bin/device --qos 2
```

monitor 印出的是 `qos=0`：實際送達的 QoS = min(發布的 QoS, 訂閱的 QoS)。
broker 對 publisher 用 QoS 2 交握，對這個 subscriber 只用 QoS 0 —— QoS 是**每一段各自**的約定，不是端到端的。

**重要觀念：QoS 2 不是應用層的 exactly-once**

QoS 只保證「一段連線、一次 session 內」不重複：publisher ↔ broker 一段、broker ↔ subscriber 一段。
應用層照樣會看到重複，例如：

- publisher 等不到確認，**應用程式自己**重送（換了新的 packet ID，broker 視為新訊息）
- clean session 斷線重連，in-flight 的狀態被丟棄後重來
- broker 重啟

所以應用程式仍然需要自己的 idempotency。Phase 5 會在 telemetry 加上序號，讓 monitor 自己判斷重複與遺漏。

> 這個 phase 只看「正常連線」下的 QoS 差異。
> 斷線時的行為差異（QoS 0 在重連期間直接丟棄、QoS 1/2 先存起來重連後再送）見 Phase 6。
> 本機沒有網路問題，`dup` 幾乎不會出現。

## Phase 5 練習：序號與 Idempotency

每筆 telemetry 帶兩個欄位：

- `seq`：每個 device 從 1 開始、每筆 +1 的序號
- `run`：device 這次 process 的識別（啟動時間）。device 重啟後 `seq` 從 1 重來，`run` 也會不同

monitor 對每個 device 只記住 `(run, 上一個處理過的 seq)`，據此判斷每筆：

| 判斷 | 條件 | 處理嗎 |
|------|------|--------|
| `accepted` | seq = last + 1（或第一次看到這個 device / 換了 run） | 是 |
| `gap(missing=N)` | seq > last + 1，中間 N 筆遺失 | 是 |
| `duplicate` | seq = last | 否 |
| `out-of-order` | seq < last | 否 |

這就是應用層的 idempotency：同一筆資料收到幾次都只處理一次。統計中的 `duplicate` 是應用層依序號看到的重複，
`dup` 則是 MQTT 協定層的 DUP flag，兩者不同。

**練習 14：QoS 2 也擋不住的重複**

```sh
./bin/monitor
./bin/device --qos 2 --dup-rate 0.3
```

```
device-001  ...  seq=7  qos=2  lat=4.1ms  accepted
device-001  ...  seq=7  qos=2  lat=4.3ms  duplicate
```

`--dup-rate` 模擬應用程式自己重送（例如等不到回應就再發一次）：對 MQTT 來說這是一則全新的訊息，
QoS 2 的交握照樣完成、broker 照樣轉發。結束時比較 device 的 `retries` 與 monitor 的 `duplicate`，兩者相同（本機實測都是 38）。

**練習 15：gap**

```sh
./bin/device --skip-rate 0.2
```

`--skip-rate` 讓序號用掉但不發出（模擬送出前就遺失）。monitor 看到 `gap(missing=N)`。

比較 device 的 `skipped` 與 monitor 的 `missing`，`missing` 通常會**少一點**（本機實測 23 vs 21）：
只有被前後兩筆夾住的遺失才看得出來。device 的第一筆就被跳過時，monitor 從第二筆開始算；
最後幾筆被跳過時，後面沒有訊息可以讓 monitor 發現 —— 序號只能偵測「之後有人來」的遺失。
Phase 6 斷線時用 QoS 0 會看到真正的遺失（練習 18）。

**練習 16：device 重啟**

device 跑一陣子後 Ctrl+C 再啟動：monitor 看到 `seq=1 ... accepted`，而不是一路 `out-of-order`。
如果只記序號、不記 `run`，重啟後的每一筆都會比舊的 last 小，直到追上為止都會被丟掉。

**練習 17：晚到的 monitor**

先啟動 device，過一陣子再啟動 monitor：第一筆可能是 `seq=57`，直接 `accepted`，不算 gap ——
monitor 不知道之前發生什麼，只能從它看到的第一筆開始算。

**只記一個數字的限制**

- MQTT 對同一個 publisher、同一個 topic 保證順序，所以本機幾乎看不到 `out-of-order`
- 一則很晚才到的舊訊息，和一則重複的舊訊息，都會落在 `out-of-order`，分不出來
- 要分得出來就得記住處理過的序號集合（例如滑動視窗），那已經是「複雜的去重系統」了 —— 這個 repo 刻意不做

## Phase 6 練習：網路中斷

`--outage-every 6s --outage-for 3s` 讓程式每 6 秒把自己的 TCP 連線直接關掉，接下來 3 秒內重新連線都會失敗。
它不送 DISCONNECT，所以對 broker 來說就和網路線被拔掉一樣；device 和 monitor 都有這兩個 flag。
（device 的所有連線一起斷，模擬整個 fleet 所在的網路中斷。）

```
2026/10/03 11:07:50 outage: cut 1 connection(s) for 3s
2026/10/03 11:07:50 device-001: connection lost: read: simulated network outage
2026/10/03 11:07:50 device-001: reconnecting
2026/10/03 11:07:51 device-001: reconnecting
2026/10/03 11:07:53 device-001: reconnecting
2026/10/03 11:07:53 device-001: status online (connected)
```

斷線那一刻 broker 就看到 TCP 關閉，monitor 會立刻收到 `status=offline  reason=lwt`；重連後 device 再宣告 `online`。
LWT 剛好把每次中斷框起來。

斷線期間 paho 的 client 處於 reconnecting 狀態，這時呼叫 `Publish`：

| QoS | paho 怎麼處理 | token | 重連後 |
|-----|---------------|-------|--------|
| 0 | 直接丟掉 | **立刻回報完成，沒有錯誤** | 沒了 |
| 1、2 | 存在 client 端（預設是記憶體） | 等到重連、broker 確認後才完成 | 依序補送 |

**練習 18：QoS 0 斷線就遺失，而且 publisher 不知道**

```sh
./bin/monitor
./bin/device --interval 500ms --qos 0 --outage-every 6s --outage-for 3s
```

```
device-001  ...  seq=19  qos=0  lat=5.5ms  gap(missing=7)
```

本機實測斷 3 秒、每 0.5 秒一筆：monitor 看到 `gap(missing=7)`，而 device 的統計是 `confirmed=29  unconfirmed=0` —— **每一筆都「成功」了**。
QoS 0 的完成只代表「交出去了」，斷線時連交出去都沒有，paho 照樣回報完成。
不靠序號，從 publisher 端完全看不出這 7 筆不見了。

**練習 19：QoS 1/2 補送，但 publisher 看到的是 unconfirmed**

```sh
./bin/monitor
./bin/device --interval 500ms --qos 1 --outage-every 6s --outage-for 3s
```

monitor 沒有任何 gap，但斷線期間那幾筆的 `lat` 是秒級：它們在 device 裡排隊，重連後一口氣送出。
device 那邊則每筆都等不到確認（每筆最多等 2 秒），統計記成 `unconfirmed` ——
**unconfirmed 不等於遺失**，它們在重連後送達了。QoS 2 的結果相同。

補送的前提是 process 還活著：排隊的訊息存在記憶體，斷線期間按 Ctrl+C 就跟著消失。
（這時 device 印出的 `status offline (graceful)` 也是 QoS 0，同樣在 reconnecting 狀態下被默默丟掉了。）

**練習 20：重連策略與 backoff**

```sh
./bin/device --qos 1 --outage-every 25s --outage-for 10s
./bin/device --qos 1 --outage-every 25s --outage-for 10s --max-reconnect-interval 1s
```

看兩者 `reconnecting` 的時間點（本機實測，斷線 10 秒）：

| `--max-reconnect-interval` | 重連嘗試（距斷線） | 恢復連線 |
|----------------------------|--------------------|----------|
| `10m`（paho 預設） | 0、1、3、7、15 秒 | 15 秒後 |
| `1s` | 每秒一次 | 10 秒後 |

paho 每次重連失敗就把等待時間加倍，上限是 MaxReconnectInterval。
backoff 讓上千台 device 不會在 broker 剛恢復時同時湧入，代價是網路已經好了，device 卻還在等下一次嘗試 ——
上表預設值那一列多斷了 5 秒，這段時間的資料全部在 device 裡排隊。
另外 `SetConnectRetryInterval` 只管**第一次**連線失敗的重試間隔，和斷線後的 backoff 是兩回事。

**練習 21：subscriber 斷線 —— clean session vs persistent session**

這次讓 monitor 斷線，device 正常發送 QoS 1：

```sh
./bin/device --interval 500ms --qos 1
./bin/monitor --qos 1 --outage-every 6s --outage-for 3s                          # clean session
./bin/monitor --qos 1 --outage-every 6s --outage-for 3s --clean-session=false    # persistent session
```

| monitor | 結果（本機實測斷 3 秒） |
|---------|------|
| clean session | `gap(missing=6)`：broker 不記得這個 client，斷線期間的訊息沒有人收 |
| persistent session | 沒有 gap、`avg_lat=548ms`：broker 依 ClientID 保留訂閱，替它把訊息排隊，重連後補送 |

注意 device 那邊斷線的是 **publisher 的 client 端佇列**（練習 19）；這裡排隊的是 **broker** —— 兩個不同的地方。

persistent session 要排得到隊，三個條件缺一不可：

- 同一個 ClientID：broker 用 ClientID 找回 session
- 實際送達的 QoS ≥ 1：發布與訂閱的 QoS 都要 ≥ 1。mosquitto 預設不替離線 client 排 QoS 0 訊息
- broker 沒有重啟：這個 repo 的 mosquitto 沒開 persistence，session 只存在記憶體

**練習 22：persistent session 跨越 process 重啟**

monitor 預設的 ClientID 帶 pid，每次啟動都不同；用環境變數固定它：

```sh
./bin/device --interval 500ms --qos 1
MQTT_CLIENT_ID=my-monitor ./bin/monitor --qos 1 --clean-session=false
# Ctrl+C，等幾秒再啟動同一行
```

```
connected to tcp://localhost:1883 as my-monitor (clean session false)
device-001  ...  seq=6   qos=1  lat=4.0361s  accepted
device-001  ...  seq=7   qos=1  lat=3.5361s  accepted
...
device-001  ...  seq=14  qos=1  lat=36.7ms   accepted
subscribed to devices/+/telemetry,devices/+/status
```

離線期間的訊息在**訂閱完成之前**就到了：broker 回 CONNACK 時就知道 session 還在，馬上補送排隊的訊息，
不用等 monitor 重新 SUBSCRIBE。`lat` 從 4 秒遞減，正好是每筆在 broker 排隊的時間。

用完之後，以同一個 ClientID 用 clean session 連一次（`MQTT_CLIENT_ID=my-monitor ./bin/monitor`），broker 就會刪掉這個 session；
不然它會一直替一個不會回來的 client 排隊（mosquitto 預設最多排 1000 則）。

**device 端的 `--clean-session`**

paho 在**自動重連**時不論 clean session 與否，都會補送 client 端排隊的訊息，所以 device 端看不太出差別。
差別在 broker：clean session 下 broker 每次連線都丟掉這個 client 的 in-flight 狀態。
理論上，QoS 2 的訊息剛送出、還沒收到 PUBREC 就斷線，重連後 paho 重送，broker 已經忘了它 —— 可能轉發兩次。
時機很難湊到，本機沒有觀察到；有興趣可以用很短的 `--outage-every` 搭配 `--qos 2` 長時間跑，看 monitor 有沒有 `duplicate`。

## Phase 7 練習：Command 與 Ack

前面的訊息都是 device → server。這個 phase 反過來：用 `ctl` 對 device 發 command，等 device 回 ack。

```sh
./bin/ctl --device device-001 config interval=500ms   # 改 telemetry 間隔，立刻生效
./bin/ctl --device device-001 reboot                  # 假重開機：斷線 → 等 3 秒 → 重連
./bin/ctl --device device-001 reboot downtime=5s
```

MQTT 本身沒有「回應」：publish 出去就結束了，publisher 不知道誰收到、結果如何。要做 request/response 得自己來：

1. ctl 訂閱 `devices/{deviceID}/ack`，**等到 SUBACK 才往下走**
2. ctl 把帶隨機 `id` 的 command 發到 `commands/{deviceID}/{command}`（QoS 1、**不 retained**）
3. device 執行後把同一個 `id` 放進 ack 發回來
4. ctl 只認 `id` 相同的 ack（同一個 ack topic 上可能有別人的 command 的回應），或等到逾時

第 1 步的順序很重要：device 回 config 的 ack 只要幾毫秒，如果先發 command 再訂閱，ack 可能在訂閱生效前就到了。
它不是 retained，broker 不會替還沒訂閱的人留著，ctl 就永遠等不到。
（MQTT 5 在協定裡加了 response topic 與 correlation data，做的就是這件事；這個 repo 用的 3.1.1 沒有。）

device 端還有一個限制：paho 收到訊息時依序呼叫 handler，handler 卡住整個收訊就卡住。
所以 handler 只把 command 丟進 channel，由 device 自己的迴圈去執行、發 ack、斷線重連。

**練習 23：config 與 request/response**

```sh
./bin/monitor
./bin/device --interval 1s
./bin/ctl config interval=300ms
```

```
device-001  ack  command=config  id=096ff14151b0b7a6  ok=true  rtt=6.1ms
```

monitor 也會印出同一則 ack（它預設訂閱 `devices/+/ack`），telemetry 從每秒一筆變成每 0.3 秒一筆。
`rtt` 是 ctl 發出 command 到收到 ack 的時間：command 經 broker 到 device、ack 再經 broker 回來。

**練習 24：失敗也要回 ack**

```sh
./bin/ctl config interval=0s
./bin/ctl selfdestruct
```

```
device-001  ack  command=config  id=e838a9fc0e2fdd37  ok=false  rtt=1ms  error=parse interval "0s": must be positive
device-001  ack  command=selfdestruct  id=02951564875ec1ac  ok=false  rtt=7ms  error=unknown command "selfdestruct"
```

ctl 以非零結束。如果 device 遇到不認得的 command 就默默忽略，發送端只能等到逾時，然後分不出「device 不在」和「device 不會做」。

**練習 25：reboot**

```sh
./bin/monitor
./bin/device --interval 1s
./bin/ctl reboot downtime=2s
```

```
device-001  status=offline  reason=reboot  online=0
device-001  ack  command=reboot  id=5c3c72ba000c0d97  ok=true
device-001  status=online  reason=connected  online=1
device-001  temp=...  seq=1  ...  accepted
```

- 是 `reason=reboot` 不是 `lwt`：device 自己宣告 offline 後正常 DISCONNECT，broker 不會發布 LWT
- 重開機後是新的 `run`，`seq` 從 1 開始，monitor 照樣 `accepted`（Phase 5 練習 16）
- ack 在**重連之後**才發，所以 ctl 的 `rtt=2.0161s` 約等於 downtime：ctl 等到的是「做完了」，而不只是「收到了」
- device 先發 online 再發 ack，monitor 卻可能先印 ack（如上）。MQTT 只保證同一個 topic 上的順序，
  online（`status`、QoS 0）和 ack（`ack`、QoS 1）是不同 topic，本機實測就出現了顛倒

**練習 26：沒有回應**

```sh
./bin/ctl --device device-999 --timeout 3s reboot
```

```
device-999 reboot 3f2a...: no ack after 3s: timeout
```

broker 收下 command（QoS 1 的 PUBACK 來自 **broker**，不是 device），但沒有人訂閱，訊息直接消失。
逾時只代表「沒等到 ack」：可能 device 不在線、command 遺失，也可能 device 執行了但 ack 遺失。
重試之前要想清楚：reboot 重送一次就是再重開一次。

**練習 27：retained command 的災難**

ctl 刻意不用 retained。用容器裡的 `mosquitto_pub` 發一則 retained 的 reboot 看看：

```sh
./bin/device --interval 1s
docker compose -f deployments/docker/docker-compose.yaml exec mosquitto \
  mosquitto_pub -t commands/device-001/reboot -r -q 1 -m '{"id":"retained-1","args":{"downtime":"1s"}}'
```

device 開始無限重開機（本機實測 6 秒內 7 次，ack 的 id 每次都是 `retained-1`）：
每次重連都重新訂閱 `commands/device-001/#`，broker 就把保留的 reboot 再送一次。用空的 retained payload 清掉才停：

```sh
docker compose -f deployments/docker/docker-compose.yaml exec mosquitto \
  mosquitto_pub -t commands/device-001/reboot -r -n
```

（podman 使用者把 `docker compose` 換成 `podman compose`。）

每次的 `id` 都一樣，device 只要記住執行過的 command id 就不會重複執行 —— 又回到 Phase 5 的 idempotency。
QoS 1 的 command 本來就可能送達兩次，同樣的道理也適用。這個 repo 刻意不做這個去重，留給你想想要記多少個 id、記多久。

**練習 28：device 斷線時發 command**

把 Phase 6 的斷線和 session 搭在一起：

```sh
./bin/device --interval 1s --outage-every 6s --outage-for 4s --max-reconnect-interval 1s
./bin/device --interval 1s --outage-every 6s --outage-for 4s --max-reconnect-interval 1s --clean-session=false
# 在 device 斷線期間（log 出現 connection lost 之後）：
./bin/ctl --timeout 8s config interval=500ms
```

| device | command QoS | 結果（本機實測，斷線 4 秒） |
|--------|-------------|------|
| clean session | 1 | `timeout`：broker 不記得 device 的訂閱，command 沒人收 |
| persistent session | 1 | `ok=true  rtt=2.9824s`：broker 替 device 排隊，重連後立刻送達 |
| persistent session | 0 | `timeout`：broker 不替離線的 client 排 QoS 0 |

persistent session 的 device 重連時，log 裡 `interval set to 500ms` 出現在 `status online` **之前**：
broker 在 CONNACK 之後就送出排隊的 command，比 device 重新訂閱、宣告 online 都早（Phase 6 練習 22 的同一個現象）。

這種「device 上線就補收離線期間的 command」很實用，但要搭配有效期限：
一個 3 小時前的 reboot 在 device 回來時才執行，可能已經不是任何人想要的了。
用完記得以同一個 ClientID 用 clean session 連一次，清掉 broker 上的 session（見練習 22）。

## 測試

```sh
task test               # 單元測試，不需要 broker
task test:integration   # 啟動 mosquitto → 跑整合測試 → 關閉 broker
task test:taskfile      # 驗證 Taskfile 的 container CLI 偵測
```

GitHub Actions（`.github/workflows/ci.yml`）在每個 PR 與 main 上跑同樣的 `build` / `vet` / `test` / `test:taskfile` / `test:integration`。

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
