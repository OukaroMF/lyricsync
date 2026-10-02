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

// Package external reads the current caption; the browser owns its timing.
package external

import (
	"encoding/json"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

const TTL = 10 * time.Second

type Snapshot struct {
	Version         int     `json:"version"`
	Type            string  `json:"type"`
	SessionID       string  `json:"sessionId"`
	Sequence        uint64  `json:"sequence"`
	Site            string  `json:"site"`
	MediaID         string  `json:"mediaId"`
	Title           string  `json:"title"`
	URL             string  `json:"url"`
	CaptionsEnabled bool    `json:"captionsEnabled"`
	Text            string  `json:"text"`
	Translation     string  `json:"translation,omitempty"`
	Status          string  `json:"status"`
	Position        float64 `json:"position"`
	Duration        float64 `json:"duration"`
	UpdatedAt       int64   `json:"updatedAt"`
	HostID          string  `json:"hostId"`
}

func StatePath() string {
	runtime := os.Getenv("XDG_RUNTIME_DIR")
	if runtime == "" {
		runtime = filepath.Join("/run/user", strconv.Itoa(os.Getuid()))
	}
	return filepath.Join(runtime, "cctracker", "subtitle.json")
}

func (s Snapshot) Valid(now time.Time) bool {
	age := now.Sub(time.UnixMilli(s.UpdatedAt))
	return s.Version == 1 && s.Type == "snapshot" && s.CaptionsEnabled && s.SessionID != "" && s.HostID != "" && s.MediaID != "" && s.Sequence > 0 &&
		(s.Site == "youtube" || s.Site == "bilibili") && (s.Status == "playing" || s.Status == "paused") &&
		s.UpdatedAt > 0 && age >= -time.Second && age < TTL && len(s.Text) <= 65536 && len(s.Translation) <= 65536 &&
		!math.IsNaN(s.Position) && !math.IsInf(s.Position, 0) && s.Position >= 0 &&
		!math.IsNaN(s.Duration) && !math.IsInf(s.Duration, 0) && s.Duration > 0
}

type Reader struct {
	Path     string
	lastRead time.Time
	cached   Snapshot
}

func NewReader() *Reader { return &Reader{Path: StatePath()} }

func (r *Reader) Current(now time.Time) (Snapshot, bool) {
	if r.lastRead.IsZero() || now.Sub(r.lastRead) >= 100*time.Millisecond || now.Before(r.lastRead) {
		r.lastRead = now
		r.cached = Snapshot{}
		file, err := os.Open(r.Path)
		if err == nil {
			decoder := json.NewDecoder(io.LimitReader(file, 128*1024+1))
			var snapshot Snapshot
			if err := decoder.Decode(&snapshot); err == nil {
				var trailing any
				if decoder.Decode(&trailing) == io.EOF {
					r.cached = snapshot
				}
			}
			file.Close()
		}
	}
	return r.cached, r.cached.Valid(now)
}
