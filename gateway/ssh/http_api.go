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
	"strings"
	"time"

	"github.com/andrebq/maestro"
	gossh "golang.org/x/crypto/ssh"
)

type (
	ctxkey byte
)

var (
	userCtxKey = ctxKey(1)
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
	public := http.NewServeMux()
	public.HandleFunc("GET /health/liveness", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, struct {
			Now time.Time `json:"now"`
		}{Now: time.Now()})
	})
	public.HandleFunc("GET /gateway/ssh/certificates/host_ca.pub", func(w http.ResponseWriter, r *http.Request) {
		pubkeyTxt := gossh.MarshalAuthorizedKey(g.casigner.PublicKey())
		w.Header().Add("Content-Type", "text/plain")
		w.Header().Add("Content-Length", strconv.Itoa(len(pubkeyTxt)))
		w.WriteHeader(http.StatusOK)
		w.Write(pubkeyTxt)
	})
	public.HandleFunc("GET /gateway/ssh/certificates/self", func(w http.ResponseWriter, r *http.Request) {
		pubkeyTxt := gossh.MarshalAuthorizedKey(g.host.cert)
		w.Header().Add("Content-Type", "text/plain")
		w.Header().Add("Content-Length", strconv.Itoa(len(pubkeyTxt)))
		w.WriteHeader(http.StatusOK)
		w.Write(pubkeyTxt)
	})
	public.HandleFunc("GET /gateway/ssh/certificates/known_hosts", g.protectHttpFunc(func(w http.ResponseWriter, req *http.Request) {
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
	}))

	srv.Handler = public
	slog.Info("Starting HTTPD server", "addr", srv.Addr)
	err := srv.ListenAndServe()
	ctx.Shutdown()
	return err
}

func (g *Gateway) registerKey(w http.ResponseWriter, req *http.Request) {
	var key KeyRegistration
	if err := readJSON(&key, req, w); err != nil {
		return
	}
	key.Owner, _ = getUser(req)

	key, err := g.kdb.RequestKeyRegistration(req.Context(), key)
	if err != nil {
		slog.Error("Unable to process key registration", "owner", key.Owner)
	}
	writeJSON(w, key)
}

func (g *Gateway) protectHttpFunc(fn http.HandlerFunc) http.HandlerFunc {
	const bearer = "Bearer "
	const basic = "Basic "
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		var token string
		if strings.HasPrefix(authHeader, basic) {
			var ok bool
			_, token, ok = r.BasicAuth()
			if !ok {
				http.Error(w, "Not authorized", http.StatusUnauthorized)
				return
			}
		} else if strings.HasPrefix(authHeader, bearer) {
			token = authHeader[len(bearer):]
		} else {
			http.Error(w, "Not authorized", http.StatusUnauthorized)
			return
		}
		r.SetBasicAuth("", "")
		r.Header.Del("Authorization")

		valid, owner, err := g.tdb.Valid(r.Context(), token)
		if err != nil || !valid {
			http.Error(w, "Not authorized", http.StatusUnauthorized)
			return
		}
		fn(w, setUser(r, owner))
	}
}

func readJSON(out any, req *http.Request, w http.ResponseWriter) error {
	err := json.NewDecoder(req.Body).Decode(out)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
	return err
}

func writeJSON(out http.ResponseWriter, data any) error {
	buf, err := json.Marshal(data)
	if err != nil {
		return err
	}
	out.Header().Add("Content-Type", "text/json")
	out.Header().Add("Content-Length", strconv.Itoa(len(buf)))
	out.WriteHeader(http.StatusOK)
	_, err = out.Write(buf)
	return err
}

func getUser(req *http.Request) (string, bool) {
	val := req.Context().Value(userCtxKey)
	if val == nil {
		return "", false
	}
	return val.(string), true
}

func setUser(req *http.Request, user string) *http.Request {
	return req.WithContext(context.WithValue(req.Context(), userCtxKey, user))
}
