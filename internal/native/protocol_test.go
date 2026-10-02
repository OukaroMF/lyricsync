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

package native

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func sample() Message {
	return Message{Version: 1, Type: "snapshot", SessionID: "session", Sequence: 1, Site: "youtube", MediaID: "abc", Title: "标题", URL: "https://www.youtube.com/watch?v=abc", CaptionsEnabled: true, Text: "你好\n<&字幕>", Translation: "hello &\nsecond line", Status: "paused", Position: 1, Duration: 10}
}

func TestFrames(t *testing.T) {
	var buffer bytes.Buffer
	if err := WriteFrame(&buffer, sample()); err != nil {
		t.Fatal(err)
	}
	data, err := ReadFrame(&buffer)
	if err != nil {
		t.Fatal(err)
	}
	var got Message
	if err := json.Unmarshal(data, &got); err != nil || got.Text != sample().Text || got.Translation != sample().Translation {
		t.Fatalf("UTF-8 round trip: %v, %q", err, got.Text)
	}
	for _, input := range [][]byte{{1}, {1, 0, 0, 0}, {0, 0, 0, 0}} {
		if _, err := ReadFrame(bytes.NewReader(input)); err == nil {
			t.Fatal("accepted malformed frame")
		}
	}
	var header [4]byte
	binary.NativeEndian.PutUint32(header[:], MaxMessage+1)
	if _, err := ReadFrame(bytes.NewReader(header[:])); err == nil {
		t.Fatal("accepted oversized frame")
	}
}

func TestValidation(t *testing.T) {
	for _, alter := range []func(*Message){func(m *Message) { m.Version = 2 }, func(m *Message) { m.URL = "https://evil.test/watch?v=abc" }, func(m *Message) { m.Text = string(bytes.Repeat([]byte("x"), 65537)) }, func(m *Message) { m.CaptionsEnabled = false }, func(m *Message) { m.Duration = 0 }, func(m *Message) { m.Position = -1 }, func(m *Message) { m.Sequence = 0 }} {
		m := sample()
		alter(&m)
		if m.Validate() == nil {
			t.Fatal("accepted invalid snapshot")
		}
	}
	m := sample()
	m.Site = "bilibili"
	m.URL = "https://www.bilibili.com/video/BV123abc?p=2"
	if err := m.Validate(); err != nil {
		t.Fatal(err)
	}
	if !ValidOrigin("chrome-extension://aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/") || ValidOrigin("https://evil.test") {
		t.Fatal("origin validation")
	}
}

func TestOversizedTranslation(t *testing.T) {
	m := sample()
	m.Translation = string(bytes.Repeat([]byte("x"), 65537))
	if m.Validate() == nil {
		t.Fatal("accepted oversized translation")
	}
}

func TestStorePermissionsAndOwner(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cctracker", "subtitle.json")
	s := Store{Path: path, HostID: "old"}
	if err := s.Write(sample(), time.Now()); err != nil {
		t.Fatal(err)
	}
	for path, mode := range map[string]os.FileMode{path: 0600, filepath.Dir(path): 0700} {
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm() != mode {
			t.Fatalf("permissions %s: %v", path, err)
		}
	}
	newer := Store{Path: path, HostID: "new"}
	newer.Write(sample(), time.Now())
	if err := s.Clear(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("old host erased newer snapshot")
	}
	if err := newer.Clear(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("state survives clear")
	}
}

func TestRunHeartbeatSequenceAndEOF(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cctracker", "subtitle.json")
	input, send := io.Pipe()
	receive, output := io.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- Run(ctx, input, output, &Store{Path: path, HostID: "host"}, 5*time.Millisecond) }()
	reply := func() map[string]any {
		t.Helper()
		data, err := ReadFrame(receive)
		if err != nil {
			t.Fatal(err)
		}
		var message map[string]any
		json.Unmarshal(data, &message)
		return message
	}
	WriteFrame(send, sample())
	if reply()["type"] != "ack" {
		t.Fatal("no ack")
	}
	data, _ := os.ReadFile(path)
	var before State
	json.Unmarshal(data, &before)
	time.Sleep(20 * time.Millisecond)
	data, _ = os.ReadFile(path)
	var after State
	json.Unmarshal(data, &after)
	if after.UpdatedAt <= before.UpdatedAt || after.Text != sample().Text || after.Translation != sample().Translation {
		t.Fatal("paused state did not stay alive")
	}
	WriteFrame(send, sample())
	if reply()["type"] != "error" {
		t.Fatal("accepted duplicate sequence")
	}
	m := sample()
	m.Type = "clear"
	m.Sequence = 2
	WriteFrame(send, m)
	if reply()["type"] != "ack" {
		t.Fatal("clear failed")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("clear retained snapshot")
	}
	m = sample()
	m.Sequence = 3
	WriteFrame(send, m)
	reply()
	send.Close()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("host did not exit on EOF")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("EOF left subtitles")
	}
	input.Close()
	receive.Close()
	output.Close()
}
