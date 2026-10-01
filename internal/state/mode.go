package state

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Mode string

const (
	Translation  Mode = "translation"
	Romanization Mode = "romanization"
)

func ParseMode(value string) (Mode, error) {
	switch Mode(strings.TrimSpace(value)) {
	case Translation:
		return Translation, nil
	case Romanization, "roman", "roma":
		return Romanization, nil
	default:
		return "", errors.New("显示模式必须是 translation 或 romanization")
	}
}

func Path() string {
	if runtimeDir := os.Getenv("XDG_RUNTIME_DIR"); runtimeDir != "" {
		return filepath.Join(runtimeDir, "lyricsync", "mode")
	}
	return filepath.Join(os.TempDir(), "lyricsync-"+strconv.Itoa(os.Getuid()), "mode")
}

func ReadMode() Mode {
	data, err := os.ReadFile(Path())
	if err != nil {
		return Translation
	}
	mode, err := ParseMode(string(data))
	if err != nil {
		return Translation
	}
	return mode
}

func WriteMode(mode Mode) error {
	if _, err := ParseMode(string(mode)); err != nil {
		return err
	}
	path := Path()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".mode-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err = tmp.WriteString(string(mode) + "\n"); err == nil {
		err = tmp.Chmod(0o600)
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func Toggle() (Mode, error) {
	next := Romanization
	if ReadMode() == Romanization {
		next = Translation
	}
	return next, WriteMode(next)
}
