package buildinfo

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
)

const FormatVersion = 1

type Info struct {
	Repository            string `json:"repository"`
	Commit                string `json:"commit"`
	GoVersion             string `json:"goVersion"`
	BunVersion            string `json:"bunVersion"`
	ContentVersion        string `json:"contentVersion"`
	ArtifactFormatVersion int    `json:"artifactFormatVersion"`
}

func Development() Info {
	return Info{
		Repository:            "github.com/anicolao/arknova",
		Commit:                "development",
		GoVersion:             runtime.Version(),
		BunVersion:            "development",
		ContentVersion:        "development",
		ArtifactFormatVersion: FormatVersion,
	}
}

func Load(path string) (Info, error) {
	if path == "" {
		return Development(), nil
	}
	file, err := os.Open(path)
	if err != nil {
		return Info{}, fmt.Errorf("open build information: %w", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var info Info
	if err := decoder.Decode(&info); err != nil {
		return Info{}, fmt.Errorf("decode build information: %w", err)
	}
	if err := ensureEOF(decoder); err != nil {
		return Info{}, err
	}
	if info.Repository == "" || info.Commit == "" || info.GoVersion == "" ||
		info.BunVersion == "" || info.ContentVersion == "" {
		return Info{}, errors.New("build information has an empty required field")
	}
	if info.ArtifactFormatVersion != FormatVersion {
		return Info{}, fmt.Errorf("unsupported artifact format version %d", info.ArtifactFormatVersion)
	}
	return info, nil
}

func ensureEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); errors.Is(err, io.EOF) {
		return nil
	} else if err != nil {
		return fmt.Errorf("decode trailing build information: %w", err)
	}
	return errors.New("build information contains more than one JSON value")
}
