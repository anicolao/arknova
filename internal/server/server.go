package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/anicolao/arknova/internal/eventlog"
	"github.com/anicolao/arknova/internal/game"
	"github.com/anicolao/arknova/internal/projections"
	"github.com/gorilla/websocket"
)

type Config struct{ Listen, DataDir, WebDir, PublicURL string }
type Server struct {
	config  Config
	store   *projections.Store
	games   map[string]game.State
	clients map[string]map[*websocket.Conn]int
	mu      sync.Mutex
	http    *http.Server
}

func New(config Config) (*Server, error) {
	if err := os.MkdirAll(filepath.Join(config.DataDir, "games"), 0o755); err != nil {
		return nil, err
	}
	store, err := projections.Open(filepath.Join(config.DataDir, "projections.sqlite"))
	if err != nil {
		return nil, err
	}
	s := &Server{config: config, store: store, games: map[string]game.State{}, clients: map[string]map[*websocket.Conn]int{}}
	if err := s.replay(); err != nil {
		store.Close()
		return nil, err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("POST /api/games", s.createGame)
	mux.HandleFunc("GET /api/games/{gameID}/projection", s.getProjection)
	mux.HandleFunc("GET /api/games/{gameID}/actions", s.diagnostics)
	mux.HandleFunc("GET /ws", s.stream)
	mux.Handle("/", spa(config.WebDir))
	s.http = &http.Server{Addr: config.Listen, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	return s, nil
}

func (s *Server) ListenAndServe() error {
	err := s.http.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
func (s *Server) Close() error { return s.store.Close() }

func (s *Server) replay() error {
	if err := s.store.DeleteAll(); err != nil {
		return err
	}
	entries, err := os.ReadDir(filepath.Join(s.config.DataDir, "games"))
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		id := entry.Name()
		actions, offset, err := eventlog.Read(s.actionPath(id))
		if err != nil {
			return fmt.Errorf("replay %s: %w", id, err)
		}
		state := game.State{GameID: id}
		for _, action := range actions {
			state, err = game.Apply(state, action)
			if err != nil {
				return fmt.Errorf("replay %s: %w", id, err)
			}
		}
		if state.Revision > 0 {
			s.games[id] = state
			if err := s.store.Put(state, offset); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ready", "games": len(s.games)})
}

func (s *Server) createGame(w http.ResponseWriter, r *http.Request) {
	var body struct {
		PlayerCount    int    `json:"playerCount"`
		ClientActionID string `json:"clientActionId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON", 400)
		return
	}
	if body.PlayerCount < 1 || body.PlayerCount > 4 {
		http.Error(w, "playerCount must be 1-4", 400)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	id := s.newGameID()
	if _, exists := s.games[id]; exists {
		http.Error(w, "deterministic game already exists", 409)
		return
	}
	action := game.Action{ActionID: s.newActionID(), ClientActionID: body.ClientActionID, Actor: game.Actor{Kind: "host"}, Type: "GameConfigured", SchemaVersion: 1, RecordedAtMS: s.nowMS(), Payload: game.GameConfiguredPayload{PlayerCount: body.PlayerCount}}
	state, err := game.Apply(game.State{GameID: id}, action)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	offset, err := eventlog.Append(s.actionPath(id), action)
	if err != nil {
		http.Error(w, "persist game", 500)
		return
	}
	if err := s.store.Put(state, offset); err != nil {
		http.Error(w, "project game", 500)
		return
	}
	s.games[id] = state
	writeJSON(w, http.StatusCreated, s.response(r, state, 0))
}

func (s *Server) getProjection(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, ok := s.games[strings.ToUpper(r.PathValue("gameID"))]
	if !ok {
		http.NotFound(w, r)
		return
	}
	player, _ := strconv.Atoi(r.URL.Query().Get("player"))
	projection, err := game.Project(state, player)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	writeJSON(w, 200, projection)
}

func (s *Server) diagnostics(w http.ResponseWriter, r *http.Request) {
	if os.Getenv("ARKNOVA_E2E") != "1" || os.Getenv("ARKNOVA_ALLOW_TEST_CONTROLS") != "1" {
		http.NotFound(w, r)
		return
	}
	actions, _, err := eventlog.Read(s.actionPath(strings.ToUpper(r.PathValue("gameID"))))
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, 200, actions)
}

var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool {
	origin, err := url.Parse(r.Header.Get("Origin"))
	return err == nil && origin.Host == r.Host
}}

func (s *Server) stream(w http.ResponseWriter, r *http.Request) {
	id := strings.ToUpper(r.URL.Query().Get("gameid"))
	player, _ := strconv.Atoi(r.URL.Query().Get("player"))
	s.mu.Lock()
	state, ok := s.games[id]
	s.mu.Unlock()
	if !ok {
		http.Error(w, "game not found", 404)
		return
	}
	projection, err := game.Project(state, player)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	if err := conn.WriteJSON(projection); err != nil {
		return
	}
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
	}
}

func (s *Server) response(r *http.Request, state game.State, player int) map[string]any {
	origin := s.config.PublicURL
	if origin == "" {
		origin = "http://" + r.Host
	}
	urls := make([]string, state.PlayerCount)
	for i := range urls {
		urls[i] = fmt.Sprintf("%s/play?gameid=%s&player=%d", strings.TrimRight(origin, "/"), state.GameID, i+1)
	}
	projection, _ := game.Project(state, player)
	return map[string]any{"projection": projection, "companionUrls": urls}
}

func (s *Server) actionPath(id string) string {
	return filepath.Join(s.config.DataDir, "games", id, "actions.jsonl")
}
func (s *Server) nowMS() int64 {
	if v := os.Getenv("ARKNOVA_FIXED_NOW_MS"); v != "" && testControls() {
		n, _ := strconv.ParseInt(v, 10, 64)
		return n
	}
	return time.Now().UnixMilli()
}
func (s *Server) newGameID() string {
	if testControls() && os.Getenv("ARKNOVA_DETERMINISTIC_IDS") == "1" {
		return "WILD"
	}
	b := make([]byte, 3)
	_, _ = rand.Read(b)
	return strings.ToUpper(hex.EncodeToString(b))
}
func (s *Server) newActionID() string {
	if testControls() && os.Getenv("ARKNOVA_DETERMINISTIC_IDS") == "1" {
		return "action-0001"
	}
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
func testControls() bool {
	return os.Getenv("ARKNOVA_E2E") == "1" && os.Getenv("ARKNOVA_ALLOW_TEST_CONTROLS") == "1"
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func spa(dir string) http.Handler {
	fileServer := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Clean(r.URL.Path)
		if path == "/" || path == "/table" || path == "/play" {
			index, err := os.Open(filepath.Join(dir, "index.html"))
			if err != nil {
				http.Error(w, "web client is not built", http.StatusNotFound)
				return
			}
			defer index.Close()
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = io.Copy(w, index)
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}
