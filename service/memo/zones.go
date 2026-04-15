package memo

// Zone describes one raid encounter we care about syncing.
//
// LogsID: FFLogs encounterID (used in encounterRankings).
// ZoneID: FFXIV ContentFinderCondition ID (used by memo-server duty config).
// Level:  expected party level — memo-server rejects fights whose players
//
//	aren't at this level (duty.Level sanity check).
//
// Current focus is the AAC Cruiserweight tier (M9S/M10S/M11S/M12S in Chinese
// CN-server community numbering). M12S has a door-boss intermission at
// FFLogs encounterID 104 which we skip — 105 is the "real" boss phase.
type Zone struct {
	LogsID int  // FFLogs encounterID
	ZoneID uint // FFXIV zone (CFC) id
	Level  uint // expected player level
}

var InterestZones = []Zone{
	{LogsID: 101, ZoneID: 1321, Level: 100}, // M9S
	{LogsID: 102, ZoneID: 1323, Level: 100}, // M10S
	{LogsID: 103, ZoneID: 1325, Level: 100}, // M11S
	{LogsID: 105, ZoneID: 1327, Level: 100}, // M12S (skip 104 = door boss)
}
