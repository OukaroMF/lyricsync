package mpris

import "testing"

func TestExtractSongID(t *testing.T) {
	tests := []struct {
		track, url string
		want       int64
	}{
		{"/org/mpd/Tracks/524152942", "", 524152942},
		{"/not/a/song", "https://music.163.com/song?id=123", 123},
		{"/not/a/song", "https://music.163.com/song/456", 456},
	}
	for _, test := range tests {
		if got := ExtractSongID(test.track, test.url); got != test.want {
			t.Errorf("ExtractSongID(%q, %q) = %d, want %d", test.track, test.url, got, test.want)
		}
	}
}
