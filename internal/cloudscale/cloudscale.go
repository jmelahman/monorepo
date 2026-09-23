// Package cloudscale is the control node's autoscaling bridge to AWS. It turns
// the fleet's live state into the two signals an EC2 Auto Scaling group needs to
// run the worker tier on demand from zero:
//
//   - UnservedDemand — the count of distinct previews whose placement found no
//     worker (fleet.Registry.DrainDemand; deduplicated per interval, so retries
//     of one preview read as demand 1). This is the scale-OUT-from-zero signal:
//     LoadRatio can't distinguish "idle at zero" from "saturated" (both score
//     1), so capacity is added when a request arrives with nowhere to run.
//   - FleetLoad — fleet utilization as a percentage (fleet.Registry.LoadSample),
//     published ONLY when at least one worker is fresh. It drives scale-out
//     under load and scale-in when idle. Suppressing it at zero workers keeps
//     the scale-in alarm from seeing phantom data before the fleet exists.
//   - IdleWorkers — fresh, non-draining workers serving nothing
//     (fleet.Registry.IdleWorkers), published under the same guard. Its alarm
//     reclaims an empty node during working hours, when the min-warm floor
//     keeps FleetLoad above the scale-in threshold no matter how much spare
//     capacity sits idle.
//
// It also reconciles EC2 instance scale-in protection from the fleet's
// per-instance busy map, so an ASG scale-in drains an idle node instead of
// terminating one mid-preview.
//
// This runs on the control node only. It needs exactly cloudwatch:PutMetricData
// and autoscaling:SetInstanceProtection on the instance role — busy-ness comes
// from the live fleet registry, never a Describe call, so don't widen the role
// with one. A node without those grants (or without an ASG name) simply doesn't
// start the publisher.
package cloudscale

import (
	"context"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
)

// Fleet is the slice of the fleet registry the publisher reads. fleet.Registry
// satisfies it; a fake satisfies it in tests.
type Fleet interface {
	// DrainDemand returns the count of distinct previews that went unserved
	// since the last call and resets the set.
	DrainDemand() int64
	// LoadSample returns fleet utilization percent and the count of fresh
	// workers (0 workers => don't publish load).
	LoadSample() (loadPct float64, workers int)
	// BusyByInstance maps each fresh worker's instance-id to whether it is
	// serving a preview.
	BusyByInstance() map[string]bool
	// IdleWorkers counts fresh, non-draining autoscaled workers serving
	// nothing — nodes the ASG can reclaim without disturbing any preview.
	IdleWorkers() int
}

// Config parameterizes the publisher. ASGName is required (it is the CloudWatch
// dimension and the SetInstanceProtection target); an empty one means this node
// is not part of an autoscaled deployment and the caller should not start a
// publisher.
type Config struct {
	Region    string
	ASGName   string
	Namespace string        // CloudWatch namespace; defaults to "LocalPreview"
	Interval  time.Duration // publish cadence; defaults to 30s
}

// cwAPI and asAPI are the exact AWS calls the publisher makes, narrowed to an
// interface so a test can drive Run without the real SDK.
type cwAPI interface {
	PutMetricData(ctx context.Context, in *cloudwatch.PutMetricDataInput, optFns ...func(*cloudwatch.Options)) (*cloudwatch.PutMetricDataOutput, error)
}
type asAPI interface {
	SetInstanceProtection(ctx context.Context, in *autoscaling.SetInstanceProtectionInput, optFns ...func(*autoscaling.Options)) (*autoscaling.SetInstanceProtectionOutput, error)
}

// Publisher pushes fleet metrics to CloudWatch and reconciles scale-in
// protection on an interval.
type Publisher struct {
	fleet Fleet
	cw    cwAPI
	as    asAPI
	cfg   Config

	// protected caches the last protection state we set per instance-id so a
	// steady fleet doesn't re-issue SetInstanceProtection every interval — we
	// only call on a transition (idle↔busy) or for a newly seen instance.
	protected map[string]bool
}

// New builds a Publisher with real AWS clients from the ambient credential
// chain (the instance role in production). ASGName must be non-empty.
func New(ctx context.Context, fleet Fleet, cfg Config) (*Publisher, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(cfg.Region))
	if err != nil {
		return nil, err
	}
	return newWith(fleet, cloudwatch.NewFromConfig(awsCfg), autoscaling.NewFromConfig(awsCfg), cfg), nil
}

// newWith is the test seam: it takes the two AWS interfaces directly.
func newWith(fleet Fleet, cw cwAPI, as asAPI, cfg Config) *Publisher {
	if cfg.Namespace == "" {
		cfg.Namespace = "LocalPreview"
	}
	if cfg.Interval <= 0 {
		cfg.Interval = 30 * time.Second
	}
	return &Publisher{fleet: fleet, cw: cw, as: as, cfg: cfg, protected: map[string]bool{}}
}

