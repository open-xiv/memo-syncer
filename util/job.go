package util

import "strings"

type Job struct {
	Name  string
	Role  string
	Order int
}

var JobMap = map[int]Job{
	0:  {Name: "none", Role: "Limited"},
	1:  {Name: "gladiator", Role: "Tank", Order: 1},
	2:  {Name: "pugilist", Role: "DPS", Order: 1},
	3:  {Name: "marauder", Role: "Tank", Order: 1},
	4:  {Name: "lancer", Role: "DPS", Order: 1},
	5:  {Name: "archer", Role: "DPS", Order: 3},
	6:  {Name: "conjurer", Role: "Healer", Order: 1},
	7:  {Name: "thaumaturge", Role: "DPS", Order: 4},
	8:  {Name: "carpenter", Role: "Crafter"},
	9:  {Name: "blacksmith", Role: "Crafter"},
	10: {Name: "armorer", Role: "Crafter"},
	11: {Name: "goldsmith", Role: "Crafter"},
	12: {Name: "leatherworker", Role: "Crafter"},
	13: {Name: "weaver", Role: "Crafter"},
	14: {Name: "alchemist", Role: "Crafter"},
	15: {Name: "culinarian", Role: "Crafter"},
	16: {Name: "miner", Role: "Gatherer"},
	17: {Name: "botanist", Role: "Gatherer"},
	18: {Name: "fisher", Role: "Gatherer"},
	19: {Name: "paladin", Role: "Tank", Order: 2},
	20: {Name: "monk", Role: "DPS", Order: 1},
	21: {Name: "warrior", Role: "Tank", Order: 1},
	22: {Name: "dragoon", Role: "DPS", Order: 1},
	23: {Name: "bard", Role: "DPS", Order: 3},
	24: {Name: "whitemage", Role: "Healer", Order: 1},
	25: {Name: "blackmage", Role: "DPS", Order: 4},
	26: {Name: "arcanist", Role: "DPS", Order: 4},
	27: {Name: "summoner", Role: "DPS", Order: 4},
	28: {Name: "scholar", Role: "Healer", Order: 2},
	29: {Name: "rogue", Role: "DPS", Order: 1},
	30: {Name: "ninja", Role: "DPS", Order: 1},
	31: {Name: "machinist", Role: "DPS", Order: 3},
	32: {Name: "darkknight", Role: "Tank", Order: 1},
	33: {Name: "astrologian", Role: "Healer", Order: 1},
	34: {Name: "samurai", Role: "DPS", Order: 2},
	35: {Name: "redmage", Role: "DPS", Order: 4},
	36: {Name: "bluemage", Role: "Limited"},
	37: {Name: "gunbreaker", Role: "Tank", Order: 2},
	38: {Name: "dancer", Role: "DPS", Order: 3},
	39: {Name: "reaper", Role: "DPS", Order: 2},
	40: {Name: "sage", Role: "Healer", Order: 2},
	41: {Name: "viper", Role: "DPS", Order: 1},
	42: {Name: "pictomancer", Role: "DPS", Order: 4},
}

var jobNameToID map[string]int

func init() {
	jobNameToID = make(map[string]int, len(JobMap))
	for id, job := range JobMap {
		jobNameToID[strings.ToLower(job.Name)] = id
	}
}

func GetJobID(name string) uint {
	if id, ok := jobNameToID[strings.ToLower(name)]; ok {
		return uint(id)
	}
	return 0
}
