package main

import (
	"time"

	"net/http"

	"golang.org/x/net/context"
	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/log"
	"google.golang.org/appengine/taskqueue"

	"github.com/captaincodeman/datastore-locker"
)

type (
	// Counter is a simple datastore entity intended to show chained tasks.
	// In reality you wouldn't use the locker or tasks for a simple counter
	// like this as they are better suited for more involved processing.
	Counter struct {
		locker.Lock
		Limit int `datastore:"limit"`
	}
)

var (
	// our locker instance
	l *locker.Locker
)

func init() {
	l, _ = locker.NewLocker(
		locker.AlertOnFailure,
		locker.AlertOnOverwrite,
		locker.MaxRetries(3),
	)

	// call http://localhost:8080/start to kickoff the task
	http.HandleFunc("/start", start)

	// ... and it will call /process multiple times
	http.Handle("/process", l.Handle(counterHandler, counterFactory))
}

func start(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	counter := &Counter{Limit: 10}
	key := datastore.NewKey(c, "foo", "", 1, nil)
	l.Schedule(c, key, counter, "/process", nil)
}

func counterFactory() locker.Lockable {
	return new(Counter)
}

func counterHandler(c context.Context, r *http.Request, key *datastore.Key, entity locker.Lockable) error {
	counter := entity.(*Counter)
	log.Debugf(c, "process: %d", counter.Sequence)

	// simulate some processing work
	time.Sleep(time.Duration(1) * time.Second)
	if counter.Sequence == 5 {
		// simulate a duplicate task execution by creating one ourselves
		// needless to say, you wouldn't want to be doing this in practice
		// but it should demonstrate that the locker prevents spurious
		// task execution and guarantees the correct sequencing happens
		json, _ := key.MarshalJSON()
		t := taskqueue.NewPOSTTask("/process", nil)
		t.Header.Set("X-Lock-Seq", "6")
		t.Header.Set("X-Lock-Key", string(json))
		taskqueue.Add(c, t, "")
	}

	if counter.Sequence < counter.Limit {
		return l.Schedule(c, key, counter, "/process", nil)
	}

	return l.Complete(c, key, counter)
}
