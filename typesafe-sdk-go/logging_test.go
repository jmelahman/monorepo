package typesafe

import (
	"net/http"
	"reflect"
	"testing"
)

func TestRedactHeaders(t *testing.T) {
	t.Parallel()

	h := http.Header{}
	h.Set("Authorization", "Bearer sk-live-abcdefgh1234")
	h.Set("X-Api-Key", "short")
	h.Set("Cookie", "session=1")
	h.Set("X-Refresh-Token", "abcd")
	h.Set("Content-Type", "application/json")

	want := map[string]string{
		"Authorization":   "Bearer ***1234",
		"X-Api-Key":       "***",
		"Cookie":          "***",
		"X-Refresh-Token": "***",
		"Content-Type":    "application/json",
	}
	if got := redactHeaders(h); !reflect.DeepEqual(got, want) {
		t.Errorf("redactHeaders() = %v, want %v", got, want)
	}
}

func TestParseLogLevel(t *testing.T) {
	t.Parallel()

	tests := map[string]LogLevel{
		"debug": LogDebug, "INFO": LogInfo, " warn ": LogWarn,
		"warning": LogWarn, "error": LogError, "off": LogOff,
	}
	for in, want := range tests {
		got, err := ParseLogLevel(in)
		if err != nil {
			t.Errorf("ParseLogLevel(%q) = %v", in, err)
		} else if got != want {
			t.Errorf("ParseLogLevel(%q) = %v, want %v", in, got, want)
		}
	}
	if _, err := ParseLogLevel("loud"); err == nil {
		t.Error("ParseLogLevel(\"loud\") = nil, want an error")
	}
}
