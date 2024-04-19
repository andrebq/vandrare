package ssh

import (
	"github.com/andrebq/vandrare/gateway/ssh"
	"github.com/andrebq/vandrare/internal/flagutil"
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
	return &cli.Command{
		Name:  "gateway",
		Usage: "Starts the SSH gateway",
		Flags: []cli.Flag{
			flagutil.String(&bind, "bind-addr", []string{"b"}, "Address to listen for incoming requests", false),
		},
		Action: func(ctx *cli.Context) error {
			return ssh.NewGateway(ssh.AllowAnyKey()).Run(ctx.Context, bind)
		},
	}
}
