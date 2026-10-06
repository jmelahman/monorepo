package api

import (
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"

	"github.com/jmelahman/kanban/internal/db"
	"github.com/jmelahman/kanban/internal/push"
)

func (h *handlers) pushVAPIDKey(w http.ResponseWriter, r *http.Request) {
	key, err := h.push.PublicKey(r.Context())
	if err != nil {
		h.httpError(w, err, 500)
		return
	}
	writeJSON(w, 200, map[string]string{"public_key": key})
}

// pushSubscriptionReq mirrors PushSubscription.toJSON() from the browser,
// plus the notification kinds this device wants.
type pushSubscriptionReq struct {
	Endpoint string `json:"endpoint"`
	Keys     struct {
		P256dh string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
	Events []string `json:"events"`
}

type pushEndpointReq struct {
	Endpoint string `json:"endpoint"`
}

// Bounds on what a caller may store. Real endpoints are a few hundred bytes
// and one person has a handful of devices; every stored subscription is an
// outbound request per notification.
const (
	maxPushEndpointLen   = 2048
	maxPushSubscriptions = 50
)

// validPushEndpoint rejects anything but an https URL. The server POSTs to
// whatever is stored here, and every real push service is https. Where it
// may connect is enforced when sending (push.newClient), after DNS.
func validPushEndpoint(endpoint string) error {
	if len(endpoint) > maxPushEndpointLen {
		return fmt.Errorf("endpoint is too long")
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return fmt.Errorf("endpoint must be an https URL")
	}
	return nil
}

// validPushKey checks that key is base64 for exactly size bytes: 65 for the
// browser's P-256 public key, 16 for its auth secret.
func validPushKey(key string, size int) bool {
	if len(key) > 4*size {
		return false
	}
	for _, enc := range []*base64.Encoding{base64.RawURLEncoding, base64.URLEncoding, base64.RawStdEncoding, base64.StdEncoding} {
		if b, err := enc.DecodeString(key); err == nil {
			return len(b) == size
		}
	}
	return false
}

func (h *handlers) putPushSubscription(w http.ResponseWriter, r *http.Request) {
	req, err := decodeBody[pushSubscriptionReq](r)
	if err != nil {
		h.httpError(w, err, 400)
		return
	}
	if err := validPushEndpoint(req.Endpoint); err != nil {
		h.httpError(w, err, 400)
		return
	}
	if !validPushKey(req.Keys.P256dh, 65) || !validPushKey(req.Keys.Auth, 16) {
		h.httpError(w, fmt.Errorf("keys.p256dh and keys.auth must be the browser's subscription keys"), 400)
		return
	}
	events := []string{}
	for _, e := range req.Events {
		if !push.ValidEvent(e) {
			h.httpError(w, fmt.Errorf("unknown event %q", e), 400)
			return
		}
		events = append(events, e)
	}
	sub := db.PushSubscription{
		Endpoint:  req.Endpoint,
		P256dh:    req.Keys.P256dh,
		Auth:      req.Keys.Auth,
		Events:    events,
		UserAgent: r.UserAgent(),
	}
	existing, err := h.store.ListPushSubscriptions(r.Context())
	if err != nil {
		h.httpError(w, err, 500)
		return
	}
	if len(existing) >= maxPushSubscriptions &&
		!slices.ContainsFunc(existing, func(s db.PushSubscription) bool { return s.Endpoint == sub.Endpoint }) {
		h.httpError(w, fmt.Errorf("too many subscribed devices (limit %d); turn notifications off on one first", maxPushSubscriptions), http.StatusConflict)
		return
	}
	if err := h.store.UpsertPushSubscription(r.Context(), sub); err != nil {
		h.httpError(w, err, 500)
		return
	}
	writeJSON(w, 200, map[string]any{"endpoint": sub.Endpoint, "events": sub.Events})
}

func (h *handlers) deletePushSubscription(w http.ResponseWriter, r *http.Request) {
	req, err := decodeBody[pushEndpointReq](r)
	if err != nil || req.Endpoint == "" {
		h.httpError(w, fmt.Errorf("endpoint is required"), 400)
		return
	}
	if err := h.store.DeletePushSubscription(r.Context(), req.Endpoint); err != nil {
		h.httpError(w, err, 500)
		return
	}
	w.WriteHeader(204)
}

// testPush sends a sample notification to the calling device only, so the
// settings button can't buzz every other subscribed phone.
func (h *handlers) testPush(w http.ResponseWriter, r *http.Request) {
	req, err := decodeBody[pushEndpointReq](r)
	if err != nil || req.Endpoint == "" {
		h.httpError(w, fmt.Errorf("endpoint is required"), 400)
		return
	}
	err = h.push.SendTo(r.Context(), req.Endpoint, push.Payload{
		Event: "test",
		Title: "Kanban notifications are on",
		Body:  "This device will be told when a session needs you.",
		Tag:   "test",
	})
	switch {
	case errors.Is(err, sql.ErrNoRows):
		h.httpError(w, fmt.Errorf("this device is not subscribed"), 404)
	case err != nil:
		h.httpError(w, err, http.StatusBadGateway)
	default:
		w.WriteHeader(204)
	}
}
