package fflogs

type CharacterID struct {
	CharacterData struct {
		Character struct {
			ID int `json:"id"`
		} `json:"character"`
	} `json:"characterData"`
}

type Rank struct {
	Report struct {
		Code    string `json:"code"`
		FightID int    `json:"fightID"`
	} `json:"report"`
}

type EncounterRanking struct {
	Ranks []Rank `json:"ranks"`
}

// BestFights is the batched response from best_fights.graphql where four zones are fetched in one query via GraphQL aliases (m1..m4).
type BestFights struct {
	CharacterData struct {
		Character struct {
			M1 EncounterRanking `json:"m1"`
			M2 EncounterRanking `json:"m2"`
			M3 EncounterRanking `json:"m3"`
			M4 EncounterRanking `json:"m4"`
		} `json:"character"`
	} `json:"characterData"`
}

// Zone returns the aliased ranking for a 0-indexed position (0..3).
func (b *BestFights) Zone(i int) *EncounterRanking {
	switch i {
	case 0:
		return &b.CharacterData.Character.M1
	case 1:
		return &b.CharacterData.Character.M2
	case 2:
		return &b.CharacterData.Character.M3
	case 3:
		return &b.CharacterData.Character.M4
	}
	return nil
}

type FightDetail struct {
	ReportData struct {
		Report struct {
			Zone struct {
				ID int `json:"id"`
			} `json:"zone"`
			StartTime  int `json:"startTime"`
			MasterData struct {
				Actors []struct {
					ID     int     `json:"id"`
					Name   string  `json:"name"`
					Server *string `json:"server"`
				} `json:"actors"`
			} `json:"masterData"`
			Fights []struct {
				EncounterID    int     `json:"encounterID"`
				StartTime      int     `json:"startTime"`
				EndTime        int     `json:"endTime"`
				Kill           bool    `json:"kill"`
				BossPercentage float64 `json:"bossPercentage"`
			} `json:"fights"`
			Table struct {
				Data struct {
					TotalTime   int `json:"totalTime"`
					CombatTime  int `json:"combatTime"`
					Composition []struct {
						Name  string `json:"name"`
						ID    int    `json:"id"`
						Type  string `json:"type"`
						Specs []struct {
							Spec string `json:"spec"`
							Role string `json:"role"`
						} `json:"specs"`
					} `json:"composition"`
					DeathEvents []struct {
						Name string `json:"name"`
						ID   int    `json:"id"`
						Type string `json:"type"`
					} `json:"deathEvents"`
				} `json:"data"`
			} `json:"table"`
		} `json:"report"`
	} `json:"reportData"`
}
