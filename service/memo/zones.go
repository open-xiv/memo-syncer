package memo

// LogsID is the FFLogs encounterID (used in encounterRankings).
// ZoneID is the FFXIV ContentFinderCondition ID (used by memo-server duty config).
// Level is the expected party level — memo-server rejects fights whose players aren't at this level.
type Zone struct {
	LogsID int
	ZoneID uint
	Level  uint
}

// current focus is the AAC Cruiserweight tier (M9S/M10S/M11S/M12S).
// M12S has a door-boss intermission at FFLogs encounterID 104 which we skip — 105 is the real boss phase.
var InterestZones = []Zone{
	{LogsID: 101, ZoneID: 1321, Level: 100},
	{LogsID: 102, ZoneID: 1323, Level: 100},
	{LogsID: 103, ZoneID: 1325, Level: 100},
	{LogsID: 105, ZoneID: 1327, Level: 100},
}
