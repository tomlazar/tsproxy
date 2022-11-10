package main

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"

	"tailscale.com/tsnet"
)

func main() {
	var (
		addr     = os.Getenv("ADDR")
		remote   = os.Getenv("REMOTE")
		hostname = os.Getenv("HOSTNAME")
		debug    = os.Getenv("DEBUG")
	)
	log.SetOutput(os.Stderr)

	if addr == "" {
		addr = ":443"
	}

	if err := run(addr, remote, debug, hostname); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func run(addr, remote, debug, hostname string) error {
	if addr == "" {
		return errors.New("addr cannot be blank")
	}
	if remote == "" {
		return errors.New("remote cannot be blank")
	}
	if hostname == "" {
		return errors.New("hostname cannot be blank")
	}
	remoteUrl, err := url.Parse(remote)
	if err != nil {
		return err
	}

	s := &tsnet.Server{
		Hostname:  hostname,
		Ephemeral: true,
		Logf:      func(format string, args ...any) {},
	}
	defer s.Close()

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

	log.Printf("opening reverse proxy to remote=%v", remoteUrl)
	redirect := httputil.NewSingleHostReverseProxy(remoteUrl)

	ln, err := s.Listen("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}
	defer ln.Close()

	srv := &http.Server{
		TLSConfig: &tls.Config{
			GetCertificate: lc.GetCertificate,
		},
		Handler: redirect,
	}
	log.Printf("Running TLS server on %v ...", addr)
	log.Fatal(srv.ServeTLS(ln, "", ""))

	return nil
}
