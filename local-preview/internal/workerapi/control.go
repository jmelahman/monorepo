package workerapi

import (
	"net/http"
	"net/url"
)

// Registrar is the control-side sink a self-registering worker calls into. The
// implementation (in cmd/server) builds the workerapi.Client for the endpoint,
// attaches the control-DB SpecResolver, and adds it to the fleet registry —
// exactly the wiring the static --worker-endpoint boot loop does, reused so a
// dynamic worker joins by the same path a hand-listed one does.
type Registrar interface {
	// Register adds (or replaces) a worker by its advertised worker-API
	// endpoint, reaching its preview processes at host. instanceID is the
	// worker's cloud instance-id when it has one (used for scale-in
	// protection), empty otherwise.
	Register(endpoint, host, instanceID string) error
	// Deregister removes a worker that is leaving the fleet.
	Deregister(endpoint string) error
}

// ControlServer is the control node's inbound half of the worker protocol:
// workers announce themselves here instead of being hand-listed in the control
// node's flags. It shares the worker API's shared-secret trust model and MUST
// likewise live on a private listener the ALB can never reach — a caller who
// can register an endpoint can steer preview traffic to a host of its choosing
// and make the control node dial arbitrary addresses.
type ControlServer struct {
	reg    Registrar
	secret string
}

// NewControlServer wraps reg behind the shared-secret check.
func NewControlServer(reg Registrar, secret string) *ControlServer {
	return &ControlServer{reg: reg, secret: secret}
}

// Handler returns the authenticated control-API mux. Mount it on a private
// listener only.
func (s *ControlServer) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc(pathRegister, s.handleRegister)
	mux.HandleFunc(pathDeregister, s.handleDeregister)
	return bearerAuth(s.secret, mux)
}

func (s *ControlServer) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req registerReq
	if !decode(w, r, &req) {
		return
	}
	if req.Endpoint == "" {
		http.Error(w, "bad request: empty endpoint", http.StatusBadRequest)
		return
	}
	host := req.Host
	if host == "" {
		host = hostFromURL(req.Endpoint)
	}
	if host == "" {
		http.Error(w, "bad request: endpoint has no host and none given", http.StatusBadRequest)
		return
	}
	if err := s.reg.Register(req.Endpoint, host, req.InstanceID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *ControlServer) handleDeregister(w http.ResponseWriter, r *http.Request) {
	var req deregisterReq
	if !decode(w, r, &req) {
		return
	}
	if req.Endpoint == "" {
		http.Error(w, "bad request: empty endpoint", http.StatusBadRequest)
		return
	}
	// Best-effort: a worker that is already gone is not an error.
	_ = s.reg.Deregister(req.Endpoint)
	w.WriteHeader(http.StatusNoContent)
}

func hostFromURL(baseURL string) string {
	if u, err := url.Parse(baseURL); err == nil {
		return u.Hostname()
	}
	return ""
}
