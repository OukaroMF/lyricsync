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
