// Package workerapi is the internal transport between a control node and an
// elastic worker. It is the *second transport* for the one orchestrator
// implementation: a worker runs the same supervise.Manager, and the control
// node reaches it through Client, which satisfies the same proxy.Backends
// interface the local loopback adapter does. Single-node (--role=all) is the
// degenerate case where no Client exists and serving stays loopback.
//
// The surface is deliberately tiny — exactly what the control node needs
// against a supervise.Key: start/reuse a process (Ensure), stop it, read its
// status, and a capacity heartbeat. It is a remote-code-execution surface by
// design (the worker starts arbitrary preview processes), so it is
// shared-secret authenticated and MUST live on a private listener that is never
// reachable from the ALB/internet.
package workerapi

import (
	"time"

	"github.com/jmelahman/local-preview/internal/supervise"
)

// AuthHeader carries the shared secret ("Bearer <secret>"). The secret comes
// from SSM/config on both ends.
const AuthHeader = "Authorization"

// Route paths, versioned so the wire format can evolve. The /worker/ paths are
// served on the worker (control dials in); the /control/ paths are served on
// the control node (a self-registering worker dials in) — opposite directions,
// same shared secret and same private-listener-only rule.
const (
	pathEnsure    = "/internal/worker/v1/ensure"
	pathStop      = "/internal/worker/v1/stop"
	pathStatus    = "/internal/worker/v1/status"
	pathHeartbeat = "/internal/worker/v1/heartbeat"
	pathDrain     = "/internal/worker/v1/drain"
	pathReport    = "/internal/worker/v1/report"
	pathRunLog    = "/internal/worker/v1/runlog"
	pathConfigure = "/internal/worker/v1/configure"
	// pathExec upgrades to a WebSocket carrying execstream frames — the
	// control node forwards a `preview exec` session to the worker holding
	// the process. Session parameters ride the query string (an upgrade
	// request has no usable body).
	pathExec = "/internal/worker/v1/exec"

	pathRegister   = "/internal/control/v1/register"
	pathDeregister = "/internal/control/v1/deregister"
)

// registerReq is a worker announcing itself to the control node. Endpoint is the
// worker's own worker-API base URL (what the control node will dial back to
// place work); Host is the routable host its preview processes serve on. These
// are exactly what the static --worker-endpoint/--worker-host flags carry for a
// hand-wired fleet — a self-registering worker supplies them itself, so an
// autoscaled node the control node was never configured with can still join.
type registerReq struct {
	Endpoint string `json:"endpoint"`
	Host     string `json:"host"`
	// InstanceID is the worker's cloud instance identifier (EC2 instance-id),
	// when it knows one. The control node uses it to set scale-in protection on
	// busy workers so an autoscaling group drains idle nodes rather than
	// terminating one mid-preview. Empty for a hand-wired or non-cloud worker.
	InstanceID string `json:"instance_id,omitempty"`
}

// deregisterReq is a worker leaving the fleet (graceful shutdown / the
// pre-terminate half of an ASG scale-in). Best-effort: an ungraceful exit just
// lets the worker go stale and drop out of placement.
type deregisterReq struct {
	Endpoint string `json:"endpoint"`
}

// WireKey is the JSON form of a supervise.Key crossing the boundary.
type WireKey struct {
	RepoID int64  `json:"repo_id"`
	Side   string `json:"side"`
	Hash   string `json:"hash"`
	Peer   string `json:"peer,omitempty"`
}

func fromKey(k supervise.Key) WireKey {
	return WireKey{RepoID: k.RepoID, Side: string(k.Side), Hash: k.Hash, Peer: k.Peer}
}

func (w WireKey) toKey() supervise.Key {
	return supervise.Key{RepoID: w.RepoID, Side: supervise.Side(w.Side), Hash: w.Hash, Peer: w.Peer}
}

