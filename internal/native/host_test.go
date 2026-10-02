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
