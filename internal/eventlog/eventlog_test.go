package eventlog

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/anicolao/arknova/internal/game"
)

func TestAppendReadAndRejectPartialTail(t *testing.T) {
	path := filepath.Join(t.TempDir(), "actions.jsonl")
	action := game.Action{Type: "GameConfigured", SchemaVersion: 1, Payload: game.GameConfiguredPayload{PlayerCount: 2}}
	if _, err := Append(path, action); err != nil {
		t.Fatal(err)
	}
	actions, offset, err := Read(path)
	if err != nil || len(actions) != 1 || offset == 0 {
		t.Fatalf("read: %v %#v %d", err, actions, offset)
	}
	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	_, _ = f.WriteString("{")
	_ = f.Close()
	if _, _, err := Read(path); err == nil {
		t.Fatal("expected partial-tail error")
	}
}
