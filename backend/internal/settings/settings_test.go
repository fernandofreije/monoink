package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenCreatesDefaultsAndPersists(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := st.Get()
	if !got.Enabled || got.RotateMinutes != 10 || len(got.WebUI.Token) != 48 {
		t.Fatalf("unexpected defaults: %+v", got)
	}
	if _, err := st.Update(func(s *Settings) {
		s.Screens = []string{"clock", "bogus", "clock", "weather"}
		s.PinnedScreen = "nope"
		s.RotateMinutes = -1
	}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(dir, "settings.json"))
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("settings file mode: %v %v", info, err)
	}

	again, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	s := again.Get()
	if len(s.Screens) != 2 || s.Screens[0] != "clock" || s.Screens[1] != "weather" {
		t.Errorf("screens not normalized: %v", s.Screens)
	}
	if s.PinnedScreen != "" || s.RotateMinutes != 10 {
		t.Errorf("invalid values kept: %+v", s)
	}
	if s.WebUI.Token != got.WebUI.Token {
		t.Error("token regenerated on reload")
	}
}

func TestCorruptFileIsSetAside(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "settings.json"), []byte("{nope"), 0o600)
	st, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !st.Get().Enabled {
		t.Error("defaults not applied")
	}
	if _, err := os.Stat(filepath.Join(dir, "settings.json.invalid")); err != nil {
		t.Error("corrupt file not preserved")
	}
}

func TestEmptyScreensEncodeAsArray(t *testing.T) {
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	got, _ := st.Update(func(s *Settings) { s.Screens = nil })
	b, _ := json.Marshal(got)
	if !strings.Contains(string(b), `"screens":[]`) {
		t.Fatalf("screens must encode as an empty array: %s", b)
	}
	if b, _ := json.Marshal(st.Get()); !strings.Contains(string(b), `"screens":[]`) {
		t.Fatalf("Get must not return nil screens: %s", b)
	}
}

func TestSleepScreensNormalized(t *testing.T) {
	st, _ := Open(t.TempDir())
	if s := st.Get(); s.SuspendScreen != "" || s.ShutdownScreen != "" {
		t.Errorf("sleep screens should default to leaving the display alone: %+v", s)
	}
	s, _ := st.Update(func(s *Settings) { s.SuspendScreen, s.ShutdownScreen = "image", "fireworks" })
	if s.SuspendScreen != "image" || s.ShutdownScreen != "" {
		t.Errorf("got suspend %q, shutdown %q", s.SuspendScreen, s.ShutdownScreen)
	}
}

func TestGameLayoutNormalized(t *testing.T) {
	st, _ := Open(t.TempDir())
	if got := st.Get().GameLayout; got != "cover" {
		t.Errorf("default layout %q", got)
	}
	s, _ := st.Update(func(s *Settings) { s.GameLayout = "sideways" })
	if s.GameLayout != "cover" {
		t.Errorf("invalid layout kept: %q", s.GameLayout)
	}
	s, _ = st.Update(func(s *Settings) { s.GameLayout = "timer" })
	if s.GameLayout != "timer" {
		t.Errorf("timer not kept: %q", s.GameLayout)
	}
}

func TestWeekStartMigration(t *testing.T) {
	for _, c := range []struct {
		file string
		want string
	}{
		{`{"week_starts_sunday": true}`, "sunday"}, // explicit old choice kept
		{`{"week_starts_sunday": false}`, "auto"},  // old default becomes Automatic
		{`{}`, "auto"},
		{`{"week_start": "saturday"}`, "saturday"},
		{`{"week_start": "someday"}`, "auto"},
	} {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "settings.json"), []byte(c.file), 0o600)
		st, err := Open(dir)
		if err != nil {
			t.Fatal(err)
		}
		if got := st.Get(); got.WeekStart != c.want || got.WeekStartsSun {
			t.Errorf("%s: week_start=%q legacy=%v, want %q", c.file, got.WeekStart, got.WeekStartsSun, c.want)
		}
	}
}
