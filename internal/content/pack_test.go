package content

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSyntheticPack(t *testing.T) {
	pack, err := Load(filepath.Join("..", "..", "content", "synthetic"))
	if err != nil {
		t.Fatal(err)
	}
	if pack.Version != "synthetic-v1" || len(pack.Cards) != 18 {
		t.Fatalf("unexpected pack: %#v", pack)
	}
}

func TestRejectsMissingAsset(t *testing.T) {
	root := t.TempDir()
	manifest := `{"version":"broken","assets":[{"path":"missing.svg","mediaType":"image/svg+xml","width":300,"height":420}],"cards":[{"id":"card","name":"Card","image":"missing.svg"}]}`
	if err := os.WriteFile(filepath.Join(root, "manifest.json"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(root); err == nil {
		t.Fatal("expected missing-asset error")
	}
}
