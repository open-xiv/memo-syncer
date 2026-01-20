//go:build integration
// +build integration

package fflogs

import (
	"context"
	"memo-uploader-fflog/internal/config"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const (
	testName        = "鱼里昂热"
	testRegion      = "cn"
	testServer      = "拉诺西亚"
	testCharacterID = 13601693
	testZone        = 101
	testReportCode  = "c19qTGZgvkxM3jdX"
	testFightID     = 9
)

func TestClient_Integration(t *testing.T) {
	cfg, err := config.LoadConfig()
	assert.NoError(t, err, "load env through system environment")

	client := NewClient(cfg.ClientID, cfg.ClientSecret)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	t.Run("FetchJobs", func(t *testing.T) {
		res, err := client.FetchJobs(ctx)
		assert.NoError(t, err)
		assert.NotNil(t, res)
		assert.NotEmpty(t, res.GameData.Classes)
	})

	t.Run("FetchCharacterID", func(t *testing.T) {
		id, err := client.FetchCharacterID(ctx, testName, testServer, testRegion)
		assert.NoError(t, err)
		assert.Equal(t, testCharacterID, id)
	})

	t.Run("FetchFightDetail", func(t *testing.T) {
		res, err := client.FetchFightDetail(ctx, testReportCode, testFightID)
		assert.NoError(t, err)
		assert.NotNil(t, res)
		assert.NotEmpty(t, res.ReportData.Report)
	})

	t.Run("FetchBestFightByEncounter", func(t *testing.T) {
		res, err := client.FetchBestFightByEncounter(ctx, testCharacterID, testFightID)
		assert.NoError(t, err)
		assert.NotNil(t, res)
	})
}
