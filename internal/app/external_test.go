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
