package app

import (
	"github.com/oukaromf/lyricsync/internal/external"
	"html"
	"strings"
)

func ExternalOutput(snapshot external.Snapshot, part OutputPart) Output {
	primary := html.EscapeString(snapshot.Text)
	translation := distinctSecondary(snapshot.Translation, snapshot.Text)
	text := primary
	switch part {
	case OutputTranslation, OutputSecondary:
		text = html.EscapeString(translation)
	case OutputRomanization:
		text = ""
	case OutputOriginal:
		text = primary
	default:
		if translation != "" {
			text = "<span size=\"small\" alpha=\"75%\">" + html.EscapeString(translation) + "</span>\n" + primary
		}
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
