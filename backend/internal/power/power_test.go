package power

import (
	"testing"

	"github.com/godbus/dbus/v5"
)

func TestEventOf(t *testing.T) {
	sig := func(member string, body ...any) *dbus.Signal {
		return &dbus.Signal{Name: logindIface + "." + member, Body: body}
	}
	cases := []struct {
		sig  *dbus.Signal
		want Event
		ok   bool
	}{
		{sig("PrepareForSleep", true), Suspend, true},
		{sig("PrepareForSleep", false), Resume, true},
		{sig("PrepareForShutdown", true), Shutdown, true},
		{sig("PrepareForShutdown", false), 0, false}, // shutdown cancelled
		{sig("PrepareForSleep"), 0, false},
		{sig("PrepareForSleep", "yes"), 0, false},
		{sig("SessionNew", true), 0, false},
	}
	for _, c := range cases {
		got, ok := eventOf(c.sig)
		if ok != c.ok || got != c.want {
			t.Errorf("%s %v: got %v, %v; want %v, %v", c.sig.Name, c.sig.Body, got, ok, c.want, c.ok)
		}
	}
}
