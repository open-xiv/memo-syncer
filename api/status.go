package api

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"github.com/open-xiv/memo-syncer/buildinfo"
	"github.com/open-xiv/memo-syncer/flow"
)

// healthProbeTimeout caps one dep probe. ctx is also bound by the parent
// (kubelet's probe timeoutSeconds cancels earlier) so this is the human-
// triggered /status ceiling, not the kubelet path.
const healthProbeTimeout = 5 * time.Second

type Check struct {
	OK        bool   `json:"ok"`
	LatencyMs *int64 `json:"latency_ms,omitempty"`
	Error     string `json:"error,omitempty"`
}

type StatusResponse struct {
	Service       string           `json:"service"`
	Version       string           `json:"version"`
	Build         string           `json:"build"`
	Env           string           `json:"env"`
	StartedAt     time.Time        `json:"started_at"`
	UptimeSeconds int64            `json:"uptime_seconds"`
	Status        string           `json:"status"`
	Checks        map[string]Check `json:"checks"`
}

// Status is the human/monitor-facing endpoint; runs dep probes inline (not cached, since calls are infrequent).
func Status(c *gin.Context) {
	checks := runChecks(c.Request.Context())

	overall := "ok"
	code := http.StatusOK
	for _, ch := range checks {
		if !ch.OK {
			overall = "down"
			code = http.StatusServiceUnavailable
			break
		}
	}

	c.JSON(code, StatusResponse{
		Service:       buildinfo.Service,
		Version:       buildinfo.Version,
		Build:         buildinfo.Build,
		Env:           buildinfo.Env,
		StartedAt:     buildinfo.StartedAt,
		UptimeSeconds: int64(time.Since(buildinfo.StartedAt).Seconds()),
		Status:        overall,
		Checks:        checks,
	})
}

// StatusLive is the kubelet liveness probe: 200 iff the process can answer HTTP.
// failing liveness restarts the pod, so transient dep outages must never propagate here.
func StatusLive(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// StatusReady is the kubelet readiness probe: 200 iff all critical dep checks pass, otherwise 503.
// failing readiness pulls the pod from service endpoints but doesn't restart it.
func StatusReady(c *gin.Context) {
	checks := runChecks(c.Request.Context())
	for _, ch := range checks {
		if !ch.OK {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "down"})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// runChecks fans the dep probes out across goroutines, each with its own
// context.WithTimeout, then collects them. a previous version shared one
// budget across a sequential map literal — a slow DB ping over WG could
// starve the redis check and surface as a false dual-failure that flapped
// readiness 503s without actually being a dual outage. memo-discord-bot
// hit this hard (335 restarts / 44h) because it stacked a self-kill on
// top; syncer just took intermittent kubelet pull-from-service hits.
func runChecks(parent context.Context) map[string]Check {
	type result struct {
		name  string
		check Check
	}
	out := make(chan result, 2)

	runWithCtx := func(name string, fn func(context.Context) Check) {
		ctx, cancel := context.WithTimeout(parent, healthProbeTimeout)
		defer cancel()
		out <- result{name, fn(ctx)}
	}

	go runWithCtx("database", dbCheck)
	go runWithCtx("redis", redisCheck)

	checks := make(map[string]Check, 2)
	for i := 0; i < 2; i++ {
		r := <-out
		checks[r.name] = r.check
	}
	logFailedChecks(checks)
	return checks
}

// logFailedChecks emits a warn per failed dep so transient outages surface
// in the log stream — previously the only signal was the /status JSON
// payload, which nobody reads until something else goes wrong.
func logFailedChecks(checks map[string]Check) {
	for name, ch := range checks {
		if ch.OK {
			continue
		}
		log.Warn().
			Str("event", "health.dep_failed").
			Str("dep", name).
			Str("error", ch.Error).
			Msg("dependency check failed")
	}
}

func dbCheck(ctx context.Context) Check {
	start := time.Now()
	sqlDB, err := flow.DB.DB()
	if err != nil {
		return Check{OK: false, Error: err.Error()}
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		return Check{OK: false, Error: err.Error()}
	}
	ms := time.Since(start).Milliseconds()
	return Check{OK: true, LatencyMs: &ms}
}

func redisCheck(ctx context.Context) Check {
	start := time.Now()
	if err := flow.Redis.Ping(ctx).Err(); err != nil {
		return Check{OK: false, Error: err.Error()}
	}
	ms := time.Since(start).Milliseconds()
	return Check{OK: true, LatencyMs: &ms}
}
