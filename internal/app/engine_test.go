package app

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/oukaromf/lyricsync/internal/lyrics"
	"github.com/oukaromf/lyricsync/internal/mpris"
	"github.com/oukaromf/lyricsync/internal/state"
)

type fakeFetcher struct{ data lyrics.Data }

func (f fakeFetcher) Fetch(context.Context, int64) (lyrics.Data, error) { return f.data, nil }

func TestOutputUsesTwoLinesAndWordMarkup(t *testing.T) {
	data := lyrics.Data{
		YRC: []lyrics.Line{{StartMS: 1000, Text: "hello", Words: []lyrics.Word{
			{Text: "he", StartMS: 1000, EndMS: 1200}, {Text: "llo", StartMS: 1200, EndMS: 1600},
		}}},
		Translated: []lyrics.TimedText{{StartMS: 1000, Text: "你好"}},
	}
	engine := NewEngine(fakeFetcher{data}, 0)
	track := mpris.Track{ID: 1, Position: 1300 * time.Millisecond, Status: mpris.Playing}
	if err := engine.Update(context.Background(), track); err != nil {
		t.Fatal(err)
	}
	out := engine.Output(track, state.Translation)
	if !strings.Contains(out.Text, "he<b><u>llo") || !strings.Contains(out.Text, "\n") || !strings.Contains(out.Text, "你好") {
		t.Fatalf("unexpected output: %q", out.Text)
	}
}
