package bridge

import "context"

// Observation contains fixed lifecycle labels and a local random request ID.
// It must never contain request content, account data, or raw upstream errors.
type Observation struct {
	RequestID string
	Phase     string
	Category  string
	Cleanup   bool
}

// Observer is the consuming boundary for explicit development run controls.
// Before serializes admission with stopping; Observe reports failures
// synchronously before the response is written or admission is released.
// Ordinary product serving does not install an observer.
type Observer interface {
	Before(Observation) error
	Observe(Observation)
}

type observationKey struct{}
type observationContext struct {
	id       string
	observer Observer
}

// WithObserver attaches a development observer without adding protocol fields.
func WithObserver(ctx context.Context, id string, observer Observer) context.Context {
	return context.WithValue(ctx, observationKey{}, observationContext{id, observer})
}

// Before reports an operation admission at its synchronous decision point.
func Before(ctx context.Context, phase string) error {
	if err := ctx.Err(); err != nil {
		return context.Cause(ctx)
	}
	if c, ok := ctx.Value(observationKey{}).(observationContext); ok && c.observer != nil {
		return c.observer.Before(Observation{RequestID: c.id, Phase: phase})
	}
	return nil
}

// Observe reports only labels chosen locally by the caller.
func Observe(ctx context.Context, phase, category string, cleanup bool) {
	if c, ok := ctx.Value(observationKey{}).(observationContext); ok && c.observer != nil {
		c.observer.Observe(Observation{RequestID: c.id, Phase: phase, Category: category, Cleanup: cleanup})
	}
}
