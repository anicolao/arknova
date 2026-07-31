package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/anicolao/arknova/internal/buildinfo"
	"github.com/anicolao/arknova/internal/content"
	"github.com/anicolao/arknova/internal/eventlog"
	"github.com/anicolao/arknova/internal/game"
	"github.com/anicolao/arknova/internal/projections"
	"github.com/gorilla/websocket"
)

type Config struct {
	Listen, DataDir, WebDir, ContentDir, PublicURL string
	Build                                          buildinfo.Info
}
type Server struct {
	config  Config
	catalog game.Catalog
	store   *projections.Store
	games   map[string]game.State
	clients map[string]map[*websocket.Conn]int
	mu      sync.Mutex
	http    *http.Server
	closing bool
}

func New(config Config) (*Server, error) {
	catalog, err := content.Load(config.ContentDir)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(config.DataDir, "games"), 0o755); err != nil {
		return nil, err
	}
	store, err := projections.Open(filepath.Join(config.DataDir, "projections.sqlite"))
	if err != nil {
		return nil, err
	}
	s := &Server{config: config, catalog: catalog, store: store, games: map[string]game.State{}, clients: map[string]map[*websocket.Conn]int{}}
	if err := s.replay(); err != nil {
		store.Close()
		return nil, err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /api/build", s.build)
	mux.HandleFunc("POST /api/games", s.createGame)
	mux.HandleFunc("POST /api/games/{gameID}/actions", s.submitAction)
	mux.HandleFunc("GET /api/games/{gameID}/projection", s.getProjection)
	mux.HandleFunc("GET /api/games/{gameID}/actions", s.diagnostics)
	mux.HandleFunc("GET /ws", s.stream)
	mux.Handle("GET /content/", http.StripPrefix("/content/", http.FileServer(http.Dir(config.ContentDir))))
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

func (s *Server) Serve(listener net.Listener) error {
	err := s.http.Serve(listener)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	s.closing = true
	connections := make([]*websocket.Conn, 0)
	for _, clients := range s.clients {
		for connection := range clients {
			connections = append(connections, connection)
		}
	}
	s.mu.Unlock()
	for _, connection := range connections {
		_ = connection.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseGoingAway, "server shutting down"),
			time.Now().Add(time.Second),
		)
		_ = connection.Close()
	}
	return s.http.Shutdown(ctx)
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
			state, err = game.Apply(state, action, s.catalog)
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
	s.mu.Lock()
	games := len(s.games)
	closing := s.closing
	s.mu.Unlock()
	if closing {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"status": "stopping", "games": games, "build": s.config.Build})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ready", "games": games, "build": s.config.Build})
}

func (s *Server) build(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.config.Build)
}

