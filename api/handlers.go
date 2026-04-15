package api

import (
	"memo-syncer/flow"
	"memo-syncer/model"
	"memo-syncer/service/memo"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func GetProgress(c *gin.Context) {
	// /progress is a live counter; browsers should NEVER cache it.
	// Without these headers, Chrome's heuristic cache keeps showing a
	// stale body while the sync goroutine keeps updating in the server.
	c.Header("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")

	snap := memo.CurrentSnapshot()
	total := memo.Total
	processed := memo.Processed.Load()
	lastID := uint(memo.LastID.Load())

	resp := model.ProgressResponse{
		State:          snap.State,
		Total:          total,
		Current:        processed,
		LastID:         lastID,
		WaitingWorkers: snap.WaitingWorkers,
		Counters: model.ScanCounters{
			FilteredNonCN:    memo.FilteredNonCN.Load(),
			FilteredRecent:   memo.FilteredRecent.Load(),
			Queued:           memo.Queued.Load(),
			NoFFLogsChar:     memo.MembersNoCharID.Load(),
			MembersWithData:  memo.MembersWithData.Load(),
			FightsUploaded:   memo.FightsUploaded.Load(),
			MemberSyncErrors: memo.MemberSyncErrors.Load(),
		},
	}

	if !snap.ScanStartedAt.IsZero() {
		t := snap.ScanStartedAt
		resp.ScanStartedAt = &t
	}
	if !snap.ScanFinishedAt.IsZero() {
		t := snap.ScanFinishedAt
		resp.ScanFinishedAt = &t
	}
	if !snap.NextScanAt.IsZero() {
		t := snap.NextScanAt
		resp.NextScanAt = &t
	}
	if !snap.KeysRecoverAt.IsZero() {
		t := snap.KeysRecoverAt
		resp.KeysRecoverAt = &t
	}

	if lastID > 0 {
		var m model.Member
		if err := flow.DB.First(&m, lastID).Error; err == nil {
			resp.Name = m.Name
			resp.Server = m.Server
		}
	}

	if memo.Pool != nil {
		ps := memo.Pool.PoolSummary()
		ks := &model.KeySummary{
			Total:          ps.Total,
			Active:         ps.Active,
			Disabled:       ps.Disabled,
			TotalLimit:     ps.TotalLimit,
			TotalRemaining: ps.TotalRemaining,
		}
		if !ps.EarliestReset.IsZero() {
			t := ps.EarliestReset
			ks.EarliestReset = &t
		}
		resp.Keys = ks
	}

	c.JSON(http.StatusOK, resp)
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
