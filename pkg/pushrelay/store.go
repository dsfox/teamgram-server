package pushrelay

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// Server is one server that registered with the relay.
type Server struct {
	Id      string
	Address string
	Blocked bool
}

var ErrTooMany = errors.New("pushrelay: too many registrations from this address today")

// RegistrationsPerDay is how many servers one address may register in a day.
const RegistrationsPerDay = 10

// Store keeps the servers and their keys' hashes - never the keys.
type Store interface {
	Register(ip, address string, now time.Time) (id, key string, err error)
	Lookup(keyHash string) (Server, bool, error)
	Block(id string) error
}

// KeyHash is what the store keeps of a key.
func KeyHash(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

func mint() (id, key string, err error) {
	raw := make([]byte, 40)
	if _, err := rand.Read(raw); err != nil {
		return "", "", err
	}
	return hex.EncodeToString(raw[:8]), hex.EncodeToString(raw[8:]), nil
}

func day(now time.Time) string {
	return now.UTC().Format("2006-01-02")
}

// MemoryStore is the store for tests and for a relay run without a path.
type MemoryStore struct {
	mu      sync.Mutex
	servers map[string]Server // by key hash
	byId    map[string]string // id -> key hash
	perDay  map[string]int    // ip + day
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{servers: map[string]Server{}, byId: map[string]string{}, perDay: map[string]int{}}
}

func (m *MemoryStore) Register(ip, address string, now time.Time) (string, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.perDay[ip+" "+day(now)] >= RegistrationsPerDay {
		return "", "", ErrTooMany
	}
	id, key, err := mint()
	if err != nil {
		return "", "", err
	}
	m.perDay[ip+" "+day(now)]++
	m.servers[KeyHash(key)] = Server{Id: id, Address: address}
	m.byId[id] = KeyHash(key)
	return id, key, nil
}

func (m *MemoryStore) Lookup(keyHash string) (Server, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.servers[keyHash]
	return s, ok, nil
}

func (m *MemoryStore) Block(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	hash, ok := m.byId[id]
	if !ok {
		return fmt.Errorf("pushrelay: no server %s", id)
	}
	s := m.servers[hash]
	s.Blocked = true
	m.servers[hash] = s
	return nil
}

// SQLiteStore is the store on disk: one file, two tables.
type SQLiteStore struct {
	db *sql.DB
}

func OpenSQLite(path string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	for _, ddl := range []string{
		`create table if not exists servers (
			id text primary key, key_hash text not null unique, address text not null,
			ip text not null, created_at integer not null, blocked integer not null default 0)`,
		`create table if not exists registrations (ip text not null, day text not null, count integer not null, primary key (ip, day))`,
	} {
		if _, err := db.Exec(ddl); err != nil {
			return nil, err
		}
	}
	return &SQLiteStore{db: db}, nil
}

func (s *SQLiteStore) Register(ip, address string, now time.Time) (string, string, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return "", "", err
	}
	defer tx.Rollback()
	var count int
	_ = tx.QueryRow(`select count from registrations where ip = ? and day = ?`, ip, day(now)).Scan(&count)
	if count >= RegistrationsPerDay {
		return "", "", ErrTooMany
	}
	id, key, err := mint()
	if err != nil {
		return "", "", err
	}
	if _, err := tx.Exec(`insert into servers (id, key_hash, address, ip, created_at) values (?, ?, ?, ?, ?)`,
		id, KeyHash(key), address, ip, now.Unix()); err != nil {
		return "", "", err
	}
	if _, err := tx.Exec(`insert into registrations (ip, day, count) values (?, ?, 1)
		on conflict (ip, day) do update set count = count + 1`, ip, day(now)); err != nil {
		return "", "", err
	}
	return id, key, tx.Commit()
}

func (s *SQLiteStore) Lookup(keyHash string) (Server, bool, error) {
	var srv Server
	var blocked int
	err := s.db.QueryRow(`select id, address, blocked from servers where key_hash = ?`, keyHash).Scan(&srv.Id, &srv.Address, &blocked)
	if errors.Is(err, sql.ErrNoRows) {
		return Server{}, false, nil
	}
	if err != nil {
		return Server{}, false, err
	}
	srv.Blocked = blocked != 0
	return srv, true, nil
}

func (s *SQLiteStore) Block(id string) error {
	res, err := s.db.Exec(`update servers set blocked = 1 where id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("pushrelay: no server %s", id)
	}
	return nil
}
