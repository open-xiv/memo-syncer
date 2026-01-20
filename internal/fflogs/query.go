package fflogs

import _ "embed"

//go:embed query/character_id.graphql
var characterIDQuery string

//go:embed query/best_fight.graphql
var bestFightQuery string

//go:embed query/fight_detail.graphql
var fightDetailQuery string

//go:embed query/jobs.graphql
var jobsQuery string
