package ssh

import (
	"github.com/andrebq/vandrare/gateway/ssh"
	"github.com/andrebq/vandrare/internal/flagutil"
	"github.com/andrebq/vandrare/internal/store"
	"github.com/urfave/cli/v2"
)

func Cmd() *cli.Command {
	return &cli.Command{
		Name:  "ssh",
		Usage: "Command to interact with SSH sub-subsystem of vandrare",
		Subcommands: []*cli.Command{
			gatewayCmd(),
		},
	}
}

func gatewayCmd() *cli.Command {
	bind := "127.0.0.1:2222"
	adminKeyFile := ""
	kdbStoreDir := ""
	const envPrefix = "GATEWAY_SSH"
	return &cli.Command{
		Name:  "gateway",
		Usage: "Starts the SSH gateway",
		Flags: []cli.Flag{
			flagutil.String(&bind, "bind-addr", []string{"b"}, envPrefix, "Address to listen for incoming requests", false),
			flagutil.String(&adminKeyFile, "admin-key-file", nil, envPrefix, "SSH public key file used for admin access", true),
			flagutil.String(&kdbStoreDir, "keydb-store-dir", nil, envPrefix, "Directory where key database is kept", true),
		},
		Action: func(ctx *cli.Context) error {
			key, err := ssh.ParseAuthorizedKey(adminKeyFile)
			if err != nil {
				return err
			}
			kdbStore, err := store.Open(kdbStoreDir)
			if err != nil {
				return err
			}
			return ssh.NewGateway(&ssh.DynKDB{Store: kdbStore}, key).Run(ctx.Context, bind)
		},
	}
}
