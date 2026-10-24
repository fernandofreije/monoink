# Privacy and transparency

monoink exists because the faceplate's official software reached much
further into the system than a display driver needs to. This page is the
full accounting of what monoink does instead: everything it reads, stores
and contacts, everything it never does, and how to check all of it yourself
without reading the code.

## What monoink reads

| What | Why | Details |
| --- | --- | --- |
| BlueZ, the system Bluetooth service (D-Bus) | Finding and talking to the display | Standard access any user has. No pairing, no admin rights. |
| systemd-logind (D-Bus) | Sleep & shutdown screens | Listens for the system going to sleep, waking and shutting down, and briefly delays sleep and shutdown (at most logind's limit, 5 seconds by default) so the chosen picture reaches the display first. Standard access any user has. |
| `/proc/stat`, `/proc/meminfo`, `/proc/uptime` | Performance screen | CPU, memory and uptime. |
| `/sys/class/hwmon`, `/sys/class/drm/*/device/gpu_busy_percent`, `/sys/class/power_supply` | Performance screen | Temperatures, GPU load, and the system battery if there is one (peripheral batteries such as mice are ignored). |
| `/proc/bus/input/devices` | Game screen controller icon | Device names, types and system paths only. **Serial numbers are never read**, and input devices are never opened (see below). |
| Command lines of running programs (`/proc/<pid>/cmdline`) | Game screen: which game is running | Each command line is checked for Steam's `SteamLaunch AppId=` marker. Only that number is kept; nothing else is stored or logged. |
| Start time of that one process (`/proc/<pid>/stat`) and the boot time (`/proc/stat`) | Game screen and dashboard: session time | Read only for the process Steam launched the game with, to work out when the session began. Not stored. |
| Steam's local files: `steamapps/libraryfolders.vdf`, `steamapps/appmanifest_*.acf`, `userdata/*/config/localconfig.vdf`, `appcache/librarycache/` | Game screen | Game names, total playtime and last-played time, and cover art Steam has already cached. `localconfig.vdf` contains other Steam settings too; only the per-game playtime and last-played values are used. |
| Your region setting: `~/.config/plasma-localerc` (KDE's Region & Language), the `LANG`/`LC_TIME` environment variables, and `/etc/locale.conf` | Calendar: which day the week starts on, when set to Automatic | Only the region code (for example `US`) is used. Never looked up online. |
| The image you choose for suspend or shutdown | Sleep & shutdown screens | That one file. Only read, never changed. |
| The photo folder you choose | Photo frame | Image files in that one folder (not subfolders). Your files are only read, never changed. |

## What monoink stores

Everything lives in Decky's folders for this plugin, owned by your user.
Nothing is written anywhere else.

| File | Contents | Access |
| --- | --- | --- |
| `~/homebrew/settings/monoink/settings.json` | Your settings: screens, clock options, weather location (place name and coordinates), photo folder, your display's Bluetooth address, approved providers | Only your user (`0600`) |
| `~/homebrew/data/monoink/performance-history.json` | The last 30 minutes of CPU, GPU and memory load, for the Performance graph | Only your user (`0600`) |
| `~/homebrew/data/monoink/bin/monoinkd` | A copy of monoink's own program, which the plugin runs from here so Decky can replace the plugin's files during an update. Removed when you uninstall. | Your user |
| `~/homebrew/data/monoink/art/` | Cover art downloaded from Steam's CDN, **only** if you turn on "Download missing cover art" | Only your user |
| `~/homebrew/logs/monoink/monoinkd.log` | The service log: connection events, which screen was sent, errors. Includes your display's Bluetooth address. | Only your user (`0600`) |
| `/run/user/<uid>/monoink/` | Two sockets the panel and providers use to talk to the service. Removed on shutdown. | Only your user (`0700` folder, `0600` sockets) |

The other timestamped `*.log` files in `~/homebrew/logs/monoink/` are
written by Decky Loader itself, not by monoink.

**Removing everything:** uninstall the plugin in Decky, then delete
`~/homebrew/settings/monoink`, `~/homebrew/data/monoink` and
`~/homebrew/logs/monoink`.

## What monoink contacts on the internet

| Address | When | What's sent |
| --- | --- | --- |
| `geocoding-api.open-meteo.com` | When you search for a weather location | The text you typed |
| `api.open-meteo.com` | About every 30 minutes, **only** after you've set a weather location | That location's coordinates and your unit choice |
| `cdn.akamai.steamstatic.com` | **Only** if you turn on "Download missing cover art", for games without cached art | The game's public Steam app number |

Nothing else: no analytics, no telemetry, no crash reports, no update
checks, no accounts. Each request identifies itself as `monoink` with a link
to this project, and contains nothing about you beyond what's listed above.

## What monoink never does

- **Never uses admin rights.** No sudo, no password prompts, nothing
  written to `/etc` or other system folders.
- **Never opens input devices.** It can't see a key press, a button press,
  a touch or a mouse movement, from any device.
- **Never reads other apps' data or credentials.** Integrations are separate
  [providers](providers.md) that you run and approve; they can only send a
  card, never read anything from monoink.
- **Never listens on the network.** It opens no network ports. (The planned
  Desktop Mode settings page will be off by default and limited to this
  machine.)
- **Never collects or sends usage data.**

## Check it yourself

These commands work as your normal user, in Desktop Mode's Konsole, while
the plugin is running.

Is monoink listening on any network port? (No output means no.)

```bash
ss -lntup | grep monoinkd
```

Is it connected to anything on the internet right now? (Usually no output;
a short-lived connection to `api.open-meteo.com` appears during a weather
update if you've set a location.)

```bash
ss -tnp | grep monoinkd
```

Which files does it have open? (Expect only its log file, sockets, and
system handles.)

```bash
ls -l /proc/$(pgrep -f 'monoinkd serve' | head -1)/fd
```

Who can read its files? (Expect `-rw-------` and your username.)

```bash
ls -l ~/homebrew/settings/monoink ~/homebrew/data/monoink ~/homebrew/logs/monoink/monoinkd.log
```

## How this compares

The official JSAUX E-INK V1.1 package (installer revision *OneClick-r3*,
September 2026), as examined; later versions may differ.

| | JSAUX E-INK V1.1 | monoink |
| --- | --- | --- |
| **Input devices** | Its desktop app opens the Steam Deck touchscreen, the Steam Controller puck's keyboard interface and Steam's virtual gamepads directly, and reads their raw key, button and touch events while it's open, whichever app you're using. The installer grants this with a system-wide rule in `/etc/udev/rules.d`. The events are used to navigate its own window. | Never opens input devices. Counts connected gamepads from the system's device list only. |
| **AI-assistant data** | Reads the Codex/ChatGPT login file, refreshes its tokens and writes them back, fetches the titles of the 20 most recent ChatGPT conversations, and scans Codex session logs, whenever the panel polls for status, without asking. | Never reads other apps' files or accounts. |
| **Your location** | Looks up your location from your IP address (`ipwho.is`) automatically, for the calendar and weather, and saves it in its settings. | Nothing is looked up until you search for a city yourself. |
| **Local network interface** | A web API on `127.0.0.1:39062` with no authentication or origin checks. It broadcasts status, including the AI-assistant data above, to any client, and can list image files in any folder. | Two private sockets readable only by your user. No network port is opened. |
| **Admin rights** | Uses `sudo`. If the account has no password, the installer sets a temporary random one to obtain it, then removes it. | Never uses admin rights. |
| **Files outside its own folders** | A udev rule in `/etc`, a background service in your systemd folder, desktop shortcuts and icons, and world-writable lock files in `/tmp`. | None. |
| **Disclosure** | Its README lists the components it installs (including a "Steam Deck touchscreen access rule" and a "shared user daemon") but not what they can access, what's read, or what's sent and broadcast. | This page and the README list everything. |
