package server

import "testing"

func TestDeriveAdvertise(t *testing.T) {
	t.Run("explicit --worker-advertise wins", func(t *testing.T) {
		got, err := deriveAdvertise(serveOptions{
			workerAdvertise: "http://10.0.1.5:9100",
			workerListen:    ":9100",
		})
		if err != nil || got != "http://10.0.1.5:9100" {
			t.Fatalf("got %q, err %v", got, err)
		}
	})

	t.Run("derived from host-qualified --worker-listen", func(t *testing.T) {
		got, err := deriveAdvertise(serveOptions{workerListen: "10.0.1.6:9100"})
		if err != nil || got != "http://10.0.1.6:9100" {
			t.Fatalf("got %q, err %v", got, err)
		}
	})

	// A host-less or loopback/wildcard listen can't be dialed by the control
	// node, so without an explicit advertise it must error rather than register
	// an unreachable endpoint.
	for _, listen := range []string{":9100", "0.0.0.0:9100", "127.0.0.1:9100", "localhost:9100"} {
		t.Run("unroutable listen errors: "+listen, func(t *testing.T) {
			if _, err := deriveAdvertise(serveOptions{workerListen: listen}); err == nil {
				t.Fatalf("want error for --worker-listen %q", listen)
			}
		})
	}
}
