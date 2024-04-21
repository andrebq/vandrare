package ssh

import (
	"fmt"
	"os"

	"github.com/gliderlabs/ssh"
)

// ParseAuthorizedKey form the given file, it should contain only one public key
// ie, the ed25519_pub file
func ParseAuthorizedKey(file string) (ssh.PublicKey, error) {
	buf, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("ssh: unable to read file at %v: %w", file, err)
	}
	pubkey, _, _, _, err := ssh.ParseAuthorizedKey(buf)
	return pubkey, err
}
