package memo

import (
	"context"
	"memo-syncer/flow"
	"memo-syncer/model"
	"memo-syncer/service/fflogs"
	"strings"
	"time"
	"unicode"

	"github.com/rs/zerolog/log"
)

var (
	Total  int64
	LastID uint
)

type ZonePair struct {
	LogsID int
	ZoneID uint
}

var ZoneInterest = []ZonePair{
	{101, 1321},
	{102, 1323},
	{103, 1325},
	{105, 1327},
}

func SyncMembers() error {
	if err := flow.DB.Model(&model.Member{}).Count(&Total).Error; err != nil {
		return err
	}

	var processed int64 = 0
	batchSize := 1000

	for {
		var members []model.Member
		err := flow.DB.Where("id > ?", LastID).
			Order("id ASC").
			Limit(batchSize).
			Find(&members).Error

		if err != nil {
			return err
		}

		if len(members) == 0 {
			break
		}

		for _, member := range members {
			processed++
			LastID = member.ID

			if processed > Total {
				Total = processed
			}

			// skip recently synced members
			if member.LogsSyncTime != nil && time.Since(*member.LogsSyncTime) < time.Hour {
				continue
			}

			// skip non-chinese servers
			if IsEnServer(member.Server) {
				continue
			}

			// sync zone
			if err := SyncMemberZones(member); err != nil {
				log.Error().Err(err).Msgf("sync member %s@%s failed", member.Name, member.Server)
				continue
			}

			// update sync time
			now := time.Now()
			member.LogsSyncTime = &now
			if err := flow.DB.Save(&member).Error; err != nil {
				log.Error().Err(err).Msgf("update member %s@%s sync time failed", member.Name, member.Server)
			}
		}
	}

	return nil
}

func IsEnServer(s string) bool {
	for _, r := range s {
		if r > unicode.MaxASCII {
			return false
		}
	}
	return true
}

func SyncMemberZones(member model.Member) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for _, zonePair := range ZoneInterest {
		// query from fflogs
		fight, err := fflogs.GetMemberZoneBestProgress(ctx, member.Name, member.Server, zonePair.LogsID)
		if err != nil {
			// 429 too many requests -> sleep for 1 hour
			errMsg := err.Error()
			if !strings.Contains(errMsg, "404") {
				log.Warn().Err(err).Msgf("sync member %s@%s [zone: %d] failed: likely rate limited, sleeping for 1 hour", member.Name, member.Server, zonePair.ZoneID)
				time.Sleep(30 * time.Minute)
			}
			log.Error().Err(err).Msgf("sync member %s@%s [zone: %d] failed: fetch", member.Name, member.Server, zonePair.ZoneID)
			time.Sleep(300 * time.Millisecond)
			continue
		}

		if fight == nil {
			log.Info().Msgf("sync member %s@%s [zone: %d] no data", member.Name, member.Server, zonePair.ZoneID)
			time.Sleep(300 * time.Millisecond)
			continue
		}

		// upload fight
		fight.ZoneID = zonePair.ZoneID
		if err := CreateFight(ctx, fight); err != nil {
			log.Error().Err(err).Msgf("sync member %s@%s [zone: %d] failed: post", member.Name, member.Server, zonePair.ZoneID)
			time.Sleep(300 * time.Millisecond)
			continue
		}

		log.Info().Msgf("sync member %s@%s [zone: %d] success", member.Name, member.Server, zonePair.ZoneID)
		time.Sleep(300 * time.Millisecond)
	}

	return nil
}
