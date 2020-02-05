package main // import "github.com/daaku/h2s"

import (
	"context"
	"crypto/tls"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/NYTimes/gziphandler"
	"github.com/peterbourgon/ff"
	"github.com/pkg/errors"
)

func logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s\n", r.Method, r.URL)
		next.ServeHTTP(w, r)
	})
}

type conf struct {
	addr string
	dir  string
	tls  string
}

func run(c *conf) error {
	tlsConfig := &tls.Config{}
	for _, pair := range strings.Split(c.tls, ",") {
		ck := strings.SplitN(pair, ":", 2)
		if len(ck) != 2 {
			return errors.Errorf("invalid tls cert:key pair: %s", pair)
		}
		cert, err := tls.LoadX509KeyPair(ck[0], ck[1])
		if err != nil {
			return errors.Wrapf(err, "invalid tls cert:key pair: %s", pair)
		}
		tlsConfig.Certificates = append(tlsConfig.Certificates, cert)
	}
	tlsConfig.BuildNameToCertificate()

	gzipMiddleware, err := gziphandler.GzipHandlerWithOpts(
		gziphandler.ContentTypes([]string{
			"application/javascript",
			"image/svg+xml",
			"text/css",
			"text/html",
			"text/plain",
		}),
	)
	if err != nil {
		return errors.WithStack(err)
	}

	httpsServer := &http.Server{
		Addr:      c.addr,
		Handler:   logger(gzipMiddleware(http.FileServer(http.Dir(c.dir)))),
		TLSConfig: tlsConfig,
	}

	shutdownDone := make(chan struct{})
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig

		if err := httpsServer.Shutdown(context.Background()); err != nil {
			log.Printf("HTTP server Shutdown: %+v", errors.WithStack(err))
		}
		close(shutdownDone)
	}()

	listener, err := tls.Listen("tcp", c.addr, tlsConfig)
	if err != nil {
		return errors.WithStack(err)
	}
	log.Printf("Starting server for https://%s/", c.addr)
	err = httpsServer.Serve(listener)
	if err != nil && err != http.ErrServerClosed {
		return errors.WithStack(err)
	}
	<-shutdownDone
	return nil
}

func httpsPort(addr string) int {
	_, portStr, _ := net.SplitHostPort(addr)
	port, _ := strconv.Atoi(portStr)
	return port
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	c := conf{}
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	fs.StringVar(&c.addr, "addr", "", "https server address")
	fs.StringVar(&c.dir, "dir", "", "public files directory")
	fs.StringVar(&c.tls, "tls", "", "tls cert1:key1,cert2:key2 pairs")

	ff.Parse(fs, os.Args[1:], ff.WithEnvVarPrefix("H2S"))

	if err := run(&c); err != nil {
		log.Printf("%+v", err)
		os.Exit(1)
	}
	log.Println("Graceful shutdown complete.")
}
