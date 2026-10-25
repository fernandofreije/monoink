// Package api exposes the engine over HTTP on Unix sockets (and, when the
// user enables it, on 127.0.0.1 for the desktop web UI).
//
// Two separate listeners keep privileges apart:
//   - control: full settings and device control (Decky backend, web UI)
//   - providers: may only submit cards; unapproved providers are held
package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image/png"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/v1k0d3n/monoink/backend/internal/ble"
	"github.com/v1k0d3n/monoink/backend/internal/engine"
	"github.com/v1k0d3n/monoink/backend/internal/screens"
	"github.com/v1k0d3n/monoink/backend/internal/settings"
)

// Version is reported by /api/status.
var Version = "dev"

type Server struct {
	Engine *engine.Engine
	Log    *slog.Logger
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, err error) {
	writeJSON(w, code, map[string]string{"error": err.Error()})
}

func readJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, 64<<10))
	if err := dec.Decode(v); err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	return nil
}

// publicSettings hides the web UI token from API responses.
func publicSettings(s settings.Settings) settings.Settings {
	s.WebUI.Token = ""
	return s
}

type screenInfo struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

func screenList() []screenInfo {
	out := make([]screenInfo, 0, len(settings.AllScreens))
	for _, id := range settings.AllScreens {
		out = append(out, screenInfo{id, screens.All[id].Title})
	}
	return out
}

type candidate struct {
	Address   string `json:"address"`
	Name      string `json:"name"`
	RSSI      *int16 `json:"rssi,omitempty"`
	Connected bool   `json:"connected"`
	Battery   *int   `json:"battery,omitempty"`
}

func toCandidates(cs []ble.Candidate) []candidate {
	out := make([]candidate, 0, len(cs))
	for _, c := range cs {
		x := candidate{Address: c.Address, Name: c.Name, Connected: c.Connected}
		if c.HasRSSI {
			r := c.RSSI
			x.RSSI = &r
		}
		if c.Info != nil {
			b := c.Info.Battery
			x.Battery = &b
		}
		out = append(out, x)
	}
	return out
}

