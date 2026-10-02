package main

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/oukaromf/lyricsync/internal/app"
	"github.com/oukaromf/lyricsync/internal/external"
	"github.com/oukaromf/lyricsync/internal/lyrics"
	"github.com/oukaromf/lyricsync/internal/mpris"
)

type fakeSource struct{}

func (fakeSource) Track(context.Context) (mpris.Track, error) {
	return mpris.Track{ID: 1, Player: "musicfox", Title: "song", Status: mpris.Playing, Position: time.Second}, nil
}

type blockingFetcher struct{ started, release chan struct{} }

func (f blockingFetcher) Fetch(ctx context.Context, _ int64) (lyrics.Data, error) {
	close(f.started)
	select {
	case <-f.release:
	case <-ctx.Done():
		return lyrics.Data{}, ctx.Err()
	}
	return lyrics.Data{Original: []lyrics.Line{{StartMS: 0, Text: "musicfox lyric"}}}, nil
}

func TestExternalTakesOverDuringSlowFetchAndFallsBack(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	input, output := io.Pipe()
	defer input.Close()
	defer output.Close()
	path := filepath.Join(t.TempDir(), "subtitle.json")
	fetcher := blockingFetcher{make(chan struct{}), make(chan struct{})}
	done := make(chan error, 1)
	go func() {
		done <- stream(ctx, fakeSource{}, app.NewEngine(fetcher, 0), app.OutputOriginal, 20*time.Millisecond, false, true, &external.Reader{Path: path}, output)
	}()
	frames := make(chan app.Output, 100)
	go func() {
		scanner := bufio.NewScanner(input)
		for scanner.Scan() {
			var frame app.Output
			if json.Unmarshal(scanner.Bytes(), &frame) == nil {
				frames <- frame
			}
		}
	}()
	await := func(text string) {
		t.Helper()
		deadline := time.After(2 * time.Second)
		for {
			select {
			case frame := <-frames:
				if frame.Text == text {
					return
				}
			case <-deadline:
				t.Fatalf("never displayed %q", text)
			}
		}
	}
	select {
	case <-fetcher.started:
	case <-time.After(time.Second):
		t.Fatal("fetch not started")
	}
	s := external.Snapshot{Version: 1, Type: "snapshot", SessionID: "page", Sequence: 1, Site: "youtube", MediaID: "abc", CaptionsEnabled: true, Text: "字幕 <b>", Status: "paused", Duration: 10, UpdatedAt: time.Now().UnixMilli(), HostID: "host"}
	data, _ := json.Marshal(s)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	await("字幕 &lt;b&gt;")
	close(fetcher.release)
	os.Remove(path)
	await("musicfox lyric")
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("stream didn't stop")
	}
}
