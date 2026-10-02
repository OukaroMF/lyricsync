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
	"testing"
)

func TestServeRejectsInvalidOriginBeforeReading(t *testing.T) {
	for _, args := range [][]string{nil, {"https://evil.test"}, {"chrome-extension://invalid/"}, {"chrome-extension://aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/", "extra"}} {
		var output bytes.Buffer
		if err := Serve(context.Background(), args, bytes.NewReader(nil), &output); err == nil {
			t.Fatal("accepted invalid caller")
		}
		if output.Len() != 0 {
			t.Fatal("invalid caller produced protocol output")
		}
	}
}
