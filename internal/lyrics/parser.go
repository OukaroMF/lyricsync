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

package lyrics

import (
	"bufio"
	"encoding/json"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var lrcTimeRE = regexp.MustCompile(`\[(\d+):(\d{1,2})(?:[.:](\d{1,3}))?\]`)

func ParseLRC(raw string) []TimedText {
	result := make([]TimedText, 0)
	scanner := bufio.NewScanner(strings.NewReader(raw))
	scanner.Buffer(make([]byte, 1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(strings.TrimPrefix(scanner.Text(), "\ufeff"))
		matches := lrcTimeRE.FindAllStringSubmatchIndex(line, -1)
		if len(matches) == 0 {
			continue
		}
		content := strings.TrimSpace(line[matches[len(matches)-1][1]:])
		for _, match := range matches {
			minutes, _ := strconv.ParseInt(line[match[2]:match[3]], 10, 64)
			seconds, _ := strconv.ParseInt(line[match[4]:match[5]], 10, 64)
			fraction := int64(0)
			if match[6] >= 0 {
				value := line[match[6]:match[7]]
				fraction, _ = strconv.ParseInt(value, 10, 64)
				switch len(value) {
				case 1:
					fraction *= 100
				case 2:
					fraction *= 10
				}
			}
			result = append(result, TimedText{StartMS: minutes*60000 + seconds*1000 + fraction, Text: content})
		}
	}
	sort.SliceStable(result, func(i, j int) bool { return result[i].StartMS < result[j].StartMS })
	return result
}

func ParseYRC(raw string) []Line {
	result := make([]Line, 0)
	scanner := bufio.NewScanner(strings.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 4*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		var parsed Line
		var ok bool
		if strings.HasPrefix(line, "{") {
			parsed, ok = parseJSONYRC(line)
		} else if strings.HasPrefix(line, "[") {
			parsed, ok = parseBracketYRC(line)
		}
		if ok {
			result = append(result, parsed)
		}
	}
	sort.SliceStable(result, func(i, j int) bool { return result[i].StartMS < result[j].StartMS })
	return result
}

func parseBracketYRC(raw string) (Line, bool) {
	close := strings.IndexByte(raw, ']')
	if close < 2 {
		return Line{}, false
	}
	header := strings.Split(raw[1:close], ",")
	if len(header) < 2 {
		return Line{}, false
	}
	start, err1 := strconv.ParseInt(strings.TrimSpace(header[0]), 10, 64)
	duration, err2 := strconv.ParseInt(strings.TrimSpace(header[1]), 10, 64)
	if err1 != nil || err2 != nil {
		return Line{}, false
	}
	line := Line{StartMS: start, EndMS: start + duration}
	content := raw[close+1:]
	for len(content) > 0 {
		open := strings.IndexByte(content, '(')
		if open < 0 {
			break
		}
		content = content[open+1:]
		endTiming := strings.IndexByte(content, ')')
		if endTiming < 0 {
			break
		}
		parts := strings.Split(content[:endTiming], ",")
		content = content[endTiming+1:]
		if len(parts) < 2 {
			continue
		}
		wordStart, e1 := strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 64)
		wordDuration, e2 := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
		next := strings.IndexByte(content, '(')
		wordText := content
		if next >= 0 {
			wordText, content = content[:next], content[next:]
		} else {
			content = ""
		}
		if e1 == nil && e2 == nil && wordText != "" {
			line.Words = append(line.Words, Word{Text: wordText, StartMS: wordStart, EndMS: wordStart + wordDuration})
			line.Text += wordText
		}
	}
	return line, len(line.Words) > 0
}

func parseJSONYRC(raw string) (Line, bool) {
	var payload struct {
		Time    int64 `json:"t"`
		Content []struct {
			Text   string  `json:"tx"`
			Timing []int64 `json:"tr"`
		} `json:"c"`
	}
	if json.Unmarshal([]byte(raw), &payload) != nil || len(payload.Content) == 0 {
		return Line{}, false
	}
	line := Line{StartMS: payload.Time}
	cursor := payload.Time
	for _, item := range payload.Content {
		start, end := cursor, cursor+500
		if len(item.Timing) >= 2 {
			start = payload.Time + item.Timing[0]
			end = start + item.Timing[1]
		}
		line.Words = append(line.Words, Word{Text: item.Text, StartMS: start, EndMS: end})
		line.Text += item.Text
		cursor = end
	}
	line.EndMS = cursor
	return line, true
}

func LinesFromLRC(items []TimedText) []Line {
	lines := make([]Line, len(items))
	for i, item := range items {
		lines[i] = Line{StartMS: item.StartMS, Text: item.Text}
		if i > 0 {
			lines[i-1].EndMS = item.StartMS
		}
	}
	return lines
}
