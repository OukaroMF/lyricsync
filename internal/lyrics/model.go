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

type Data struct {
	Original   []Line
	YRC        []Line
	Translated []TimedText
	Romanized  []TimedText
}

type Line struct {
	StartMS int64
	EndMS   int64
	Text    string
	Words   []Word
}

type Word struct {
	Text    string
	StartMS int64
	EndMS   int64
}

type TimedText struct {
	StartMS int64
	Text    string
}

func (d Data) ActiveLine(positionMS int64) (Line, bool) {
	lines := d.YRC
	if len(lines) == 0 {
		lines = d.Original
	}
	idx := activeIndex(lines, positionMS)
	if idx < 0 {
		return Line{}, false
	}
	return lines[idx], true
}

func activeIndex(lines []Line, positionMS int64) int {
	lo, hi := 0, len(lines)
	for lo < hi {
		mid := lo + (hi-lo)/2
		if lines[mid].StartMS <= positionMS {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	return lo - 1
}

func Closest(items []TimedText, timestampMS int64) string {
	if len(items) == 0 {
		return ""
	}
	best := ""
	bestDelta := int64(601)
	for _, item := range items {
		delta := item.StartMS - timestampMS
		if delta < 0 {
			delta = -delta
		}
		if delta < bestDelta {
			best, bestDelta = item.Text, delta
		}
	}
	return best
}
