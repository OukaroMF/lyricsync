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

package external

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func sample(now time.Time) Snapshot {
	return Snapshot{Version: 1, Type: "snapshot", SessionID: "browser", Sequence: 1, Site: "youtube", MediaID: "abc", URL: "https://www.youtube.com/watch?v=abc", CaptionsEnabled: true, Text: "你好", Status: "paused", Duration: 10, UpdatedAt: now.UnixMilli(), HostID: "host"}
}

func TestReaderLifecycle(t *testing.T) {
	now := time.Now()
	path := filepath.Join(t.TempDir(), "subtitle.json")
	r := Reader{Path: path}
	if _, ok := r.Current(now); ok {
		t.Fatal("missing file active")
	}
	write := func(s Snapshot) {
		data, _ := json.Marshal(s)
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	s := sample(now)
	s.Translation = "翻译\n第二行"
	write(s)
	if got, ok := r.Current(now.Add(time.Second)); !ok || got.Text != "你好" || got.Translation != s.Translation {
		t.Fatal("paused caption not loaded")
	}
	s.Text = ""
	s.Translation = ""
	write(s)
	if got, ok := r.Current(now.Add(2 * time.Second)); !ok || got.Text != "" {
		t.Fatal("cue gap released ownership")
	}
	if _, ok := r.Current(now.Add(TTL)); ok {
		t.Fatal("expired snapshot active")
	}
	s.UpdatedAt = now.Add(20 * time.Second).UnixMilli()
	write(s)
	if _, ok := r.Current(now.Add(3 * time.Second)); ok {
		t.Fatal("future timestamp accepted")
	}
	os.WriteFile(path, []byte("{broken"), 0600)
	if _, ok := r.Current(now.Add(4 * time.Second)); ok {
		t.Fatal("malformed snapshot active")
	}
	s = sample(now)
	write(s)
	os.Remove(path)
	if _, ok := r.Current(now.Add(5 * time.Second)); ok {
		t.Fatal("deleted snapshot active")
	}
}

func TestOversizedTranslation(t *testing.T) {
	now := time.Now()
	s := sample(now)
	s.Translation = strings.Repeat("x", 65537)
	if s.Valid(now) {
		t.Fatal("oversized translation accepted")
	}
}

func TestUnsupportedSnapshots(t *testing.T) {
	now := time.Now()
	for _, alter := range []func(*Snapshot){func(s *Snapshot) { s.Version = 2 }, func(s *Snapshot) { s.Type = "clear" }, func(s *Snapshot) { s.CaptionsEnabled = false }, func(s *Snapshot) { s.Status = "ended" }, func(s *Snapshot) { s.Duration = 0 }, func(s *Snapshot) { s.HostID = "" }} {
		s := sample(now)
		alter(&s)
		if s.Valid(now) {
			t.Fatal("invalid snapshot is active")
		}
	}
}
