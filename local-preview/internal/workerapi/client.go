package workerapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/jmelahman/local-preview/internal/execstream"
	"github.com/jmelahman/local-preview/internal/supervise"
)

// Client is the control-node transport to one worker. Its EnsureRunning
// satisfies proxy.Backends, so the proxy routes to a remote worker exactly as
// it routes to a local Manager — the "one orchestrator, two transports"
// property. host is the worker's routable address (how the control node reaches
// the worker's supervised processes); the worker returns only the port.
type Client struct {
	base   string // worker-API base URL, e.g. http://10.0.1.5:9100
	host   string // routable host for the worker's processes, e.g. 10.0.1.5
	secret string
	hc     *http.Client

	// SpecResolver resolves the control-side run spec shipped with each
	// ensure request (plus the peer backend's, for a process-mode frontend) —
	// supervise.(*Manager).ResolveWireSpec in production. Nil sends none,
	// which only serves against a worker whose own DB knows the artifact;
	// outside tests that means every ensure fails "not provisioned".
	SpecResolver func(k supervise.Key) (supervise.WireSpec, error)

	// InitMarker records a backend init proven complete on the worker —
	// supervise.(*Manager).AdoptRemoteInitDone in production. A successful
	// backend ensure implies the init steps succeeded (the worker fails the
	// ensure otherwise), so the control DB can adopt the result without any
	// wire change, old workers included. Without it, init_done_at never
	// leaves the worker's in-memory spec cache, and every cold placement on
	// a fresh worker re-runs init — a fleet resting at zero workers re-ran
	// it on nearly every wake. Nil skips the write.
	InitMarker func(k supervise.Key) error

	// InitRevoker withdraws a backend init the control DB records as done —
	// supervise.(*Manager).RevokeInitDone in production, the mirror image of
	// InitMarker. It fires when an ensure that SHIPPED InitDone=true came
	// back as a start failure the worker actually reported: the worker
	// skipped init on our say-so and the process never went healthy, so the
	// effects init owns (the per-preview database, say) may be gone. The
	// next ensure then ships InitDone=false and the worker re-runs init.
	//
	// Only a worker-reported failure counts. A transport error, a timeout,
	// or a cancelled context proves nothing — the worker may never have run
	// a thing — and revoking on those would re-run init for every network
	// blip. That distinction is why post returns a typed *HTTPError.
	// Nil skips the write.
	InitRevoker func(k supervise.Key) error
}

