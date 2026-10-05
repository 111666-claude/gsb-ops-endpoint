# ops-endpoint

游戏服务的只读运维端点：健康检查、水位快照与按玩家的限流窗口。仓库里没有命令行入口，
现状值用 `go test -run TestSampleAnchors -v` 打印，只用 Go 标准库（`net/http` 加 `httptest`）。

```
go test -run TestSampleAnchors -v ./ops
go test ./...
go vet ./...
```

## 口径（README 为准）

- **路由**：只有 `/healthz` 与 `/stats` 两个端点；其它路径一律 404；方法不是 GET 一律 405。
- **`/healthz`**：返回 200 与 `ok`。
- **`/stats`**：返回 JSON，至少包含 `queue`、`workers`、`requests` 三个字段，值取当前快照。
- **限流**：按请求头 `X-Player` 分客户端计数，同一客户端同一整秒内超过 `limit` 次返回 429，
  并在响应头写 `Retry-After: 1`；不同客户端互不影响。窗口按整秒分桶，进入下一秒重新计算。
- **只读**：端点不改变队列与 worker 水位。
- **代价**：限流判定不许每次全扫客户端表，`Work()` 不随客户端数乘请求数增长；10 万客户端每秒 10 万次请求。

## 输出契约（不改格式）

`go test -run TestSampleAnchors -v` 打印：

```
unknown_status=..
method_status=..
second_player_status=.. retry_after=..
```

## 目录

```
ops/server.go           路由、水位快照与限流窗口
ops/server_test.go      端到端用例与现状值样例
```
