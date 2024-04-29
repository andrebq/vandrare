package ssh

import (
	"context"
	"encoding/base64"

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