// NewClient dials a worker. baseURL is its private worker-API URL; host is the
// address the control node reaches its processes on; secret is the shared
// secret. A nil httpClient gets a default with a sane timeout.
func NewClient(baseURL, host, secret string, hc *http.Client) *Client {
	if hc == nil {
		hc = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{base: baseURL, host: host, secret: secret, hc: hc}
}

// EnsureRunning starts (or reuses) the keyed process on the worker and returns
// the "host:port" the control node's proxy dials.
func (c *Client) EnsureRunning(ctx context.Context, k supervise.Key, repoName string) (string, error) {
	req := ensureReq{Key: fromKey(k), Repo: repoName}
	if c.SpecResolver != nil {
		spec, err := c.SpecResolver(k)
		if err != nil {
			// Same failure a local start would report — don't bother the worker.
			return "", err
		}
		req.Spec = &spec
		if k.Side == supervise.SideFrontend && k.Peer != "" {
			peer, err := c.SpecResolver(supervise.BackendKey(k.RepoID, k.Peer))
			if err != nil {
				return "", err
			}
			req.PeerSpec = &peer
		}
	}
	var resp ensureResp
	if err := c.post(ctx, pathEnsure, req, &resp); err != nil {
		if c.InitRevoker != nil && k.Side == supervise.SideBackend &&
			req.Spec != nil && req.Spec.InitDone && isStartFailure(err) {
			if rerr := c.InitRevoker(k); rerr != nil {
				log.Printf("worker ensure: revoking init done for %s/%s: %v", repoName, k.Hash, rerr)
			}
		}
		return "", err
	}
	// Backend ensures only: a frontend ensure may or may not have started the
	// co-placed backend (only when its config references {backend_url}), so
	// nothing about the peer is proven here. The proxy ensures the backend
	// directly on every API route, which is where its init gets adopted.
	if c.InitMarker != nil && k.Side == supervise.SideBackend && req.Spec != nil && !req.Spec.InitDone {
		if err := c.InitMarker(k); err != nil {
			// The preview is serving; failing the ensure over a bookkeeping
			// write would take it down. The next un-adopted ensure retries.
			log.Printf("worker ensure: recording init done for %s/%s: %v", repoName, k.Hash, err)
		}
	}
	return c.host + ":" + strconv.Itoa(resp.Port), nil
}

// Stop asks the worker to stop the keyed process.
func (c *Client) Stop(ctx context.Context, k supervise.Key, reason string) error {
	return c.post(ctx, pathStop, stopReq{Key: fromKey(k), Reason: reason}, nil)
}

// Status reads the keyed process's status from the worker, with the failure
// detail when it is "crashed".
func (c *Client) Status(ctx context.Context, k supervise.Key) (status, detail string, err error) {
	var resp statusResp
	if err := c.post(ctx, pathStatus, statusReq{Key: fromKey(k)}, &resp); err != nil {
		return "", "", err
	}
	return resp.Status, resp.Error, nil
}

// Report fetches the worker's full live-state dump: every non-idle key's
// status, failure detail, and resource sample.
func (c *Client) Report(ctx context.Context) ([]supervise.ProcReport, error) {
	var resp reportResp
	if err := c.post(ctx, pathReport, struct{}{}, &resp); err != nil {
		return nil, err
	}
	out := make([]supervise.ProcReport, 0, len(resp.Procs))
	for _, p := range resp.Procs {
		out = append(out, supervise.ProcReport{
			Key: p.Key.toKey(), Repo: p.Repo, Status: p.Status, Error: p.Error, Stats: p.Stats,
			LastTouch: p.LastTouch,
		})
	}
	return out, nil
}

// RunLog fetches an incremental run-log slice from the worker's disk.
func (c *Client) RunLog(ctx context.Context, repo, side, hash string, attempt int, offset int64) (supervise.RunLog, error) {
	var chunk supervise.RunLog
	err := c.post(ctx, pathRunLog, runLogReq{Repo: repo, Side: side, Hash: hash, Attempt: attempt, Offset: offset}, &chunk)
	return chunk, err
}

// Exec forwards one `preview exec` session to the worker: it dials the
// worker's exec endpoint (WebSocket upgrade) and copies execstream frames
// blindly in both directions — the frames need no re-framing per hop. It
// returns when the worker ends the session (normally after FrameExit) or the
// transport fails; ctx bounds only the handshake.
func (c *Client) Exec(ctx context.Context, k supervise.Key, opts supervise.ExecOptions, stream io.ReadWriter) error {
	q := url.Values{
		"repo_id": {strconv.FormatInt(k.RepoID, 10)},
		"side":    {string(k.Side)},
		"hash":    {k.Hash},
		"peer":    {k.Peer},
		"cmd":     opts.Cmd,
	}
	if opts.TTY {
		q.Set("tty", "1")
	}
	if opts.Stdin {
		q.Set("stdin", "1")
	}
	if opts.Term != "" {
		q.Set("term", opts.Term)
	}
	hctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	hdr := http.Header{}
	hdr.Set(AuthHeader, "Bearer "+c.secret)
	conn, err := execstream.Dial(hctx, c.base+pathExec+"?"+q.Encode(), hdr)
	if err != nil {
		return err
	}
	defer conn.Close()
	// Client → worker: ends when the caller's stream closes; closing conn
	// then tears the worker side down too.
	go func() {
		defer conn.Close()
		io.Copy(conn, stream) //nolint:errcheck // either side closing ends the session
	}()
	// Worker → client: EOF here is the end of the session.
	_, err = io.Copy(stream, conn)
	return err
}

// Drain marks the worker draining (or clears it) — used by a control-node
// lifecycle handler winding a worker down before its instance terminates.
func (c *Client) Drain(ctx context.Context, draining bool) error {
	return c.post(ctx, pathDrain, drainReq{Draining: draining}, nil)
}

// Configure pushes runtime settings to the worker — the dashboard-edited
// warm cap and idle-timeout override. The control node's heartbeat loop
// re-pushes after a worker reboot, so the settings survive the fleet.
func (c *Client) Configure(ctx context.Context, cfg WorkerConfig) error {
	return c.post(ctx, pathConfigure, cfg, nil)
}

// Heartbeat reads the worker's current capacity.
func (c *Client) Heartbeat(ctx context.Context) (Heartbeat, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+pathHeartbeat, nil)
	if err != nil {
		return Heartbeat{}, err
	}
	req.Header.Set(AuthHeader, "Bearer "+c.secret)
	res, err := c.hc.Do(req)
	if err != nil {
		return Heartbeat{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return Heartbeat{}, fmt.Errorf("worker heartbeat: %s", res.Status)
	}
	var hb Heartbeat
	if err := json.NewDecoder(res.Body).Decode(&hb); err != nil {
		return Heartbeat{}, err
	}
	return hb, nil
}

// HTTPError is a response the peer actually produced: the request reached it
// and it answered non-2xx. Callers that must tell "the far side ran something
// and it failed" from "the far side was never reached" (EnsureRunning's init
// revocation) match on this; a transport error, a timeout, or a cancelled
// context is a plain error instead.
type HTTPError struct {
	Status int
	Body   string
}

func (e *HTTPError) Error() string { return e.Body }

// isStartFailure reports whether err is the worker's own "this start failed"
// answer — the 502 handleEnsure returns for a crashed or unstartable process
// (any other status is a protocol/auth problem, which says nothing about
// init's effects).
func isStartFailure(err error) bool {
	var he *HTTPError
	return errors.As(err, &he) && he.Status == http.StatusBadGateway
}

// post sends a JSON body to path and decodes an optional JSON response. A
// non-2xx carries the worker's error text so the proxy renders the real reason.
func (c *Client) post(ctx context.Context, path string, body, out any) error {
	return postJSON(ctx, c.hc, c.base+path, c.secret, body, out)
}

// postJSON POSTs body as JSON to url with the shared-secret bearer header and
// decodes an optional JSON response. A non-2xx carries the peer's error text.
// Shared by both directions of the protocol (control→worker and worker→control).
func postJSON(ctx context.Context, hc *http.Client, url, secret string, body, out any) error {
	buf, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header.Set(AuthHeader, "Bearer "+secret)
	req.Header.Set("Content-Type", "application/json")
	res, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return &HTTPError{Status: res.StatusCode, Body: string(bytes.TrimSpace(msg))}
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(res.Body).Decode(out)
}

// ControlClient is the worker-side transport to the control node's registration
// endpoint — the reverse direction of Client. A self-registering worker calls
// Register on boot and periodically (so a restarted control node re-learns it),
// and Deregister on graceful shutdown.
type ControlClient struct {
	base   string // control node's control-API base URL, e.g. http://10.0.1.1:9101
	secret string
	hc     *http.Client
}

// NewControlClient dials the control node's registration API. A nil httpClient
// gets a default with a sane timeout.
func NewControlClient(baseURL, secret string, hc *http.Client) *ControlClient {
	if hc == nil {
		hc = &http.Client{Timeout: 15 * time.Second}
	}
	return &ControlClient{base: baseURL, secret: secret, hc: hc}
}

// Register announces this worker to the control node: endpoint is this worker's
// own worker-API base URL, host the routable host its processes serve on, and
// instanceID its cloud instance-id (empty if it has none) so the control node
// can scale-in-protect it while busy.
func (c *ControlClient) Register(ctx context.Context, endpoint, host, instanceID string) error {
	return postJSON(ctx, c.hc, c.base+pathRegister, c.secret, registerReq{Endpoint: endpoint, Host: host, InstanceID: instanceID}, nil)
}

// Deregister removes this worker from the control node's fleet.
func (c *ControlClient) Deregister(ctx context.Context, endpoint string) error {
	return postJSON(ctx, c.hc, c.base+pathDeregister, c.secret, deregisterReq{Endpoint: endpoint}, nil)
}
