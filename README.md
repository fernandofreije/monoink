# monoink

> [!NOTE]
> **This is a personal fork of [v1k0d3n/monoink](https://github.com/v1k0d3n/monoink)
> by Brandon B. Jozsa.** All credit for monoink goes to the original
> project; this fork only adds a few changes for my own use, and is not an
> official release or a replacement for it. The rest of this README is the
> upstream text: "I" there means the original author, and its links (issues,
> releases, the install URL) point to the upstream project. To use the
> original, install it from upstream.
>
> **What this fork changes:**
>
> - **Show game while playing works in every mode.** Upstream only switches
>   to the game screen when rotating. Here a running game also takes over a
>   pinned (**Always …**) screen, which comes back when the game exits. The
>   toggle is shown in every mode.
> - **"Now playing" goes back to "Last played" after a session.** The game
>   screen and dashboard redraw as soon as a game starts or exits, instead
>   of showing "Now playing" for up to five more minutes.
> - **Sleep & shutdown screens.** A new panel section chooses, separately
>   for suspend and for shutdown, what stays on the display: the last screen
>   as it was (upstream behavior, the default), the last played game, an
>   image of your choice, or the Steam logo. monoink briefly delays sleep and
>   shutdown through systemd-logind (5 seconds at most by default) so the
>   picture reaches the display first, and goes back to normal on wake.
>
> To install the fork, build it with `./package.sh` (see
> [For developers](#for-developers)) and use **Install Plugin from ZIP
> File** in Decky's Developer tab with `out/monoink.zip`.

**An open-source, privacy-first Decky plugin for the JSAUX E-Ink faceplate
on SteamOS.** Show the time, weather, what you're playing, system stats and
your own photos on the 5.83" e-paper display, configured entirely from
Gaming Mode. No terminal, no sudo, no tracking.

<p align="center">
  <img src="docs/images/screens/dashboard.png" width="45%" alt="Dashboard screen">
  <img src="docs/images/screens/game.png" width="45%" alt="Game screen">
  <br>
  <img src="docs/images/screens/weather.png" width="45%" alt="Weather screen">
  <img src="docs/images/screens/clock.png" width="45%" alt="Clock screen">
  <br>
  <img src="docs/images/screens/photo.png" width="45%" alt="Photo frame screen, photo shown whole">
  <img src="docs/images/screens/photo-fill.png" width="45%" alt="Photo frame screen, photo filling the display">
</p>

> **Status: actively developed.** See
> [Releases](https://github.com/v1k0d3n/monoink/releases) for the latest
> version and what's new. Tested on a Steam Machine running a SteamOS preview
> build. It should work on any SteamOS device with Decky Loader (including
> Steam Deck), but those aren't tested yet.
> [Reports welcome](https://github.com/v1k0d3n/monoink/issues/new/choose).
>
> monoink is an independent community project. It is not affiliated with or
> endorsed by JSAUX.

## Install

You need [Decky Loader](https://decky.xyz) (the plugin system for SteamOS)
and the faceplate switched on. Don't pair the faceplate in Steam's
Bluetooth settings; monoink finds it by itself.

1. In Gaming Mode, press **⋯** → **Decky** (plug icon) → **⚙ Settings** and
   turn on **Developer mode**.
2. Open Decky's new **Developer** tab → **Install Plugin from URL** and
   enter:

   ```
   https://github.com/v1k0d3n/monoink/releases/latest/download/monoink.zip
   ```

3. Open **E-Ink Faceplate** in the Decky menu. It finds your display by
   itself, and within a few seconds the faceplate shows the dashboard.

New to Decky, prefer downloading the zip, or need to update or uninstall?
The **[install guide](docs/install.md)** covers everything step by step,
including installing Decky itself. If you used JSAUX's own installer
before, run their uninstaller first.

## What it does

- **Screens:** clock, calendar, weather, performance (CPU/GPU/RAM and
  temperatures), the game you're playing or last played with its cover art,
  a photo frame, and a combined dashboard.
- **Rotate or pin** screens, and switch to the game automatically while
  you play, in either mode.
- **Sleep & shutdown screens:** keep the last screen, or show your last
  played game, your own image or the Steam logo while the device is off.
- **Custom cards:** your own scripts can put status information on the
  display (build status, smart-home sensors, anything) once you approve
  them. See **[docs/providers.md](docs/providers.md)**.
- **Reliable connection:** finds the faceplate in about a second,
  reconnects on its own whenever the connection drops, and remembers your
  display so a neighbour's is never picked up.

## Why this project exists

I bought the faceplate and found the official software didn't work on a
current SteamOS build. When I looked into why, I found design choices I
wasn't comfortable with. **I couldn't find a public source repository for
the official software**; it ships as an installer containing Python code,
which is what I reviewed. Rather than patch it, I wrote monoink from
scratch with different goals:

- **It should just work**, from Gaming Mode, for people who never open a
  terminal.
- **It should be impossible to break with a SteamOS update.**
- **It should never touch anything it doesn't need**, and tell you exactly
  what it does use.
- **It should be open**, so anyone can check those claims.

No JSAUX code is included. monoink speaks the same Bluetooth protocol, which
I learned from their package (see [NOTICE](NOTICE)).

## How it compares

Compared with the official **JSAUX E-INK V1.1** package (installer revision
*OneClick-r3*, September 2026), which is the version I examined:

| | JSAUX E-INK V1.1 | monoink |
| --- | --- | --- |
| **Source code** | No public repository found; Python files inside an installer | Open source (MIT) on GitHub, with tests and public CI builds |
| **Installation** | Unzip in Desktop Mode, run a `.desktop` installer, approve an admin prompt | Install a zip from Decky in Gaming Mode |
| **Admin rights** | Uses `sudo`. If your account has no password, the installer **sets a temporary random password on it** to get sudo, then removes it | Never uses sudo or asks for a password (Decky Loader's own one-time installer may) |
| **System changes** | Adds a udev rule in `/etc` for raw touchscreen access, a background service in your user's systemd folder, desktop shortcuts and icons | None outside Decky's own plugin and settings folders |
| **Uninstalling** | On my machine, left the background service enabled and restarting every two seconds, plus the `/etc` rule and backup folders | Uninstall in Decky; only the settings folder remains |
| **Works on current SteamOS** | No: its bundled Python libraries are built only for Python 3.11/3.13, and current SteamOS ships 3.14, so the background service fails to start and the plugin shows "off" | Yes: one self-contained program with no Python libraries or system dependencies |
| **Bluetooth reliability** | A failed connection isn't retried until you toggle it again; a saved display that isn't found isn't searched for | Never gives up while enabled: retries automatically with backoff and explains what's wrong in plain language |
| **Update speed** | Reconnects for every frame | Keeps the connection open; about 3 seconds per update |
| **AI-assistant data** | Reads your **Codex/ChatGPT login file** (`~/.codex/auth.json`), refreshes the login tokens and **writes them back into that file**, fetches the titles of your **20 most recent ChatGPT conversations**, and scans your Codex session logs. This runs whenever the plugin's panel polls for status, whether or not you use that feature, and nothing asks for consent | Never reads other apps' files or accounts. Integrations are separate programs you choose to run, and nothing is shown until you approve them |
| **Your location** | Looks up your location from your IP address (`ipwho.is`) automatically, for the calendar and weather | Nothing is looked up until you type a city for weather |
| **Local network interface** | A web API on `127.0.0.1:39062` with no authentication or origin checks; it broadcasts status (including the AI-assistant data above) to any connected client, and can list image files in any folder | Private sockets readable only by your user account; no network port is opened |
| **Temporary files** | Lock files in `/tmp` created world-writable (`0666`) | None in shared locations |
| **Input devices** | The desktop app opens the touchscreen, the Steam Controller puck's keyboard interface and gamepads directly, and reads their raw key, button and touch events while it's open, whichever app you're using; the installer grants this with a system-wide rule in `/etc`. Used to navigate its own window | Never opens input devices; only counts connected gamepads from the system's device list (names and types, never serial numbers) |

This describes their package as it was when I examined it; later JSAUX
releases may differ.

## Privacy and security

These are rules the code follows, and that every contribution must keep:

- **No sudo, no system changes.** monoink runs as your normal user and only
  writes to its own settings and cache folders.
- **Nothing about you leaves the device unless you turn it on.** Each
  network request below is listed with what triggers it.
- **No access to other apps' data.** monoink never reads other programs'
  files, passwords or accounts.
- **Approval for integrations.** Programs that send cards are held until you
  approve them, and you can revoke them at any time.

Everything monoink reads or contacts is below. **[docs/privacy.md](docs/privacy.md)** has the full accounting: what it stores and where, what it never does, and commands to check all of it yourself.

| Source | When |
| --- | --- |
| BlueZ (the system Bluetooth service) | always: finding and talking to the display |
| systemd-logind (the system login service) | always: noticing sleep, wake and shutdown, to show the screen chosen for them |
| `/proc`, `/sys` | performance screen: CPU, memory, temperatures, GPU load |
| `/proc/bus/input/devices` (device names and types only, never serial numbers) | game screen: whether a game controller is connected |
| Command lines of running programs | game screen: finding the game Steam launched (only its app number and start time are kept) |
| Steam's local files (`steamapps/*.acf`, `userdata/*/config/localconfig.vdf`, `appcache/librarycache`) | game screen: name, playtime, cached cover art |
| Your region setting (`~/.config/plasma-localerc`, `LANG`/`LC_TIME`, `/etc/locale.conf`) | calendar: first day of the week, when set to Automatic |
| The image you choose for suspend or shutdown | only when you choose one |
| The photo folder you choose | photo frame |
| `api.open-meteo.com`, `geocoding-api.open-meteo.com` | only after you set a weather location |
| `cdn.akamai.steamstatic.com` | only if you enable "Download missing cover art" |

## Related projects

- **[pixel-faceplate](https://github.com/hodapp/pixel-faceplate)**: an
  independent Decky plugin for JSAUX's **Dot Matrix** faceplate (a different
  device that connects over USB rather than Bluetooth).

## Documentation

- [Installing](docs/install.md): step-by-step install, update and troubleshooting.
- [Screens](docs/screens.md): every screen, in light and dark mode.
- [Privacy](docs/privacy.md): everything monoink reads, stores and contacts, and how to verify it.
- [Custom cards](docs/providers.md): show your own information on the display.
- [Development](docs/development.md): building, testing, CI and releases.
- [Contributors](docs/contributors.md): who built this and how to help.

## For developers

Only `podman` (preinstalled on SteamOS) or `docker` is needed; the whole
toolchain is defined in [`Containerfile`](Containerfile).

```bash
./package.sh          # test and build → out/monoink.zip
```

The plugin's background program, `monoinkd`, also works from a terminal:

```bash
monoinkd probe -test-pattern     # find the display, show its info, draw a test pattern
monoinkd send picture.jpg        # dither and display an image
monoinkd push -id my-tool -title "Build" -line "main: passing"   # send a card
```

## License

MIT. See [LICENSE](LICENSE) and [NOTICE](NOTICE).

---

**Disclaimer:** I used Claude Code (Opus 5.5) to write this project. If there
are any issues, concerns, or comments, please
[create an issue](https://github.com/v1k0d3n/monoink/issues/new/choose) or
[open a pull request](https://github.com/v1k0d3n/monoink/compare).
