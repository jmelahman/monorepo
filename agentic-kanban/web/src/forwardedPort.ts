// Link to a session port the backend forwards on its host (13000–13099).
//
// The port is open on the backend machine, so the link uses the name this
// page was reached by — a LAN address or a Tailscale name works from a phone,
// where "localhost" would be the phone itself. Always plain HTTP: the
// forwarder is a raw TCP pipe with no TLS of its own.
export function forwardedPortUrl(hostPort: number): string {
  const host = window.location.hostname;
  // An IPv6 literal needs its brackets back to carry a port.
  return `http://${host.includes(":") ? `[${host}]` : host}:${hostPort}`;
}
