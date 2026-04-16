package service

import (
	"context"
	"strings"
	"sync"
	"time"
)

const (
	retryAttempts = 30
	retryDelay    = 50 * time.Millisecond
)

type retryExcludeKey struct{}

// RetryExclude is a mutable struct passed via context. When a liteserver returns
// "state already gc'd", the retry loop adds that server to Excluded and retries
// with a different server. The client sets LastUsed before each request so the
// retry loop knows which server to exclude.
type RetryExclude struct {
	mu       sync.Mutex
	Excluded map[int]bool // client IDs to exclude for this retry session
	LastUsed int          // set by client before each request
}

// isPermanentError returns true for liteserver errors that will never succeed on retry.
func isPermanentError(err error) bool {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "no connections available"):
		return true
	default:
		return false
	}
}

// isStateGCError returns true when the server doesn't have the block (GC'd or
// not an archive); another server (e.g. archive) might have it.
func isStateGCError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "state already gc'd") || strings.Contains(msg, "ltdb: block not found")
}

// retry calls fn up to retryAttempts times, sleeping retryDelay between attempts.
// The liteapi connection pool rotates between servers on each call,
// so retrying effectively switches to a different liteserver.
// Permanent errors (e.g. block not found) are returned immediately without retrying.
func retry[T any](fn func() (T, error)) (T, error) {
	ctx, _ := WithRetryExclude(context.Background())
	return retryWithContext(ctx, fn)
}

// retryWithExclude is like retry but uses ctx to pass RetryExclude. The caller
// must inject RetryExclude via WithRetryExclude before calling. When the
// client returns "state already gc'd", the failing server is added to the
// exclude list and retry continues with other servers.
func retryWithExclude[T any](ctx context.Context, fn func() (T, error)) (T, error) {
	return retryWithContext(ctx, fn)
}

// WithRetryExclude returns a context with RetryExclude attached for use with retryWithExclude.
func WithRetryExclude(ctx context.Context) (context.Context, *RetryExclude) {
	exclude := &RetryExclude{Excluded: make(map[int]bool)}
	return context.WithValue(ctx, retryExcludeKey{}, exclude), exclude
}

func retryWithContext[T any](ctx context.Context, fn func() (T, error)) (T, error) {
	var lastErr error
	excludeVal := ctx.Value(retryExcludeKey{})
	var exclude *RetryExclude
	if excludeVal != nil {
		exclude = excludeVal.(*RetryExclude)
	}
	for i := range retryAttempts {
		result, err := fn()
		if err == nil {
			return result, nil
		}
		lastErr = err
		if isPermanentError(err) {
			break
		}
		if exclude != nil && isStateGCError(err) {
			exclude.mu.Lock()
			exclude.Excluded[exclude.LastUsed] = true
			exclude.mu.Unlock()
			// log.Printf("retry: excluding liteserver client_id=%d (state already gc'd), retrying", exclude.LastUsed)
		}
		if i < retryAttempts-1 {
			// log.Printf("retry %d/%d: %v", i+1, retryAttempts-1, err)
			time.Sleep(retryDelay)
		}
	}
	var zero T
	return zero, lastErr
}
