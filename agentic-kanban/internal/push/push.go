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

// SendTo delivers p to one subscription regardless of its event opt-ins.
func (s *Service) SendTo(ctx context.Context, endpoint string, p Payload) error {
	sub, err := s.store.GetPushSubscription(ctx, endpoint)
	if err != nil {
		return err
	}
	sent, err := s.deliver(ctx, p, []db.PushSubscription{*sub})
	if err != nil {
		return err
	}
	if sent == 0 {
		return errors.New("push service rejected the notification")
	}
	return nil
}

func (s *Service) deliver(ctx context.Context, p Payload, subs []db.PushSubscription) (int, error) {
	if len(subs) == 0 {
		return 0, nil
	}
	private, public, err := s.keys(ctx)
	if err != nil {
		return 0, err
	}
	body, err := json.Marshal(p)
	if err != nil {
		return 0, err
	}
	opts := &webpush.Options{
		HTTPClient:      s.client,
		Subscriber:      vapidSubject,
		VAPIDPublicKey:  public,
		VAPIDPrivateKey: private,
		TTL:             ttlSeconds,
		Urgency:         webpush.UrgencyHigh,
	}
	var sent atomic.Int32
	var wg sync.WaitGroup
	for _, sub := range subs {
		wg.Go(func() {
			if s.deliverOne(ctx, body, sub, opts) {
				sent.Add(1)
			}
		})
	}
	wg.Wait()
	return int(sent.Load()), nil
}

// deliverOne sends body to one subscription under its own deadline and
// reports whether the push service accepted it.
func (s *Service) deliverOne(ctx context.Context, body []byte, sub db.PushSubscription, opts *webpush.Options) bool {
	ctx, cancel := context.WithTimeout(ctx, sendTimeout)
	defer cancel()
	resp, err := webpush.SendNotificationWithContext(ctx, body, &webpush.Subscription{
		Endpoint: sub.Endpoint,
		Keys:     webpush.Keys{P256dh: sub.P256dh, Auth: sub.Auth},
	}, opts)
	if err != nil {
		log.Printf("push: send to %s: %v", endpointHost(sub.Endpoint), err)
		return false
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone:
		// The browser unsubscribed or the subscription expired; it will
		// never accept another message.
		if err := s.store.DeletePushSubscription(ctx, sub.Endpoint); err != nil {
			log.Printf("push: prune %s: %v", endpointHost(sub.Endpoint), err)
		}
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return true
	default:
		log.Printf("push: %s answered %d", endpointHost(sub.Endpoint), resp.StatusCode)
	}
	return false
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
