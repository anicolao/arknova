package game

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestDeterministicSetupXTokenTurnAndReplay(t *testing.T) {
	catalog := testCatalog()
	configured := configuredAction()
	first, err := Apply(State{GameID: "WILD"}, configured, catalog)
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := Apply(State{GameID: "WILD"}, configured, catalog)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, replayed) {
		t.Fatal("the same action lineage did not reproduce setup")
	}
	if len(first.Display) != 6 || len(first.Players[0].Hand) != 3 || len(first.Players[1].Hand) != 3 {
		t.Fatalf("unexpected deal: %#v", first)
	}

	action := Action{
		ActionID: "action-0002", ClientActionID: "player-1:1",
		Actor: Actor{Kind: "player", Seat: 1}, Type: "XTokenTaken",
		SchemaVersion: 1, ExpectedRevision: 1, Payload: ActionPayload{ActionCard: "Animals"},
	}
	next, err := Apply(first, action, catalog)
	if err != nil {
		t.Fatal(err)
	}
	if next.ActivePlayer != 2 || next.Revision != 2 || next.Players[0].XTokens != 1 {
		t.Fatalf("unexpected next turn: %#v", next)
	}
	wantOrder := []string{"Animals", "Cards", "Build", "Association", "Sponsors"}
	if !reflect.DeepEqual(next.Players[0].ActionCards, wantOrder) {
		t.Fatalf("action cards: got %v want %v", next.Players[0].ActionCards, wantOrder)
	}
	if first.Players[0].XTokens != 0 || first.Players[0].ActionCards[0] != "Cards" {
		t.Fatal("Apply mutated its input state")
	}
}

func TestProjectionDoesNotLeakOtherHands(t *testing.T) {
	catalog := testCatalog()
	state, err := Apply(State{GameID: "WILD"}, configuredAction(), catalog)
	if err != nil {
		t.Fatal(err)
	}
	table, _ := Project(state, 0, catalog)
	seat1, _ := Project(state, 1, catalog)
	tableJSON, _ := json.Marshal(table)
	seat1JSON, _ := json.Marshal(seat1)
	for _, id := range state.Players[0].Hand {
		card, _ := catalog.card(id)
		if strings.Contains(string(tableJSON), `"`+id+`"`) || strings.Contains(string(tableJSON), `"`+card.Name+`"`) {
			t.Fatalf("table projection leaked %s", id)
		}
	}
	for _, id := range state.Players[1].Hand {
		card, _ := catalog.card(id)
		if strings.Contains(string(seat1JSON), `"`+id+`"`) || strings.Contains(string(seat1JSON), `"`+card.Name+`"`) {
			t.Fatalf("seat 1 projection leaked seat 2 card %s", id)
		}
	}
}

func TestRejectsWrongTurnAndStaleRevision(t *testing.T) {
	catalog := testCatalog()
	state, _ := Apply(State{GameID: "WILD"}, configuredAction(), catalog)
	action := Action{Actor: Actor{Kind: "player", Seat: 2}, Type: "XTokenTaken", SchemaVersion: 1, ExpectedRevision: 1, Payload: ActionPayload{ActionCard: "Animals"}}
	if _, err := Apply(state, action, catalog); err == nil {
		t.Fatal("expected wrong-turn rejection")
	}
	action.Actor.Seat = 1
	action.ExpectedRevision = 0
	if _, err := Apply(state, action, catalog); err == nil {
		t.Fatal("expected stale-revision rejection")
	}
}

func TestPlayersCanAlternateXTokenTurns(t *testing.T) {
	catalog := testCatalog()
	state, _ := Apply(State{GameID: "WILD"}, configuredAction(), catalog)
	for turn := 0; turn < 12; turn++ {
		seat := state.ActivePlayer
		action := Action{
			ActionID:       fmt.Sprintf("action-%04d", state.Revision+1),
			ClientActionID: fmt.Sprintf("player-%d:%d", seat, turn),
			Actor:          Actor{Kind: "player", Seat: seat}, Type: "XTokenTaken",
			SchemaVersion: 1, ExpectedRevision: state.Revision,
			Payload: ActionPayload{ActionCard: state.Players[seat-1].ActionCards[2]},
		}
		var err error
		state, err = Apply(state, action, catalog)
		if err != nil {
			t.Fatalf("turn %d: %v", turn, err)
		}
	}
	if state.Revision != 13 || state.ActivePlayer != 1 || state.Players[0].XTokens != 6 || state.Players[1].XTokens != 6 {
		t.Fatalf("unexpected alternating state: %#v", state)
	}
}

func configuredAction() Action {
	return Action{
		ActionID: "action-0001", ClientActionID: "host:1", Actor: Actor{Kind: "host"},
		Type: "GameConfigured", SchemaVersion: 1, ExpectedRevision: 0,
		Payload: ActionPayload{PlayerCount: 2, Seed: "setup-seed", RulesetVersion: "base-v1", ContentVersion: "synthetic-v1", RNGVersion: "sha256-rank-v1"},
	}
}

func testCatalog() Catalog {
	cards := make([]Card, 12)
	for index := range cards {
		cards[index] = Card{ID: string(rune('a' + index)), Name: "Synthetic " + string(rune('A'+index)), Image: "cards/placeholder.svg"}
	}
	return Catalog{Version: "synthetic-v1", Cards: cards}
}
