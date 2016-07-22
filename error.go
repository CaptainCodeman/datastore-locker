package locker

import (
	"net/http"
)

type (
	// Error is a custom error that includes the recommended http response to
	// return to control task completion / re-attempts.
	Error struct {
		Response int
		text     string
	}
)

var (
	// ErrLockFailed signals that a lock attempt failed and the task trying to
	// aquire it should be retried. Using ServiceUnavailable (503) causes a task
	// retry without flooding the logs with visible errors.
	ErrLockFailed = Error{http.StatusServiceUnavailable, "lock failed (retry)"}

	// ErrTaskExpired signals that the task's sequence no is behind the sequence
	// stored on the entity. This means processing has already moved on and this
	// task should be dropped. This is likely caused by a spurious task re-execution.
	// Using OK (200) causes a task to be marked as successful so it won't be retried.
	ErrTaskExpired = Error{http.StatusOK, "task expired (abandon)"}

	// ErrTaskFailed signals that the task has failed permanently (tried more than
	// the MaxRetries allowed) so should be abandoned.
	// Using OK (200) causes a task to be marked as successful so it won't be retried.
	ErrTaskFailed = Error{http.StatusOK, "task failed permanently (abandon)"}
)

func (e Error) Error() string {
	return e.text
}
