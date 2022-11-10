package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"strings"

	"golang.org/x/sync/errgroup"
	"tailscale.com/client/tailscale"
	"tailscale.com/tsnet"
)

type listenon struct {
	tls     bool
	addr    string
	network string
}

func (l listenon) listen(ctx context.Context, s *tsnet.Server, lc *tailscale.LocalClient, proxy http.Handler) error {
	srv := &http.Server{
		TLSConfig: &tls.Config{
			GetCertificate: lc.GetCertificate,
		},
		Handler: proxy,
	}

	if l.network == "" {
		l.network = "tcp"
	}
	ln, err := s.Listen(l.network, l.addr)
	if err != nil {
		return err
	}
	defer ln.Close()

	log.Printf("listening network=%v addr=%v tls=%v", l.network, l.addr, l.tls)
	if l.tls {
		return srv.ServeTLS(ln, "", "")
	} else {
		return srv.Serve(ln)
	}
}

func parselisten(listen string) ([]listenon, error) {
	listenOns := []listenon{}
	if listen == "" {
		listenOns = []listenon{
			{tls: true, addr: ":443"},
		}
	} else {
		for _, set := range strings.Split(listen, ";") {
			on := listenon{tls: true}
			tlsWasSet := false
			for _, pair := range strings.Split(set, ",") {
				pairarr := strings.Split(pair, "=")
				if len(pairarr) != 2 {
					return nil, errors.New("all values must be in the form key=value")
				}

				switch pairarr[0] {
				case "tls":
					should, _ := strconv.ParseBool(pairarr[1])
					on.tls = should
					tlsWasSet = true
				case "addr":
					on.addr = pairarr[1]
				case "port":
					on.addr = ":" + pairarr[1]
				case "network":
					on.network = pairarr[1]
				}
			}

			if !tlsWasSet && strings.HasSuffix(on.addr, ":80") {
				on.tls = false
			}

			listenOns = append(listenOns, on)
		}
	}

	return listenOns, nil
}

func main() {
	var (
		listen   = os.Getenv("LISTEN")
		remote   = os.Getenv("REMOTE")
		hostname = os.Getenv("HOSTNAME")
		strategy = os.Getenv("STRATEGY")
		debug    = os.Getenv("DEBUG")
		dir      = os.Getenv("DIR")
	)
	log.SetOutput(os.Stderr)

	if err := run(listen, remote, debug, hostname, strategy, dir); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func run(listen, remote, debug, hostname, strategy, dir string) error {
	if remote == "" {
		return errors.New("remote cannot be blank")
	}
	if hostname == "" {
		return errors.New("hostname cannot be blank")
	}

	listenOns, err := parselisten(listen)
	if err != nil {
		return err
	}

	target, err := url.Parse(remote)
	if err != nil {
		return err
	}
	targetQuery := target.RawQuery

	s := &tsnet.Server{
		Hostname:  hostname,
		Ephemeral: true,
		Logf:      func(format string, args ...any) {},
	}
	defer s.Close()

	if dir != "" {
		s.Dir = dir
	}

	if debug != "" {
		log.SetOutput(os.Stderr)
	} else {
		log.SetOutput(io.Discard)
	}

	if debug == "full" {
		s.Logf = log.Printf
	}
	lc, err := s.LocalClient()
	if err != nil {
		return err
	}

	log.Printf("opening reverse proxy to remote=%v hostname=%v strategy=%v", target, hostname, strategy)

	var proxy http.Handler

	switch strategy {
	case "docker":
		proxy = &httputil.ReverseProxy{
			Director: func(req *http.Request) {
				req.URL.Scheme = target.Scheme
				req.URL.Host = target.Host
				req.URL.Path, req.URL.RawPath = joinURLPath(target, req.URL)
				if targetQuery == "" || req.URL.RawQuery == "" {
					req.URL.RawQuery = targetQuery + req.URL.RawQuery
				} else {
					req.URL.RawQuery = targetQuery + "&" + req.URL.RawQuery
				}
				if _, ok := req.Header["User-Agent"]; !ok {
					// explicitly disable User-Agent so it's not set to default value
					req.Header.Set("User-Agent", "")
				}

				req.Header.Set("X-Forwarded-For", fmt.Sprintf("for=%v,proto=http", req.RemoteAddr))
				req.Header.Set("X-Real-Ip", req.RemoteAddr)
				req.Header.Set("X-Forwarded-Proto", "http")
				req.Header.Set("X-Original-URI", req.URL.RawPath)
				req.Header.Set("Docker-Distribution-Api-Version", "registry/2.0")
			},
		}
	default:
		proxy = httputil.NewSingleHostReverseProxy(target)
	}

	g := errgroup.Group{}
	for _, ln := range listenOns {
		ln := ln
		g.Go(func() error {
			return ln.listen(context.Background(), s, lc, proxy)
		})
	}

	return g.Wait()
}

func joinURLPath(a, b *url.URL) (path, rawpath string) {
	if a.RawPath == "" && b.RawPath == "" {
		return singleJoiningSlash(a.Path, b.Path), ""
	}
	// Same as singleJoiningSlash, but uses EscapedPath to determine
	// whether a slash should be added
	apath := a.EscapedPath()
	bpath := b.EscapedPath()

	aslash := strings.HasSuffix(apath, "/")
	bslash := strings.HasPrefix(bpath, "/")

	switch {
	case aslash && bslash:
		return a.Path + b.Path[1:], apath + bpath[1:]
	case !aslash && !bslash:
		return a.Path + "/" + b.Path, apath + "/" + bpath
	}
	return a.Path + b.Path, apath + bpath
}

func singleJoiningSlash(a, b string) string {
	aslash := strings.HasSuffix(a, "/")
	bslash := strings.HasPrefix(b, "/")
	switch {
	case aslash && bslash:
		return a + b[1:]
	case !aslash && !bslash:
		return a + "/" + b
	}
	return a + b
}
