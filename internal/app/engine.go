// Copyright (C) 2026  OukaroMF
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

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

type OutputPart string

const (
	OutputCombined     OutputPart = "combined"
	OutputOriginal     OutputPart = "original"
	OutputTranslation  OutputPart = "translation"
	OutputRomanization OutputPart = "romanization"
	OutputSecondary    OutputPart = "secondary"
)

func ParseOutputPart(value string) (OutputPart, error) {
	part := OutputPart(strings.ToLower(strings.TrimSpace(value)))
	switch part {
	case OutputCombined, OutputOriginal, OutputTranslation, OutputRomanization, OutputSecondary:
		return part, nil
	default:
		return "", fmt.Errorf("未知输出类型 %q；可用值：combined、original、translation、romanization、secondary", value)
	}
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

func (e *Engine) Output(track mpris.Track, mode state.Mode, part OutputPart) Output {
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
		return selectOutputPart(Output{Text: "󰝚  未检测到 go-musicfox", Tooltip: "请确认播放器已启动且 MPRIS 已启用", Class: "stopped"}, part)
	}
	if loadedID != track.ID || fetchErr != nil {
		text := html.EscapeString(strings.TrimSpace(strings.Join([]string{title, artist}, " — ")))
		if text == "" {
			text = "󰎆  正在获取歌词…"
		}
		if fetchErr != nil {
			tooltip += "\n" + html.EscapeString(fetchErr.Error())
		}
		return selectOutputPart(Output{Text: text, Tooltip: tooltip, Class: class}, part)
	}
	positionMS := track.Position.Milliseconds() + e.offset.Milliseconds()
	line, ok := data.ActiveLine(positionMS)
	if !ok {
		return selectOutputPart(Output{Text: html.EscapeString(title), Tooltip: tooltip, Class: class}, part)
	}
	primary := renderLine(line, positionMS)
	translation := distinctSecondary(lyrics.Closest(data.Translated, line.StartMS), line.Text)
	romanization := distinctSecondary(lyrics.Closest(data.Romanized, line.StartMS), line.Text)
	secondary := translation
	if mode == state.Romanization {
		secondary = romanization
		if secondary == "" {
			secondary = translation
		}
	} else if secondary == "" {
		secondary = romanization
	}
	text := primary
	switch part {
	case OutputOriginal:
		text = primary
	case OutputTranslation:
		text = html.EscapeString(translation)
	case OutputRomanization:
		text = html.EscapeString(romanization)
	case OutputSecondary:
		text = html.EscapeString(secondary)
	default:
		if secondary != "" {
			text = "<span size=\"small\" alpha=\"75%\">" + html.EscapeString(secondary) + "</span>\n" + primary
		}
	}
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

func selectOutputPart(output Output, part OutputPart) Output {
	if part == OutputTranslation || part == OutputRomanization || part == OutputSecondary {
		output.Text = ""
	}
	return output
}

func distinctSecondary(text, original string) string {
	if strings.TrimSpace(text) == strings.TrimSpace(original) {
		return ""
	}
	return text
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
