package version

// reported via X-Client-Version on uploads; memo-server gates against utils.minClientVersions["SuMemo.Syncer"].
const Current = "7.5.2.1"

// must match the family-aware ClientAuth entry in memo-server/utils/client_priority.go.
const ClientName = "SuMemo.Syncer"
