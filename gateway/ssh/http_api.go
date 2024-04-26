package ssh

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/andrebq/maestro"
	gossh "golang.org/x/crypto/ssh"
)

func (g *Gateway) runHTTPD(ctx maestro.Context) error {
	srv := http.Server{
		ReadTimeout:       time.Minute,
		WriteTimeout:      time.Minute,
		ReadHeaderTimeout: time.Second * 10,
		MaxHeaderBytes:    1_000_000,
		Addr:              g.Binding.HTTP,
	}
	go func() {
		<-ctx.Done()
		timeout, cancel := context.WithTimeout(context.Background(), time.Minute)
		srv.Shutdown(timeout)
		cancel()
	}()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/liveness", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(struct {
			Now time.Time `json:"now"`
		}{Now: time.Now()})
	})
	mux.HandleFunc("GET /gateway/ssh/certificates/known_hosts", func(w http.ResponseWriter, req *http.Request) {
		pubkeyTxt := string(bytes.TrimSpace(gossh.MarshalAuthorizedKey(g.casigner.PublicKey())))
		buf := bytes.Buffer{}
		fmt.Fprintf(&buf, "# vandrare gateway / CA fingerprint: %v\n", gossh.FingerprintSHA256(g.casigner.PublicKey()))
		for _, p := range g.host.cert.ValidPrincipals {
			fmt.Fprintf(&buf, "@cert-authority %v %v\n", p, pubkeyTxt)
		}
		for _, d := range g.Subdomains {
			fmt.Fprintf(&buf, "@cert-authority *.%v %v\n", d, pubkeyTxt)
		}
		w.Header().Add("Content-Type", "text/plain")
		w.Header().Add("Content-Length", strconv.Itoa(buf.Len()))
		w.WriteHeader(http.StatusOK)
		io.Copy(w, &buf)
	})
	mux.HandleFunc("GET /gateway/ssh/certificates/host_ca.pub", func(w http.ResponseWriter, r *http.Request) {
		pubkeyTxt := gossh.MarshalAuthorizedKey(g.casigner.PublicKey())
		w.Header().Add("Content-Type", "text/plain")
		w.Header().Add("Content-Length", strconv.Itoa(len(pubkeyTxt)))
		w.WriteHeader(http.StatusOK)
		w.Write(pubkeyTxt)
	})
	mux.HandleFunc("GET /gateway/ssh/certificates/self", func(w http.ResponseWriter, r *http.Request) {
		pubkeyTxt := gossh.MarshalAuthorizedKey(g.host.cert)
		w.Header().Add("Content-Type", "text/plain")
		w.Header().Add("Content-Length", strconv.Itoa(len(pubkeyTxt)))
		w.WriteHeader(http.StatusOK)
		w.Write(pubkeyTxt)
	})
	srv.Handler = mux
	slog.Info("Starting HTTPD server", "addr", srv.Addr)
	err := srv.ListenAndServe()
	ctx.Shutdown()
	return err
}
