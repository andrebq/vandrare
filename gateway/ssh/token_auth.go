package ssh

import "context"

func (g *Gateway) validToken(ctx context.Context, user, pwd string) bool {
	// TODO: add proper authentication here
	return true
}
