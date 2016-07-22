package locker

import (
	"time"
)

type (
	// Lock adds additional information to a datastore entity used to ensure that
	// only a single instance of a task can execute and a sequence of tasks will
	// execute in the correct order.
	//
	// The appengine taskqueue guaranteed at-least-once task execution so we need
	// to try to detect and prevent spurious re-execution. At the same time, these
	// repeat executions may be necessary if a task has died or disconnected and
	// left things in an unknown state - the lock information can be used to avoid
	// deadlocks by allowing a lease to timeout in a controlled manner.
	//
	// This lock struct should be embedded within the entity:
	//
	//     MyEntity struct {
	//         locker.Lock
	//         Value           string `datastore:"value"`
	//     }
	//
	Lock struct {
		// Timestamp is the time that this lock was written
		Timestamp time.Time `datastore:"lock_ts"`

		// Request is the request id that obtained the current lock
		RequestID string `datastore:"lock_req"`

		// Sequence is the task sequence number
		Sequence int `datastore:"lock_seq"`

		// Retries is the number of retries that have been attempted
		Retries int `datastore:"lock_try"`
	}

	// Lockable is the interface that lockable entities must implement
	// they will do this automatically simply by embedding lock in the struct
	// This is used to ensure than entities we deal with have our Lock struct
	// embedded and gives us a way to access it
	Lockable interface {
		getLock() *Lock
	}
)

func (l *Lock) getLock() *Lock {
	return l
}
