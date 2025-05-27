package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestSoundURLsAreValid(t *testing.T) {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	for _, sound := range sounds {
		t.Run(sound.name, func(t *testing.T) {
			// First validate URL format
			_, err := url.ParseRequestURI(sound.url)
			if err != nil {
				t.Errorf("Invalid URL format for %q: %v", sound.name, err)
				return
			}

			// Check the URL is reachable
			resp, err := client.Head(sound.url)
			if err != nil {
				t.Errorf("Failed to HEAD %q: %v", sound.name, err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("Got non-200 status for %q: %d", sound.name, resp.StatusCode)
			}
		})
	}
}
