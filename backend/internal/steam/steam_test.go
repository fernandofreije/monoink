package steam

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseVDF(t *testing.T) {
	n, err := ParseVDF(strings.NewReader(`
"Root"
{
	// comment
	"Name"		"Quoted \"value\""
	"Child"	{ "Key" "1" }
}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := n.Str("root", "name"); got != `Quoted "value"` {
		t.Errorf("name = %q", got)
	}
	if got := n.Str("Root", "child", "KEY"); got != "1" {
		t.Errorf("nested = %q", got)
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	os.MkdirAll(filepath.Dir(path), 0o755)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCurrentGame(t *testing.T) {
	root := t.TempDir()
	proc := t.TempDir()
	c := &Client{Root: root, Proc: proc}
	write(t, filepath.Join(root, "steamapps", "appmanifest_100.acf"), `"AppState" { "name" "Old Game" }`)
	write(t, filepath.Join(root, "steamapps", "appmanifest_200.acf"), `"AppState" { "name" "New Game" }`)
	write(t, filepath.Join(root, "userdata", "1", "config", "localconfig.vdf"), `
"UserLocalConfigStore" { "Software" { "Valve" { "Steam" { "apps" {
	"100" { "LastPlayed" "1000" "Playtime" "60" }
	"200" { "LastPlayed" "2000" "Playtime" "90" }
	"300" { "LastPlayed" "9999" }
} } } } }`)
	write(t, filepath.Join(root, "appcache", "librarycache", "200", "library_600x900.jpg"), "x")

	g, ok := c.Current()
	if !ok || g.AppID != 200 || g.Running || g.Name != "New Game" || g.Playtime != 90*time.Minute {
		t.Fatalf("recent game wrong: %+v", g) // 300 is skipped: not installed
	}
	if g.ArtPath == "" {
		t.Error("art not found")
	}

	write(t, filepath.Join(root, "appcache", "librarycache", "100", "abc123", "library_header.jpg"), "x")
	write(t, filepath.Join(root, "appcache", "librarycache", "100", "def456", "library_capsule.jpg"), "x")
	if got := c.ArtPath(100); filepath.Base(got) != "library_capsule.jpg" {
		t.Errorf("hashed layout: got %q, want the portrait capsule", got)
	}

	if g, ok := c.App(100); !ok || g.Name != "Old Game" || g.Running || g.LastPlayed.Unix() != 1000 {
		t.Errorf("App(100): %+v, %v", g, ok)
	}
	if _, ok := c.App(300); ok {
		t.Error("App should refuse apps without a manifest")
	}

	write(t, filepath.Join(proc, "4242", "cmdline"), "reaper\x00SteamLaunch\x00AppId=100\x00--\x00game.exe\x00")
	g, ok = c.Current()
	if !ok || g.AppID != 100 || !g.Running || g.Name != "Old Game" {
		t.Fatalf("running game wrong: %+v", g)
	}
}

func TestParseStartTicks(t *testing.T) {
	// Field 22 is starttime. Program names can contain spaces and ')'.
	rest := " S 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 424242 21 22"
	for _, comm := range []string{"(game.exe)", "(My Game (DX12))", "(a ) b)"} {
		got, ok := parseStartTicks("1234 " + comm + rest)
		if !ok || got != 424242 {
			t.Errorf("comm %q: got %d ok=%v", comm, got, ok)
		}
	}
	for _, bad := range []string{"", "1234 game S 1 2", "1234 (x) S 1 2 3"} {
		if _, ok := parseStartTicks(bad); ok {
			t.Errorf("accepted malformed stat %q", bad)
		}
	}
}

func TestRunningGameSession(t *testing.T) {
	root, proc := t.TempDir(), t.TempDir()
	c := &Client{Root: root, Proc: proc}
	write(t, filepath.Join(root, "steamapps", "appmanifest_100.acf"), `"AppState" { "name" "Old Game" }`)
	write(t, filepath.Join(proc, "stat"), "cpu  1 2 3 4\nbtime 1700000000\nprocesses 10\n")
	write(t, filepath.Join(proc, "4242", "cmdline"), "reaper\x00SteamLaunch\x00AppId=100\x00--\x00game.exe\x00")
	// Started 3600.5 s after boot (360050 ticks at 100 Hz).
	write(t, filepath.Join(proc, "4242", "stat"), "4242 (reaper) S 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 360050 0 0")

	g, ok := c.Current()
	if !ok || !g.Running {
		t.Fatalf("running game not found: %+v", g)
	}
	want := time.Unix(1700000000, 0).Add(3600*time.Second + 500*time.Millisecond)
	if !g.Started.Equal(want) {
		t.Fatalf("started %v, want %v", g.Started, want)
	}
	if s := g.Session(want.Add(83 * time.Minute)); s != 83*time.Minute {
		t.Errorf("session %v", s)
	}
	if s := (Game{Running: false, Started: want}).Session(want.Add(time.Hour)); s != 0 {
		t.Errorf("a game that isn't running has no session, got %v", s)
	}
}
