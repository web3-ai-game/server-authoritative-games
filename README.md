# server-authoritative-games

五個遊戲引擎。結果由伺服器的 `crypto/rand` 決定。前端只負責演出。

Five game engines. The server draws the result with `crypto/rand`. The client only plays the animation.

這是技術示範。沒有真錢，沒有博彩服務，沒有入金。私有原庫裡的頁面、部署和帳號系統沒有放進來。

This is a technical demo. No real money, no gambling service, no deposits. The pages, deploy files, and account system from the private original are not here.

## 為什麼結果不能放在前端

如果機率在瀏覽器裡算，玩家改一個請求就能改獎勵。這裡每個引擎都是純函式加一個 RNG。呼叫端把 RNG 傳進去，測試可以換一個固定序列，正式路徑用 `crypto/rand`。

If the odds run in the browser, a player can edit one request and change the prize. Each engine here is a pure function plus an RNG. The caller passes the RNG in. Tests inject a fixed sequence. Production passes `crypto/rand`.

捕魚的單次期望值寫成 RTP 乘上彈價。擊殺機率是 RTP 除以賠率。瞄準哪一隻，不改變期望值。RTP 由呼叫端傳入，並在呼叫端夾在合理區間。

For the fishing game, expected value per shot is RTP times the bullet price. Kill probability is RTP divided by the payout multiplier. Which target you aim at does not change the expected value. The caller passes RTP in and clamps it.

| 套件 | 規則 |
|---|---|
| `internal/slot` | 5 輪 3 行，20 條賠付線。Wild 不替代 Scatter。5 reels, 3 rows, 20 lines. Wild does not replace Scatter. |
| `internal/fish` | 期望值與瞄準目標無關。Expected value does not depend on the target. |
| `internal/wheel` | 權重輪盤，`crypto/rand`。Weighted wheel. |
| `internal/bj` | 二十一點牌局。Blackjack shoe. |
| `internal/poker` | 牌型比較。Hand comparison. |

每個套件旁邊都有測試。

Each package has tests beside it.

## 本地執行

需要 Go 1.23 以上。這個目錄沒有第三方依賴。

Needs Go 1.23 or newer. This directory has no third-party modules.

```bash
go test ./internal/...
```

沒有 `main`。這是引擎，不是一個可上線的站。接上你自己的帳號和餘額之後，再把結果送去前端演出。

There is no `main`. This is the engine, not a site you can deploy. Attach your own accounts and balances, then send the result to a client for the animation.

## 沒有放進來的東西

登入、餘額帳本、Telegram 登入驗證、頁面、部署腳本。那些留在私有庫。

Login, the balance ledger, Telegram login verification, the pages, and the deploy scripts. Those stay in the private repo.

## 授權

未另附授權檔。看原始碼可以，拿去商用前先來信 one@oece-x.com。

No license file is attached. Reading the source is fine. Ask before commercial use: one@oece-x.com.
