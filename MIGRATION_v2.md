# Engine 2.0 syncer migration

Minimal — syncer has no DB schema dependency on `progress_subphase`, just a JSON DTO that maps to memo-server's expected payload.

## Changes

| File | Change |
|---|---|
| `model/fight.go` | `Progress`: drop `Subphase`, add `PhaseName string` |
| `service/fflogs/service.go` | Build `Progress{ Phase: 0, PhaseName: "", ... }` (FFLogs has no engine-side phase name) |
| `service/memo/client.go` | Send `X-Client-Name: memo-syncer` and `X-Client-Version: <Version>` headers |
| `version/version.go` | New — `Version = "7.5.0.0"`, `ClientName = "memo-syncer"` |

## Coordination

Push **after** memo-server v2 is up and assets v2 is loaded. Otherwise upload requests with `X-Client-Version: 7.5.0.0` will succeed (server's `minClientVersions["memo-syncer"] = "7.5.0.0"`), but new payloads without `progress.subphase` won't fit v1 server's expectations.

## Build verification

```bash
go build ./...
```

Compiles clean on macOS — no platform-specific dependencies.
