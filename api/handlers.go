package api

import (
	"net/http"
	"strings"

	"github.com/open-xiv/memo-syncer/flow"
	"github.com/open-xiv/memo-syncer/model"
	"github.com/open-xiv/memo-syncer/service/memo"

	"github.com/gin-gonic/gin"
)

// GetProgress returns the live progress payload. shape is defined in /openapi.yaml.
func GetProgress(c *gin.Context) {
	// /progress is a live counter; browsers should not cache it.
	c.Header("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")

	snap := memo.CurrentSnapshot()
	resp := model.ProgressResponse{
		State:        snap.State,
		TotalMembers: memo.TotalMembers.Load(),
		Scan:         buildCurrentScan(snap),
		LastScan:     buildLastScan(),
		Workers: model.WorkerStatsResponse{
			Count:   memo.WorkerCount,
			Waiting: snap.WaitingWorkers,
		},
		Keys: buildKeyStats(),
	}

	if !snap.NextScanAt.IsZero() {
		t := snap.NextScanAt
		resp.NextScanAt = &t
	}
	if !snap.KeysRecoverAt.IsZero() {
		t := snap.KeysRecoverAt
		resp.KeysRecoverAt = &t
	}

	c.JSON(http.StatusOK, resp)
}

func buildCurrentScan(snap memo.Snapshot) model.CurrentScanResponse {
	scan := model.CurrentScanResponse{
		Walked:         memo.Walked.Load(),
		ExpectedQueued: memo.ExpectedQueued.Load(),
		Counters: model.CountersResponse{
			FilteredNonCN:   memo.FilteredNonCN.Load(),
			FilteredRecent:  memo.FilteredRecent.Load(),
			Queued:          memo.Queued.Load(),
			NoFFLogsChar:    memo.MembersNoCharID.Load(),
			MembersWithData: memo.MembersWithData.Load(),
			FightsUploaded:  memo.FightsUploaded.Load(),
			Errors:          memo.MemberSyncErrors.Load(),
		},
	}
	if !snap.ScanStartedAt.IsZero() {
		t := snap.ScanStartedAt
		scan.StartedAt = &t
	}
	if m := memo.CurrentMember.Load(); m != nil {
		scan.AtMember = &model.MemberRefDTO{
			ID:     m.ID,
			Name:   m.Name,
			Server: m.Server,
		}
	}
	return scan
}

func buildLastScan() *model.LastScanResponse {
	ls := memo.LastScan()
	if ls == nil {
		return nil
	}
	return &model.LastScanResponse{
		StartedAt:      ls.StartedAt,
		FinishedAt:     ls.FinishedAt,
		DurationMs:     ls.DurationMs,
		Walked:         ls.Walked,
		ExpectedQueued: ls.ExpectedQueued,
		Counters: model.CountersResponse{
			FilteredNonCN:   ls.FilteredNonCN,
			FilteredRecent:  ls.FilteredRecent,
			Queued:          ls.Queued,
			NoFFLogsChar:    ls.NoFFLogsChar,
			MembersWithData: ls.MembersWithData,
			FightsUploaded:  ls.FightsUploaded,
			Errors:          ls.Errors,
		},
	}
}

func buildKeyStats() *model.KeyStatsResponse {
	if memo.Pool == nil {
		return nil
	}
	ps := memo.Pool.PoolSummary()
	resp := &model.KeyStatsResponse{
		Total:          ps.Total,
		Active:         ps.Active,
		Disabled:       ps.Disabled,
		TotalLimit:     ps.TotalLimit,
		TotalRemaining: ps.TotalRemaining,
	}
	if !ps.EarliestReset.IsZero() {
		t := ps.EarliestReset
		resp.EarliestReset = &t
	}
	return resp
}

func GetMemberProgress(c *gin.Context) {
	raw := c.Param("name")

	parts := strings.Split(raw, "@")
	if len(parts) != 2 {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "invalid member format, expect name@server"})
		return
	}
	name := strings.TrimSpace(parts[0])
	server := strings.TrimSpace(parts[1])

	var m model.Member
	if err := flow.DB.Where("name = ? AND server = ?", name, server).
		First(&m).Error; err != nil {
		c.JSON(http.StatusNotFound, model.ErrorResponse{Error: "member not found"})
		return
	}

	c.JSON(http.StatusOK, model.MemberResponse{
		Name:         m.Name,
		Server:       m.Server,
		LastSyncTime: m.LogsSyncTime,
	})
}
