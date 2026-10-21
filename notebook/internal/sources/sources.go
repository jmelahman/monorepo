// Package sources turns a web page into the readable text of a source.
package sources

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"path"
	"regexp"
	"strings"
	"syscall"
	"time"

	"codeberg.org/readeck/go-readability/v2"
)

// MaxBytes is the most of a page that is read.
const MaxBytes = 10 << 20

// Source is a fetched page.
type Source struct {
	URL      string `json:"url"`
	Title    string `json:"title"`
	Byline   string `json:"byline,omitempty"`
	SiteName string `json:"siteName,omitempty"`
	Text     string `json:"text"`
}

// Client fetches pages; the zero value uses a client with a timeout.
type Client struct {
	HTTP *http.Client
}

func (c *Client) http() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.Proxy = nil // the dialer checks where pages are, not where a proxy is
	t.DialContext = (&net.Dialer{Timeout: 10 * time.Second, Control: public}).DialContext
	return &http.Client{Timeout: 30 * time.Second, Transport: t}
}

// public keeps fetches off this computer and its cloud metadata service:
// addresses come from Claude's searches as well as from the user, and the
// page's text goes back to Claude. Pages on the local network are allowed,
// for a homelab's own.
func public(network, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return err
	}
	ip = ip.Unmap()
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast() {
		return fmt.Errorf("won't fetch from %s, on this computer or its link", ip)
	}
	return nil
}

// Fetch gets rawURL and extracts its readable text: the article of an HTML
// page, or the whole of a plain text or Markdown one.
func (c *Client) Fetch(ctx context.Context, rawURL string) (Source, error) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return Source{}, fmt.Errorf("not a web address: %q", rawURL)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return Source{}, err
	}
	// Some sites turn away clients that don't look like a browser.
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,text/plain,text/markdown;q=0.9,*/*;q=0.5")
	resp, err := c.http().Do(req)
	if err != nil {
		return Source{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Source{}, fmt.Errorf("%s: %s", u, resp.Status)
	}
	final := resp.Request.URL
	body := io.LimitReader(resp.Body, MaxBytes)
	mt, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	switch mt {
	case "text/plain", "text/markdown", "text/x-markdown":
		b, err := io.ReadAll(body)
		if err != nil {
			return Source{}, err
		}
		text := strings.TrimSpace(string(b))
		return Source{URL: final.String(), Title: markdownTitle(text, final), Text: text}, nil
	case "", "text/html", "application/xhtml+xml":
		article, err := readability.FromReader(body, final)
		if err != nil {
			return Source{}, fmt.Errorf("couldn't read %s: %w", final, err)
		}
		var b strings.Builder
		if err := article.RenderText(&b); err != nil {
			return Source{}, err
		}
		text := tidy(b.String())
		if text == "" {
			return Source{}, errors.New("found no readable text at " + final.String())
		}
		return Source{
			URL:      final.String(),
			Title:    cmp.Or(strings.TrimSpace(article.Title()), final.Host),
			Byline:   strings.TrimSpace(article.Byline()),
			SiteName: strings.TrimSpace(article.SiteName()),
			Text:     text,
		}, nil
	}
	return Source{}, fmt.Errorf("%s is %s, which can't be read as text", final, mt)
}

var blankLines = regexp.MustCompile(`\n[ \t]*(\n[ \t]*)+`)

// tidy trims each line and collapses runs of blank lines into one.
func tidy(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimSpace(l)
	}
	return strings.TrimSpace(blankLines.ReplaceAllString(strings.Join(lines, "\n"), "\n\n"))
}

// markdownTitle is a document's first heading, or else the file's name.
func markdownTitle(text string, u *url.URL) string {
	for line := range strings.SplitSeq(text, "\n") {
		if t, ok := strings.CutPrefix(line, "# "); ok {
			return strings.TrimSpace(t)
		}
	}
	if name := path.Base(u.Path); name != "/" && name != "." {
		return name
	}
	return u.Host
}
