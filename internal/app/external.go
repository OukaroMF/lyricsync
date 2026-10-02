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
