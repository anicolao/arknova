package main

import (
	"flag"
	"log"
	"os"

	"github.com/anicolao/arknova/internal/server"
)

func main() {
	listen := flag.String("listen", env("ARKNOVA_LISTEN", "127.0.0.1:8080"), "HTTP listen address")
	data := flag.String("data", env("ARKNOVA_DATA_DIR", "./data"), "game data directory")
	web := flag.String("web", env("ARKNOVA_WEB_DIR", "./web/build"), "built web client directory")
	publicURL := flag.String("public-url", env("ARKNOVA_PUBLIC_URL", ""), "public URL used in companion QR codes")
	flag.Parse()

	s, err := server.New(server.Config{Listen: *listen, DataDir: *data, WebDir: *web, PublicURL: *publicURL})
	if err != nil {
		log.Fatal(err)
	}
	defer s.Close()
	log.Printf("Ark Nova server listening on http://%s", *listen)
	if err := s.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
