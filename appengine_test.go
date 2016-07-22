package locker

import (
	"os"
	"testing"

	"google.golang.org/appengine/aetest"
)

var (
	instance aetest.Instance
)

func TestMain(m *testing.M) {
	var err error
	instance, err = aetest.NewInstance(nil)
	if err != nil {
		panic(err)
	}

	defer instance.Close()

	os.Exit(m.Run())
}
