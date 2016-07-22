# Datastore Locker
Provides a lock / lease mechanism to prevent duplicate execution of
appengine tasks and an easy way to continue long-running processes. Used
by [datastore-mapper](https://github.com/CaptainCodeman/datastore-mapper).

Sometimes it's extremely difficult if not impossible to make tasks truly
idempotent. e.g. if your task sends an email or charges a credit card
then executing it twice could be 'a bad thing'. Actually preventing a task
from running more than once can be challenging though ...

Although named tasks can be used to prevent duplicate tasks being *enqueued*
they cannot be used together with datastore transactions which leaves you
with the possibility that either the task scheduling or the datastore write
could fail.

An unnamed task *can* be enqueued within a transaction to ensure that both
happen or neither happen but that then leaves the possibility of *that*
operation being repeated (if running in it's own task) which could cause
duplicate tasks.

Whichever approach is used, the taskqueue only promises *at-least-once* 
delivery so there is also the chance that appengine will execute a task more
than once.

The solution is to coordinate execution using information in the task with
information in a datastore entity. This package aims to make it easy to
restrict task execution by using a lease / lock mechanism.

By obtaining a lock on an entity within a datastore transaction we ensure
that only a single instance of any task will be executed at once and, once
processed, that duplicate execution will be prevented.

It can be used with single tasks or to chain a series of tasks in sequence
with the sequence number used to prevent any old tasks being re-executed.

An exceptional situation can occur if a failure happens during processing
of a task and the result cannot be communicated back to appengine (this is
a platform issue). In this case the lock / lease is already held but the
system cannot determine if the task completed or maybe it just failed to 
clear the lock. The locker will allow a timeout before querying the appengine
logs to determine the task status. In the case of a complete failure with
no log information, a timeout will prevent deadlock by overwriting the
expired lock / lease.

Both overwritten locks and permanently failing tasks (past a configurable
number of retries) can be alerted by email as needing further investigation.

## Usage
See the example project for a simple demonstration of locker being used.

Embed the `locker.Lock` field within the struct you want lock on.

    Foo struct {
        locker.Struct
        Value string `datastore:"value"`  
    }

Create an instance of the locker and configure as required:

    l := locker.NewLocker()

    l := locker.NewLocker(
      locker.LeaseDuration(time.Duration(5)*time.Minute),
      locker.LeaseTimeout(time.Duration(15)*time.Minute),
      locker.AlertOnOverwrite,
    )

    l := locker.NewLocker(locker.LogVerbose)

Schedule a task to be executed once:

    key := datastore.NewKey(c, "foo", "", 1, nil)
    entity := &Foo{Value:"bar"}
    err := l.Schedule(c, key, entity, "/task/handler/url", nil)
    if err != nil {
      // operation failed (entity not saved and task not enqueued)
    }

Handle the task execution:

    func init() {
      http.Handle("/task/handler/url", locker.Handle(fooHandler, fooFactory)
    }

    // the task handler needs a factory to construct an instance of our entity
    func fooFactory() interface{} {
      return new(Foo)
    }

    // the handler for the task will be passed the appengine context, request, datastore key and entity
    func foohandler(c context.Context, r *http.Request, key *datastore.Key, entity interface{}) error {
      foo := entity.(*Foo)

      switch foo.Sequence {
        case 1:
          // step 1 processing, e.g. charge credit card
          // schedule another task to follow this one:
          return l.Schedule(c, key, entity, "/task/handler/url", nil)
        case 2:
          // step 2 processing, e.g. send confirmation email
          // mark the task as completed (to prevent the last task re-executing)
          return l.Complete(c, key, entity)
      }

      // returning an error would cause the task to be failed and retried (normal task semantics)
      // a configurable number of retries can be set to prevent endless attempts from happening
      return nil
    }
