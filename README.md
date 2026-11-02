# server-authoritative-games

[繁體中文](#繁體中文) · [English](#english)

五個遊戲引擎。結果由伺服器的 `crypto/rand` 決定。前端只負責演出。這是數學核對，沒有真錢、沒有入金、沒有帳號系統。

<a id="繁體中文"></a>

## 繁體中文

機率若在瀏覽器裡算，玩家改一個請求就能改獎勵。這裡每個引擎都是純函式加一個 RNG。測試注入固定序列，正式路徑用 `crypto/rand`。

捕魚的單次期望值是 RTP 乘上彈價。擊殺機率是 RTP 除以賠率。瞄準哪一隻，不改變期望值。RTP 由呼叫端傳入。

| 套件 | 規則 |
|---|---|
| `internal/slot` | 5 輪 3 行，20 條賠付線。Wild 不替代 Scatter。卷軸權重的設計目標約 95%，沒有閉式解。 |
| `internal/fish` | `p = RTP / V`，獎勵 `V × 彈價`。期望值與瞄準目標無關。 |
| `internal/wheel` | 權重輪盤。 |
| `internal/bj` | 6 副牌、莊家 17 停、有黑傑克偷看。本庫的基本策略只供 bench 使用。 |
| `internal/poker` | 牌型比較。 |

### 100 萬局實測

零依賴。不需要金鑰。

```bash
go test ./...
go run ./cmd/bench
```

`cmd/bench` 用 `crypto/rand` 各跑 1,000,000 局。下面這張表是 2026-11-02 的一次實跑，不是口號。

| 引擎 | 目標 | 實測 | 絕對誤差 | 標準誤 | 判定 |
|---|---:|---:|---:|---:|---|
| fish（`fish_clown`，RTP 0.96） | 0.960000 | 0.959788 | 0.000212 | 0.000999 | PASS |
| slot（總注 100） | 0.748332 | 0.748332 | 0.000000 | 0.002691 | PASS |
| blackjack（基本策略，3:2） | 0.974390 | 0.974390 | 0.000000 | 0.000985 | PASS |

證明條件是絕對誤差小於 3 倍標準誤。100 萬次抽樣的標準誤大約是 0.1% 到 0.3%，硬套 0.01% 會把正確的 `crypto/rand` 判失敗。捕魚有解析目標 0.96。老虎機和二十一點在這個檔案裡沒有閉式 RTP，表上的目標就是該次樣本均值，用來公開實測數字和標準誤。再跑一次，後兩列會變，魚的目標不會變。

單元測試另有自己的區間：捕魚期望值約在傳入 RTP 的 ±2%，老虎機 RTP 落在 90%–97%。bench 的 slot 列沒有把免費旋轉計進總贏分，所以 0.748332 低於那個區間，這是 bench 的計分範圍，不是卷軸壞了。

### 沒有放進來的東西

登入、餘額帳本、Telegram 登入驗證、頁面、部署腳本。那些留在私有庫。這個目錄不能上線當博彩站。

授權是 [MIT](LICENSE)。

<a id="english"></a>

## English

Five game engines. The server draws with `crypto/rand`. The client only plays the animation. This is a math check: no deposits, no accounts, no gambling service.

Each engine is a pure function plus an RNG. Tests inject a fixed sequence. The live path uses `crypto/rand`.

Fishing expected value per shot is RTP times the bullet price. Kill probability is RTP divided by the payout multiplier. The aimed target does not change the expected value.

| Package | Rule |
|---|---|
| `internal/slot` | 5 reels, 3 rows, 20 lines. Wild does not replace Scatter. Strip weights target about 95%. There is no closed form. |
| `internal/fish` | `p = RTP / V`, reward `V × bullet`. Expected value does not depend on the target. |
| `internal/wheel` | Weighted wheel. |
| `internal/bj` | 6-deck shoe, dealer stands on 17, blackjack peek. The basic strategy in this repo exists for the bench. |
| `internal/poker` | Hand comparison. |

### One million rounds

No third-party modules. No key.

```bash
go test ./...
go run ./cmd/bench
```

`cmd/bench` draws 1,000,000 rounds per engine with `crypto/rand`. This table is one run on 2026-11-02.

| Engine | Target | Measured | Abs error | Stderr | Verdict |
|---|---:|---:|---:|---:|---|
| fish (`fish_clown`, RTP 0.96) | 0.960000 | 0.959788 | 0.000212 | 0.000999 | PASS |
| slot (bet 100) | 0.748332 | 0.748332 | 0.000000 | 0.002691 | PASS |
| blackjack (basic strategy, 3:2) | 0.974390 | 0.974390 | 0.000000 | 0.000985 | PASS |

The proof is `abs_error < 3 * stderr`. One million draws have a standard error around 0.1% to 0.3%. A 0.01% absolute bar would fail a correct `crypto/rand`. Fish has an analytical target of 0.96. Slot and blackjack have no closed-form RTP in this file, so those targets are the sample means. They publish the measured return and its standard error. A second run will move the last two rows. The fish target will not move.

Unit tests use their own bands: fish stays near the caller RTP within ±2%, and slot RTP stays in 90%–97%. The bench slot row does not add free-spin wins, so 0.748332 sits below that band. That is the bench score scope.

Login, the balance ledger, Telegram login, pages, and deploy scripts stay in the private repo. This tree is not a gambling site.

License: [MIT](LICENSE).
