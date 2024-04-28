package store

import (
	"context"
	"encoding/json"
)

type (
	HasError interface {
		Err() error
	}
	ByteOps interface {
		HasError
		GetBytes(context.Context, []byte, string) []byte
		SetBytes(context.Context, string, []byte)
	}
)

func GetJSON(ctx context.Context, out any, ops ByteOps, key string) error {
	buf := ops.GetBytes(ctx, nil, key)
	if buf == nil {
		return errNotFound
	}
	return json.Unmarshal(buf, out)
}

func PutJSON(ctx context.Context, ops ByteOps, key string, val any) error {
	buf, err := json.Marshal(val)
	if err != nil {
		return err
	}
	ops.SetBytes(ctx, key, buf)
	return ops.Err()
}