// Run publishes immediately and every Interval until ctx is cancelled. It never
// returns an error: a transient AWS failure is logged and retried next tick, so
// one bad call can't wedge the control node's scaling.
func (p *Publisher) Run(ctx context.Context) {
	p.tick(ctx)
	t := time.NewTicker(p.cfg.Interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			p.tick(ctx)
		}
	}
}

func (p *Publisher) tick(ctx context.Context) {
	p.publishMetrics(ctx)
	p.reconcileProtection(ctx)
}

// publishMetrics sends UnservedDemand (always) and FleetLoad (only when the
// fleet has a fresh worker). DrainDemand resets the counter, so a failed put
// drops that interval's demand rather than double-counting it next tick — an
// acceptable trade for not letting a stuck put inflate the signal.
func (p *Publisher) publishMetrics(ctx context.Context) {
	demand := p.fleet.DrainDemand()
	loadPct, workers := p.fleet.LoadSample()

	dims := []cwtypes.Dimension{{
		Name:  aws.String("AutoScalingGroupName"),
		Value: aws.String(p.cfg.ASGName),
	}}
	now := time.Now()
	data := []cwtypes.MetricDatum{{
		MetricName: aws.String("UnservedDemand"),
		Dimensions: dims,
		Timestamp:  aws.Time(now),
		Value:      aws.Float64(float64(demand)),
		Unit:       cwtypes.StandardUnitCount,
	}}
	// Only report load once a worker exists: at zero workers LoadSample returns
	// (0,0), and publishing a 0 there would make the scale-in alarm fire on an
	// empty fleet it can do nothing about.
	if workers > 0 {
		data = append(data, cwtypes.MetricDatum{
			MetricName: aws.String("FleetLoad"),
			Dimensions: dims,
			Timestamp:  aws.Time(now),
			Value:      aws.Float64(loadPct),
			Unit:       cwtypes.StandardUnitPercent,
		})
		// IdleWorkers rides the same at-least-one-worker guard: at zero there
		// is nothing to reclaim, and its alarm treats missing data as fine.
		// This is the reclaim signal a load ratio can't be: the min-warm
		// floor keeps utilization comfortably above any sane scale-in
		// threshold, so an EMPTY worker (protection already released) would
		// otherwise idle until the floor's off-hours window lets load
		// collapse.
		data = append(data, cwtypes.MetricDatum{
			MetricName: aws.String("IdleWorkers"),
			Dimensions: dims,
			Timestamp:  aws.Time(now),
			Value:      aws.Float64(float64(p.fleet.IdleWorkers())),
			Unit:       cwtypes.StandardUnitCount,
		})
	}
	if _, err := p.cw.PutMetricData(ctx, &cloudwatch.PutMetricDataInput{
		Namespace:  aws.String(p.cfg.Namespace),
		MetricData: data,
	}); err != nil {
		log.Printf("cloudscale: PutMetricData: %v", err)
	}
}

// reconcileProtection protects busy instances from scale-in and releases idle
// ones, batching by target state and skipping instances already in the desired
// state. An instance that drops out of the fleet (terminated, or no longer
// fresh) is forgotten from the cache — we don't try to unprotect an instance we
// can no longer see, and a re-registering one is treated as newly seen.
func (p *Publisher) reconcileProtection(ctx context.Context) {
	busy := p.fleet.BusyByInstance()

	var protect, release []string
	for id, isBusy := range busy {
		prev, known := p.protected[id]
		if known && prev == isBusy {
			continue // already in the desired state
		}
		if isBusy {
			protect = append(protect, id)
		} else {
			release = append(release, id)
		}
	}
	// Forget instances no longer in the fleet so the cache can't grow without
	// bound and a returning instance re-syncs from scratch.
	for id := range p.protected {
		if _, ok := busy[id]; !ok {
			delete(p.protected, id)
		}
	}

	if len(protect) > 0 {
		p.setProtection(ctx, protect, true)
	}
	if len(release) > 0 {
		p.setProtection(ctx, release, false)
	}
}

func (p *Publisher) setProtection(ctx context.Context, ids []string, on bool) {
	_, err := p.as.SetInstanceProtection(ctx, &autoscaling.SetInstanceProtectionInput{
		AutoScalingGroupName: aws.String(p.cfg.ASGName),
		InstanceIds:          ids,
		ProtectedFromScaleIn: aws.Bool(on),
	})
	if err != nil {
		// A just-terminated instance yields a validation error; that's benign
		// and self-heals next tick when it drops out of the fleet.
		log.Printf("cloudscale: SetInstanceProtection(%v protected=%v): %v", ids, on, err)
		return
	}
	for _, id := range ids {
		p.protected[id] = on
	}
}
