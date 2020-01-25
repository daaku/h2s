package main // import "github.com/daaku/h2s"

import (
	"context"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/NYTimes/gziphandler"
	"github.com/peterbourgon/ff"
	"github.com/pkg/errors"
)

type conf struct {
	httpsAddr   string
	publicDir   string
	tlsCertFile string
	tlsKeyFile  string
}

func run(c *conf) error {
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
		Addr:    c.httpsAddr,
		Handler: gzipMiddleware(http.FileServer(http.Dir(c.publicDir))),
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

	log.Printf("Starting server for https://%s/", c.httpsAddr)
	err = httpsServer.ListenAndServeTLS(c.tlsCertFile, c.tlsKeyFile)
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
	fs.StringVar(&c.httpsAddr, "https-addr", "", "https server address")
	fs.StringVar(&c.publicDir, "public-dir", "", "public files directory")
	fs.StringVar(&c.tlsCertFile, "tls-cert-file", "", "tls cert file")
	fs.StringVar(&c.tlsKeyFile, "tls-key-file", "", "tls key file")

	ff.Parse(fs, os.Args[1:], ff.WithEnvVarPrefix("H2S"))

	if err := run(&c); err != nil {
		log.Printf("%+v", err)
		os.Exit(1)
	}
	log.Println("Graceful shutdown complete.")
}
