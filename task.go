package locker

import (
	"strconv"
	"time"

	"math/rand"
	"net/http"
	"net/url"

	"golang.org/x/net/context"
	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/log"
	"google.golang.org/appengine/taskqueue"
)

// Parse returns the namespace, datatore.Key, sequence and queue name from a task request
func Parse(c context.Context, r *http.Request) (*datastore.Key, int, error) {
	key := new(datastore.Key)
	if err := key.UnmarshalJSON([]byte(r.Header.Get("X-Lock-Key"))); err != nil {
		return nil, 0, err
	}
	seq, err := strconv.Atoi(r.Header.Get("X-Lock-Seq"))
	if err != nil {
		return nil, 0, err
	}
	return key, seq, nil
}

// NewTask creates a new taskqueue.Task for the entity with the correct
// headers set to match those on the entity
func (l *Locker) NewTask(key *datastore.Key, entity Lockable, path string, params url.Values) *taskqueue.Task {
	// prepare the lock entries
	lock := entity.getLock()
	lock.Timestamp = getTime()
	lock.RequestID = ""
	lock.Retries = 0
	lock.Sequence++

	json, _ := key.MarshalJSON()

	// set task headers so that we can retrieve the matching entity
	// and check that the executing task is the one we're expecting
	task := taskqueue.NewPOSTTask(path, params)
	task.Header.Set("X-Lock-Seq", strconv.Itoa(lock.Sequence))
	task.Header.Set("X-Lock-Key", string(json))

	return task
}

// Schedule schedules a task with lock
func (l *Locker) Schedule(c context.Context, key *datastore.Key, entity Lockable, path string, params url.Values) error {
	task := l.NewTask(key, entity, path, params)

	// write the datastore entity and schedule the task within a
	// transaction to guarantees that both happen and the entity
	// will be committed to the datastore when the task executes but
	// the task won't be scheduled if our entity update fails
	err := storage.RunInTransaction(c, func(tc context.Context) error {
		if _, err := storage.Put(tc, key, entity); err != nil {
			return err
		}
		if _, err := taskqueue.Add(tc, task, l.Queue); err != nil {
			return err
		}
		return nil
	}, &datastore.TransactionOptions{XG: false, Attempts: 3})

	return err
}

// GetLock attempts to get and lock an entity with the given identifier
// If successful it will write a new lock entity to the datastore
// and return nil, otherwise it will return an error to indicate
// the reason for failure.
func (l *Locker) GetLock(c context.Context, key *datastore.Key, entity Lockable, sequence int) error {
	requestID := appengine.RequestID(c)
	lock := new(Lock)
	success := false

	// we need to run in a transaction for consistency guarantees
	// in case two tasks start at the exact same moment and each
	// of them sees no lock in place
	err := storage.RunInTransaction(c, func(tc context.Context) error {
		// reset flag here in case of transaction retries
		success = false

		if err := storage.Get(tc, key, entity); err != nil {
			return err
		}

		// we got the entity successfully, check if it's locked
		// and try to claim the lease if it isn't
		lock = entity.getLock()
		if lock.RequestID == "" && lock.Sequence == sequence {
			lock.Timestamp = getTime()
			lock.RequestID = requestID
			if _, err := storage.Put(tc, key, entity); err != nil {
				return err
			}
			success = true
			return nil
		}

		// lock already exists, return nil because there is no point doing
		// any more retries but we'll need to figure out if we can claim it
		return nil
	}, nil)

	// if there was any error then we failed to get the lock due to datastore
	// errors. Returning an error indicates to the caller that they should mark
	// the task as failed so it will be re-attempted
	if err != nil {
		return ErrLockFailed
	}

	// success is true if we got the lock
	if success {
		return nil
	}

	// If there wasn't any error but we weren't successful then a lock is
	// already in place. We're most likely here because a duplicate task has
	// been scheduled or executed so we need to examine the lock itself
	log.Debugf(c, "lock %v %d %d %s", lock.Timestamp, lock.Sequence, lock.Retries, lock.RequestID)

	// if the lock sequence is already past this task so it should be dropped
	if lock.Sequence > sequence {
		return ErrTaskExpired
	}

	// if the lock is within the lease duration we return that it's locked so
	// that this task will be retried
	if lock.Timestamp.Add(l.LeaseDuration).After(getTime()) {
		return ErrLockFailed
	}

	// if the lock has been held for longer than the lease duration then we
	// start querying the logs api to see if the previous request completed.
	// if it has then we will be overwriting the lock. It's possible that the
	// log entry is missing or we simply don't have access to them (managed VM)
	// so the lease timeout is a failsafe to catch extreme undetectable failures
	if lock.Timestamp.Add(l.LeaseTimeout).Before(getTime()) && l.previousRequestEnded(c, lock.RequestID) {
		if err := l.overwriteLock(c, key, entity, requestID); err == nil {
			// success (at least we grabbed the lock)
			return nil
		}
	}

	return ErrLockFailed
}

