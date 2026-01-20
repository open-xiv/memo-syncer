package fflogs

type CharacterID struct {
	Data struct {
		CharacterData struct {
			Character struct {
				Id int `json:"id"`
			} `json:"character"`
		} `json:"characterData"`
	} `json:"data"`
}

type EncounterRanked struct {
	Data struct {
		CharacterData struct {
			Character struct {
				Name              string `json:"name"`
				EncounterRankings struct {
					BestAmount         float64 `json:"bestAmount"`
					MedianPerformance  string  `json:"medianPerformance"`
					AveragePerformance string  `json:"averagePerformance"`
					TotalKills         int     `json:"totalKills"`
					FastestKill        int     `json:"fastestKill"`
					Difficulty         int     `json:"difficulty"`
					Metric             string  `json:"metric"`
					Partition          int     `json:"partition"`
					Zone               int     `json:"zone"`
					Ranks              []struct {
						LockedIn              bool   `json:"lockedIn"`
						RankPercent           string `json:"rankPercent"`
						HistoricalPercent     string `json:"historicalPercent"`
						TodayPercent          string `json:"todayPercent"`
						RankTotalParses       int    `json:"rankTotalParses"`
						HistoricalTotalParses int    `json:"historicalTotalParses"`
						TodayTotalParses      int    `json:"todayTotalParses"`
						Report                struct {
							Code      string `json:"code"`
							StartTime int64  `json:"startTime"`
							FightID   int    `json:"fightID"`
						} `json:"report"`
						Duration    int     `json:"duration"`
						StartTime   int64   `json:"startTime"`
						Amount      float64 `json:"amount"`
						BracketData float64 `json:"bracketData"`
						Spec        string  `json:"spec"`
						BestSpec    string  `json:"bestSpec"`
						Class       int     `json:"class"`
					} `json:"ranks"`
				} `json:"encounterRankings"`
			} `json:"character"`
		} `json:"characterData"`
	} `json:"data"`
}

type FightDetail struct {
	Data struct {
		ReportData struct {
			Report struct {
				Zone struct {
					Id int `json:"id"`
				} `json:"zone"`
				StartTime  int `json:"startTime"`
				MasterData struct {
					Actors []struct {
						Id     int     `json:"id"`
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
							Id    int    `json:"id"`
							Type  string `json:"type"`
							Specs []struct {
								Spec string `json:"spec"`
								Role string `json:"role"`
							} `json:"specs"`
						} `json:"composition"`
						DeathEvents []struct {
							Name string `json:"name"`
							Id   int    `json:"id"`
							Type string `json:"type"`
						} `json:"deathEvents"`
					} `json:"data"`
				} `json:"table"`
			} `json:"report"`
		} `json:"reportData"`
	} `json:"data"`
}

type Jobs struct {
	Data struct {
		GameData struct {
			Classes []struct {
				Id    int    `json:"id"`
				Name  string `json:"name"`
				Specs []struct {
					Id   int    `json:"id"`
					Name string `json:"name"`
					Slug string `json:"slug"`
				} `json:"specs"`
			} `json:"classes"`
		} `json:"gameData"`
	} `json:"data"`
}
