package usernames

// UsernameGeneratedDO is a row of username_generated: a username handed out
// by the server rather than chosen by the person (#239). The file name is the
// table name, so tests/schema_gate.py checks these columns against the SQL.
type UsernameGeneratedDO struct {
	UserId    int64  `db:"user_id"`
	Username  string `db:"username"`
	CreatedAt int64  `db:"created_at"`
}
