package engine

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"

	"github.com/v1k0d3n/monoink/backend/internal/ble"
	"github.com/v1k0d3n/monoink/backend/internal/conn"
	"github.com/v1k0d3n/monoink/backend/internal/proto"
	"github.com/v1k0d3n/monoink/backend/internal/render"
	"github.com/v1k0d3n/monoink/backend/internal/settings"
	"github.com/v1k0d3n/monoink/backend/internal/steam"
	"github.com/v1k0d3n/monoink/backend/internal/sysinfo"
)

// fakeBus is an always-present faceplate that accepts every frame.
type fakeBus struct{}

func (fakeBus) Powered() (bool, error)          { return true, nil }
func (fakeBus) Close() error                    { return nil }
func (fakeBus) Known() ([]ble.Candidate, error) { return nil, nil }
func (fakeBus) Scan(context.Context, func(ble.Candidate) bool) ([]ble.Candidate, error) {
	return []ble.Candidate{{Path: "/dev_X", Address: "00:11:22:33:44:55", Name: proto.AdvertisedName}}, nil
}
func (fakeBus) Connect(context.Context, dbus.ObjectPath) (conn.Link, error) {
	l := &fakeLink{ch: make(chan []byte, 4)}
	l.up.Store(true)
	return l, nil
}

type fakeLink struct {
	up atomic.Bool
	ch chan []byte
}

func (l *fakeLink) Notifications() <-chan []byte { return l.ch }
func (l *fakeLink) Drain()                       {}
func (l *fakeLink) Connected() bool              { return l.up.Load() }
func (l *fakeLink) Close() error                 { l.up.Store(false); return nil }
func (l *fakeLink) Write(_ context.Context, b []byte, _ bool) error {
	if !l.up.Load() {
		return errors.New("not connected")
	}
	if b[1] == proto.CmdQueryInfo {
		l.ch <- []byte{0x91, 0x00, 0x01, 0x00, 0x02, 0x88, 0x01, 0xE0, 0x00, 0x6C, 0x50, 0x19}
	}
	return nil
}

func writeFile(t *testing.T, path, data string) {
	t.Helper()
	os.MkdirAll(filepath.Dir(path), 0o755)
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

// flowEngine runs an engine against a fake display and a fake Steam with
// game 100 running.
func flowEngine(t *testing.T, edit func(*settings.Settings)) (e *Engine, stopGame func()) {
	root, proc := t.TempDir(), t.TempDir()
	writeFile(t, filepath.Join(root, "steamapps", "appmanifest_100.acf"), `"AppState" { "name" "Some Game" }`)
	writeFile(t, filepath.Join(root, "userdata", "1", "config", "localconfig.vdf"),
		`"UserLocalConfigStore" { "Software" { "Valve" { "Steam" { "apps" { "100" { "LastPlayed" "1000" } } } } } }`)
	writeFile(t, filepath.Join(proc, "4242", "cmdline"), "reaper\x00SteamLaunch\x00AppId=100\x00--\x00game\x00")

	st, err := settings.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	st.Update(edit)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	m := conn.New(func() (conn.Bus, error) { return fakeBus{}, nil }, nil)
	e = New(&Engine{Store: st, Conn: m, Sys: sysinfo.New(), Steam: &steam.Client{Root: root, Proc: proc}})
	e.ApplySettings()
	go m.Run(ctx)
	return e, func() { os.RemoveAll(filepath.Join(proc, "4242")) }
}

// settle steps the engine until the frame showing want is on the display.
func settle(t *testing.T, e *Engine, want string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		e.step(context.Background())
		e.mu.Lock()
		done := !e.sending && e.lastFrame != nil && e.shownGame == want && !e.force
		e.mu.Unlock()
		if done {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("display never showed %q (showing %q)", want, e.shownGame)
}

func TestPinnedGameShowsLastPlayedWhenGameCloses(t *testing.T) {
	e, stopGame := flowEngine(t, func(s *settings.Settings) { s.PinnedScreen = "game" })
	settle(t, e, "100 true")
	playing := e.lastFrame

	e.GameEvent(1628350, false) // a tool, not the game: ignored
	if e.stopped != 0 {
		t.Fatal("a stop notice for another app must be ignored")
	}

	// Steam reports the exit at once; the game's processes take a while.
	e.GameEvent(100, false)
	settle(t, e, "100 false")
	if string(e.lastFrame) == string(playing) {
		t.Fatal("the display still shows the Now playing frame")
	}

	// Started again: the new session shows as soon as it's seen.
	e.GameEvent(100, true)
	e.refreshGame()
	settle(t, e, "100 true")

	stopGame() // without Steam's report, the process going away is enough
	e.refreshGame()
	settle(t, e, "100 false")
}

func TestSuspendWhilePlayingShowsLastPlayed(t *testing.T) {
	e, _ := flowEngine(t, func(s *settings.Settings) { s.PinnedScreen, s.SuspendScreen = "game", "last_played" })
	settle(t, e, "100 true")
	playing := e.lastFrame

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	e.Sleep(ctx, false) // the game is still running as the Deck sleeps
	if !e.asleep || string(e.lastFrame) == string(playing) {
		t.Fatal("suspend should replace Now playing with Last played")
	}
	want := e.sleepImage(ctx, e.Store.Get(), "last_played", "")
	if string(e.lastFrame) != string(render.Pack(want)) {
		t.Fatal("the suspend frame isn't the Last played screen")
	}

	e.step(ctx)
	if e.sending {
		t.Fatal("updates must stay paused while asleep")
	}
	e.Wake()
	settle(t, e, "100 true")
}

func TestSteamSuspendRequestThenLogind(t *testing.T) {
	e, _ := flowEngine(t, func(s *settings.Settings) { s.SuspendScreen = "steam_logo" })
	settle(t, e, "100 true")

	e.SuspendSoon() // Steam asks first
	deadline := time.Now().Add(5 * time.Second)
	for {
		e.mu.Lock()
		started := e.asleep
		e.mu.Unlock()
		if started {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("Steam's suspend request didn't start the suspend screen")
		}
		time.Sleep(5 * time.Millisecond)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	e.Sleep(ctx, false) // logind follows: waits for the same frame, sends no second one
	e.mu.Lock()
	gen, frame := e.sleepGen, e.lastFrame
	e.mu.Unlock()
	if gen != 1 || string(frame) != string(render.Pack(steamLogo(false))) {
		t.Fatalf("expected one suspend screen, got %d", gen)
	}
}
