package game

import (
	"errors"
	"fmt"
)

const SchemaVersion = 1

type Actor struct {
	Kind string `json:"kind"`
}

type GameConfiguredPayload struct {
	PlayerCount int `json:"playerCount"`
}

type Action struct {
	ActionID       string                `json:"actionId"`
	ClientActionID string                `json:"clientActionId"`
	Actor          Actor                 `json:"actor"`
	Type           string                `json:"type"`
	SchemaVersion  int                   `json:"schemaVersion"`
	RecordedAtMS   int64                 `json:"recordedAtMs"`
	Payload        GameConfiguredPayload `json:"payload"`
}

type State struct {
	GameID      string `json:"gameId"`
	PlayerCount int    `json:"playerCount"`
	Revision    int    `json:"revision"`
}

type Projection struct {
	GameID      string `json:"gameId"`
	PlayerCount int    `json:"playerCount"`
	Player      int    `json:"player,omitempty"`
	Revision    int    `json:"revision"`
}

func Apply(state State, action Action) (State, error) {
	if action.SchemaVersion != SchemaVersion {
		return state, fmt.Errorf("unsupported schema version %d", action.SchemaVersion)
	}
	if action.Type != "GameConfigured" || state.Revision != 0 {
		return state, errors.New("expected the initial GameConfigured action")
	}
	if action.Payload.PlayerCount < 1 || action.Payload.PlayerCount > 4 {
		return state, errors.New("player count must be between 1 and 4")
	}
	state.PlayerCount = action.Payload.PlayerCount
	state.Revision = 1
	return state, nil
}

func Project(state State, player int) (Projection, error) {
	if player < 0 || player > state.PlayerCount {
		return Projection{}, errors.New("invalid player")
	}
	return Projection{GameID: state.GameID, PlayerCount: state.PlayerCount, Player: player, Revision: state.Revision}, nil
}
