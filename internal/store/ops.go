package store

import (
	"context"
	"database/sql"
)

func (o *ops) KV() KVOps {
	return &kvops{
		err:    o.err,
		sqler:  o,
		clock:  o.clock,
		cached: make(map[string][]byte),
	}
}

func (o *ops) Tokens() TokenOps {
	return &tokenOps{
		clock: o.clock,
		sqler: o,
	}
}

func (o *ops) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return o.tx.ExecContext(ctx, query, args...)
}

func (o *ops) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return o.tx.QueryContext(ctx, query, args...)
}

func (o *ops) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return o.tx.QueryRowContext(ctx, query, args...)
}

func (o *ops) Commit() error {
	if o.err != nil {
		return o.err
	}
	if o.parent != nil {
		return nil
	}
	o.err = o.tx.Commit()
	if o.err == nil {
		o.err = sql.ErrTxDone
	}
	return nil
}

func (o *ops) Rollback() error {
	if o.err != nil {
		return o.err
	}
	if o.parent != nil {
		return o.parent.Rollback()
	}
	o.err = o.tx.Rollback()
	if o.err == nil {
		o.err = sql.ErrTxDone
	}
	return nil
}

func (o *ops) Close() error {
	if o.parent != nil {
		// only the parent can close the transaction
		return o.err
	}
	switch {
	case o.closed:
		return o.err
	case o.autocommit && o.err == nil:
		return o.Commit()
	default:
		return o.Rollback()
	}
}

func (o *ops) Err() error {
	if o.err != nil {
		return o.err
	}
	if o.parent != nil {
		return o.parent.err
	}
	return nil
}

func (o *ops) Fail(err error) {
	if o.parent != nil {
		o.Fail(err)
	}
	switch {
	case err == nil:
		return
	case o.err != nil:
		// already failed
		return
	default:
		o.err = err
	}
}
