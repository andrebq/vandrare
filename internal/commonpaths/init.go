package commonpaths

import (
	"fmt"
	"path/filepath"

	"github.com/mitchellh/go-homedir"
)

var (
	home string
)

func init() {
	var err error
	home, err = homedir.Dir()
	if err != nil {
		panic(err)
	}
}

func DefaultSSHPubKey() string {
	return filepath.Join(DefaultSSHDir(), "id_ed25519.pub")
}

func DefaultSSHPrivateKey() string {
	return filepath.Join(DefaultSSHDir(), "id_ed25519")
}

func DefaultSSHDir() string {
	return filepath.Join(home, ".ssh")
}

func DefaultCAPubKey(gatewayName string) string {
	return filepath.Join(DefaultSSHDir(), fmt.Sprintf("ca_%v.pub", gatewayName))
}
