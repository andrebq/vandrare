package ssh

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/andrebq/vandrare/internal/store"
	"github.com/gliderlabs/ssh"
	gossh "golang.org/x/crypto/ssh"
)

type (
	DynKDB struct {
		Store *store.Store
	}

	KeyConfig struct {
		ExpiresAt    time.Time
		ValidFrom    time.Time
		AllowedHosts []string
		Description  string
	}

	KeyPermissions struct {
		Entries []Permission
	}

	Permission struct {
		Operation string
		Resource  string
		Action    string
	}

	KeyRegistration struct {
		PublicKey   SSHPubKey `json:"pubkey"`
		UseCases    []string  `json:"useCase"`
		Hosts       []string  `json:"hosts"`
		Description string    `json:"description"`
		Owner       string    `json:"owner"`
	}

	SSHPubKey struct {
		ssh.PublicKey
	}
)

func (s *SSHPubKey) MarshalJSON() ([]byte, error) {
	if s.PublicKey == nil {
		return nil, nil
	}
	return json.Marshal(string(gossh.MarshalAuthorizedKey(*s)))
}

func (s *SSHPubKey) UnmarshalJSON(buf []byte) error {
	s.PublicKey = nil
	var str string
	err := json.Unmarshal(buf, &str)
	if err != nil {
		return err
	}
	key, _, _, _, err := gossh.ParseAuthorizedKey([]byte(str))
	if err != nil {
		return err
	}
	s.PublicKey = key
	return nil
}

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

func (d *DynKDB) SetPermission(ctx context.Context, key ssh.PublicKey, operation, resource, action string) error {
	lookupKey := d.computeKeyPermissionLookup(key)
	ops := d.Store.Ops(false)
	defer ops.Close()

	kv := ops.KV()
	permissions := KeyPermissions{}
	err := store.GetJSON(ctx, &permissions, kv, lookupKey)
	if store.IsNotFound(err) {
		err = nil
	} else if err != nil {
		return err
	}
	foundIdx := -1
	for i, perm := range permissions.Entries {
		if perm.Operation == operation && perm.Resource == resource {
			if action == "deny" {
				permissions.Entries[i] = Permission{}
				foundIdx = i
				break
			}
		}
	}
	if foundIdx == -1 && action != "deny" {
		permissions.Entries = append(permissions.Entries, Permission{
			Action:    action,
			Operation: operation,
			Resource:  resource,
		})
	} else if foundIdx != -1 {
		switch {
		case foundIdx == len(permissions.Entries)-1:
			permissions.Entries = permissions.Entries[:foundIdx-1]
		case foundIdx == 0:
			permissions.Entries = permissions.Entries[1:]
		default:
			permissions.Entries[foundIdx] = permissions.Entries[len(permissions.Entries)-1]
			permissions.Entries = permissions.Entries[:len(permissions.Entries)-1]
		}
		sort.Slice(permissions.Entries, func(i, j int) bool {
			pi, pj := permissions.Entries[i], permissions.Entries[j]
			return pi.Resource < pj.Resource &&
				pi.Operation < pj.Operation &&
				pi.Action < pj.Action
		})
	}
	ops.Fail(store.PutJSON(ctx, kv, lookupKey, permissions))
	return ops.Commit()
}

func (d *DynKDB) AuthN(ctx context.Context, key ssh.PublicKey) error {
	_, err := d.lookupAndVerifyConfig(ctx, key)
	return err
}

func (d *DynKDB) AuthZ(ctx context.Context, key ssh.PublicKey, operation, resource string) error {
	if operation != "expose-endpoint" {
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

func (d *DynKDB) RequestKeyRegistration(ctx context.Context, key KeyRegistration) (KeyRegistration, error) {
	regLookup := d.computeKeyLookupRegistration(key.PublicKey)
	ops := d.Store.Ops(false)
	defer ops.Close()
	kv := ops.KV()

	var oldreg KeyRegistration

	err := store.GetJSON(ctx, &oldreg, kv, regLookup)
	if err == nil {
		oldLookup := d.computeKeyLookupRegistration(oldreg.PublicKey.PublicKey)
		if oldLookup != regLookup {
			slog.Error("Key registration on database does not match its body", "lookupKey", regLookup, "bodyKey", oldLookup)
			return key, errors.New("unexpected state")
		}
	}
	ops.Fail(store.PutJSON(ctx, kv, regLookup, key))
	return key, ops.Commit()
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

func (d *DynKDB) computeKeyPermissionLookup(key ssh.PublicKey) string {
	return fmt.Sprintf("kdb:key-perm:%v", gossh.FingerprintSHA256(key))
}

func (d *DynKDB) computeKeyLookupRegistration(key ssh.PublicKey) string {
	return fmt.Sprintf("kdb:key-reg:%v", gossh.FingerprintSHA256(key))
}