// ensureReq/ensureResp: start or reuse a process, returning the port it serves
// on (loopback on the worker; the control-side Client pairs it with the
// worker's routable host).
//
// Spec carries the control-resolved run contract for Key — a worker's DB has
// no artifact rows, so without it every ensure fails "artifact not
// provisioned". PeerSpec is the same for a process-mode frontend's co-placed
// backend, which the worker's own Manager starts to satisfy {backend_url}.
// Both are optional additions to the v1 shape: an old control node omits them
// (the worker fails exactly as before), and an old worker ignores them.
type ensureReq struct {
	Key      WireKey             `json:"key"`
	Repo     string              `json:"repo"`
	Spec     *supervise.WireSpec `json:"spec,omitempty"`
	PeerSpec *supervise.WireSpec `json:"peer_spec,omitempty"`
}
type ensureResp struct {
	Port int `json:"port"`
}

type stopReq struct {
	Key    WireKey `json:"key"`
	Reason string  `json:"reason"`
}

type statusReq struct {
	Key WireKey `json:"key"`
}
type statusResp struct {
	Status string `json:"status"`
	// Error carries the failure detail behind a "crashed" status.
	Error string `json:"error,omitempty"`
}

// report: the worker's full live-state dump — every non-idle key's status,
// failure detail, and resource sample — so the control node renders the
// dashboard from one round trip per worker instead of one per field per row.
type reportResp struct {
	Procs []wireProc `json:"procs"`
}
type wireProc struct {
	Key    WireKey                 `json:"key"`
	Repo   string                  `json:"repo,omitempty"`
	Status string                  `json:"status"`
	Error  string                  `json:"error,omitempty"`
	Stats  *supervise.ProcessStats `json:"stats,omitempty"`
	// LastTouch feeds the control node's fleet-wide min-warm ranking. An
	// optional v1 addition: a worker that predates it omits the field and its
	// processes rank least-recent.
	LastTouch time.Time `json:"last_touch,omitzero"`
}

// runlog: an incremental slice of a run log on the worker's disk. Logs are
// addressed by (repo, side, hash) — the artifact identity — not by key,
// because they outlive the process that wrote them.
type runLogReq struct {
	Repo    string `json:"repo"`
	Side    string `json:"side"`
	Hash    string `json:"hash"`
	Attempt int    `json:"attempt"`
	Offset  int64  `json:"offset"`
}

type drainReq struct {
	Draining bool `json:"draining"`
}

// WorkerConfig is the control-pushed runtime settings bundle. Fields are
// pointers so a push only touches what it names; a rebooted worker falls back
// to its boot flags until the control node's reconcile loop pushes again.
type WorkerConfig struct {
	// MaxWarm is the soft warm target; MinWarm the floor of most-recent
	// processes exempt from idle timeouts.
	MaxWarm *int `json:"max_warm,omitempty"`
	MinWarm *int `json:"min_warm,omitempty"`
	// IdleTimeoutSeconds > 0 overrides every manifest's idle_timeout; 0
	// restores per-manifest values.
	IdleTimeoutSeconds *int `json:"idle_timeout_seconds,omitempty"`
}

// Heartbeat is the worker's capacity report, the input to fleet placement and
// scale-out. Draining marks a worker an ASG lifecycle hook is winding down: the
// control node routes new work elsewhere but keeps serving what is already
// warm. The runtime-config fields echo what the worker currently applies, so
// the control node's reconcile loop can spot (and re-push onto) a worker that
// rebooted back to its boot flags.
type Heartbeat struct {
	Running            int  `json:"running"`
	MaxWarm            int  `json:"max_warm"`
	MinWarm            int  `json:"min_warm,omitempty"`
	IdleTimeoutSeconds int  `json:"idle_timeout_seconds,omitempty"`
	Draining           bool `json:"draining"`
	// WarmHits and ColdStarts are the worker's cumulative EnsureRunning
	// outcome counts since it started — summed across the fleet for the
	// dashboard's warm-ratio statistic. They reset when a worker reboots
	// (routine on a spot tier), so the control node's registry detects the
	// regression and banks the previous values, keeping fleet totals
	// monotonic for as long as the control node runs.
	WarmHits   int64 `json:"warm_hits"`
	ColdStarts int64 `json:"cold_starts"`
	// Events are the process events recorded on this worker since its last
	// heartbeat (drained at read — at-most-once). A worker's own database is
	// ephemeral, so the control node replays these into its process_events
	// trail, which feeds the dashboard's startup-latency percentiles.
	Events []supervise.ProcEventRecord `json:"events,omitempty"`
}
