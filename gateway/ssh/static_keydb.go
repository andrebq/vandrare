package ssh

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/andrebq/vandrare/internal/store"
	"github.com/gliderlabs/ssh"
	gossh "golang.org/x/crypto/ssh"
)

type (
	allowAll struct{}

	DynKDB struct {
		Store *store.Store
	}

	KeyConfig struct {
		ExpiresAt    time.Time
		ValidFrom    time.Time
		AllowedHosts []string
		Description  string
	}
)

var (
	errNotAuthorized = errors.New("ssh: not authorized")
)

func (d *DynKDB) RegisterKey(ctx context.Context, key ssh.PublicKey, validFrom, expiresAt time.Time, allowedHosts []string) error {
	lookupKey := d.computeKeyLookup(key)
	ops := d.Store.Ops(false)
	defer ops.Close()
	cfg := KeyConfig{
		ExpiresAt:    expiresAt,
		ValidFrom:    validFrom,
		AllowedHosts: allowedHosts,
		Description:  string(gossh.MarshalAuthorizedKey(key)),
	}
	kv := ops.KV()
	buf, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	kv.SetBytes(ctx, lookupKey, buf)
	ops.Fail(kv.Err())
	return ops.Commit()
}

func (d *DynKDB) AuthN(ctx context.Context, key ssh.PublicKey) error {
	_, err := d.lookupAndVerifyConfig(ctx, key)
	return err
}

func (d *DynKDB) AuthZ(ctx context.Context, key ssh.PublicKey, action, resource string) error {
	if action != "expose-endpoint" {
		// for now, this is the only action, other than admin, that can run
		// and admin sessions are authorized by a different key
		return errNotAuthorized
	}
	cfg, err := d.lookupAndVerifyConfig(ctx, key)
	if err != nil {
		return err
	}
	for _, s := range cfg.AllowedHosts {
		if s == resource {
			return nil
		}
	}
	return errNotAuthorized
}

func (d *DynKDB) lookupAndVerifyConfig(ctx context.Context, key ssh.PublicKey) (KeyConfig, error) {
	lookupKey := d.computeKeyLookup(key)
	ops := d.Store.Ops(false)
	defer ops.Close()
	val := ops.KV().GetBytes(ctx, nil, lookupKey)
	if val == nil {
		return KeyConfig{}, errNotAuthorized
	}
	var cfg KeyConfig
	if err := json.Unmarshal(val, &cfg); err != nil {
		slog.Error("Invalid key-config from database", "lookup", lookupKey)
		return KeyConfig{}, errNotAuthorized
	}
	now := time.Now()
	if cfg.ValidFrom.After(now) || cfg.ExpiresAt.Before(now) {
		return KeyConfig{}, errNotAuthorized
	}
	return cfg, nil
}

func (d *DynKDB) computeKeyLookup(key ssh.PublicKey) string {
	sig := gossh.FingerprintSHA256(key)
	return fmt.Sprintf("kdb:key:%v", sig)
}
