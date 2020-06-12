package lease

import (
	"math"
	"math/rand"
	"time"
)

// CalculateBackoffTime calculates the backoff time for the current retry
// Reference: https://cloud.google.com/storage/docs/exponential-backoff
func CalculateBackoffTime(numRetry int, maxBackoff time.Duration) time.Duration {
	waitMillis := time.Duration(rand.Intn(1000)) * time.Millisecond
	baseWaitTime := math.Exp2(float64(numRetry))
	waitTime := time.Duration(baseWaitTime)*time.Second + waitMillis

	if waitTime <= maxBackoff {
		return waitTime
	}
	return maxBackoff
}
