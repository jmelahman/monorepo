// Package push delivers Web Push notifications about session state to the
// browsers that subscribed from the app's settings.
package push

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unicode/utf8"

	webpush "github.com/SherClockHolmes/webpush-go"

	"github.com/jmelahman/kanban/internal/db"
)

// Notification kinds a device can opt into. They are stored in
// push_subscriptions.events and sent as the payload's "event".
const (
	// EventAwaitingPerm: the agent is blocked on a permission prompt.
	EventAwaitingPerm = "awaiting_perm"
	// EventFinished: the agent went from working to idle.
	EventFinished = "finished"
	// EventError: the session failed to start, or its container died.
	EventError = "error"
)

// Events lists every kind, in the order the settings UI shows them.
var Events = []string{EventAwaitingPerm, EventFinished, EventError}

// ValidEvent reports whether e is a known notification kind.
func ValidEvent(e string) bool { return slices.Contains(Events, e) }

// vapidSubject identifies the sender to the push service. It has to be a
// real https: or mailto: URL: Apple's push service rejects placeholder
// subjects such as mailto:…@localhost with BadJwtToken.
const vapidSubject = "https://jamison.lahman.dev/agentic-kanban/"

// ttlSeconds is how long the push service holds a message for an offline
// device. A prompt that sat unanswered for hours is still worth showing.
const ttlSeconds = 12 * 60 * 60

// sendTimeout bounds one delivery to one device. Deliveries run side by side,
// so a push service that hangs only costs its own devices.
const sendTimeout = 15 * time.Second

// queueSize is how many session notifications may wait behind the one being
// delivered before new ones are dropped.
const queueSize = 64

// Web Push carries one encrypted record of about 4 KB, and the library
// rejects anything larger. Ticket titles can be pasted paragraphs, so the
// text is cut well short of that; a notification shows a line or two anyway.
const (
	maxTitleRunes = 120
	maxBoardRunes = 60
)

// Payload is the JSON the service worker receives (web/public/sw.js).
type Payload struct {
	Event    string `json:"event"`
	Title    string `json:"title"`
	Body     string `json:"body"`
	Tag      string `json:"tag"`
	BoardID  int64  `json:"board_id,omitempty"`
	TicketID int64  `json:"ticket_id,omitempty"`
}

// Service sends notifications to the stored subscriptions.
type Service struct {
	store  *db.Store
	client webpush.HTTPClient

	// queue feeds one worker so a session's notifications go out in the
	// order they happened; a newer one replaces the older on the device.
	queue     chan sessionEvent
	startOnce sync.Once
}

type sessionEvent struct {
	sessionID int64
	event     string
}

func New(store *db.Store) *Service {
	return &Service{store: store, client: newClient(), queue: make(chan sessionEvent, queueSize)}
}

