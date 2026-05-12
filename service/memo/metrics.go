package memo

import (
	"github.com/open-xiv/memo-syncer/service/keypool"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

func init() {
	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "syncer_total_members",
		Help: "members table COUNT(*) snapshot at scan start (denominator)",
	}, func() float64 { return float64(TotalMembers.Load()) })

	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "syncer_expected_queued",
		Help: "stale+never-synced CN members at scan start (work-to-do estimate)",
	}, func() float64 { return float64(ExpectedQueued.Load()) })

	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "syncer_walked",
		Help: "members walked by producer in current scan (includes filtered)",
	}, func() float64 { return float64(Walked.Load()) })

	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "syncer_filtered_non_cn",
		Help: "members skipped because server isn't CN (current scan)",
	}, func() float64 { return float64(FilteredNonCN.Load()) })

	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "syncer_filtered_recent",
		Help: "members skipped because synced within RecentSyncSkip (current scan)",
	}, func() float64 { return float64(FilteredRecent.Load()) })

	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "syncer_queued",
		Help: "members queued to workers in current scan",
	}, func() float64 { return float64(Queued.Load()) })

	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "syncer_no_fflogs_char",
		Help: "members for which FFLogs has no character entry (current scan)",
	}, func() float64 { return float64(MembersNoCharID.Load()) })

	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "syncer_members_with_data",
		Help: "members FFLogs returned at least one fight for (current scan)",
	}, func() float64 { return float64(MembersWithData.Load()) })

	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "syncer_fights_uploaded",
		Help: "fights successfully POSTed to memo-server (current scan)",
	}, func() float64 { return float64(FightsUploaded.Load()) })

	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "syncer_member_errors",
		Help: "per-member sync errors (FFLogs / memo-server / DB) in current scan",
	}, func() float64 { return float64(MemberSyncErrors.Load()) })

	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "syncer_last_scan_duration_seconds",
		Help: "duration of the most recently completed scan",
	}, func() float64 {
		s := lastScanSnapshot.Load()
		if s == nil {
			return 0
		}
		return float64(s.DurationMs) / 1000.0
	})
	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "syncer_last_scan_fights_uploaded",
		Help: "fights uploaded in the most recently completed scan",
	}, func() float64 {
		s := lastScanSnapshot.Load()
		if s == nil {
			return 0
		}
		return float64(s.FightsUploaded)
	})
	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "syncer_last_scan_errors",
		Help: "per-member errors in the most recently completed scan",
	}, func() float64 {
		s := lastScanSnapshot.Load()
		if s == nil {
			return 0
		}
		return float64(s.Errors)
	})

	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "syncer_keypool_total",
		Help: "total FFLogs keys loaded",
	}, func() float64 {
		if Pool == nil {
			return 0
		}
		return float64(Pool.PoolSummary().Total)
	})
	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "syncer_keypool_active",
		Help: "FFLogs keys currently usable",
	}, func() float64 {
		if Pool == nil {
			return 0
		}
		return float64(Pool.PoolSummary().Active)
	})
	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "syncer_keypool_disabled",
		Help: "FFLogs keys disabled (auth/cooldown)",
	}, func() float64 {
		if Pool == nil {
			return 0
		}
		return float64(Pool.PoolSummary().Disabled)
	})
	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "syncer_keypool_quota_total",
		Help: "sum of LimitPerHour across active keys",
	}, func() float64 {
		if Pool == nil {
			return 0
		}
		return float64(Pool.PoolSummary().TotalLimit)
	})
	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "syncer_keypool_quota_remaining",
		Help: "sum of (Limit - SpentEstimate) across active keys",
	}, func() float64 {
		if Pool == nil {
			return 0
		}
		return float64(Pool.PoolSummary().TotalRemaining)
	})
	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "syncer_keypool_earliest_reset_unix",
		Help: "unix timestamp of earliest active key reset (0 if none)",
	}, func() float64 {
		if Pool == nil {
			return 0
		}
		s := Pool.PoolSummary()
		if s.EarliestReset.IsZero() {
			return 0
		}
		return float64(s.EarliestReset.Unix())
	})

	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "syncer_workers_total",
		Help: "configured worker pool size",
	}, func() float64 { return float64(WorkerCount) })

	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "syncer_workers_waiting",
		Help: "workers currently blocked on key acquisition",
	}, func() float64 { return float64(stateStore.waitingCount.Load()) })

	_ = keypool.ErrAllExhausted
}
