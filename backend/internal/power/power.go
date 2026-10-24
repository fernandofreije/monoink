// Package power reports the system going to sleep, waking up and shutting
// down, from systemd-logind's PrepareForSleep and PrepareForShutdown
// signals on the system bus.
package power

import (
	"context"
	"errors"
	"log/slog"
	"syscall"

	"github.com/godbus/dbus/v5"
)

// Event is a change in the system's power state.
type Event int

const (
	Suspend Event = iota
	Resume
	Shutdown
)

func (e Event) String() string {
	switch e {
	case Suspend:
		return "suspend"
	case Resume:
		return "resume"
	}
	return "shutdown"
}

const (
	logindName  = "org.freedesktop.login1"
	logindPath  = "/org/freedesktop/login1"
	logindIface = "org.freedesktop.login1.Manager"
)

// eventOf translates a logind signal. Both signals carry true before the
// change; PrepareForSleep carries false again after resuming.
func eventOf(sig *dbus.Signal) (Event, bool) {
	if len(sig.Body) != 1 {
		return 0, false
	}
	before, ok := sig.Body[0].(bool)
	if !ok {
		return 0, false
	}
	switch sig.Name {
	case logindIface + ".PrepareForSleep":
		if before {
			return Suspend, true
		}
		return Resume, true
	case logindIface + ".PrepareForShutdown":
		if before {
			return Shutdown, true
		}
	}
	return 0, false
}

// Watch calls fn for each power event until ctx ends. While fn handles a
// suspend or shutdown, a logind delay inhibitor holds the system back (for
// at most logind's InhibitDelayMaxSec, 5 s by default) so fn can update
// the display first. Without permission to inhibit, events still arrive
// but the system may go down before fn finishes.
func Watch(ctx context.Context, log *slog.Logger, fn func(Event)) error {
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return err
	}
	defer conn.Close()
	for _, member := range []string{"PrepareForSleep", "PrepareForShutdown"} {
		err := conn.AddMatchSignal(dbus.WithMatchObjectPath(logindPath),
			dbus.WithMatchInterface(logindIface), dbus.WithMatchMember(member))
		if err != nil {
			return err
		}
	}
	sigs := make(chan *dbus.Signal, 8)
	conn.Signal(sigs)
	defer conn.RemoveSignal(sigs)

	lock := inhibit(conn, log)
	defer release(lock)
	for {
		select {
		case <-ctx.Done():
			return nil
		case sig, ok := <-sigs:
			if !ok {
				return errors.New("system bus connection closed")
			}
			ev, ok := eventOf(sig)
			if !ok {
				continue
			}
			log.Info("power event", "event", ev)
			fn(ev)
			if ev == Resume {
				if lock < 0 {
					lock = inhibit(conn, log)
				}
			} else {
				release(lock) // lets the system go ahead
				lock = -1
			}
		}
	}
}

// inhibit takes a delay lock on sleep and shutdown, returning its file
// descriptor, or -1 if logind refused.
func inhibit(conn *dbus.Conn, log *slog.Logger) int {
	var fd dbus.UnixFD
	err := conn.Object(logindName, logindPath).Call(logindIface+".Inhibit", 0,
		"sleep:shutdown", "monoink", "Updating the e-ink faceplate", "delay").Store(&fd)
	if err != nil {
		log.Warn("can't delay sleep or shutdown; the display may not update in time", "err", err)
		return -1
	}
	return int(fd)
}

func release(fd int) {
	if fd >= 0 {
		syscall.Close(fd)
	}
}
