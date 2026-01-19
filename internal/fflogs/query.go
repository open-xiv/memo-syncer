package fflogs

import _ "embed"

//go:embed query/character_id_query.graphql
var characterIDQuery string

//go:embed query/best_fight_query.graphql
var bestFightQuery string

//go:embed query/fight_detail_query.graphql
var fightDetailQuery string
