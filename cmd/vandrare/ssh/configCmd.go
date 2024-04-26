package ssh

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"

	"github.com/andrebq/vandrare/gateway"
	"github.com/andrebq/vandrare/internal/commonpaths"
	"github.com/andrebq/vandrare/internal/flagutil"
	"github.com/urfave/cli/v2"
)

func configCmd() *cli.Command {
	envPrefix := fmt.Sprintf("%v_%v", envPrefix, "CONFIG")
	gateway := "http://localhost:8222/"
	allowHTTP := false
	var baseURL *url.URL
	var token string
	tokenFlag := flagutil.String(&token, "token", nil, envPrefix, "Token to authenticate against the gateway", false)
	tokenFlag.Hidden = true

	return &cli.Command{
		Name:        "config",
		Description: "Create or modify SSH configurations required by client/servers that wish to integrate via vandrare gateways",
		Flags: []cli.Flag{
			flagutil.String(&gateway, "endpoint", []string{"gt"}, envPrefix, "URL of your gateway HTTP API", true),
			flagutil.Bool(&allowHTTP, "allow-http", nil, envPrefix, "Allow HTTP connections to the gateway", false),
		},
		Before: func(ctx *cli.Context) error {
			var err error
			baseURL, err = url.Parse(gateway)
			if err != nil {
				return err
			}
			if baseURL.Scheme == "http" && !allowHTTP {
				return errors.New("HTTP access to gateway is not allowed")
			}
			return nil
		},
		Subcommands: []*cli.Command{
			clientConfigCmd(&baseURL, &token),
		},
	}
}

func clientConfigCmd(base **url.URL, token *string) *cli.Command {
	envPrefix := fmt.Sprintf("%v_%v", envPrefix, "CONFIG_CLIENT")
	var host string
	var jumpserver, jumpalias string
	var identityFile string = commonpaths.DefaultSSHPrivateKey()
	var hostCABase string = commonpaths.DefaultCAPubKey("vandrare-jump")
	var includeJump bool
	return &cli.Command{
		Name:  "client",
		Usage: "Creates a client config file which uses the give gateway to access a specific host:port",
		Flags: []cli.Flag{
			flagutil.String(&jumpalias, "jump-alias", nil, envPrefix, "SSH alias of the jump server", true),
			flagutil.Bool(&includeJump, "include-jump", nil, envPrefix, "Include the jumpserver definition in the generated config, uses either jump-server or gateway host on port 2222 as address", false),
			flagutil.String(&jumpserver, "jump-server", nil, envPrefix, "Proper address of the jump-server, read only if include-jump is true", false),
			flagutil.String(&host, "host", nil, "", "Desired host", true),
			flagutil.String(&identityFile, "identity-file", []string{"identity"}, envPrefix, "Identity file with private key", false),
			flagutil.String(&hostCABase, "gateway-pubkey", nil, envPrefix, "Path where the gateway CA pubkey will be stored", false),
		},
		Action: func(ctx *cli.Context) error {
			if includeJump {
				if jumpserver == "" {
					jumpserver = net.JoinHostPort((*base).Hostname(), "2222")
					slog.Warn("Missing flag jump-server, using gateway instead", "jump-server", jumpserver)
				}
				err := gateway.GenerateJumpConfig(ctx.Context,
					ctx.App.Writer,
					*base,
					gateway.Token(*token),
					gateway.JumpAlias(jumpalias),
					gateway.IdentityPath(identityFile),
					gateway.CAPubkeyPath(hostCABase),
					jumpserver)
				if err != nil {
					return err
				}
			}
			return gateway.GenerateClientConfig(ctx.Context,
				ctx.App.Writer,
				*base,
				gateway.Token(*token),
				gateway.JumpAlias(jumpalias),
				gateway.IdentityPath(identityFile),
				gateway.CAPubkeyPath(hostCABase),
				host)
		},
	}
}
