package buildinfo

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "build.json")
	data := `{"repository":"github.com/anicolao/arknova","commit":"0123456789012345678901234567890123456789","goVersion":"1.24","bunVersion":"1.2","contentVersion":"synthetic-1","artifactFormatVersion":1}`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	info, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Commit != "0123456789012345678901234567890123456789" {
		t.Fatalf("unexpected commit %q", info.Commit)
	}
}

func TestLoadRejectsUnknownFieldsAndVersions(t *testing.T) {
	for name, data := range map[string]string{
		"unknown":  `{"repository":"repo","commit":"sha","goVersion":"go","bunVersion":"bun","contentVersion":"content","artifactFormatVersion":1,"extra":true}`,
		"version":  `{"repository":"repo","commit":"sha","goVersion":"go","bunVersion":"bun","contentVersion":"content","artifactFormatVersion":2}`,
		"trailing": `{"repository":"repo","commit":"sha","goVersion":"go","bunVersion":"bun","contentVersion":"content","artifactFormatVersion":1} {}`,
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "build.json")
			if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := Load(path); err == nil {
				t.Fatal("expected invalid build information to be rejected")
			}
		})
	}
}
