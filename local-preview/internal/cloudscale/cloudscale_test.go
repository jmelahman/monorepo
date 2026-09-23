package cloudscale

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
)

// fakeFleet is a scripted Fleet.
type fakeFleet struct {
	demand  int64
	loadPct float64
	workers int
	busy    map[string]bool
	idle    int
}

func (f *fakeFleet) DrainDemand() int64              { return f.demand }
func (f *fakeFleet) LoadSample() (float64, int)      { return f.loadPct, f.workers }
func (f *fakeFleet) BusyByInstance() map[string]bool { return f.busy }
func (f *fakeFleet) IdleWorkers() int                { return f.idle }

type fakeCW struct {
	puts [][]cwtypes.MetricDatum
}

func (c *fakeCW) PutMetricData(ctx context.Context, in *cloudwatch.PutMetricDataInput, _ ...func(*cloudwatch.Options)) (*cloudwatch.PutMetricDataOutput, error) {
	c.puts = append(c.puts, in.MetricData)
	return &cloudwatch.PutMetricDataOutput{}, nil
}

type protectCall struct {
	ids []string
	on  bool
}
type fakeAS struct {
	calls []protectCall
}

func (a *fakeAS) SetInstanceProtection(ctx context.Context, in *autoscaling.SetInstanceProtectionInput, _ ...func(*autoscaling.Options)) (*autoscaling.SetInstanceProtectionOutput, error) {
	a.calls = append(a.calls, protectCall{ids: in.InstanceIds, on: *in.ProtectedFromScaleIn})
	return &autoscaling.SetInstanceProtectionOutput{}, nil
}

func metricNames(data []cwtypes.MetricDatum) map[string]float64 {
	out := map[string]float64{}
	for _, d := range data {
		out[*d.MetricName] = *d.Value
	}
	return out
}

// At zero workers the publisher reports demand but NOT load — an empty fleet
// must not feed the scale-in alarm.
func TestPublishSuppressesLoadAtZeroWorkers(t *testing.T) {
	cw := &fakeCW{}
	p := newWith(&fakeFleet{demand: 3, workers: 0}, cw, &fakeAS{}, Config{ASGName: "asg"})
	p.publishMetrics(context.Background())

	if len(cw.puts) != 1 {
		t.Fatalf("puts = %d, want 1", len(cw.puts))
	}
	m := metricNames(cw.puts[0])
	if m["UnservedDemand"] != 3 {
		t.Fatalf("UnservedDemand = %v, want 3", m["UnservedDemand"])
	}
	if _, ok := m["FleetLoad"]; ok {
		t.Fatalf("FleetLoad published at zero workers: %v", m)
	}
}

// With workers present all three metrics go out — including IdleWorkers, the
// reclaim signal for an empty node the floor-propped load ratio can't expose.
func TestPublishReportsLoadWithWorkers(t *testing.T) {
	cw := &fakeCW{}
	p := newWith(&fakeFleet{demand: 0, loadPct: 42, workers: 2, idle: 1}, cw, &fakeAS{}, Config{ASGName: "asg"})
	p.publishMetrics(context.Background())

	m := metricNames(cw.puts[0])
	if m["UnservedDemand"] != 0 {
		t.Fatalf("UnservedDemand = %v, want 0", m["UnservedDemand"])
	}
	if m["FleetLoad"] != 42 {
		t.Fatalf("FleetLoad = %v, want 42", m["FleetLoad"])
	}
	if m["IdleWorkers"] != 1 {
		t.Fatalf("IdleWorkers = %v, want 1", m["IdleWorkers"])
	}
}

// IdleWorkers rides FleetLoad's zero-workers suppression: an empty fleet has
// nothing to reclaim, and its alarm must see missing data, not a zero.
func TestPublishSuppressesIdleWorkersAtZeroWorkers(t *testing.T) {
	cw := &fakeCW{}
	p := newWith(&fakeFleet{workers: 0, idle: 0}, cw, &fakeAS{}, Config{ASGName: "asg"})
	p.publishMetrics(context.Background())

	if _, ok := metricNames(cw.puts[0])["IdleWorkers"]; ok {
		t.Fatalf("IdleWorkers published at zero workers: %v", cw.puts[0])
	}
}

// A busy instance is protected, an idle one released, and a steady state issues
// no further calls.
func TestReconcileProtection(t *testing.T) {
	as := &fakeAS{}
	f := &fakeFleet{busy: map[string]bool{"i-busy": true, "i-idle": false}}
	p := newWith(f, &fakeCW{}, as, Config{ASGName: "asg"})

	p.reconcileProtection(context.Background())
	if len(as.calls) == 0 {
		t.Fatal("no protection calls on first reconcile")
	}
	// Verify the busy instance was protected and the idle one released.
	var sawProtect, sawRelease bool
	for _, c := range as.calls {
		for _, id := range c.ids {
			if id == "i-busy" && c.on {
				sawProtect = true
			}
			if id == "i-idle" && !c.on {
				sawRelease = true
			}
		}
	}
	if !sawProtect || !sawRelease {
		t.Fatalf("protect=%v release=%v, want both true (calls=%+v)", sawProtect, sawRelease, as.calls)
	}

	// Steady state: same busy map, no new calls.
	before := len(as.calls)
	p.reconcileProtection(context.Background())
	if len(as.calls) != before {
		t.Fatalf("steady state issued %d extra calls", len(as.calls)-before)
	}

	// Transition: idle instance becomes busy → exactly one more call.
	f.busy["i-idle"] = true
	p.reconcileProtection(context.Background())
	if len(as.calls) != before+1 {
		t.Fatalf("transition issued %d calls, want 1", len(as.calls)-before)
	}
	last := as.calls[len(as.calls)-1]
	if !last.on || len(last.ids) != 1 || last.ids[0] != "i-idle" {
		t.Fatalf("transition call = %+v, want protect [i-idle]", last)
	}
}

// An instance that leaves the fleet is dropped from the cache (no unprotect
// attempt on a vanished instance).
func TestReconcileForgetsDepartedInstance(t *testing.T) {
	as := &fakeAS{}
	f := &fakeFleet{busy: map[string]bool{"i-1": true}}
	p := newWith(f, &fakeCW{}, as, Config{ASGName: "asg"})
	p.reconcileProtection(context.Background())

	f.busy = map[string]bool{} // i-1 gone
	before := len(as.calls)
	p.reconcileProtection(context.Background())
	if len(as.calls) != before {
		t.Fatalf("issued a call for a departed instance")
	}
	if _, ok := p.protected["i-1"]; ok {
		t.Fatal("departed instance still cached")
	}
}
