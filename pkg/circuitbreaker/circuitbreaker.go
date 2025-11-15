package circuitbreaker

import (
	"time"

	"github.com/sony/gobreaker"
)

func CircuitBreaker(name string) *gobreaker.CircuitBreaker {
	settings := gobreaker.Settings{
		Name:        name,
		MaxRequests: 3,
		Interval:    10 * time.Second,
		Timeout:     1 * time.Minute,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.TotalFailures >= 3
		},
	}
	return gobreaker.NewCircuitBreaker(settings)
}
