package resilience

import (
	"fmt"
	"time"

	"github.com/sony/gobreaker"
)

// CircuitBreakerConfig holds configuration for circuit breaker
type CircuitBreakerConfig struct {
	Name         string
	MaxRequests  uint32
	Interval     time.Duration
	Timeout      time.Duration
	ErrorPercent float64
}

// DefaultConfig returns default circuit breaker configuration
func DefaultConfig(name string) CircuitBreakerConfig {
	return CircuitBreakerConfig{
		Name:         name,
		MaxRequests:  100,
		Interval:     10 * time.Second,
		Timeout:      60 * time.Second,
		ErrorPercent: 50,
	}
}

// NewCircuitBreaker creates a new circuit breaker with given configuration
func NewCircuitBreaker(config CircuitBreakerConfig) *gobreaker.CircuitBreaker {
	return gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        config.Name,
		MaxRequests: config.MaxRequests,
		Interval:    config.Interval,
		Timeout:     config.Timeout,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= 3 && failureRatio >= config.ErrorPercent/100
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			fmt.Printf("Circuit breaker '%s' state changed from '%s' to '%s'\n", name, from, to)
		},
	})
}

// ExecuteWithRetry executes a function with retry mechanism
func ExecuteWithRetry(cb *gobreaker.CircuitBreaker, fn func() (interface{}, error), maxRetries int) (interface{}, error) {
	var lastErr error
	for i := 0; i <= maxRetries; i++ {
		result, err := cb.Execute(fn)
		if err == nil {
			return result, nil
		}
		lastErr = err
		if i < maxRetries {
			time.Sleep(time.Duration(i+1) * time.Second) // Exponential backoff
		}
	}
	return nil, fmt.Errorf("all retries failed: %v", lastErr)
}
