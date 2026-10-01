package mpris

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
)

type Status string

const (
	Playing Status = "Playing"
	Paused  Status = "Paused"
	Stopped Status = "Stopped"
)

var ErrNoPlayer = errors.New("未找到可用的 MPRIS 播放器")

type Track struct {
	ID       int64
	Player   string
	Title    string
	Artists  []string
	Album    string
	Status   Status
	Position time.Duration
	Length   time.Duration
}

type sample struct {
	track Track
	at    time.Time
}

type Source struct {
	conn     *dbus.Conn
	wanted   string
	poll     time.Duration
	mu       sync.Mutex
	last     sample
	lastPoll time.Time
}

func New(player string, poll time.Duration) (*Source, error) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, err
	}
	if poll < 100*time.Millisecond {
		poll = 100 * time.Millisecond
	}
	return &Source{conn: conn, wanted: strings.TrimPrefix(player, "org.mpris.MediaPlayer2."), poll: poll}, nil
}

func (s *Source) Close() error { return s.conn.Close() }

func (s *Source) Track(ctx context.Context) (Track, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	if s.last.track.ID != 0 && now.Sub(s.lastPoll) < s.poll {
		return interpolate(s.last, now), nil
	}
	track, err := s.read(ctx)
	s.lastPoll = now
	if err != nil {
		if errors.Is(err, ErrNoPlayer) {
			s.last = sample{}
			return Track{}, err
		}
		if s.last.track.ID != 0 {
			return interpolate(s.last, now), err
		}
		return Track{}, err
	}
	s.last = sample{track: track, at: now}
	return track, nil
}

func interpolate(last sample, now time.Time) Track {
	track := last.track
	if track.Status == Playing {
		track.Position += now.Sub(last.at)
		if track.Length > 0 && track.Position > track.Length {
			track.Position = track.Length
		}
	}
	return track
}

func (s *Source) read(ctx context.Context) (Track, error) {
	names, err := s.playerNames(ctx)
	if err != nil {
		return Track{}, err
	}
	if len(names) == 0 {
		return Track{}, ErrNoPlayer
	}
	var fallback Track
	for _, name := range names {
		track, err := s.readPlayer(ctx, name)
		if err != nil {
			continue
		}
		if track.Status == Playing {
			return track, nil
		}
		if fallback.ID == 0 {
			fallback = track
		}
	}
	if fallback.ID != 0 {
		return fallback, nil
	}
	return Track{}, ErrNoPlayer
}

func (s *Source) playerNames(ctx context.Context) ([]string, error) {
	var names []string
	call := s.conn.BusObject().CallWithContext(ctx, "org.freedesktop.DBus.ListNames", 0)
	if err := call.Store(&names); err != nil {
		return nil, err
	}
	filtered := names[:0]
	for _, name := range names {
		if !strings.HasPrefix(name, "org.mpris.MediaPlayer2.") {
			continue
		}
		short := strings.TrimPrefix(name, "org.mpris.MediaPlayer2.")
		if s.wanted == "auto" || s.wanted == "" || short == s.wanted || strings.HasPrefix(short, s.wanted+".") {
			filtered = append(filtered, name)
		}
	}
	sort.Strings(filtered)
	return filtered, nil
}

func (s *Source) readPlayer(ctx context.Context, name string) (Track, error) {
	obj := s.conn.Object(name, dbus.ObjectPath("/org/mpris/MediaPlayer2"))
	statusValue, err := property(ctx, obj, "PlaybackStatus")
	if err != nil {
		return Track{}, err
	}
	metadataValue, err := property(ctx, obj, "Metadata")
	if err != nil {
		return Track{}, err
	}
	metadata, ok := metadataValue.Value().(map[string]dbus.Variant)
	if !ok {
		return Track{}, fmt.Errorf("%s Metadata 类型无效", name)
	}
	positionValue, err := property(ctx, obj, "Position")
	if err != nil {
		return Track{}, err
	}
	track := Track{Player: name, Status: Status(variantString(statusValue)), Position: microseconds(positionValue.Value())}
	track.Title = metadataString(metadata, "xesam:title")
	track.Album = metadataString(metadata, "xesam:album")
	track.Artists = metadataStrings(metadata, "xesam:artist")
	track.Length = microseconds(metadataValueRaw(metadata, "mpris:length"))
	track.ID = ExtractSongID(metadataValueRaw(metadata, "mpris:trackid"), metadataString(metadata, "xesam:url"))
	return track, nil
}

func property(ctx context.Context, obj dbus.BusObject, name string) (dbus.Variant, error) {
	var value dbus.Variant
	call := obj.CallWithContext(ctx, "org.freedesktop.DBus.Properties.Get", 0, "org.mpris.MediaPlayer2.Player", name)
	if err := call.Store(&value); err != nil {
		return dbus.Variant{}, err
	}
	return value, nil
}

var trailingID = regexp.MustCompile(`(?:^|/)(\d+)$`)

func ExtractSongID(trackID any, rawURL string) int64 {
	value := fmt.Sprint(trackID)
	if match := trailingID.FindStringSubmatch(value); len(match) == 2 {
		id, _ := strconv.ParseInt(match[1], 10, 64)
		return id
	}
	if parsed, err := url.Parse(rawURL); err == nil {
		for _, key := range []string{"id", "songId"} {
			if id, err := strconv.ParseInt(parsed.Query().Get(key), 10, 64); err == nil && id > 0 {
				return id
			}
		}
		if id, err := strconv.ParseInt(path.Base(parsed.Path), 10, 64); err == nil && id > 0 {
			return id
		}
	}
	return 0
}

func variantString(value dbus.Variant) string {
	result, _ := value.Value().(string)
	return result
}

func metadataValueRaw(metadata map[string]dbus.Variant, key string) any {
	if value, ok := metadata[key]; ok {
		return value.Value()
	}
	return nil
}

func metadataString(metadata map[string]dbus.Variant, key string) string {
	value, _ := metadataValueRaw(metadata, key).(string)
	return value
}

func metadataStrings(metadata map[string]dbus.Variant, key string) []string {
	value, _ := metadataValueRaw(metadata, key).([]string)
	return value
}

func microseconds(value any) time.Duration {
	switch n := value.(type) {
	case int64:
		return time.Duration(n) * time.Microsecond
	case uint64:
		return time.Duration(n) * time.Microsecond
	case int32:
		return time.Duration(n) * time.Microsecond
	case uint32:
		return time.Duration(n) * time.Microsecond
	default:
		return 0
	}
}
