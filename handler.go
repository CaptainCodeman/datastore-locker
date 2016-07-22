package locker

import (
	"net/http"

	"golang.org/x/net/context"
	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/log"
)

type (
	// EntityFactory creates a new entity instance for use by the lock handler
	EntityFactory func() Lockable

	// TaskHandler is the signature of the task handler
	TaskHandler func(c context.Context, r *http.Request, key *datastore.Key, entity Lockable) error
)

// Handle wraps a task handler with task / lock processing
func (l *Locker) Handle(handler TaskHandler, factory EntityFactory) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		c := appengine.NewContext(r)

		// ensure request is a task request
		if r.Method != "POST" || r.Header.Get("X-Appengine-TaskName") == "" {
			log.Warningf(c, "non task request")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// use the same queue name for any tasks scheduled by this handler
		queue := r.Header.Get("X-Appengine-QueueName")
		c = WithQueue(c, queue)

		key, seq, err := l.Parse(c, r)
		if err != nil {
			log.Warningf(c, "parse failed: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		entity := factory()
		err = l.Aquire(c, key, entity, seq)
		if err != nil {
			log.Warningf(c, "lock failed: %v", err)
			// if we have a lock error, it provides the http response to use
			if lerr, ok := err.(Error); ok {
				w.WriteHeader(lerr.Response)
			} else {
				w.WriteHeader(http.StatusInternalServerError)
			}
			return
		}

		// TODO: explore having handler return something to indicate
		// if the task needs to continue with the next seq or be completed
		err = handler(c, r, key, entity)
		if err != nil {
			log.Warningf(c, "handler failed: %v", err)
			// clear the lock to allow the next retry
			if err := l.clearLock(c, key, entity); err != nil {
				log.Warningf(c, "clearLock failed: %v", err)
				// if we have a lock error, it provides the http response to use
				if lerr, ok := err.(Error); ok {
					w.WriteHeader(lerr.Response)
				} else {
					w.WriteHeader(http.StatusInternalServerError)
				}
			} else {
				w.WriteHeader(http.StatusInternalServerError)
			}
			return
		}

		w.WriteHeader(http.StatusOK)
	}

	return http.HandlerFunc(fn)
}
