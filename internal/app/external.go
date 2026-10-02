package app

import (
	"github.com/oukaromf/lyricsync/internal/external"
	"html"
	"strings"
)

func ExternalOutput(snapshot external.Snapshot, part OutputPart) Output {
	text := ""
	if part == OutputOriginal || part == OutputCombined {
		text = html.EscapeString(snapshot.Text)
	}
	percentage := 0
	if snapshot.Duration > 0 {
		if snapshot.Position >= snapshot.Duration {
			percentage = 100
		} else {
			percentage = int(snapshot.Position / snapshot.Duration * 100)
		}
	}
	tooltip := html.EscapeString(strings.Join([]string{snapshot.Title, snapshot.Site, snapshot.URL}, "\n"))
	return Output{Text: text, Tooltip: tooltip, Class: snapshot.Status, Percentage: percentage}
}
