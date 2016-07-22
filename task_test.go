package locker

import (
	"testing"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
)

type (
	Foo struct {
		Lock
		Value string `datastore:"value"`
	}
)

func TestGetLockAvailable(t *testing.T) {
	r, _ := instance.NewRequest("GET", "/", nil)
	r.Header.Set("X-AppEngine-Request-Log-Id", "locked")
	c := appengine.NewContext(r)

	f := &Foo{
		Value: "test",
		Lock: Lock{
			Timestamp: time.Date(2016, 7, 21, 11, 10, 0, 0, time.UTC),
			RequestID: "",
			Sequence:  1,
			Retries:   0,
		},
	}

	k := datastore.NewKey(c, "foo", "", 1, nil)
	datastore.RunInTransaction(c, func(c context.Context) error {
		if _, err := datastore.Put(c, k, f); err != nil {
			return err
		}
		return nil
	}, nil)

	n := time.Date(2016, 7, 21, 11, 15, 0, 0, time.UTC)
	getTime = func() time.Time {
		return n
	}
	l, _ := NewLocker()
	err := l.GetLock(c, k, f, 1)
	if err != nil {
		t.Errorf("failed to lock %v", err)
	}

	datastore.Get(c, k, f)
	if f.RequestID != "locked" {
		t.Errorf("failed to set request id")
	}
	if f.Timestamp.UTC() != n {
		t.Errorf("failed to set request timestamp %s %s", n, f.Timestamp)
	}
}

func TestGetLockAlreadyLocked(t *testing.T) {
	r, _ := instance.NewRequest("GET", "/", nil)
	r.Header.Set("X-AppEngine-Request-Log-Id", "locked")
	c := appengine.NewContext(r)

	getTime = getTimeDefault
	f := &Foo{
		Value: "test",
		Lock: Lock{
			Timestamp: getTime(),
			RequestID: "previous",
			Sequence:  1,
			Retries:   0,
		},
	}

	k := datastore.NewKey(c, "foo", "", 1, nil)
	datastore.RunInTransaction(c, func(c context.Context) error {
		if _, err := datastore.Put(c, k, f); err != nil {
			return err
		}
		return nil
	}, nil)

	l, _ := NewLocker()
	err := l.GetLock(c, k, f, 1)
	if err != ErrLockFailed {
		t.Errorf("expected failed lock, got %v", err)
	}

	datastore.Get(c, k, f)
	t.Logf("%v", f)
}
