package locker

import (
	"golang.org/x/net/context"
)

// unexported to prevent collisions with context keys defined in other packages.
type key int

// queueKey is the context key for the queue name
const queueKey key = 0

// WithQueue creates a new context with the queue value
func WithQueue(c context.Context, queue string) context.Context {
	return context.WithValue(c, queueKey, queue)
}

// retrieve the per-request queue from context
func queueFromContext(c context.Context) (string, bool) {
	queue, ok := c.Value(queueKey).(string)
	return queue, ok
}
