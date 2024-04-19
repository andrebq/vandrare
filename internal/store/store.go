package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	_ "modernc.org/sqlite"
)

type (
	noval struct{}

	txclock struct {
		ts   time.Time
		trid int64
	}

	Store struct {
		db   *sql.DB
		trid int64
	}

	ops struct {
		err        error
		parent     *ops
		tx         *sql.Tx
		clock      txclock
		autocommit bool
		closed     bool
	}

	Ops interface {
		Err() error
		ExecContext(context.Context, string, ...any) (sql.Result, error)
		QueryContext(context.Context, string, ...any) (*sql.Rows, error)
		QueryRowContext(context.Context, string, ...any) *sql.Row
		KV() KVOps
		Commit() error
		Rollback() error
		Close() error
		Fail(error)
	}
)

func OpenMemory() (*Store, error) {
	s := &Store{}
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return nil, fmt.Errorf("unable to create database file: %w", err)
	}
	s.db = db
	return s, s.openDB()
}

func Open(dir string) (*Store, error) {
	s := &Store{}
	var err error
	dir, err = filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	mainfile := filepath.Join(dir, "db", "main.sqlite")
	err = os.MkdirAll(filepath.Dir(mainfile), 0755)
	if err != nil {
		return nil, fmt.Errorf("unable to create database directory: %w", err)
	}
	db, err := sql.Open("sqlite3", mainfile)
	if err != nil {
		return nil, fmt.Errorf("unable to create database file: %w", err)
	}
	s.db = db
	return s, s.openDB()
}

func (s *Store) Ops(autocommit bool) Ops {
	tx, err := s.db.Begin()
	if err != nil {
		return &ops{err: err}
	}
	return &ops{
		parent:     nil,
		tx:         tx,
		autocommit: autocommit,
		clock: txclock{
			ts:   time.Now(),
			trid: atomic.AddInt64(&s.trid, 1),
		},
	}
}

func (s *Store) openDB() error {
	err := initDB(s.db)
	if err != nil {
		return fmt.Errorf("unable to initialize database: %w", err)
	}
	return nil
}
