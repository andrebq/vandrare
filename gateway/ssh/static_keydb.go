package ssh

import (
	"context"

	"github.com/gliderlabs/ssh"
)

type (
	allowAll struct{}
)

// AllowAnyKey returns a KeyDB which allows any key to perform any operation
func AllowAnyKey() KeyDB                                              { return allowAll{} }
func (_ allowAll) AuthN(ctx context.Context, key ssh.PublicKey) error { return nil }
func (_ allowAll) AuthZ(ctx context.Context, key ssh.PublicKey, action, resource string) error {
	return nil
}
