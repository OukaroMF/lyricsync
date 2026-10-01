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

func TestOutputParts(t *testing.T) {
	data := lyrics.Data{
		YRC: []lyrics.Line{{StartMS: 1000, Text: "hello", Words: []lyrics.Word{
			{Text: "he", StartMS: 1000, EndMS: 1200}, {Text: "llo", StartMS: 1200, EndMS: 1600},
		}}},
		Translated: []lyrics.TimedText{{StartMS: 1000, Text: "你好"}},
		Romanized:  []lyrics.TimedText{{StartMS: 1000, Text: "he-lo"}},
	}
	engine := NewEngine(fakeFetcher{data}, 0)
	track := mpris.Track{ID: 1, Position: 1300 * time.Millisecond, Status: mpris.Playing}
	if err := engine.Update(context.Background(), track); err != nil {
		t.Fatal(err)
	}
	out := engine.Output(track, state.Translation, OutputCombined)
	if !strings.HasPrefix(out.Text, "<span size=\"small\" alpha=\"75%\">你好</span>\n") || !strings.Contains(out.Text, "he<b><u>llo") {
		t.Fatalf("unexpected output: %q", out.Text)
	}

	tests := []struct {
		name string
		part OutputPart
		mode state.Mode
		want string
	}{
		{name: "original", part: OutputOriginal, mode: state.Translation, want: "he<b><u>llo"},
		{name: "translation", part: OutputTranslation, mode: state.Romanization, want: "你好"},
		{name: "romanization", part: OutputRomanization, mode: state.Translation, want: "he-lo"},
		{name: "secondary translation", part: OutputSecondary, mode: state.Translation, want: "你好"},
		{name: "secondary romanization", part: OutputSecondary, mode: state.Romanization, want: "he-lo"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := engine.Output(track, tt.mode, tt.part).Text
			if !strings.Contains(got, tt.want) || strings.Contains(got, "\n") {
				t.Fatalf("unexpected output: %q", got)
			}
		})
	}
}

func TestSecondaryOutputFallsBackWithoutCombiningLines(t *testing.T) {
	data := lyrics.Data{
		Original:   []lyrics.Line{{StartMS: 1000, Text: "原文"}},
		Translated: []lyrics.TimedText{{StartMS: 1000, Text: "translation"}},
	}
	engine := NewEngine(fakeFetcher{data}, 0)
	track := mpris.Track{ID: 1, Position: 1300 * time.Millisecond, Status: mpris.Playing}
	if err := engine.Update(context.Background(), track); err != nil {
		t.Fatal(err)
	}
	out := engine.Output(track, state.Romanization, OutputSecondary)
	if out.Text != "translation" {
		t.Fatalf("unexpected fallback: %q", out.Text)
	}
}

func TestParseOutputPart(t *testing.T) {
	for _, value := range []string{"combined", "original", "translation", "romanization", "secondary"} {
		if _, err := ParseOutputPart(value); err != nil {
			t.Fatalf("ParseOutputPart(%q): %v", value, err)
		}
	}
	if _, err := ParseOutputPart("invalid"); err == nil {
		t.Fatal("expected invalid output part error")
	}
}