// Control returns the handler for the trusted control socket.
func (s *Server) Control() http.Handler {
	e := s.Engine
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/status", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{
			"version":    Version,
			"connection": e.Conn.Status(),
			"screen":     e.Current(),
			"screens":    screenList(),
			"settings":   publicSettings(e.Store.Get()),
			"week_start": e.AutoWeekStart(),
		})
	})

	mux.HandleFunc("GET /api/settings", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, publicSettings(e.Store.Get()))
	})

	// PATCH merges the given JSON object into the current settings.
	mux.HandleFunc("PATCH /api/settings", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(io.LimitReader(r.Body, 64<<10))
		if err != nil {
			writeErr(w, 400, err)
			return
		}
		// Decode onto a copy first so a bad patch changes nothing.
		patched := e.Store.Get()
		if err := json.Unmarshal(body, &patched); err != nil {
			writeErr(w, 400, fmt.Errorf("invalid settings: %w", err))
			return
		}
		next, err := e.Store.Update(func(cur *settings.Settings) {
			token := cur.WebUI.Token
			*cur = patched
			cur.WebUI.Token = token // never settable over the API
		})
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		e.ApplySettings()
		go e.RefreshWeather(context.Background())
		writeJSON(w, 200, publicSettings(next))
	})

	mux.HandleFunc("POST /api/scan", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		found, err := e.Conn.Scan(ctx)
		if err != nil {
			writeErr(w, 503, err)
			return
		}
		writeJSON(w, 200, toCandidates(found))
	})

	mux.HandleFunc("POST /api/reconnect", func(w http.ResponseWriter, r *http.Request) {
		e.Conn.Reconnect()
		e.Refresh()
		writeJSON(w, 200, e.Conn.Status())
	})

	mux.HandleFunc("POST /api/refresh", func(w http.ResponseWriter, r *http.Request) {
		e.Refresh()
		writeJSON(w, 200, map[string]bool{"ok": true})
	})

	mux.HandleFunc("POST /api/show", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Screen string `json:"screen"`
		}
		if err := readJSON(r, &req); err != nil {
			writeErr(w, 400, err)
			return
		}
		if err := e.Show(req.Screen); err != nil {
			writeErr(w, 400, err)
			return
		}
		writeJSON(w, 200, map[string]bool{"ok": true})
	})

	// Steam events the panel relays: Steam knows about them before the
	// system does.
	mux.HandleFunc("POST /api/steam/game", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			AppID   int  `json:"app_id"`
			Running bool `json:"running"`
		}
		if err := readJSON(r, &req); err != nil {
			writeErr(w, 400, err)
			return
		}
		if req.AppID <= 0 {
			writeErr(w, 400, errors.New("app_id is required"))
			return
		}
		e.GameEvent(req.AppID, req.Running)
		writeJSON(w, 200, map[string]bool{"ok": true})
	})

	mux.HandleFunc("POST /api/steam/{event}", func(w http.ResponseWriter, r *http.Request) {
		switch r.PathValue("event") {
		case "suspend":
			e.SuspendSoon()
		case "resume":
			e.Wake()
		default:
			writeErr(w, 404, errors.New("unknown event"))
			return
		}
		writeJSON(w, 200, map[string]bool{"ok": true})
	})

	mux.HandleFunc("POST /api/next", func(w http.ResponseWriter, r *http.Request) {
		id, err := e.Next()
		if err != nil {
			writeErr(w, 409, err)
			return
		}
		writeJSON(w, 200, map[string]string{"screen": id})
	})

	mux.HandleFunc("GET /api/preview/{screen}", func(w http.ResponseWriter, r *http.Request) {
		img, err := e.Preview(r.Context(), r.PathValue("screen"))
		if err != nil {
			writeErr(w, 404, err)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "no-store")
		png.Encode(w, img)
	})

	mux.HandleFunc("GET /api/photos", func(w http.ResponseWriter, r *http.Request) {
		list := engine.Photos(e.Store.Get().PhotoDir)
		names := make([]string, len(list))
		for i, p := range list {
			names[i] = filepath.Base(p)
		}
		writeJSON(w, 200, names)
	})

	mux.HandleFunc("POST /api/weather/search", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Query string `json:"query"`
		}
		if err := readJSON(r, &req); err != nil || strings.TrimSpace(req.Query) == "" {
			writeErr(w, 400, errors.New("query is required"))
			return
		}
		if e.Weather == nil {
			writeErr(w, 503, errors.New("weather is unavailable"))
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		places, err := e.Weather.Search(ctx, req.Query)
		if err != nil {
			writeErr(w, 502, err)
			return
		}
		type place struct {
			Label     string  `json:"label"`
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
		}
		out := make([]place, 0, len(places))
		for _, p := range places {
			out = append(out, place{p.Label(), p.Latitude, p.Longitude})
		}
		writeJSON(w, 200, out)
	})

	mux.HandleFunc("GET /api/providers", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, e.Providers())
	})
	mux.HandleFunc("POST /api/providers/{id}/{action}", func(w http.ResponseWriter, r *http.Request) {
		var approve bool
		switch r.PathValue("action") {
		case "approve":
			approve = true
		case "revoke":
		default:
			writeErr(w, 404, errors.New("action must be approve or revoke"))
			return
		}
		if err := e.SetProvider(r.PathValue("id"), approve); err != nil {
			writeErr(w, 400, err)
			return
		}
		writeJSON(w, 200, e.Providers())
	})
	return mux
}

// Providers returns the handler for the provider socket.
func (s *Server) Providers() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]string{"status": "ok", "version": Version})
	})
	mux.HandleFunc("POST /v1/card", func(w http.ResponseWriter, r *http.Request) {
		var in engine.CardInput
		if err := readJSON(r, &in); err != nil {
			writeErr(w, 400, err)
			return
		}
		switch err := s.Engine.PushCard(in); {
		case errors.Is(err, engine.ErrPending):
			writeJSON(w, 202, map[string]string{"status": "pending_approval", "message": err.Error()})
		case err != nil:
			writeErr(w, 400, err)
		default:
			writeJSON(w, 200, map[string]string{"status": "accepted"})
		}
	})
	return mux
}

// ListenUnix serves h on a fresh user-only Unix socket at path.
func ListenUnix(ctx context.Context, path string, h http.Handler, log *slog.Logger) error {
	if len(path) >= 108 { // sun_path limit on Linux
		return fmt.Errorf("socket path is too long (%d bytes, max 107): %s", len(path), path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	os.Remove(path) // stale socket from a previous run
	l, err := net.Listen("unix", path)
	if err != nil {
		return err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		l.Close()
		return err
	}
	srv := &http.Server{Handler: h, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		sctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		srv.Shutdown(sctx)
		os.Remove(path)
	}()
	log.Info("listening", "socket", path)
	if err := srv.Serve(l); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
