package store

import (
	"context"
	"database/sql"
	"errors"
)

type (
	KVOps interface {
		SetBytes(context.Context, string, []byte)
		GetBytes(context.Context, []byte, string) []byte
		Err() error
	}

	kvops struct {
		sqler Ops
		err   error

		clock txclock

		cached map[string][]byte
	}
)

// GetBytes from key appended to out. If the key exists returns true.
func (kv *kvops) GetBytes(ctx context.Context, out []byte, key string) []byte {
	if kv.err != nil {
		return nil
	}
	var buf []byte
	if buf, ok := kv.cached[key]; ok {
		out = append(out, buf...)
		return out
	}
	err := kv.sqler.QueryRowContext(ctx, "select item_val from dt_key_value where item_key = $1", key).Scan(&out)
	if errors.Is(err, sql.ErrNoRows) {
		err = nil
	} else if err == nil {
		kv.cached[key] = buf
		out = append(out, buf...)
		return out
	}
	kv.err = err
	return nil
}

// PutBytes from buf into key
func (kv *kvops) SetBytes(ctx context.Context, key string, buf []byte) {
	if kv.err != nil {
		return
	}
	_, kv.err = kv.sqler.ExecContext(ctx,
		`insert into dt_key_value
		(item_key, item_val, clk_updated_at_unixms, clk_trid)
		values
		($1, $2, $3, $4)
		on conflict (item_key) do
			update set
				item_val = excluded.item_val,
				clk_updated_at_unixms = excluded.clk_updated_at_unixms,
				clk_trid = excluded.clk_trid`,
		key, buf, kv.clock.ts.UnixMilli(), kv.clock.trid)
	if kv.err == nil {
		kv.cached[key] = append([]byte(nil), buf...)
	}
}

func (kv *kvops) Err() error {
	if kv.err != nil {
		return kv.err
	}
	return kv.sqler.Err()
}
