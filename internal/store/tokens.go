package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/andrebq/vandrare/internal/monads"
	"golang.org/x/crypto/bcrypt"
)

type (
	Maybe[T any] struct {
	}

	tokenOps struct {
		sqler Ops

		clock txclock
	}

	TokenInfo struct {
		Owner       string
		Description string
		ExpiresAt   monads.Maybe[time.Time]
		ID          string
	}

	TokenOps interface {
		Valid(ctx context.Context, plaintext []byte) (bool, string, error)
		Issue(ctx context.Context, user, description string, ttl time.Duration) (plaintext *[32]byte, err error)
		List(ctx context.Context, user string, onlyActive bool) ([]TokenInfo, error)
		Remove(ctx context.Context, id string) error
	}

	errMsg string
)

const (
	ErrInvalidToken = errMsg("invalid token")
)

func (e errMsg) Error() string { return string(e) }

func (t *tokenOps) Valid(ctx context.Context, plaintext []byte) (bool, string, error) {
	if len(plaintext) != 32 {
		return false, "", ErrInvalidToken
	}
	lookup := base64.RawURLEncoding.EncodeToString(plaintext[:8])
	secret := plaintext[8:]

	var found []byte
	var expires sql.NullInt64
	var user string

	err := t.sqler.QueryRowContext(ctx, "select salted_token, user, expires_at_unixms from dt_token_set where token_id = ?", lookup).Scan(&found, &user, &expires)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			slog.Error("Unable to read tokens from database", "err", err)
		}
		return false, "", ErrInvalidToken
	}

	now := time.Now().UnixMilli()
	if expires.Valid && now > expires.Int64 {
		return false, "", ErrInvalidToken
	}

	err = bcrypt.CompareHashAndPassword(found, secret)
	if err != nil {
		slog.Info("Failed authentication attempt", "lookup", lookup)
		return false, "", ErrInvalidToken
	}
	return true, user, nil
}

func (t *tokenOps) Issue(ctx context.Context, user, description string, ttl time.Duration) (plaintext *[32]byte, err error) {
	var idAndSecret [32]byte
	_, err = rand.Read(idAndSecret[:])
	if err != nil {
		return
	}
	lookupID := idAndSecret[:8]
	secret := idAndSecret[8:]
	salted, err := bcrypt.GenerateFromPassword(secret, bcrypt.DefaultCost)
	if err != nil {
		return
	}
	lookup := base64.RawURLEncoding.EncodeToString(lookupID[:8])

	var expire sql.NullInt64
	if ttl > 0 {
		expire.Int64 = time.Now().Add(ttl).UnixMilli()
		expire.Valid = true
	}

	_, err = t.sqler.ExecContext(ctx, `
		insert into dt_token_set (
			token_id,
			salted_token,
			user,
			description,
			expires_at_unixms,
			clk_updated_at_unixms,
			clk_trid
		) values (
			?,
			?,
			?,
			?,
			?,
			?,
			?
		)`, lookup, salted, user, description, expire, t.clock.ts, t.clock.trid)
	if err != nil {
		return
	}
	plaintext = &idAndSecret
	return
}

func (t *tokenOps) List(ctx context.Context, user string, onlyActive bool) ([]TokenInfo, error) {
	cmd := `select token_id, user, description, expires_at_unixms
		from vw_token_set where user = ?`
	if onlyActive {
		cmd = fmt.Sprintf("%v and is_active", cmd)
	}
	rows, err := t.sqler.QueryContext(ctx, cmd, user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TokenInfo
	for rows.Next() {
		var ti TokenInfo
		var unixTime sql.NullInt64
		err := rows.Scan(&ti.ID, &ti.Owner, &ti.Description, &unixTime)
		if err != nil {
			return nil, err
		}
		if unixTime.Valid {
			ti.ExpiresAt = monads.Some(time.UnixMilli(unixTime.Int64))
		}
		out = append(out, ti)
	}
	return out, nil
}

func (t *tokenOps) Remove(ctx context.Context, id string) error {
	_, err := t.sqler.ExecContext(ctx, "delete from dt_token_set where token_id = ?", id)
	return err
}
