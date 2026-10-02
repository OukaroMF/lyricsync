// Package native receives CCTracker snapshots directly in the LyricSync binary.
package native

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

const HostName = "com.oukaromf.lyricsync"

// Serve runs before any MPRIS or Waybar setup. Stdout is only native frames.
func Serve(ctx context.Context, args []string, input io.Reader, output io.Writer) error {
	if len(args) != 1 || !ValidOrigin(args[0]) {
		return errors.New("native-host requires a chrome-extension:// origin")
	}
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return err
	}
	return Run(ctx, input, output, &Store{Path: StatePath(), HostID: hex.EncodeToString(random[:])}, 2*time.Second)
}

// Install registers this executable directly; no helper binary is installed.
func Install(args []string, output io.Writer) error {
	flags := flag.NewFlagSet("install-native-host", flag.ContinueOnError)
	browser := flags.String("browser", "chromium", "chrome, chromium, or brave")
	id := flags.String("extension-id", "", "32-character extension ID from the browser's extensions page")
	profile := flags.String("profile-dir", "", "override browser user-data root (not Default/Profile 1)")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if !regexp.MustCompile(`^[a-p]{32}$`).MatchString(*id) {
		return errors.New("invalid extension ID")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	config := os.Getenv("XDG_CONFIG_HOME")
	if config == "" {
		config = filepath.Join(home, ".config")
	}
	paths := map[string]string{"chrome": "google-chrome", "chromium": "chromium", "brave": "BraveSoftware/Brave-Browser"}
	browserPath, ok := paths[*browser]
	if !ok {
		return errors.New("unsupported browser")
	}
	if *profile == "" {
		*profile = filepath.Join(config, browserPath)
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected installation argument")
	}
	profileRoot, err := filepath.Abs(*profile)
	if err != nil {
		return err
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	binaryPath, err := filepath.EvalSymlinks(executable)
	if err != nil {
		return err
	}
	manifest := map[string]any{"name": HostName, "description": "LyricSync receives CCTracker subtitles directly", "path": binaryPath,
		"type": "stdio", "allowed_origins": []string{"chrome-extension://" + *id + "/"}}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	manifestDir := filepath.Join(profileRoot, "NativeMessagingHosts")
	if err := os.MkdirAll(manifestDir, 0700); err != nil {
		return err
	}
	path := filepath.Join(manifestDir, HostName+".json")
	if err := os.WriteFile(path, append(data, '\n'), 0600); err != nil {
		return err
	}
	fmt.Fprintln(output, "Installed native host:", path)
	fmt.Fprintln(output, "Allowed extension:", *id)
	return nil
}
