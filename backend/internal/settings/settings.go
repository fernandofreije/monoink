// Package settings persists user configuration as JSON with atomic writes.
package settings

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sync"
)

// Settings is the complete persisted configuration. Every field has a
// usable zero value or is filled in by Defaults.
type Settings struct {
	// Connection
	Enabled       bool   `json:"enabled"`
	DeviceAddress string `json:"device_address"` // empty = first faceplate found
	KeepConnected bool   `json:"keep_connected"` // hold the BLE link between frames

	// Screens
	Screens       []string `json:"screens"`        // rotation order of enabled screens
	PinnedScreen  string   `json:"pinned_screen"`  // non-empty disables rotation
	RotateMinutes int      `json:"rotate_minutes"` // time on each screen
	Clock24h      bool     `json:"clock_24h"`
	DarkMode      bool     `json:"dark_mode"`                    // white on black; photos unchanged
	WeekStart     string   `json:"week_start"`                   // "auto" (from region), "monday", "sunday", "saturday"
	WeekStartsSun bool     `json:"week_starts_sunday,omitempty"` // deprecated: read only to migrate to WeekStart
	YearProgress  bool     `json:"year_progress"`                // day/week of the year on the clock

	// Weather (no location = weather disabled; nothing is looked up automatically)
	Weather Weather `json:"weather"`

	// Game screen
	AllowSteamCDN    bool   `json:"allow_steam_cdn"`    // fetch missing cover art from Steam's CDN
	GameWhilePlaying bool   `json:"game_while_playing"` // switch to the game screen while one runs
	GameLayout       string `json:"game_layout"`        // "cover" (default) or "timer"

	// Suspend and shutdown: what stays on the display while the system is
	// off. One of SleepScreens; "" leaves the last screen as it was.
	SuspendScreen  string `json:"suspend_screen"`
	SuspendImage   string `json:"suspend_image"` // for "image"
	ShutdownScreen string `json:"shutdown_screen"`
	ShutdownImage  string `json:"shutdown_image"` // for "image"

	// Photo frame
	PhotoDir     string `json:"photo_dir"`
	PhotoMinutes int    `json:"photo_minutes"`
	PhotoFill    bool   `json:"photo_fill"` // crop to fill the screen instead of fitting

	// Provider cards
	Providers    map[string]Provider `json:"providers"`
	CardsPreempt bool                `json:"cards_preempt"` // show a fresh card immediately

	// Desktop web UI
	WebUI WebUI `json:"web_ui"`
}

type Weather struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Place     string  `json:"place"`
	Imperial  bool    `json:"imperial"`
}

// Configured reports whether a location has been chosen.
func (w Weather) Configured() bool { return w.Place != "" || w.Latitude != 0 || w.Longitude != 0 }

type Provider struct {
	Approved bool   `json:"approved"`
	Name     string `json:"name,omitempty"`
}

type WebUI struct {
	Enabled bool   `json:"enabled"`
	Port    int    `json:"port"`
	Token   string `json:"token"`
}

// AllScreens lists every screen ID in default rotation order.
var AllScreens = []string{"clock", "weather", "performance", "game", "calendar", "photo", "card", "dashboard"}

// SleepScreens lists the choices for SuspendScreen and ShutdownScreen.
var SleepScreens = []string{"", "last_played", "image", "steam_logo"}

// Defaults returns a fresh configuration.
func Defaults() Settings {
	return Settings{
		Enabled:          true,
		KeepConnected:    true,
		Screens:          []string{"dashboard", "clock", "game", "calendar"},
		RotateMinutes:    10,
		GameWhilePlaying: true,
		YearProgress:     true,
		PhotoMinutes:     15,
		Providers:        map[string]Provider{},
		WebUI:            WebUI{Port: 39063},
	}
}

func (s *Settings) normalize() {
	d := Defaults()
	if s.RotateMinutes <= 0 {
		s.RotateMinutes = d.RotateMinutes
	}
	if s.PhotoMinutes <= 0 {
		s.PhotoMinutes = d.PhotoMinutes
	}
	if s.Providers == nil {
		s.Providers = map[string]Provider{}
	}
	if s.GameLayout != "timer" {
		s.GameLayout = "cover"
	}
	switch s.WeekStart {
	case "auto", "monday", "sunday", "saturday":
	case "":
		// Settings from before week_start existed: an explicit "Sunday"
		// choice is kept; otherwise the old default becomes Automatic.
		s.WeekStart = "auto"
		if s.WeekStartsSun {
			s.WeekStart = "sunday"
		}
	default:
		s.WeekStart = "auto"
	}
	s.WeekStartsSun = false
	if !slices.Contains(SleepScreens, s.SuspendScreen) {
		s.SuspendScreen = ""
	}
	if !slices.Contains(SleepScreens, s.ShutdownScreen) {
		s.ShutdownScreen = ""
	}
	if s.WebUI.Port <= 0 || s.WebUI.Port > 65535 {
		s.WebUI.Port = d.WebUI.Port
	}
	valid := map[string]bool{}
	for _, id := range AllScreens {
		valid[id] = true
	}
	seen := map[string]bool{}
	screens := []string{}
	for _, id := range s.Screens {
		if valid[id] && !seen[id] {
			screens = append(screens, id)
			seen[id] = true
		}
	}
	s.Screens = screens
	if s.PinnedScreen != "" && !valid[s.PinnedScreen] {
		s.PinnedScreen = ""
	}
}

// Store guards a Settings value and its file.
type Store struct {
	path string
	mu   sync.Mutex
	cur  Settings
}

// Open loads dir/settings.json, creating it with defaults if missing.
func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	st := &Store{path: filepath.Join(dir, "settings.json"), cur: Defaults()}
	data, err := os.ReadFile(st.path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return nil, err
	default:
		if err := json.Unmarshal(data, &st.cur); err != nil {
			// Keep the broken file for inspection and start fresh.
			os.Rename(st.path, st.path+".invalid")
			st.cur = Defaults()
		}
	}
	st.cur.normalize()
	if st.cur.WebUI.Token == "" {
		st.cur.WebUI.Token = randomToken()
	}
	return st, st.save()
}

// Get returns a deep-enough copy of the current settings.
func (s *Store) Get() Settings {
	s.mu.Lock()
	defer s.mu.Unlock()
	return clone(s.cur)
}

// Update applies fn to a copy and persists the result.
func (s *Store) Update(fn func(*Settings)) (Settings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := clone(s.cur)
	fn(&next)
	next.normalize()
	prev := s.cur
	s.cur = next
	if err := s.save(); err != nil {
		s.cur = prev
		return clone(prev), err
	}
	return clone(next), nil
}

func (s *Store) save() error {
	data, err := json.MarshalIndent(s.cur, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func clone(in Settings) Settings {
	out := in
	out.Screens = append([]string{}, in.Screens...) // never nil: encodes as [] not null
	out.Providers = make(map[string]Provider, len(in.Providers))
	for k, v := range in.Providers {
		out.Providers[k] = v
	}
	return out
}

func randomToken() string {
	b := make([]byte, 24)
	rand.Read(b)
	return hex.EncodeToString(b)
}
