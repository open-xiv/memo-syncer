package version

// Current follows the same major.minor.engine.patch convention as the
// uploader plugins: game major . game minor . engine major . sync patch.
// Bump manually with each release. Reported via X-Client-Version on uploads
// to memo-server, which gates against utils.minClientVersions["SuMemo.Syncer"].
const Current = "7.5.2.1"

// ClientName is a SuMemo-family variant: the auth key resolves to "SuMemo"
// and family-aware ClientAuth (memo-server middleware/auth.go) accepts the
// dotted-hierarchy form. Must match the entry in
// memo-server/utils/client_priority.go.
const ClientName = "SuMemo.Syncer"
