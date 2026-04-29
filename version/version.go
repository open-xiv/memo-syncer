package version

// Current follows the same major.minor.engine.patch convention as the
// uploader plugins: game major . game minor . engine major . sync patch.
// Bump manually with each release. Reported via X-Client-Version on uploads
// to memo-server, which gates against utils.minClientVersions["memo-syncer"].
const Current = "7.5.0.0"

// ClientName matches the entry in memo-server/utils/client_priority.go and
// must equal the auth-key-derived name (see middleware/auth.go ClientAuth).
const ClientName = "memo-syncer"
