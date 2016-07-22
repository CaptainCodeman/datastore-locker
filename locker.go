package locker

import (
	"time"
)

type (
	// Locker is the instance that stores configuration
	Locker struct {
		// Once a lock has been held longer than this duration the logs API
		// will be checked to determine if the request has completed or not
		LeaseDuration time.Duration

		// On rare occassions entries may be missing from the logs so if a
		// lock has been held for more than this duration we assume that the
		// task has died. 10 mins is the task timeout on a frontend instance.
		LeaseTimeout time.Duration

		// MaxRetries is the maximum number of retries to allow
		MaxRetries int

		// LogVerbose sets verbose logging of lock operations
		LogVerbose bool

		// AlertOnFailure will set an alert email to be sent to admins if
		// a task fails permanently (more than the MaxRetries reached)
		AlertOnFailure bool

		// AlertOnOverwrite will set an alert email to be sent to admins
		// if a lock is being overwritten. This is normally an exceptional
		// situation but may investigation to ensure correct operation of
		// the system
		AlertOnOverwrite bool

		// DefaultQueue is the name of the task-queue to schedule tasks on.
		// The default (empty string) is to use the default task queue.
		DefaultQueue string
	}
)

// NewLocker creates a new configured Locker instance
func NewLocker(options ...func(*Locker) error) (*Locker, error) {
	locker := &Locker{
		LeaseDuration: time.Duration(1) * time.Minute,
		LeaseTimeout:  time.Duration(10)*time.Minute + time.Duration(30)*time.Second,
		MaxRetries:    10,
	}

	for _, option := range options {
		if err := option(locker); err != nil {
			return nil, err
		}
	}
	return locker, nil
}

// LeaseDuration sets the config setting for a locker
func LeaseDuration(duration time.Duration) func(*Locker) error {
	return func(l *Locker) error {
		l.LeaseDuration = duration
		return nil
	}
}

// LeaseTimeout sets the config setting for a locker
func LeaseTimeout(duration time.Duration) func(*Locker) error {
	return func(l *Locker) error {
		l.LeaseTimeout = duration
		return nil
	}
}

// MaxRetries sets the config setting for a locker
func MaxRetries(retries int) func(*Locker) error {
	return func(l *Locker) error {
		l.MaxRetries = retries
		return nil
	}
}

// LogVerbose sets the config setting for a locker
func LogVerbose(l *Locker) error {
	l.LogVerbose = true
	return nil
}

// AlertOnFailure sets the config setting for a locker
func AlertOnFailure(l *Locker) error {
	l.AlertOnFailure = true
	return nil
}

// AlertOnOverwrite sets the config setting for a locker
func AlertOnOverwrite(l *Locker) error {
	l.AlertOnOverwrite = true
	return nil
}

// DefaultQueue sets the config setting for a locker
func DefaultQueue(queue string) func(*Locker) error {
	return func(l *Locker) error {
		l.DefaultQueue = queue
		return nil
	}
}
