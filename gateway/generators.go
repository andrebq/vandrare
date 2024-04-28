package gateway

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"text/template"
)

type (
	Token        string
	JumpAlias    string
	IdentityPath string
	CAPubkeyPath string
)

func must[T any](val T, err error) T {
	if err != nil {
		panic(err)
	}
	return val
}

var (
	gatewayKnownHostsPath = must(url.Parse("./gateway/ssh/certificates/known_hosts"))
	clientConfigTemplates = template.Must(template.New("__root__").Parse(`
{{ define "jumphost" }}
Host {{ .Alias }}
	Hostname {{ .Hostname }}
	Port {{ .Port }}
	IdentitiesOnly yes
	IdentityFile {{ .Identity }}
	UserKnownHostsFile {{ .HostCAFile }}
{{ end }}

{{ define "host" }}
Host {{ .Alias}}
	Hostname {{ .Hostname }}
	Port 22
	IdentitiesOnly yes
	IdentityFile {{ .Identity }}
	ProxyJump {{ .Jump }}
	UserKnownHostsFile {{ .HostCAFile }}
{{ end }}
	`))
)

func GenerateJumpConfig(ctx context.Context, output io.Writer, gateway *url.URL, token Token, alias JumpAlias, identity IdentityPath, cakey CAPubkeyPath, host string) error {
	host, port, err := net.SplitHostPort(host)
	if err != nil {
		return err
	}
	data := struct {
		Alias      string
		Hostname   string
		Port       string
		Identity   IdentityPath
		HostCAFile CAPubkeyPath
	}{
		HostCAFile: cakey,
		Identity:   identity,
		Hostname:   host,
		Alias:      string(alias),
		Port:       port,
	}
	ref := gateway.ResolveReference(gatewayKnownHostsPath)
	err = downloadCAFile(ctx, ref, token, cakey)
	if err != nil {
		return err
	}
	return clientConfigTemplates.ExecuteTemplate(output, "jumphost", data)
}

func GenerateClientConfig(ctx context.Context, output io.Writer, gateway *url.URL, token Token, jump JumpAlias, identity IdentityPath, cakey CAPubkeyPath, host string) error {
	data := struct {
		Alias      string
		Hostname   string
		Identity   IdentityPath
		Jump       JumpAlias
		HostCAFile CAPubkeyPath
	}{
		Jump:       jump,
		HostCAFile: cakey,
		Identity:   identity,
		Hostname:   host,
		Alias:      host,
	}
	return clientConfigTemplates.ExecuteTemplate(output, "host", data)
}

func downloadCAFile(ctx context.Context, url *url.URL, token Token, cafile CAPubkeyPath) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url.String(), nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth("vandrare", string(token))
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected response from vandrare gatewayt: %v", res.StatusCode)
	}
	return copyToFile(string(cafile), res.Body)
}

func copyToFile(filepath string, input io.Reader) error {
	file, err := os.Create(filepath)
	if err != nil {
		return err
	}
	_, err = io.Copy(file, input)
	if err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	return file.Close()
}
