package subscriptions

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

type reconcilingProvider struct {
	interval time.Duration
	runs     atomic.Int64
	repeated chan struct{}
}

func (provider *reconcilingProvider) Name() string                     { return "reconciling" }
func (provider *reconcilingProvider) Webhooks() []Route                { return nil }
func (provider *reconcilingProvider) ReconcileInterval() time.Duration { return provider.interval }
func (provider *reconcilingProvider) Reconcile(context.Context) error {
	if provider.runs.Add(1) == 3 {
		close(provider.repeated)
	}
	return nil
}

func TestModuleRunReconcilesUntilCancelled(t *testing.T) {
	previous := reconcileStartDelay
	reconcileStartDelay = 0
	t.Cleanup(func() { reconcileStartDelay = previous })

	provider := &reconcilingProvider{interval: 5 * time.Millisecond, repeated: make(chan struct{})}
	module := &Module{Provider: provider, Policy: AllowAllPolicy{}}

	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { result <- module.Run(ctx) }()

	select {
	case <-provider.repeated:
	case <-time.After(2 * time.Second):
		t.Fatalf("Reconcile ran %d times, want it to repeat on the interval", provider.runs.Load())
	}

	cancel()
	select {
	case err := <-result:
		if err != nil {
			t.Fatalf("Run() error = %v, want nil on cancellation", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run() did not return after its context was cancelled")
	}
}

// Run is started unconditionally at boot, so it must not block when there is
// nothing to do.
func TestModuleRunReturnsImmediatelyWhenIdle(t *testing.T) {
	modules := map[string]*Module{
		"disabled module":         {Policy: AllowAllPolicy{}},
		"reconciliation disabled": {Provider: &reconcilingProvider{}, Policy: AllowAllPolicy{}},
	}
	for name, module := range modules {
		t.Run(name, func(t *testing.T) {
			done := make(chan error, 1)
			go func() { done <- module.Run(context.Background()) }()
			select {
			case err := <-done:
				if err != nil {
					t.Fatalf("Run() error = %v", err)
				}
			case <-time.After(time.Second):
				t.Fatal("Run() blocked with nothing to reconcile")
			}
		})
	}
}
