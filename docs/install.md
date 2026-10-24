# Installing monoink

monoink is a plugin for [Decky Loader](https://decky.xyz), the plugin
system for SteamOS. If you already have Decky, skip to
[part 2](#2-turn-on-decky-developer-mode).

**You'll need:** a SteamOS device (Steam Machine or Steam Deck), the JSAUX
E-Ink faceplate switched on, and an internet connection.

**You won't need:** to pair the faceplate in Steam's Bluetooth settings
(leave it unpaired; monoink finds it by itself), a terminal, or any
commands.

If you previously installed JSAUX's own software, remove it first with
their **Uninstall JSAUX E-INK** shortcut, so two programs don't fight over
the display.

## 1. Install Decky Loader (once)

Decky is installed from Desktop Mode. This is the only part that happens
outside Gaming Mode.

1. Press the **Steam** button → **Power** → **Switch to Desktop**.
2. Open a web browser (Firefox is in the app menu, bottom-left) and go to
   **[decky.xyz](https://decky.xyz)**.
3. Click **Download**, then open the downloaded installer from your
   **Downloads** folder (double-click it; choose **Execute** or **Continue**
   if asked).
4. When it asks which version, choose the latest **release** (not a
   pre-release).
5. When it finishes, double-click **Return to Gaming Mode** on the desktop.

Decky's installer may ask you to create an administrator ("sudo")
password if your device doesn't have one. That's a one-time requirement of
Decky itself; monoink never asks for a password or uses admin rights.

## 2. Turn on Decky Developer mode

Decky only installs plugins from its own store unless Developer mode is on.
monoink isn't in the store yet.

1. In Gaming Mode, press the **⋯** (Quick Access) button.
2. Open the **Decky** tab (the plug icon) and select the **⚙ gear**.
3. Under **General**, turn on **Developer mode**.

## 3. Install monoink

Choose **one** of these.

### Easiest: install from a link

1. In Decky's settings, open the new **Developer** tab.
2. Under **Install Plugin from URL**, enter:

   ```
   https://github.com/v1k0d3n/monoink/releases/latest/download/monoink.zip
   ```

   (Press the **Steam + X** buttons to bring up the keyboard if it doesn't
   appear.)
3. Select **Install** and confirm.

This link always points at the newest release.

Installing from a link can pause for a minute or two at the start: Decky
first reports the install to its own plugin-store server, and waits for it
even when that server is slow. Leave it running; pressing Cancel doesn't
stop it, and it continues on its own.

### Or: install from a downloaded file

1. Download `monoink-vX.Y.Z.zip` from the
   [Releases page](https://github.com/v1k0d3n/monoink/releases/latest)
   (under **Assets**) into your **Downloads** folder, for example from
   Desktop Mode's browser.
2. In Gaming Mode: Decky settings → **Developer** → **Install Plugin from
   ZIP File** → choose the zip.

## 4. Start using it

Open **⋯** → **Decky** → **E-Ink Faceplate**. Within a few seconds the
plugin shows **Searching…**, then **Connected** with the display's battery
level, and the faceplate shows the dashboard. The first display it connects
to is remembered, so a neighbour's faceplate will never be picked up by
mistake.

Everything is in the plugin panel:

- **On the display**: a live copy of what's on the faceplate, plus **Next
  screen** and **Redraw now**.
- **Screens**: choose **Rotate** or pin one screen with **Always …**, and
  pick which screens rotate. **Show game while playing** switches to the
  game screen while a Steam game is running, in either mode, and goes back
  to the rotation or your pinned screen when the game exits.
- **Clock & weather**: 12/24-hour clock, first day of the week, and your
  weather location (search by city name).
- **Photo frame**: choose a folder of pictures (PNG, JPEG, GIF or WebP).
- **Sleep & shutdown**: what stays on the display while the system is
  suspended or off: the last screen as it was, your last played game, an
  image of your choice, or the Steam logo.
- **Providers**: approve or remove programs that send status cards. See
  [providers.md](providers.md).

## What to expect from the display

- **It flashes when it changes.** E-ink panels like this one repaint the
  whole screen with a brief black-and-white flash on every update. That's
  normal, and it also stops old images from leaving faint "ghosts".
- **How often it changes:** Clock, Dashboard and Performance update every
  minute. Weather updates about every 30 minutes, the game screen every few
  minutes (and immediately when a game starts or exits), the calendar hourly, and
  photos on the interval you choose. Nothing is sent if the picture hasn't
  changed.
- **Each update takes about 3 seconds** over Bluetooth, sometimes longer
  when many wireless devices are nearby. **Redraw now** shows progress
  while it works.
- **The picture stays when it's off.** E-ink needs no power to hold an
  image, so the last screen remains visible if the device sleeps or the
  faceplate is switched off. Under **Sleep & shutdown** you can choose a
  different picture for those times.
- **The USB-C port on the faceplate is for power only.** Everything goes
  over Bluetooth, whether or not it's plugged in.

## Update

Install again using either method above. Your settings are kept.

## Troubleshooting

| What you see | Try this |
| --- | --- |
| **Retrying — Display not found** | Make sure the faceplate is switched on and close by. Tap **Find displays** to check it's visible. |
| **Retrying — Bluetooth is turned off** | Turn Bluetooth on in Steam's settings. |
| It connects to the wrong display | **Find displays** → pick the right one. |
| The panel says the service isn't running | Restart Decky (Decky settings → General), or reboot. |
| The faceplate shows an old picture | Tap **Redraw now** and wait for "Display updated". |
| Decky's install dialog keeps spinning | Installs from a link can take a minute or two (see above). If you're updating **from v0.1.1 with a ZIP file**, the install can stop part-way: use **Install Plugin from URL** instead, or restart Decky (Decky settings → General) and try again. Updates from v0.1.2 onward aren't affected. |

If it still doesn't work, please open an issue with the **Something isn't
working** form and attach the log file:
`~/homebrew/logs/monoink/monoinkd.log`. (In Desktop Mode, open the Dolphin
file manager, press **Ctrl+H** to show hidden folders, then go to
**homebrew → logs → monoink**.)

## Uninstall

**⋯** → **Decky** → **⚙ gear** → **Plugins** → **E-Ink Faceplate** →
**Uninstall**.

monoink doesn't change anything outside Decky's folders. Its settings stay
in `~/homebrew/settings/monoink/` in case you reinstall; delete that folder
to remove them too.
