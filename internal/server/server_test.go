package server

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/anicolao/arknova/internal/buildinfo"
)

func TestHealthReportsInstalledBuildAndShutdownStopsServing(t *testing.T) {
	webDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(webDir, "index.html"), []byte("ready"), 0o600); err != nil {
		t.Fatal(err)
	}
	build := buildinfo.Info{
		Repository:            "github.com/anicolao/arknova",
		Commit:                "0123456789012345678901234567890123456789",
		GoVersion:             "1.24",
		BunVersion:            "1.2",
		ContentVersion:        "synthetic-1",
		ArtifactFormatVersion: buildinfo.FormatVersion,
	}
	server, err := New(Config{DataDir: t.TempDir(), WebDir: webDir, Build: build})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	serveResult := make(chan error, 1)
	go func() { serveResult <- server.Serve(listener) }()

	response, err := http.Get("http://" + listener.Addr().String() + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var health struct {
		Status string         `json:"status"`
		Build  buildinfo.Info `json:"build"`
	}
	if err := json.NewDecoder(response.Body).Decode(&health); err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || health.Status != "ready" || health.Build != build {
		t.Fatalf("unexpected health response: status=%d body=%+v", response.StatusCode, health)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-serveResult; err != nil {
		t.Fatal(err)
	}
}
