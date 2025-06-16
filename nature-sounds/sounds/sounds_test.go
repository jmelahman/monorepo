package sounds

import (
	"net/http"
	"net/url"
	"testing"
	"time"
)

func TestSoundURLsAreValid(t *testing.T) {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	for _, sound := range Sounds {
		t.Run(sound.Name, func(t *testing.T) {
			// First validate URL format
			_, err := url.ParseRequestURI(sound.Url)
			if err != nil {
				t.Errorf("Invalid URL format for %q: %v", sound.Name, err)
				return
			}

			// Check the URL is reachable
			resp, err := client.Head(sound.Url)
			if err != nil {
				t.Errorf("Failed to HEAD %q: %v", sound.Name, err)
				return
			}
			defer func() {
				if err := resp.Body.Close(); err != nil {
					t.Logf("Error closing response body: %v", err)
				}
			}()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("Got non-200 status for %q: %d", sound.Name, resp.StatusCode)
			}
		})
	}
}