func (s *Server) createGame(w http.ResponseWriter, r *http.Request) {
	var body struct {
		PlayerCount    int    `json:"playerCount"`
		ClientActionID string `json:"clientActionId"`
		Seed           string `json:"seed"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON", 400)
		return
	}
	if body.PlayerCount < 1 || body.PlayerCount > 4 {
		http.Error(w, "playerCount must be 1-4", 400)
		return
	}
	if body.ClientActionID == "" {
		http.Error(w, "clientActionId is required", 400)
		return
	}
	if testControls() && os.Getenv("ARKNOVA_GAME_SEED") != "" {
		body.Seed = os.Getenv("ARKNOVA_GAME_SEED")
	}
	if body.Seed == "" {
		http.Error(w, "seed is required", 400)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	id := s.newGameID()
	if _, exists := s.games[id]; exists {
		http.Error(w, "deterministic game already exists", 409)
		return
	}
	action := game.Action{
		ActionID: s.newActionID(1), ClientActionID: body.ClientActionID,
		Actor: game.Actor{Kind: "host"}, Type: "GameConfigured", SchemaVersion: 1,
		RecordedAtMS: s.nowMS(), ExpectedRevision: 0,
		Payload: game.ActionPayload{
			PlayerCount: body.PlayerCount, Seed: body.Seed,
			RulesetVersion: "base-v1", ContentVersion: s.catalog.Version, RNGVersion: "sha256-rank-v1",
		},
	}
	state, err := game.Apply(game.State{GameID: id}, action, s.catalog)
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

func (s *Server) submitAction(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Player           int                `json:"player"`
		Type             string             `json:"type"`
		SchemaVersion    int                `json:"schemaVersion"`
		ExpectedRevision int                `json:"expectedRevision"`
		ClientActionID   string             `json:"clientActionId"`
		Payload          game.ActionPayload `json:"payload"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON", 400)
		return
	}
	if body.ClientActionID == "" {
		http.Error(w, "clientActionId is required", 400)
		return
	}
	id := strings.ToUpper(r.PathValue("gameID"))
	s.mu.Lock()
	defer s.mu.Unlock()
	state, ok := s.games[id]
	if !ok {
		http.NotFound(w, r)
		return
	}
	if revision, duplicate := game.AcceptedRevision(state, body.ClientActionID); duplicate {
		writeJSON(w, http.StatusOK, map[string]any{"revision": revision, "duplicate": true})
		return
	}
	if body.ExpectedRevision != state.Revision {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "stale revision", "revision": state.Revision})
		return
	}
	action := game.Action{
		ActionID: s.newActionID(state.Revision + 1), ClientActionID: body.ClientActionID,
		Actor: game.Actor{Kind: "player", Seat: body.Player}, Type: body.Type,
		SchemaVersion: body.SchemaVersion, RecordedAtMS: s.nowMS(),
		ExpectedRevision: body.ExpectedRevision, Payload: body.Payload,
	}
	next, err := game.Apply(state, action, s.catalog)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	offset, err := eventlog.Append(s.actionPath(id), action)
	if err != nil {
		http.Error(w, "persist action", 500)
		return
	}
	if err := s.store.Put(next, offset); err != nil {
		http.Error(w, "project action", 500)
		return
	}
	s.games[id] = next
	s.broadcastLocked(id, next)
	writeJSON(w, http.StatusCreated, map[string]any{"revision": next.Revision, "actionId": action.ActionID})
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
	projection, err := game.Project(state, player, s.catalog)
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
	if !ok {
		s.mu.Unlock()
		http.Error(w, "game not found", 404)
		return
	}
	_, err := game.Project(state, player, s.catalog)
	s.mu.Unlock()
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	s.mu.Lock()
	if s.closing {
		s.mu.Unlock()
		_ = conn.Close()
		return
	}
	state = s.games[id]
	projection, err := game.Project(state, player, s.catalog)
	if err != nil {
		s.mu.Unlock()
		_ = conn.Close()
		return
	}
	if s.clients[id] == nil {
		s.clients[id] = map[*websocket.Conn]int{}
	}
	s.clients[id][conn] = player
	if err := conn.WriteJSON(projection); err != nil {
		delete(s.clients[id], conn)
		s.mu.Unlock()
		_ = conn.Close()
		return
	}
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.clients[id], conn)
		if len(s.clients[id]) == 0 {
			delete(s.clients, id)
		}
		s.mu.Unlock()
		_ = conn.Close()
	}()
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
	projection, _ := game.Project(state, player, s.catalog)
	return map[string]any{"projection": projection, "companionUrls": urls}
}

func (s *Server) broadcastLocked(id string, state game.State) {
	for conn, player := range s.clients[id] {
		projection, err := game.Project(state, player, s.catalog)
		if err != nil || conn.WriteJSON(projection) != nil {
			_ = conn.Close()
			delete(s.clients[id], conn)
		}
	}
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
func (s *Server) newActionID(revision int) string {
	if testControls() && os.Getenv("ARKNOVA_DETERMINISTIC_IDS") == "1" {
		return fmt.Sprintf("action-%04d", revision)
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
