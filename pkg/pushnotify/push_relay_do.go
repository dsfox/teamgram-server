package pushnotify

// PushRelayDO is what the relay gave this server when it registered (#167):
// one row per relay URL. The schema gate reads the tags.
type PushRelayDO struct {
	Url          string `db:"url"`
	ServerId     string `db:"server_id"`
	RelayKey     string `db:"relay_key"`
	RegisteredAt int64  `db:"registered_at"`
}
