package eventlog

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/anicolao/arknova/internal/game"
)

func Append(path string, action game.Action) (int64, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return 0, err
	}
	line, err := json.Marshal(action)
	if err != nil {
		return 0, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	if _, err = f.Write(append(line, '\n')); err != nil {
		return 0, err
	}
	if err = f.Sync(); err != nil {
		return 0, err
	}
	info, err := f.Stat()
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

func Read(path string) ([]game.Action, int64, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, 0, nil
	}
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()
	var actions []game.Action
	reader := bufio.NewReader(f)
	var offset int64
	for {
		line, readErr := reader.ReadBytes('\n')
		if len(line) > 0 {
			offset += int64(len(line))
			if line[len(line)-1] != '\n' {
				return nil, 0, fmt.Errorf("partial JSONL line at byte %d", offset-int64(len(line)))
			}
			var action game.Action
			if err := json.Unmarshal(line, &action); err != nil {
				return nil, 0, fmt.Errorf("decode action: %w", err)
			}
			actions = append(actions, action)
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return nil, 0, readErr
		}
	}
	return actions, offset, nil
}
