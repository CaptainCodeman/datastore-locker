package locker

import (
	"fmt"

	"golang.org/x/net/context"
	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/mail"
)

func alertAdmins(c context.Context, key *datastore.Key, entity Lockable, reason string) error {
	sender := "locker@" + appengine.AppID(c) + ".appspot.com"

	msg := &mail.Message{
		Sender:  sender,
		Subject: reason,
		Body:    fmt.Sprintf("key: %s, entity: %#v", key.String(), entity),
	}

	return mail.SendToAdmins(c, msg)
}
