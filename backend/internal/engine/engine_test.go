package engine

import (
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/v1k0d3n/monoink/backend/internal/proto"
	"github.com/v1k0d3n/monoink/backend/internal/settings"
	"github.com/v1k0d3n/monoink/backend/internal/steam"
)

func newTest(t *testing.T) *Engine {
	st, err := settings.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return New(&Engine{Store: st})
}

func TestDue(t *testing.T) {
	base := time.Date(2026, 1, 1, 10, 0, 30, 0, time.UTC)
	if !due(time.Minute, time.Time{}, base) {
		t.Error("first render must be due")
	}
	if due(time.Minute, base, base.Add(20*time.Second)) {
		t.Error("same minute should not be due")
	}
	if !due(time.Minute, base, base.Add(31*time.Second)) {
		t.Error("minute boundary should be due")
	}
	if due(0, base, base.Add(time.Hour)) {
		t.Error("zero cadence is event-driven only")
	}
}

func TestScreenPrecedence(t *testing.T) {
	e := newTest(t)
	e.Store.Update(func(s *settings.Settings) { s.Screens = []string{"clock", "weather"}; s.RotateMinutes = 1 })
	e.ApplySettings()
	s := e.Store.Get()
	t0 := time.Unix(600, 0) // slot 10 → index 0
	if got := e.activeScreen(t0, s, false); got != "clock" {
		t.Errorf("slot 10: %s", got)
	}
	if got := e.activeScreen(t0.Add(time.Minute), s, false); got != "weather" {
		t.Errorf("slot 11: %s", got)
	}
	if got := e.activeScreen(t0, s, true); got != "game" {
		t.Errorf("running game should take over: %s", got)
	}
	e.override, e.overrideTo = "calendar", t0.Add(30*time.Second)
	if got := e.activeScreen(t0, s, true); got != "calendar" {
		t.Errorf("Next override should beat the game: %s", got)
	}
	s.PinnedScreen = "dashboard"
	if got := e.activeScreen(t0, s, false); got != "dashboard" {
		t.Errorf("pin must beat the override and rotation: %s", got)
	}
	if got := e.activeScreen(t0, s, true); got != "game" {
		t.Errorf("running game should take over a pin too: %s", got)
	}
	s.GameWhilePlaying = false
	if got := e.activeScreen(t0, s, true); got != "dashboard" {
		t.Errorf("game took over with Show game while playing off: %s", got)
	}
	s.GameWhilePlaying = true
	if got := e.activeScreen(t0.Add(31*time.Second), settings.Settings{Screens: s.Screens, RotateMinutes: 1}, false); got != "clock" {
		t.Errorf("override not expired: %s", got)
	}
}

func TestChangingScreensCancelsOverride(t *testing.T) {
	e := newTest(t)
	e.ApplySettings()
	e.Show("calendar")
	e.ApplySettings() // unrelated save: override survives
	if e.override != "calendar" {
		t.Fatal("override dropped by unrelated settings change")
	}
	e.Store.Update(func(s *settings.Settings) { s.PinnedScreen = "clock" })
	e.ApplySettings()
	if e.override != "" {
		t.Fatal("pinning a screen must cancel the override")
	}
	if _, err := e.Next(); err == nil {
		t.Fatal("Next should refuse while pinned")
	}
}

func TestProviderApprovalFlow(t *testing.T) {
	e := newTest(t)
	in := CardInput{ID: "ci-status", Name: "CI", Title: "Builds", Lines: []string{"ok"}, TTL: 60}
	if err := e.PushCard(in); !errors.Is(err, ErrPending) {
		t.Fatalf("unapproved push: %v", err)
	}
	if e.freshCardLocked(time.Now()) != nil {
		t.Fatal("pending card must not be shown")
	}
	ps := e.Providers()
	if len(ps) != 1 || !ps[0].Pending {
		t.Fatalf("pending provider not listed: %+v", ps)
	}
	if err := e.SetProvider("ci-status", true); err != nil {
		t.Fatal(err)
	}
	if c := e.freshCardLocked(time.Now()); c == nil || c.Title != "Builds" {
		t.Fatal("approved card not shown")
	}
	if err := e.PushCard(in); err != nil {
		t.Fatalf("approved push: %v", err)
	}
	e.SetProvider("ci-status", false)
	if e.freshCardLocked(time.Now()) != nil || len(e.Store.Get().Providers) != 0 {
		t.Fatal("revoke did not remove provider")
	}
	if err := e.PushCard(CardInput{ID: "../etc"}); err == nil || errors.Is(err, ErrPending) {
		t.Fatal("bad id accepted")
	}
}

func TestPhotosAndLoad(t *testing.T) {
	dir := t.TempDir()
	img := image.NewGray(image.Rect(0, 0, 4, 4))
	img.SetGray(0, 0, color.Gray{Y: 255})
	for _, n := range []string{"b.png", "a.PNG"} {
		f, _ := os.Create(filepath.Join(dir, n))
		png.Encode(f, img)
		f.Close()
	}
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0o644)
	list := Photos(dir)
	if len(list) != 2 || filepath.Base(list[0]) != "a.PNG" {
		t.Fatalf("photos: %v", list)
	}
	if _, err := loadImage(list[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := loadImage(filepath.Join(dir, "notes.txt")); err == nil {
		t.Fatal("non-image decoded")
	}
}

func TestGameStateTracksStartAndExit(t *testing.T) {
	running := &steam.Game{AppID: 620, Running: true}
	exited := &steam.Game{AppID: 620}
	if gameState(running) == gameState(exited) {
		t.Error("a game exiting must change what the game screens show")
	}
	if gameState(nil) == gameState(exited) {
		t.Error("the first game must change what the game screens show")
	}
}

func TestSleepImage(t *testing.T) {
	e := newTest(t)
	s := e.Store.Get()
	black := func(img *image.Gray) (n int) {
		if img.Bounds() != image.Rect(0, 0, proto.Width, proto.Height) {
			t.Fatalf("wrong size %v", img.Bounds())
		}
		for _, v := range img.Pix {
			if v < 128 {
				n++
			}
		}
		return n
	}

	logo := black(e.sleepImage(context.Background(), s, "steam_logo", ""))
	if logo < 1000 || logo > len(e.sleepImage(context.Background(), s, "steam_logo", "").Pix)/2 {
		t.Fatalf("Steam logo looks wrong: %d black pixels", logo)
	}
	if got := black(e.sleepImage(context.Background(), s, "image", "/nonexistent.png")); got != logo {
		t.Error("an unreadable image should fall back to the Steam logo")
	}
	s.DarkMode = true
	if got := black(e.sleepImage(context.Background(), s, "steam_logo", "")); got != proto.Width*proto.Height-logo {
		t.Error("dark mode should invert the logo")
	}

	path := filepath.Join(t.TempDir(), "sleep.png")
	solid := image.NewGray(image.Rect(0, 0, proto.Width, proto.Height)) // all black
	f, _ := os.Create(path)
	png.Encode(f, solid)
	f.Close()
	if got := black(e.sleepImage(context.Background(), s, "image", path)); got != proto.Width*proto.Height {
		t.Errorf("custom image not shown (or inverted): %d black pixels", got)
	}
}

func TestSleepWithNothingChosenLeavesUpdatesRunning(t *testing.T) {
	e := newTest(t)
	e.Sleep(context.Background(), false) // default: leave the display as it is
	if e.asleep {
		t.Fatal("updates paused although no suspend screen is chosen")
	}
	e.asleep = true
	e.Wake()
	if e.asleep || !e.force {
		t.Fatal("Wake should resume updates and redraw")
	}
}
