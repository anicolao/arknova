package game

import "testing"

func TestApplyAndProjectConfiguredGame(t *testing.T) {
	action := Action{Type: "GameConfigured", SchemaVersion: 1, Payload: GameConfiguredPayload{PlayerCount: 2}}
	state, err := Apply(State{GameID: "WILD"}, action)
	if err != nil {
		t.Fatal(err)
	}
	projection, err := Project(state, 1)
	if err != nil {
		t.Fatal(err)
	}
	if projection.GameID != "WILD" || projection.Player != 1 || projection.Revision != 1 {
		t.Fatalf("unexpected projection: %#v", projection)
	}
}