// Complete marks a task as completed
func (l *Locker) Complete(c context.Context, key *datastore.Key, entity Lockable) error {
	// prepare the lock entries
	lock := entity.getLock()
	lock.Timestamp = getTime()
	lock.RequestID = ""
	lock.Retries = 0
	lock.Sequence++

	// TODO: do we need to re-fetch the entity to guarantee freshness?
	err := storage.RunInTransaction(c, func(tc context.Context) error {
		if _, err := storage.Put(tc, key, entity); err != nil {
			return err
		}
		return nil
	}, nil)

	return err
}

// clearLock clears the current lease, it should be called at the end of every task
// execution if things fail, to try and prevent unecessary locks and to count the
// number of retries
func (l *Locker) clearLock(c context.Context, key *datastore.Key, entity Lockable) error {
	lock := entity.getLock()
	if lock.Retries == l.MaxRetries {
		if l.AlertOnFailure {
			if err := alertAdmins(c, key, entity, "Permanent task failure"); err != nil {
				log.Errorf(c, "failed to send alert email for permanent task failure: %v", err)
			}
		}
		return ErrTaskFailed
	}
	err := storage.RunInTransaction(c, func(tc context.Context) error {
		if err := storage.Get(tc, key, entity); err != nil {
			log.Debugf(c, "clearLock get %v", err)
			return err
		}
		lock := entity.getLock()
		lock.Timestamp = getTime()
		lock.RequestID = ""
		lock.Retries++
		if _, err := storage.Put(tc, key, entity); err != nil {
			log.Debugf(c, "clearLock put %v", err)
			return err
		}
		return nil
	}, nil)
	return err
}

// overwrite the current lock
func (l *Locker) overwriteLock(c context.Context, key *datastore.Key, entity Lockable, requestID string) error {
	log.Debugf(c, "overwriteLock %s %s", key.String(), requestID)
	if l.AlertOnOverwrite {
		if err := alertAdmins(c, key, entity, "Lock overwrite"); err != nil {
			log.Errorf(c, "failed to send alert email for lock overwrite: %v", err)
		}
	}
	err := storage.RunInTransaction(c, func(tc context.Context) error {
		if err := storage.Get(tc, key, entity); err != nil {
			return err
		}
		lock := entity.getLock()
		lock.Timestamp = getTime()
		lock.RequestID = requestID
		if _, err := storage.Put(tc, key, entity); err != nil {
			return err
		}
		return nil
	}, nil)
	return err
}

// determine whether old request has ended according to logs
func (l *Locker) previousRequestEnded(c context.Context, requestID string) bool {
	q := &log.Query{
		RequestIDs: []string{requestID},
	}
	results := q.Run(c)
	record, err := results.Next()
	if err == log.Done {
		// no record found so it hasn't ended
		if l.LogVerbose {
			log.Warningf(c, "no log found for previous request %s", requestID)
		}
		return false
	}
	if err != nil {
		// Managed VMs do not have access to the logservice API
		if l.LogVerbose {
			log.Warningf(c, "err getting log for previous request %s %v", requestID, err)
		}
		return false
	}
	if l.LogVerbose {
		log.Debugf(c, "found previous request log %v", record)
	}
	return record.Finished
}

func randomDelay() {
	d := time.Duration(rand.Int63n(4)+1) * time.Second
	time.Sleep(d)
}
