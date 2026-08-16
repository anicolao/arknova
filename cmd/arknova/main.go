package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/anicolao/arknova/internal/activation"
	"github.com/anicolao/arknova/internal/buildinfo"
	"github.com/anicolao/arknova/internal/server"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	listen := flag.String("listen", env("ARKNOVA_LISTEN", "127.0.0.1:8080"), "HTTP listen address")
	data := flag.String("data", env("ARKNOVA_DATA_DIR", "./data"), "game data directory")
	web := flag.String("web", env("ARKNOVA_WEB_DIR", "./web/build"), "built web client directory")
	publicURL := flag.String("public-url", env("ARKNOVA_PUBLIC_URL", ""), "public URL used in companion QR codes")
	buildInfoPath := flag.String("build-info", env("ARKNOVA_BUILD_INFO", ""), "installed build.json path")
	flag.Parse()

	build, err := buildinfo.Load(*buildInfoPath)
	if err != nil {
		return err
	}
	s, err := server.New(server.Config{Listen: *listen, DataDir: *data, WebDir: *web, PublicURL: *publicURL, Build: build})
	if err != nil {
		return err
	}
	defer func() {
		if err := s.Close(); err != nil {
			log.Printf("close projection store: %v", err)
		}
	}()

	listener, activated, err := activation.Listen(*listen)
	if err != nil {
		return err
	}
	defer listener.Close()
	log.Printf("Ark Nova server listening on %s (systemd activation: %t, commit: %s)", listener.Addr(), activated, build.Commit)

	signals, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()
	serveResult := make(chan error, 1)
	go func() { serveResult <- s.Serve(listener) }()

	select {
	case err := <-serveResult:
		return err
	case <-signals.Done():
		shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		shutdownErr := s.Shutdown(shutdownContext)
		serveErr := <-serveResult
		if shutdownErr != nil || serveErr != nil {
			return fmt.Errorf("shutdown: %w", errors.Join(shutdownErr, serveErr))
		}
		return nil
	}
}

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
