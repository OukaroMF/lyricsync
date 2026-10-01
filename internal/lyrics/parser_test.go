package lyrics

import "testing"

func TestParseLRCFractionsAndMultipleTags(t *testing.T) {
	got := ParseLRC("[00:01.2][00:02.34] hello\n[01:03.456]world")
	if len(got) != 3 || got[0].StartMS != 1200 || got[1].StartMS != 2340 || got[2].StartMS != 63456 {
		t.Fatalf("unexpected LRC: %#v", got)
	}
}

func TestParseBracketYRC(t *testing.T) {
	got := ParseYRC("[1000,900](1000,400,0)你(1400,500,0)好")
	if len(got) != 1 || got[0].Text != "你好" || len(got[0].Words) != 2 || got[0].Words[1].EndMS != 1900 {
		t.Fatalf("unexpected YRC: %#v", got)
	}
}

func TestParseJSONYRC(t *testing.T) {
	got := ParseYRC(`{"t":1000,"c":[{"tx":"hi","tr":[100,300]},{"tx":"!"}]}`)
	if len(got) != 1 || got[0].Words[0].StartMS != 1100 || got[0].Text != "hi!" {
		t.Fatalf("unexpected JSON YRC: %#v", got)
	}
}

func TestClosestAllowsSmallTimestampDrift(t *testing.T) {
	items := []TimedText{{StartMS: 990, Text: "translated"}}
	if got := Closest(items, 1000); got != "translated" {
		t.Fatalf("got %q", got)
	}
}
