package native

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const MaxMessage = 128 * 1024

type Message struct {
	Version         int     `json:"version"`
	Type            string  `json:"type"`
	SessionID       string  `json:"sessionId"`
	Sequence        uint64  `json:"sequence"`
	Site            string  `json:"site,omitempty"`
	MediaID         string  `json:"mediaId,omitempty"`
	Title           string  `json:"title,omitempty"`
	URL             string  `json:"url,omitempty"`
	CaptionsEnabled bool    `json:"captionsEnabled,omitempty"`
	Text            string  `json:"text"`
	Status          string  `json:"status,omitempty"`
	Position        float64 `json:"position,omitempty"`
	Duration        float64 `json:"duration,omitempty"`
}

type State struct {
	Message
	UpdatedAt int64  `json:"updatedAt"`
	HostID    string `json:"hostId"`
}

var biliPath = regexp.MustCompile(`^/(video/(BV[\w]+|av\d+)|bangumi/play/(ep|ss)\d+)/?$`)

func (m Message) Validate() error {
	if m.Version != 1 || (m.Type != "snapshot" && m.Type != "clear") || m.SessionID == "" || len(m.SessionID) > 128 || m.Sequence == 0 || m.Sequence > 9007199254740991 {
		return errors.New("invalid protocol envelope")
	}
	if m.Type == "clear" {
		return nil
	}
	u, err := url.Parse(m.URL)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" || len(m.URL) > 8192 {
		return errors.New("invalid page URL")
	}
	validSite := m.Site == "youtube" && u.Host == "www.youtube.com" && u.Path == "/watch" && u.Query().Get("v") != "" ||
		m.Site == "bilibili" && u.Host == "www.bilibili.com" && biliPath.MatchString(u.Path)
	if !validSite || !m.CaptionsEnabled || (m.Status != "playing" && m.Status != "paused") || m.MediaID == "" || len(m.MediaID) > 2048 || len(m.Title) > 16384 || len(m.Text) > 65536 {
		return errors.New("invalid subtitle snapshot")
	}
	if math.IsNaN(m.Position) || math.IsInf(m.Position, 0) || m.Position < 0 || math.IsNaN(m.Duration) || math.IsInf(m.Duration, 0) || m.Duration <= 0 {
		return errors.New("invalid media time")
	}
	return nil
}

func StatePath() string {
	runtime := os.Getenv("XDG_RUNTIME_DIR")
	if runtime == "" {
		runtime = filepath.Join("/run/user", strconv.Itoa(os.Getuid()))
	}
	return filepath.Join(runtime, "cctracker", "subtitle.json")
}

func ReadFrame(reader io.Reader) ([]byte, error) {
	var header [4]byte
	if _, err := io.ReadFull(reader, header[:]); err != nil {
		return nil, err
	}
	size := binary.NativeEndian.Uint32(header[:])
	if size == 0 || size > MaxMessage {
		return nil, errors.New("native message exceeds limit or is empty")
	}
	data := make([]byte, size)
	_, err := io.ReadFull(reader, data)
	return data, err
}

func WriteFrame(writer io.Writer, message any) error {
	data, err := json.Marshal(message)
	if err != nil {
		return err
	}
	if len(data) > MaxMessage {
		return errors.New("native reply too large")
	}
	var header [4]byte
	binary.NativeEndian.PutUint32(header[:], uint32(len(data)))
	for _, part := range [][]byte{header[:], data} {
		for len(part) > 0 {
			n, err := writer.Write(part)
			if err != nil {
				return err
			}
			if n == 0 {
				return io.ErrShortWrite
			}
			part = part[n:]
		}
	}
	return nil
}

type Store struct {
	Path    string
	HostID  string
	current *Message
}

func (s *Store) Write(message Message, now time.Time) error {
	dir := filepath.Dir(s.Path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0700); err != nil {
		return err
	}
	data, err := json.Marshal(State{Message: message, UpdatedAt: now.UnixMilli(), HostID: s.HostID})
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(dir, ".subtitle-*")
	if err != nil {
		return err
	}
	defer os.Remove(temporary.Name())
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporary.Name(), s.Path); err != nil {
		return err
	}
	s.current = &message
	return nil
}

func (s *Store) Clear() error {
	s.current = nil
	data, err := os.ReadFile(s.Path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}
	// An old host must never erase a newer connection's snapshot on exit.
	if state.HostID != s.HostID {
		return nil
	}
	err = os.Remove(s.Path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func Run(ctx context.Context, reader io.Reader, writer io.Writer, store *Store, heartbeat time.Duration) error {
	defer store.Clear()
	type incoming struct {
		data []byte
		err  error
	}
	frames := make(chan incoming, 1)
	go func() {
		for {
			data, err := ReadFrame(reader)
			select {
			case frames <- incoming{data, err}:
			case <-ctx.Done():
				return
			}
			if err != nil {
				return
			}
		}
	}()
	ticker := time.NewTicker(heartbeat)
	defer ticker.Stop()
	var session string
	var sequence uint64
	for {
		select {
		case <-ctx.Done():
			return nil
		case now := <-ticker.C:
			if store.current != nil {
				if err := store.Write(*store.current, now); err != nil {
					return err
				}
			}
		case frame := <-frames:
			if errors.Is(frame.err, io.EOF) {
				return nil
			}
			if frame.err != nil {
				return frame.err
			}
			var message Message
			err := json.Unmarshal(frame.data, &message)
			if err == nil {
				err = message.Validate()
			}
			if err == nil && session != "" && (session != message.SessionID || message.Sequence <= sequence) {
				err = errors.New("out-of-order native message")
			}
			if err != nil {
				if err := WriteFrame(writer, map[string]any{"type": "error", "error": err.Error()}); err != nil {
					return err
				}
				continue
			}
			session, sequence = message.SessionID, message.Sequence
			if message.Type == "clear" {
				err = store.Clear()
			} else {
				err = store.Write(message, time.Now())
			}
			if err != nil {
				return fmt.Errorf("write subtitle state: %w", err)
			}
			if err := WriteFrame(writer, map[string]any{"type": "ack", "sequence": sequence}); err != nil {
				return err
			}
		}
	}
}

func ValidOrigin(origin string) bool {
	return strings.HasPrefix(origin, "chrome-extension://") && regexp.MustCompile(`^chrome-extension://[a-p]{32}/?$`).MatchString(origin)
}
