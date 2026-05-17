## memo-syncer

后台同步器：从 FFLogs 拉取成员日志，归一化后写入 SuMemo Postgres。常驻进程，闲时挂起后自动续上。

完整 schema 见 [`openapi.yaml`](openapi.yaml)。

### Run

```bash
cp .env.example .env        # 至少 DATABASE_URL / REDIS_URL；FFLogs key 见下
go run .
docker compose up -d        # 起本地 pg / redis
```

### Config

| env | required | what it tunes |
|---|---|---|
| `DATABASE_URL` | ✅ | Postgres DSN |
| `REDIS_URL` | ✅ | char_id 缓存与限流 |
| `MEMO_KEY` | ✅* | 回传 `memo-server /fight` 时的 `X-Auth-Key`，仅对外上报时需要 |
| `GRAPH_ID` / `GRAPH_SECRET` | ✅* | FFLogs OAuth 兜底凭据，`logs_keys` 表非空可省 |
| `SYNCER_WORKERS` | | worker 数，默认 8 |
| `RESET_STALE_SYNC` | | 设为 `true` 启动一次会清 `members.logs_sync_time` 触发全量重扫，之后记得 unset |
| `LOG_LEVEL` / `MEMO_ENV` | | 同其他服务 |

完整 env 见 [`.env.example`](.env.example)。

启动时优先读 `logs_keys` 表里的捐赠 key；表空且 env 未设置则拒绝启动。

### Endpoints

| route | response | semantics |
|---|---|---|
| `GET /status/live` | 200 | 进程存活 |
| `GET /status/ready` | 200 / 503 | pg + redis ping，见 [observability.md](https://github.com/open-xiv/memo-docs/blob/main/standards/observability.md) |
| `GET /status` | memo 标准 body | 人 / 面板可读 |
| `GET /metrics` | prometheus | |
| `GET /progress` | 当前轮次状态 + 下次起跑时间 | rate-limited |
| `GET /progress/:name` | 单成员同步状态 | rate-limited |
