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
	"strings"
	"testing"
)

func TestExternalOutput(t *testing.T) {
	s := external.Snapshot{Text: "第一行 <b>&\n第二行", Title: "标题 <&>", Site: "youtube", URL: "https://www.youtube.com/watch?v=abc", Status: "paused", Position: 20, Duration: 10}
	for _, part := range []OutputPart{OutputOriginal, OutputCombined, OutputSecondary, OutputTranslation, OutputRomanization} {
		out := ExternalOutput(s, part)
		if part == OutputOriginal || part == OutputCombined {
			if out.Text != "第一行 &lt;b&gt;&amp;\n第二行" {
				t.Fatalf("markup or lines: %q", out.Text)
			}
		} else if out.Text != "" {
			t.Fatal("external secondary must be empty")
		}
		if out.Class != "paused" || out.Percentage != 100 || strings.Contains(out.Tooltip, "<&>") {
			t.Fatalf("bad metadata: %+v", out)
		}
	}
	s.Text = ""
	if ExternalOutput(s, OutputOriginal).Text != "" {
		t.Fatal("cue gap shows title")
	}
}

func TestExternalBilingualLayout(t *testing.T) {
	s := external.Snapshot{Text: "original <&>\nsecond line", Translation: "翻译 &\n第二行"}
	for part, want := range map[OutputPart]string{
		OutputOriginal:     "original &lt;&amp;&gt;\nsecond line",
		OutputTranslation:  "翻译 &amp;\n第二行",
		OutputSecondary:    "翻译 &amp;\n第二行",
		OutputRomanization: "",
		OutputCombined:     "<span size=\"small\" alpha=\"75%\">翻译 &amp;\n第二行</span>\noriginal &lt;&amp;&gt;\nsecond line",
	} {
		if got := ExternalOutput(s, part).Text; got != want {
			t.Fatalf("%s: got %q, want %q", part, got, want)
		}
	}
	s.Translation = s.Text
	if ExternalOutput(s, OutputSecondary).Text != "" || ExternalOutput(s, OutputCombined).Text != ExternalOutput(s, OutputOriginal).Text {
		t.Fatal("duplicate translation should be suppressed like music lyrics")
	}
	s.Text, s.Translation = "", ""
	if ExternalOutput(s, OutputCombined).Text != "" {
		t.Fatal("cue gap must clear both rows")
	}
}
