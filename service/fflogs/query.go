package fflogs

import _ "embed"

//go:embed query/character_id.graphql
var characterIDQuery string

//go:embed query/best_fights.graphql
var bestFightsQuery string

//go:embed query/fight_detail.graphql
var fightDetailQuery string
