package ssh

import (
	"context"
	"encoding/base64"
	"time"

	"github.com/andrebq/vandrare/internal/store"
)

type (
	TokenDB struct {
		Store store.Store
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
