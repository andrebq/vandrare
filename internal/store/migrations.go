package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
)

//go:embed migrations/*.sql
var migrations embed.FS

type (
	migrationFile struct {
		name     string
		version  version
		content  []byte
		checksum [sha256.Size]byte
	}

	version [3]int
)

const (
	seedFile = "migrations/0.0.0-seed-migration.sql"
)

func initDB(db *sql.DB) error {
	if err := seedMigrations(db); err != nil {
		return err
	}
	migrations, err := loadMigrations()
	if err != nil {
		// if this failed, the binary has been corrupted
		panic(err)
	}
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].version.Less(migrations[j].version)
	})

	var lastVersion version
	err = db.QueryRow("select ver_major, ver_minor, ver_patch from t_migrations order by 1, 2, 3 limit 1").Scan(&lastVersion[0], &lastVersion[1], &lastVersion[2])
	if errors.Is(err, sql.ErrNoRows) {
		err = nil
	}
	if err != nil {
		return err
	}
	for _, m := range migrations {
		if m.version.Less(lastVersion) {
			continue
		}
		err = applyMigration(db, m)
		if err != nil {
			return fmt.Errorf("unable to apply migration %v: %w", m.name, err)
		}
	}
	return nil
}

func applyMigration(db *sql.DB, m migrationFile) error {
	return inTX(db, func(tx *sql.Tx) error {
		_, err := tx.Exec(string(m.content))
		if err != nil {
			return err
		}
		_, err = tx.Exec("insert into t_migrations(ver_major, ver_minor, ver_patch, filename, content, checksum) values ($1, $2, $3, $4, $5, $6)",
			m.version[0], m.version[1], m.version[2], m.name, string(m.content), m.checksum[:])
		if err != nil {
			return err
		}
		return tx.Commit()
	})
}

func inTX(db *sql.DB, txn func(tx *sql.Tx) error) error {
	tx, err := db.BeginTx(context.Background(), &sql.TxOptions{})
	if err != nil {
		return err
	}
	defer func() {
		if tx != nil {
			tx.Rollback()
		}
	}()
	err = txn(tx)
	if err == nil {
		tx = nil
	}
	return err
}

func (v version) Less(other version) bool {
	return v[0] < other[0] &&
		v[1] < other[1] &&
		v[2] < other[2]
}

func loadMigrations() ([]migrationFile, error) {
	files, err := fs.ReadDir(migrations, "migrations")
	if err != nil {
		return nil, err
	}
	var ret []migrationFile
	for _, f := range files {
		mf := migrationFile{
			name: path.Base(f.Name()),
		}
		ver, err := semver.StrictNewVersion(strings.Split(mf.name, "-")[0])
		if err != nil {
			return nil, err
		}
		mf.version = version{int(ver.Major()), int(ver.Minor()), int(ver.Patch())}
		if mf.version == (version{}) {
			// skip 0.0.0 as it is the seed-migration.sql file anyway
			continue
		}
		mf.content, err = fs.ReadFile(migrations, path.Join("migrations", f.Name()))
		if err != nil {
			return nil, err
		}
		mf.checksum = sha256.Sum256(mf.content)
		ret = append(ret, mf)
	}
	return ret, nil
}

func seedMigrations(db *sql.DB) error {
	content, err := fs.ReadFile(migrations, seedFile)
	if err != nil {
		return err
	}
	_, err = db.Exec(string(content))
	return err
}
