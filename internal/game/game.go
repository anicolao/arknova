package game

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"sort"
)

const SchemaVersion = 1

var initialActionCards = []string{"Cards", "Build", "Animals", "Association", "Sponsors"}

type Actor struct {
	Kind string `json:"kind"`
	Seat int    `json:"seat,omitempty"`
}

type ActionPayload struct {
	PlayerCount    int    `json:"playerCount,omitempty"`
	Seed           string `json:"seed,omitempty"`
	RulesetVersion string `json:"rulesetVersion,omitempty"`
	ContentVersion string `json:"contentVersion,omitempty"`
	RNGVersion     string `json:"rngVersion,omitempty"`
	ActionCard     string `json:"actionCard,omitempty"`
}

type Action struct {
	ActionID         string        `json:"actionId"`
	ClientActionID   string        `json:"clientActionId"`
	Actor            Actor         `json:"actor"`
	Type             string        `json:"type"`
	SchemaVersion    int           `json:"schemaVersion"`
	RecordedAtMS     int64         `json:"recordedAtMs"`
	ExpectedRevision int           `json:"expectedRevision"`
	Payload          ActionPayload `json:"payload"`
}

type Card struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Image string `json:"image"`
}

type Catalog struct {
	Version string `json:"version"`
	Cards   []Card `json:"cards"`
}

type PlayerState struct {
	Seat        int      `json:"seat"`
	Hand        []string `json:"hand"`
	ActionCards []string `json:"actionCards"`
	XTokens     int      `json:"xTokens"`
}

type HistoryEntry struct {
	Revision int    `json:"revision"`
	Seat     int    `json:"seat,omitempty"`
	Summary  string `json:"summary"`
}

type State struct {
	GameID                string         `json:"gameId"`
	PlayerCount           int            `json:"playerCount"`
	Revision              int            `json:"revision"`
	RulesetVersion        string         `json:"rulesetVersion,omitempty"`
	ContentVersion        string         `json:"contentVersion,omitempty"`
	RNGVersion            string         `json:"rngVersion,omitempty"`
	Seed                  string         `json:"seed,omitempty"`
	ActivePlayer          int            `json:"activePlayer,omitempty"`
	Display               []string       `json:"display,omitempty"`
	Players               []PlayerState  `json:"players,omitempty"`
	History               []HistoryEntry `json:"history,omitempty"`
	AcceptedClientActions map[string]int `json:"acceptedClientActions,omitempty"`
}

type CardProjection struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Image string `json:"image"`
}

type PlayerProjection struct {
	Seat        int      `json:"seat"`
	ActionCards []string `json:"actionCards"`
	XTokens     int      `json:"xTokens"`
}

type Projection struct {
	GameID       string             `json:"gameId"`
	PlayerCount  int                `json:"playerCount"`
	Player       int                `json:"player,omitempty"`
	Revision     int                `json:"revision"`
	ActivePlayer int                `json:"activePlayer,omitempty"`
	Display      []CardProjection   `json:"display,omitempty"`
	Players      []PlayerProjection `json:"players,omitempty"`
	Hand         []CardProjection   `json:"hand,omitempty"`
	History      []HistoryEntry     `json:"history,omitempty"`
}

func Apply(state State, action Action, catalog Catalog) (State, error) {
	if action.SchemaVersion != SchemaVersion {
		return state, fmt.Errorf("unsupported schema version %d", action.SchemaVersion)
	}
	if action.ExpectedRevision != state.Revision {
		return state, fmt.Errorf("expected revision %d, got %d", state.Revision, action.ExpectedRevision)
	}
	if state.Revision == 0 {
		return configure(state, action, catalog)
	}
	if action.Type != "XTokenTaken" {
		return state, fmt.Errorf("unsupported action type %q", action.Type)
	}
	if action.Actor.Kind != "player" || action.Actor.Seat < 1 || action.Actor.Seat > len(state.Players) || action.Actor.Seat != state.ActivePlayer {
		return state, errors.New("only the active player can take an X-token")
	}
	if _, exists := state.AcceptedClientActions[action.ClientActionID]; exists {
		return state, errors.New("duplicate client action")
	}

	next := cloneState(state)
	player := &next.Players[action.Actor.Seat-1]
	index := indexOf(player.ActionCards, action.Payload.ActionCard)
	if index < 0 {
		return state, errors.New("action card is not in this player's action row")
	}
	strength := index + 1
	selected := player.ActionCards[index]
	copy(player.ActionCards[1:index+1], player.ActionCards[0:index])
	player.ActionCards[0] = selected
	player.XTokens++
	next.Revision++
	next.ActivePlayer = next.ActivePlayer%next.PlayerCount + 1
	next.AcceptedClientActions[action.ClientActionID] = next.Revision
	next.History = append(next.History, HistoryEntry{
		Revision: next.Revision,
		Seat:     action.Actor.Seat,
		Summary:  fmt.Sprintf("Player %d took an X-token with %s at strength %d", action.Actor.Seat, selected, strength),
	})
	return next, nil
}

