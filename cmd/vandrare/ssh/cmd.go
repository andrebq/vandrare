package ssh

import (
	"crypto/ed25519"
	"encoding/hex"
	"errors"
	"os"

	"github.com/andrebq/vandrare/gateway/ssh"
	"github.com/andrebq/vandrare/internal/flagutil"
	"github.com/andrebq/vandrare/internal/store"
	"github.com/urfave/cli/v2"
)

const envPrefix = "VANDRARE_GATEWAY_SSH"

func Cmd() *cli.Command {
	return &cli.Command{
		Name:  "ssh",
		Usage: "Command to interact with SSH sub-subsystem of vandrare",
		Subcommands: []*cli.Command{
			gatewayCmd(),
			configCmd(),
		},
	}
}

func gatewayCmd() *cli.Command {
	bind := "127.0.0.1:2222"
	bindHTTP := "127.0.0.1:8222"
	adminKeyFile := ""
	kdbStoreDir := ""
	caSeed := ""
	caSeedFlag := flagutil.String(&caSeed, "ca-seed", nil, envPrefix, "32-bytes, hex-encoded, seed used to generate a ed25519 private key, use the environment variable", true)
	caSeedFlag.Hidden = true
	subdomains := cli.StringSlice{}
	selfDomains := cli.StringSlice{}
	return &cli.Command{
		Name:  "gateway",
		Usage: "Starts the SSH gateway",
		Flags: []cli.Flag{
			flagutil.String(&bind, "bind-addr", []string{"b"}, envPrefix, "Address to listen for incoming requests", false),
			flagutil.String(&adminKeyFile, "admin-key-file", nil, envPrefix, "SSH public key file used for admin access", true),
			flagutil.String(&kdbStoreDir, "keydb-store-dir", nil, envPrefix, "Directory where key database is kept", true),
			flagutil.String(&bindHTTP, "bind-http-addr", []string{"bh"}, envPrefix, "Address to listen for HTTP Requests", false),
			flagutil.StringSlice(&selfDomains, "self-domain", []string{"self"}, envPrefix, "Address (domain:port) of the gateway itself. Must be a value recognized by clients", true),
			flagutil.StringSlice(&subdomains, "domain", []string{"d"}, envPrefix, "One or more sub-domains which can be authorized by this gateway", true),
			caSeedFlag,
		},
		Action: func(ctx *cli.Context) error {
			var buf [ed25519.SeedSize]byte
			n, err := hex.Decode(buf[:], []byte(caSeed))
			if err != nil {
				return err
			} else if n != ed25519.SeedSize {
				return errors.New("ca-seed should be 32-byte long, hex-encoded")
			}
			// clear the environment
			for _, v := range caSeedFlag.EnvVars {
				os.Setenv(v, "")
			}

			key, err := ssh.ParseAuthorizedKey(adminKeyFile)
			if err != nil {
				return err
			}
			kdbStore, err := store.Open(kdbStoreDir)
			if err != nil {
				return err
			}
			gateway, err := ssh.NewGateway(
				&ssh.DynKDB{Store: kdbStore},
				&ssh.TokenDB{Store: *kdbStore},
				key, ssh.GenerateCAKey(buf))
			if err != nil {
				return err
			}
			gateway.Binding.SSH = bind
			gateway.Binding.HTTP = bindHTTP
			gateway.Binding.Domains = selfDomains.Value()
			ssh.WrapIP(gateway.Binding.Domains)

			gateway.Subdomains = subdomains.Value()
			ssh.WrapIP(gateway.Subdomains)
			return gateway.Run(ctx.Context)
		},
	}
}