// newClient builds the client used to reach push services. Endpoints come
// from whoever can call the API, so the server must not be talked into
// POSTing at its own network: connections to anything but a public address
// are refused at dial time (after DNS, so a name can't smuggle one in), and
// redirects are not followed. With an HTTPS proxy configured the proxy does
// the dialling, so only the redirect rule applies.
func newClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if os.Getenv("HTTPS_PROXY") == "" && os.Getenv("https_proxy") == "" {
		transport.Proxy = nil
		transport.DialContext = (&net.Dialer{
			Timeout: sendTimeout,
			Control: func(_, address string, _ syscall.RawConn) error {
				ap, err := netip.ParseAddrPort(address)
				if err != nil {
					return err
				}
				if !publicAddr(ap.Addr()) {
					return fmt.Errorf("push endpoint resolves to non-public address %s", ap.Addr())
				}
				return nil
			},
		}).DialContext
	}
	return &http.Client{
		Timeout:   sendTimeout,
		Transport: transport,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// cgnat is the shared address space (RFC 6598), which tailnets also use.
var cgnat = netip.MustParsePrefix("100.64.0.0/10")

// publicAddr reports whether a is an address a real push service could have.
func publicAddr(a netip.Addr) bool {
	a = a.Unmap()
	return a.IsGlobalUnicast() && !a.IsPrivate() && !a.IsLoopback() &&
		!a.IsLinkLocalUnicast() && !cgnat.Contains(a)
}

// SetHTTPClient replaces the client used to reach push services. Tests point
// it at an httptest server.
func (s *Service) SetHTTPClient(c webpush.HTTPClient) { s.client = c }

// keys returns the VAPID keypair, generating and storing it on first use.
func (s *Service) keys(ctx context.Context) (private, public string, err error) {
	private, public, err = s.store.VAPIDKeys(ctx)
	if err == nil {
		return private, public, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", "", err
	}
	private, public, err = webpush.GenerateVAPIDKeys()
	if err != nil {
		return "", "", err
	}
	if err := s.store.SetVAPIDKeysIfUnset(ctx, private, public); err != nil {
		return "", "", err
	}
	// Re-read rather than return what we generated: a concurrent first use
	// may have won the insert, and every caller must see the same key.
	return s.store.VAPIDKeys(ctx)
}

// PublicKey is the VAPID public key browsers pass to pushManager.subscribe.
func (s *Service) PublicKey(ctx context.Context) (string, error) {
	_, public, err := s.keys(ctx)
	return public, err
}

// Send delivers p to every subscription opted into p.Event and returns how
// many deliveries the push services accepted. A subscription the push
// service reports as gone (404/410) is deleted.
func (s *Service) Send(ctx context.Context, p Payload) (int, error) {
	subs, err := s.store.ListPushSubscriptions(ctx)
	if err != nil {
		return 0, err
	}
	targets := subs[:0]
	for _, sub := range subs {
		if slices.Contains(sub.Events, p.Event) {
			targets = append(targets, sub)
		}
	}
	return s.deliver(ctx, p, targets)
}

// ErrGone means the push service no longer knows the subscription: the
// browser unsubscribed, it expired, or it was made against a different VAPID
// key. The stored row has been deleted; the browser has to subscribe again.
var ErrGone = errors.New("push service no longer recognises this subscription")

// SendTo delivers p to one subscription regardless of its event opt-ins.
// A refusal is returned as an error that says what the push service answered.
func (s *Service) SendTo(ctx context.Context, endpoint string, p Payload) error {
	sub, err := s.store.GetPushSubscription(ctx, endpoint)
	if err != nil {
		return err
	}
	opts, body, err := s.prepare(ctx, p)
	if err != nil {
		return err
	}
	return s.deliverOne(ctx, body, *sub, opts)
}

func (s *Service) deliver(ctx context.Context, p Payload, subs []db.PushSubscription) (int, error) {
	if len(subs) == 0 {
		return 0, nil
	}
	opts, body, err := s.prepare(ctx, p)
	if err != nil {
		return 0, err
	}
	var sent atomic.Int32
	var wg sync.WaitGroup
	for _, sub := range subs {
		wg.Go(func() {
			if err := s.deliverOne(ctx, body, sub, opts); err != nil {
				log.Printf("push: %v", err)
				return
			}
			sent.Add(1)
		})
	}
	wg.Wait()
	return int(sent.Load()), nil
}

func (s *Service) prepare(ctx context.Context, p Payload) (*webpush.Options, []byte, error) {
	private, public, err := s.keys(ctx)
	if err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(p)
	if err != nil {
		return nil, nil, err
	}
	return &webpush.Options{
		HTTPClient:      s.client,
		Subscriber:      vapidSubject,
		VAPIDPublicKey:  public,
		VAPIDPrivateKey: private,
		TTL:             ttlSeconds,
		Urgency:         webpush.UrgencyHigh,
	}, body, nil
}

// deliverOne sends body to one subscription under its own deadline. The
// error names the push service's host and what it answered, never the full
// endpoint (see endpointHost).
func (s *Service) deliverOne(ctx context.Context, body []byte, sub db.PushSubscription, opts *webpush.Options) error {
	ctx, cancel := context.WithTimeout(ctx, sendTimeout)
	defer cancel()
	host := endpointHost(sub.Endpoint)
	resp, err := webpush.SendNotificationWithContext(ctx, body, &webpush.Subscription{
		Endpoint: sub.Endpoint,
		Keys:     webpush.Keys{P256dh: sub.P256dh, Auth: sub.Auth},
	}, opts)
	if err != nil {
		return fmt.Errorf("send to %s: %w", host, err)
	}
	// Push services explain a refusal in a short body (a VAPID key mismatch,
	// a bad token); it is the only clue to why nothing arrived.
	detail, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	resp.Body.Close()
	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return nil
	case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone:
		// It will never accept another message, so stop sending to it.
		if err := s.store.DeletePushSubscription(ctx, sub.Endpoint); err != nil {
			log.Printf("push: prune %s: %v", host, err)
		}
		return fmt.Errorf("%w: %s answered %d %s", ErrGone, host, resp.StatusCode, oneLine(detail))
	default:
		return fmt.Errorf("%s answered %d %s", host, resp.StatusCode, oneLine(detail))
	}
}

// oneLine makes a push service's response body safe to log and show.
func oneLine(b []byte) string {
	return truncate(strings.Join(strings.Fields(strings.ToValidUTF8(string(b), "")), " "), 200)
}

// NotifySession sends event for a session, describing it by its ticket and
// board. Delivery happens in the background, in the order of the calls, and
// never blocks or fails the caller: a notification is best-effort.
func (s *Service) NotifySession(sessionID int64, event string) {
	s.startOnce.Do(func() { go s.run() })
	select {
	case s.queue <- sessionEvent{sessionID, event}:
	default:
		log.Printf("push: session %d %s: dropped, %d notifications already waiting", sessionID, event, queueSize)
	}
}

func (s *Service) run() {
	for ev := range s.queue {
		ctx, cancel := context.WithTimeout(context.Background(), 2*sendTimeout)
		if _, err := s.sendSession(ctx, ev.sessionID, ev.event); err != nil {
			log.Printf("push: session %d %s: %v", ev.sessionID, ev.event, err)
		}
		cancel()
	}
}

func (s *Service) sendSession(ctx context.Context, sessionID int64, event string) (int, error) {
	sess, err := s.store.GetSession(ctx, sessionID)
	if err != nil {
		return 0, err
	}
	t, err := s.store.GetTicket(ctx, sess.TicketID)
	if err != nil {
		return 0, err
	}
	p := Payload{
		Event:    event,
		Title:    truncate(t.Title, maxTitleRunes),
		Body:     eventText(event),
		Tag:      fmt.Sprintf("session-%d", sessionID),
		BoardID:  t.BoardID,
		TicketID: t.ID,
	}
	if b, err := s.store.GetBoard(ctx, t.BoardID); err == nil && b != nil {
		p.Body = fmt.Sprintf("%s · %s", p.Body, truncate(b.Name, maxBoardRunes))
	}
	return s.Send(ctx, p)
}

// truncate cuts s to at most n runes, marking the cut with an ellipsis.
func truncate(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n-1]) + "…"
}

func eventText(event string) string {
	switch event {
	case EventAwaitingPerm:
		return "Waiting for permission"
	case EventFinished:
		return "Finished"
	case EventError:
		return "Session stopped unexpectedly"
	}
	return event
}

// endpointHost is the push service's host, for logs: the full endpoint URL
// is a capability (anyone holding it can address the device).
func endpointHost(endpoint string) string {
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" {
		return "(invalid endpoint)"
	}
	return u.Host
}
