// Package provider defines the AI provider interface used by agents. The model
// never receives tools; it gets text in and returns text (JSON) out.
package provider

import (
	"errors"
	"fmt"
)

type Request struct {
	Agent           string
	Model           string
	SchemaVersion   string
	FixtureKey      string
	System          string
	User            string
	MaxOutputTokens int
}

type Response struct {
	Content     string
	Warnings    []string
	FixtureUsed string
}

type Usage struct {
	InputTokens     int
	OutputTokens    int
	CacheReadTokens int
	CostUSD         *float64 // set only when the provider reports cost itself
	Measurable      bool     // false when token counts are estimates
}

type Capabilities struct {
	MeasurableCost bool
	Network        bool
}

type Provider interface {
	Name() string
	Capabilities() Capabilities
	Complete(ctx interface{ Done() <-chan struct{}; Err() error }, req Request) (Response, Usage, error)
}

// Error kinds.
const (
	KindAuth           = "auth"
	KindBilling        = "billing"
	KindRateLimit      = "rate_limit"
	KindTimeout        = "timeout"
	KindInvalidRequest = "invalid_request"
	KindServer         = "server"
	KindUnknown        = "unknown"
)

type Error struct {
	Kind string
	Err  error
}

func (e *Error) Error() string { return fmt.Sprintf("provider %s error: %v", e.Kind, e.Err) }
func (e *Error) Unwrap() error { return e.Err }

// Retryable reports whether a bounded retry makes sense.
func Retryable(err error) bool {
	var pe *Error
	if errors.As(err, &pe) {
		return pe.Kind == KindRateLimit || pe.Kind == KindTimeout || pe.Kind == KindServer
	}
	return false
}

// Fatal reports errors that must stop the run immediately (never retried).
func Fatal(err error) bool {
	var pe *Error
	if errors.As(err, &pe) {
		return pe.Kind == KindAuth || pe.Kind == KindBilling
	}
	return false
}

// EstimateTokens approximates token count as runes/4 (conservative for Indonesian text).
func EstimateTokens(s string) int {
	n := 0
	for range s {
		n++
	}
	return (n + 3) / 4
}