func configure(state State, action Action, catalog Catalog) (State, error) {
	if action.Type != "GameConfigured" {
		return state, errors.New("expected the initial GameConfigured action")
	}
	if action.Payload.PlayerCount < 1 || action.Payload.PlayerCount > 4 {
		return state, errors.New("player count must be between 1 and 4")
	}
	state.PlayerCount = action.Payload.PlayerCount
	state.Revision = 1

	// Logs created by Increment 001 did not pin content. They remain replayable
	// as their original empty connected-player projection.
	if action.Payload.ContentVersion == "" {
		return state, nil
	}
	if action.Payload.ContentVersion != catalog.Version {
		return State{}, fmt.Errorf("game requires content %q, loaded %q", action.Payload.ContentVersion, catalog.Version)
	}
	if action.Payload.Seed == "" {
		return State{}, errors.New("game seed is required")
	}
	const displaySize, handSize = 6, 3
	required := displaySize + action.Payload.PlayerCount*handSize
	if len(catalog.Cards) < required {
		return State{}, fmt.Errorf("content pack needs at least %d cards", required)
	}

	ordered := append([]Card(nil), catalog.Cards...)
	sort.Slice(ordered, func(i, j int) bool {
		left := initialCardRank(action.Payload, action.ActionID, ordered[i].ID)
		right := initialCardRank(action.Payload, action.ActionID, ordered[j].ID)
		if comparison := bytes.Compare(left[:], right[:]); comparison != 0 {
			return comparison < 0
		}
		return ordered[i].ID < ordered[j].ID
	})

	state.Seed = action.Payload.Seed
	state.RulesetVersion = action.Payload.RulesetVersion
	state.ContentVersion = action.Payload.ContentVersion
	state.RNGVersion = action.Payload.RNGVersion
	state.ActivePlayer = 1
	state.AcceptedClientActions = map[string]int{action.ClientActionID: 1}
	state.Display = cardIDs(ordered[:displaySize])
	state.Players = make([]PlayerState, action.Payload.PlayerCount)
	for seat := 1; seat <= action.Payload.PlayerCount; seat++ {
		state.Players[seat-1] = PlayerState{
			Seat:        seat,
			ActionCards: append([]string(nil), initialActionCards...),
		}
	}
	for round := 0; round < handSize; round++ {
		for seat := 0; seat < action.Payload.PlayerCount; seat++ {
			index := displaySize + round*action.Payload.PlayerCount + seat
			state.Players[seat].Hand = append(state.Players[seat].Hand, ordered[index].ID)
		}
	}
	state.History = []HistoryEntry{{Revision: 1, Summary: "The game was configured and starting cards were dealt"}}
	return state, nil
}

func Project(state State, player int, catalog Catalog) (Projection, error) {
	if player < 0 || player > state.PlayerCount {
		return Projection{}, errors.New("invalid player")
	}
	projection := Projection{
		GameID:       state.GameID,
		PlayerCount:  state.PlayerCount,
		Player:       player,
		Revision:     state.Revision,
		ActivePlayer: state.ActivePlayer,
		History:      append([]HistoryEntry(nil), state.History...),
	}
	for _, id := range state.Display {
		card, err := catalog.card(id)
		if err != nil {
			return Projection{}, err
		}
		projection.Display = append(projection.Display, CardProjection(card))
	}
	for _, source := range state.Players {
		projection.Players = append(projection.Players, PlayerProjection{
			Seat:        source.Seat,
			ActionCards: append([]string(nil), source.ActionCards...),
			XTokens:     source.XTokens,
		})
	}
	if player > 0 && len(state.Players) > 0 {
		for _, id := range state.Players[player-1].Hand {
			card, err := catalog.card(id)
			if err != nil {
				return Projection{}, err
			}
			projection.Hand = append(projection.Hand, CardProjection(card))
		}
	}
	return projection, nil
}

func AcceptedRevision(state State, clientActionID string) (int, bool) {
	revision, ok := state.AcceptedClientActions[clientActionID]
	return revision, ok
}

func (catalog Catalog) card(id string) (Card, error) {
	for _, card := range catalog.Cards {
		if card.ID == id {
			return card, nil
		}
	}
	return Card{}, fmt.Errorf("unknown card %q", id)
}

func cloneState(state State) State {
	next := state
	next.Display = append([]string(nil), state.Display...)
	next.History = append([]HistoryEntry(nil), state.History...)
	next.Players = make([]PlayerState, len(state.Players))
	for index, player := range state.Players {
		next.Players[index] = player
		next.Players[index].Hand = append([]string(nil), player.Hand...)
		next.Players[index].ActionCards = append([]string(nil), player.ActionCards...)
	}
	next.AcceptedClientActions = make(map[string]int, len(state.AcceptedClientActions)+1)
	for id, revision := range state.AcceptedClientActions {
		next.AcceptedClientActions[id] = revision
	}
	return next
}

func initialCardRank(payload ActionPayload, actionID, cardID string) [sha256.Size]byte {
	input := payload.Seed + "\x00" + payload.RulesetVersion + "\x00" + payload.ContentVersion + "\x00" +
		payload.RNGVersion + "\x00" + actionID + "\x00initial-card-order\x00" + cardID
	return sha256.Sum256([]byte(input))
}

func cardIDs(cards []Card) []string {
	ids := make([]string, len(cards))
	for index, card := range cards {
		ids[index] = card.ID
	}
	return ids
}

func indexOf(values []string, target string) int {
	for index, value := range values {
		if value == target {
			return index
		}
	}
	return -1
}
