package app

import (
	"context"
	"fmt"
	"html"
	"strings"
	"sync"
	"time"

	"github.com/oukaromf/lyricsync/internal/lyrics"
	"github.com/oukaromf/lyricsync/internal/mpris"
	"github.com/oukaromf/lyricsync/internal/state"
)

type Fetcher interface {
	Fetch(context.Context, int64) (lyrics.Data, error)
}

type Output struct {
	Text       string `json:"text"`
	Tooltip    string `json:"tooltip"`
	Class      string `json:"class"`
	Percentage int    `json:"percentage,omitempty"`
}

type Engine struct {
	mu      sync.RWMutex
	fetcher Fetcher
	offset  time.Duration
	id      int64
	data    lyrics.Data
	err     error
}

func NewEngine(fetcher Fetcher, offset time.Duration) *Engine {
	return &Engine{fetcher: fetcher, offset: offset}
}

func (e *Engine) Update(ctx context.Context, track mpris.Track) error {
	data, err := e.fetcher.Fetch(ctx, track.ID)
	e.mu.Lock()
	defer e.mu.Unlock()
	e.id, e.err = track.ID, err
	if err == nil {
		e.data = data
	} else {
		e.data = lyrics.Data{}
	}
	return err
}

func (e *Engine) Output(track mpris.Track, mode state.Mode) Output {
	e.mu.RLock()
	data, fetchErr, loadedID := e.data, e.err, e.id
	e.mu.RUnlock()
	class := strings.ToLower(string(track.Status))
	if class == "" {
		class = "stopped"
	}
	title := strings.TrimSpace(track.Title)
	artist := strings.Join(track.Artists, ", ")
	tooltip := html.EscapeString(strings.Trim(strings.Join([]string{title, artist, track.Album}, "\n"), "\n"))
	if track.ID == 0 {
		return Output{Text: "󰝚  未检测到 go-musicfox", Tooltip: "请确认播放器已启动且 MPRIS 已启用", Class: "stopped"}
	}
	if loadedID != track.ID || fetchErr != nil {
		text := html.EscapeString(strings.TrimSpace(strings.Join([]string{title, artist}, " — ")))
		if text == "" {
			text = "󰎆  正在获取歌词…"
		}
		if fetchErr != nil {
			tooltip += "\n" + html.EscapeString(fetchErr.Error())
		}
		return Output{Text: text, Tooltip: tooltip, Class: class}
	}
	positionMS := track.Position.Milliseconds() + e.offset.Milliseconds()
	line, ok := data.ActiveLine(positionMS)
	if !ok {
		return Output{Text: html.EscapeString(title), Tooltip: tooltip, Class: class}
	}
	primary := renderLine(line, positionMS)
	secondary := ""
	if mode == state.Romanization {
		secondary = lyrics.Closest(data.Romanized, line.StartMS)
		if secondary == "" {
			secondary = lyrics.Closest(data.Translated, line.StartMS)
		}
	} else {
		secondary = lyrics.Closest(data.Translated, line.StartMS)
		if secondary == "" {
			secondary = lyrics.Closest(data.Romanized, line.StartMS)
		}
	}
	text := ""
	if secondary != "" && strings.TrimSpace(secondary) != strings.TrimSpace(line.Text) {
		text = "<span size=\"small\" alpha=\"75%\">" + html.EscapeString(secondary) + "</span>\n"
	}
	text += primary
	percentage := 0
	if track.Length > 0 {
		percentage = int(float64(track.Position) / float64(track.Length) * 100)
		if percentage > 100 {
			percentage = 100
		}
	}
	modeLabel := "翻译"
	if mode == state.Romanization {
		modeLabel = "罗马音"
	}
	tooltip = fmt.Sprintf("%s\n显示：%s · 左键切换", tooltip, modeLabel)
	return Output{Text: text, Tooltip: tooltip, Class: class, Percentage: percentage}
}

func renderLine(line lyrics.Line, positionMS int64) string {
	if len(line.Words) == 0 {
		return html.EscapeString(line.Text)
	}
	var played, current, remaining strings.Builder
	for _, word := range line.Words {
		switch {
		case positionMS >= word.EndMS:
			played.WriteString(html.EscapeString(word.Text))
		case positionMS >= word.StartMS:
			current.WriteString(html.EscapeString(word.Text))
		default:
			remaining.WriteString(html.EscapeString(word.Text))
		}
	}
	var output strings.Builder
	output.WriteString(played.String())
	if current.Len() > 0 {
		output.WriteString("<b><u>")
		output.WriteString(current.String())
		output.WriteString("</u></b>")
	}
	if remaining.Len() > 0 {
		output.WriteString("<span alpha=\"55%\">")
		output.WriteString(remaining.String())
		output.WriteString("</span>")
	}
	return output.String()
}
