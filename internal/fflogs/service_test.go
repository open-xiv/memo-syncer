package fflogs

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func loadTestFile[T any](t *testing.T, name string) T {
	t.Helper()

	path := filepath.Join("testdata", name)
	bytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read test file %s: %v", name, err)
	}

	var target T
	if err := json.Unmarshal(bytes, &target); err != nil {
		t.Fatalf("failed to unmarshal test file %s: %v", name, err)
	}

	return target
}

func TestService_MapToMemo(t *testing.T) {
	jobs := loadTestFile[Jobs](t, "jobs.json")
	detail := loadTestFile[FightDetail](t, "fight_detail_completed.json")

	s := NewService(nil)
	s.jobs = &jobs

	result := s.mapToMemo(detail)

	assert.NotNil(t, result)
	assert.Equal(t, uint32(detail.ReportData.Report.Zone.Id), result.ZoneID)
	assert.NotEmpty(t, result.Players)
}
