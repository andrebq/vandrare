package ssh

import (
	"context"
	"encoding/base64"
	"time"

	"github.com/andrebq/vandrare/internal/monads"
	"github.com/andrebq/vandrare/internal/store"
)

type (
	TokenDB struct {
		Store store.Store
	}

	TokenInfo struct {
		ID          string
		Description string
		ExpiresAt   int64
		Active      bool
	}
)

func (t *TokenDB) Valid(ctx context.Context, token string) (bool, string, error) {
	plaintext, err := base64.URLEncoding.DecodeString(token)
	if err != nil {
		plaintext, err = base64.StdEncoding.DecodeString(token)
		if err != nil {
			return false, "", err
		}
	}
	ops := t.Store.Ops(false)
	defer ops.Close()

	tko := ops.Tokens()
	return tko.Valid(ctx, plaintext)
}

func (t *TokenDB) Issue(ctx context.Context, owner, description string, ttl time.Duration) (string, error) {
	ops := t.Store.Ops(false)
	defer ops.Close()

	tko := ops.Tokens()
	plain, err := tko.Issue(ctx, owner, description, ttl)
	ops.Fail(err)
	ops.Commit()
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString((*plain)[:]), nil
}

func (t *TokenDB) ListActive(ctx context.Context, owner string) ([]TokenInfo, error) {
	ops := t.Store.Ops(false)
	defer ops.Close()

	tko := ops.Tokens()
	entries, err := tko.List(ctx, owner, true)
	if err != nil {
		return nil, err
	}
	ret := make([]TokenInfo, len(entries))
	for i, v := range entries {
		ret[i] = TokenInfo{
			ID:          v.ID,
			Description: v.Description,
			ExpiresAt:   monads.Default[time.Time](v.ExpiresAt, monads.Self(time.Time{})).UnixMilli(),
			Active:      true,
		}
	}
	return ret, nil
}

func (t *TokenDB) Revoke(ctx context.Context, id string) error {
	ops := t.Store.Ops(false)
	defer ops.Close()

	tko := ops.Tokens()
	ops.Fail(tko.Remove(ctx, id))
	return ops.Commit()
}
