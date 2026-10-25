import {
  ButtonItem,
  DialogButton,
  DropdownItem,
  Field,
  Focusable,
  ModalRoot,
  PanelSection,
  PanelSectionRow,
  SliderField,
  Spinner,
  TextField,
  ToggleField,
  showModal,
  staticClasses,
} from "@decky/ui";
import { FileSelectionType, definePlugin, openFilePicker, toaster } from "@decky/api";
import { useCallback, useEffect, useState } from "react";
import { MdTabletAndroid } from "react-icons/md";

import * as api from "./api";
import type { Candidate, Place, Provider, Settings, SleepScreen, Status } from "./api";
import { relaySteamEvents } from "./steamEvents";

const ROTATE_STEPS = [1, 2, 5, 10, 15, 30, 60];
const PHOTO_STEPS = [1, 5, 10, 15, 30, 60];

const STATE_LABEL: Record<api.ConnState, string> = {
  off: "Off",
  searching: "Searching…",
  connecting: "Connecting…",
  connected: "Connected",
  standby: "Ready",
  backoff: "Retrying",
};

function notify(body: string) {
  toaster.toast({ title: "E-Ink Faceplate", body });
}

function errText(e: unknown): string {
  return e instanceof Error ? e.message : String(e);
}

function nearestIndex(steps: number[], value: number): number {
  let best = 0;
  steps.forEach((s, i) => {
    if (Math.abs(s - value) < Math.abs(steps[best] - value)) best = i;
  });
  return best;
}

function minutesLabel(m: number): string {
  return m >= 60 ? `${m / 60} h` : `${m} min`;
}

// ---- modals ----------------------------------------------------------------

function ScanModal({ closeModal, onPick }: { closeModal?: () => void; onPick: (addr: string) => void }) {
  const [found, setFound] = useState<Candidate[] | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    api.scan().then(setFound).catch((e) => setError(errText(e)));
  }, []);

  const pick = (addr: string) => {
    onPick(addr);
    closeModal?.();
  };

  return (
    <ModalRoot closeModal={closeModal}>
      <div className={staticClasses.Title} style={{ marginBottom: "12px" }}>
        Find displays
      </div>
      {found === null && !error && (
        <div style={{ display: "flex", gap: "8px", alignItems: "center" }}>
          <Spinner style={{ width: "24px" }} /> Scanning for nearby faceplates…
        </div>
      )}
      {error && <div>Scan failed: {error}</div>}
      {found && found.length === 0 && <div>No faceplates found. Make sure the display is switched on.</div>}
      <Focusable style={{ display: "flex", flexDirection: "column", gap: "8px" }}>
        {found?.map((c) => (
          <DialogButton key={c.address} onClick={() => pick(c.address)}>
            {c.name} · {c.address}
            {c.rssi !== undefined ? ` · signal ${c.rssi} dBm` : ""}
            {c.battery !== undefined ? ` · ${c.battery}%` : ""}
            {c.connected ? " · connected" : ""}
          </DialogButton>
        ))}
        <DialogButton onClick={() => pick("")}>Use any display (forget the saved one)</DialogButton>
      </Focusable>
    </ModalRoot>
  );
}

function LocationModal({ closeModal, onPick }: { closeModal?: () => void; onPick: (p: Place | null) => void }) {
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<Place[] | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  const search = async () => {
    if (!query.trim()) return;
    setBusy(true);
    setError("");
    try {
      setResults(await api.searchPlaces(query.trim()));
    } catch (e) {
      setError(errText(e));
    } finally {
      setBusy(false);
    }
  };

  const pick = (p: Place | null) => {
    onPick(p);
    closeModal?.();
  };

  return (
    <ModalRoot closeModal={closeModal}>
      <div className={staticClasses.Title} style={{ marginBottom: "8px" }}>
        Weather location
      </div>
      <div style={{ marginBottom: "8px", opacity: 0.8 }}>
        Search by city name. The search and forecasts use Open-Meteo; nothing else is sent.
      </div>
      <TextField label="City" value={query} onChange={(e) => setQuery(e.target.value)} />
      <Focusable style={{ display: "flex", flexDirection: "column", gap: "8px", marginTop: "8px" }}>
        <DialogButton onClick={search} disabled={busy}>
          {busy ? "Searching…" : "Search"}
        </DialogButton>
        {error && <div>{error}</div>}
        {results?.length === 0 && <div>No places found.</div>}
        {results?.map((p) => (
          <DialogButton key={`${p.latitude},${p.longitude}`} onClick={() => pick(p)}>
            {p.label}
          </DialogButton>
        ))}
        <DialogButton onClick={() => pick(null)}>Turn weather off</DialogButton>
      </Focusable>
    </ModalRoot>
  );
}

// ---- sleep & shutdown -------------------------------------------------------

const SLEEP_OPTIONS: { data: SleepScreen; label: string }[] = [
  { data: "", label: "Leave as it is" },
  { data: "last_played", label: "Last played game" },
  { data: "image", label: "An image" },
  { data: "steam_logo", label: "Steam logo" },
];

function SleepScreenRows(props: {
  label: string;
  screen: SleepScreen;
  image: string;
  onScreen: (v: SleepScreen) => void;
  onImage: (path: string) => void;
}) {
  const chooseImage = async () => {
    try {
      const start = props.image ? props.image.replace(/\/[^/]*$/, "") : `${(await api.info()).home}/Pictures`;
      const res = await openFilePicker(FileSelectionType.FILE, start);
      props.onImage(res.realpath || res.path);
    } catch {
      // picker cancelled
    }
  };

  return (
    <>
      <PanelSectionRow>
        <DropdownItem
          label={props.label}
          rgOptions={SLEEP_OPTIONS}
          selectedOption={props.screen}
          onChange={(o) => props.onScreen(o.data)}
        />
      </PanelSectionRow>
      {props.screen === "image" && (
        <PanelSectionRow>
          <ButtonItem
            layout="below"
            description={props.image ? props.image.split("/").pop() : "None chosen — the Steam logo is shown."}
            onClick={chooseImage}
          >
            Choose image
          </ButtonItem>
        </PanelSectionRow>
      )}
    </>
  );
}

// ---- main panel ------------------------------------------------------------

function Content() {
  const [status, setStatus] = useState<Status | null>(null);
  const [error, setError] = useState("");
  const [provs, setProvs] = useState<Provider[]>([]);
  const [image, setImage] = useState<string | null>(null);
  // Tracks a Redraw in progress: when it started and the frame it replaces.
  const [redraw, setRedraw] = useState<{ since: number; prevFrame?: string } | null>(null);

  const load = useCallback(async () => {
    try {
      setStatus(await api.getStatus());
      setProvs(await api.providers());
      setError("");
    } catch (e) {
      setError(errText(e));
    }
  }, []);

  useEffect(() => {
    load();
    const t = setInterval(load, redraw ? 500 : 2000);
    return () => clearInterval(t);
  }, [load, redraw !== null]);

  // Finish a Redraw once a new frame has landed (or give up after a minute).
  useEffect(() => {
    if (!redraw || !status) return;
    const c = status.connection;
    if (c.last_frame && c.last_frame !== redraw.prevFrame) {
      const secs = Math.max(1, Math.round((Date.now() - redraw.since) / 1000));
      notify(`Display updated (${secs} s)`);
      setRedraw(null);
    } else if (Date.now() - redraw.since > 60000) {
      notify(`The display didn't update${c.reason ? `: ${c.reason}` : "."}`);
      setRedraw(null);
    }
  }, [status, redraw]);

  // The preview mirrors what is on the display; refresh it whenever a new
  // frame lands or the screen changes.
  const onDisplay = status?.screen || "";
  const lastFrame = status?.connection.last_frame;
  useEffect(() => {
    if (onDisplay) api.preview(onDisplay).then(setImage);
  }, [onDisplay, lastFrame]);

  const patch = async (p: Partial<Settings>) => {
    try {
      await api.patchSettings(p);
      await load();
    } catch (e) {
      notify(`Couldn't save: ${errText(e)}`);
    }
  };

  if (!status) {
    return (
      <PanelSection>
        <PanelSectionRow>
          <div>{error ? `The display service isn't running: ${error}` : "Starting…"}</div>
        </PanelSectionRow>
      </PanelSection>
    );
  }

  const s = { ...status.settings, screens: status.settings.screens ?? [] };
  const c = status.connection;
  const titles = Object.fromEntries(status.screens.map((x) => [x.id, x.title]));
  const stateLine = [
    STATE_LABEL[c.state] ?? c.state,
    c.sending ? `sending ${c.progress}%` : "",
    c.info ? `battery ${c.info.Battery}%` : "",
  ]
    .filter(Boolean)
    .join(" · ");

  const toggleScreen = (id: string, on: boolean) => {
    const set = new Set(s.screens);
    if (on) set.add(id);
    else set.delete(id);
    patch({ screens: status.screens.map((x) => x.id).filter((x) => set.has(x)) });
  };

  const choosePhotoFolder = async () => {
    try {
      const res = await openFilePicker(FileSelectionType.FOLDER, s.photo_dir || `${(await api.info()).home}/Pictures`);
      await patch({ photo_dir: res.realpath || res.path });
    } catch {
      // picker cancelled
    }
  };

  return (
    <>
      <PanelSection title="Display">
        <PanelSectionRow>
          <ToggleField
            label="Enabled"
            description={c.reason && c.state !== "connected" ? `${stateLine} — ${c.reason}` : stateLine}
            checked={s.enabled}
            onChange={(v) => patch({ enabled: v })}
          />
        </PanelSectionRow>
        <PanelSectionRow>
          <Field label="Device" description={s.device_address || "First faceplate found"} />
        </PanelSectionRow>
        <PanelSectionRow>
          <ButtonItem
            layout="below"
            onClick={() => showModal(<ScanModal onPick={(addr) => patch({ device_address: addr })} />)}
          >
            Find displays
          </ButtonItem>
        </PanelSectionRow>
        <PanelSectionRow>
          <ButtonItem layout="below" onClick={() => api.reconnect().then(load)}>
            Reconnect
          </ButtonItem>
        </PanelSectionRow>
      </PanelSection>

      <PanelSection title="On the display">
        {image && (
          <PanelSectionRow>
            <img src={image} style={{ width: "100%", border: "1px solid #555", background: "#fff" }} />
          </PanelSectionRow>
        )}
        <PanelSectionRow>
          <Field
            label={titles[onDisplay] ?? "Nothing yet"}
            description={s.pinned_screen ? "Pinned in Screens below" : "Rotating"}
          />
        </PanelSectionRow>
        {!s.pinned_screen && (
          <PanelSectionRow>
            <ButtonItem
              layout="below"
              onClick={() =>
                api
                  .next()
                  .then((r) => notify(`Showing ${titles[r.screen] ?? r.screen}`))
                  .catch((e) => notify(errText(e)))
              }
            >
              Next screen
            </ButtonItem>
          </PanelSectionRow>
        )}
        <PanelSectionRow>
          <ButtonItem
            layout="below"
            disabled={redraw !== null}
            description="Fetches fresh info and redraws. Takes a few seconds over Bluetooth."
            onClick={() => {
              setRedraw({ since: Date.now(), prevFrame: c.last_frame });
              api.refresh().catch((e) => {
                notify(`Couldn't redraw: ${errText(e)}`);
                setRedraw(null);
              });
            }}
          >
            {redraw ? (c.sending ? `Updating display… ${c.progress}%` : "Preparing…") : "Redraw now"}
          </ButtonItem>
        </PanelSectionRow>
      </PanelSection>

      <PanelSection title="Screens">
        <PanelSectionRow>
          <ToggleField
            label="Dark mode"
            description="White on black. Photos and cover art keep their colors."
            checked={s.dark_mode}
            onChange={(v) => patch({ dark_mode: v })}
          />
        </PanelSectionRow>
        <PanelSectionRow>
          <DropdownItem
            label="Mode"
            rgOptions={[
              { data: "", label: "Rotate" },
              ...status.screens.map((x) => ({ data: x.id, label: `Always ${x.title}` })),
            ]}
            selectedOption={s.pinned_screen}
            onChange={(o) => patch({ pinned_screen: o.data })}
          />
        </PanelSectionRow>
        <PanelSectionRow>
          <ToggleField
            label="Show game while playing"
            description={`Switch to the Game screen while a Steam game is running, then back to ${
              s.pinned_screen ? titles[s.pinned_screen] : "the rotation"
            } when it exits.`}
            checked={s.game_while_playing}
            onChange={(v) => patch({ game_while_playing: v })}
          />
        </PanelSectionRow>
        <PanelSectionRow>
          <DropdownItem
            label="Game screen layout"
            description="Timer shows a large clock of how long you've been playing."
            rgOptions={[
              { data: "cover", label: "Cover art" },
              { data: "timer", label: "Session timer" },
            ]}
            selectedOption={s.game_layout}
            onChange={(o) => patch({ game_layout: o.data })}
          />
        </PanelSectionRow>
        {!s.pinned_screen && (
          <>
            <PanelSectionRow>
              <SliderField
                label="Time per screen"
                value={nearestIndex(ROTATE_STEPS, s.rotate_minutes)}
                min={0}
                max={ROTATE_STEPS.length - 1}
                step={1}
                notchCount={ROTATE_STEPS.length}
                notchTicksVisible
                validValues="steps"
                description={minutesLabel(s.rotate_minutes)}
                onChange={(i) => patch({ rotate_minutes: ROTATE_STEPS[i] })}
              />
            </PanelSectionRow>
            {s.screens.length === 0 && (
              <PanelSectionRow>
                <div style={{ fontSize: "12px" }}>No screens selected — the clock is shown.</div>
              </PanelSectionRow>
            )}
            {status.screens.map((x) => (
              <PanelSectionRow key={x.id}>
                <ToggleField
                  label={x.title}
                  checked={s.screens.includes(x.id)}
                  onChange={(v) => toggleScreen(x.id, v)}
                />
              </PanelSectionRow>
            ))}
          </>
        )}
      </PanelSection>

      <PanelSection title="Clock & weather">
        <PanelSectionRow>
          <ToggleField label="24-hour clock" checked={s.clock_24h} onChange={(v) => patch({ clock_24h: v })} />
        </PanelSectionRow>
        <PanelSectionRow>
          <DropdownItem
            label="Week starts on"
            rgOptions={[
              {
                data: "auto",
                label: `Automatic (${status.week_start.day}${status.week_start.region ? `, ${status.week_start.region}` : ""})`,
              },
              { data: "monday", label: "Monday" },
              { data: "sunday", label: "Sunday" },
              { data: "saturday", label: "Saturday" },
            ]}
            selectedOption={s.week_start}
            onChange={(o) => patch({ week_start: o.data })}
          />
        </PanelSectionRow>
        <PanelSectionRow>
          <ToggleField
            label="Show year progress"
            description="Day and week of the year on the Clock screen."
            checked={s.year_progress}
            onChange={(v) => patch({ year_progress: v })}
          />
        </PanelSectionRow>
        <PanelSectionRow>
          <Field label="Weather location" description={s.weather.place || "Off"} />
        </PanelSectionRow>
        <PanelSectionRow>
          <ButtonItem
            layout="below"
            onClick={() =>
              showModal(
                <LocationModal
                  onPick={(p) =>
                    patch({
                      weather: p
                        ? { ...s.weather, latitude: p.latitude, longitude: p.longitude, place: p.label }
                        : { ...s.weather, latitude: 0, longitude: 0, place: "" },
                    })
                  }
                />,
              )
            }
          >
            Set location
          </ButtonItem>
        </PanelSectionRow>
        <PanelSectionRow>
          <ToggleField
            label="Fahrenheit"
            checked={s.weather.imperial}
            onChange={(v) => patch({ weather: { ...s.weather, imperial: v } })}
          />
        </PanelSectionRow>
      </PanelSection>

      <PanelSection title="Photo frame">
        <PanelSectionRow>
          <Field label="Folder" description={s.photo_dir || "None selected"} />
        </PanelSectionRow>
        <PanelSectionRow>
          <ButtonItem layout="below" onClick={choosePhotoFolder}>
            Choose folder
          </ButtonItem>
        </PanelSectionRow>
        <PanelSectionRow>
          <ToggleField
            label="Fill the screen"
            description="Crop photos to fill the whole display instead of showing them whole."
            checked={s.photo_fill}
            onChange={(v) => patch({ photo_fill: v })}
          />
        </PanelSectionRow>
        <PanelSectionRow>
          <SliderField
            label="Next photo every"
            value={nearestIndex(PHOTO_STEPS, s.photo_minutes)}
            min={0}
            max={PHOTO_STEPS.length - 1}
            step={1}
            notchCount={PHOTO_STEPS.length}
            notchTicksVisible
            validValues="steps"
            description={minutesLabel(s.photo_minutes)}
            onChange={(i) => patch({ photo_minutes: PHOTO_STEPS[i] })}
          />
        </PanelSectionRow>
      </PanelSection>

      <PanelSection title="Providers">
        <PanelSectionRow>
          <div style={{ fontSize: "12px", opacity: 0.8 }}>
            Other apps can send status cards. Nothing is shown until you approve the app here.
          </div>
        </PanelSectionRow>
        {provs.length === 0 && (
          <PanelSectionRow>
            <div style={{ fontSize: "12px" }}>No providers yet.</div>
          </PanelSectionRow>
        )}
        {provs.map((p) => (
          <PanelSectionRow key={p.id}>
            <ButtonItem
              layout="below"
              label={p.name || p.id}
              description={
                p.pending ? `Waiting for approval${p.card ? `: “${p.card.title}”` : ""}` : `Approved (${p.id})`
              }
              onClick={() => api.setProvider(p.id, p.pending).then(setProvs)}
            >
              {p.pending ? "Approve" : "Revoke"}
            </ButtonItem>
          </PanelSectionRow>
        ))}
        <PanelSectionRow>
          <ToggleField
            label="Show new cards immediately"
            checked={s.cards_preempt}
            onChange={(v) => patch({ cards_preempt: v })}
          />
        </PanelSectionRow>
      </PanelSection>

      <PanelSection title="Sleep & shutdown">
        <PanelSectionRow>
          <div style={{ fontSize: "12px", opacity: 0.8 }}>
            What stays on the display while the system is asleep or switched off.
          </div>
        </PanelSectionRow>
        <SleepScreenRows
          label="When suspended"
          screen={s.suspend_screen}
          image={s.suspend_image}
          onScreen={(v) => patch({ suspend_screen: v })}
          onImage={(v) => patch({ suspend_image: v })}
        />
        <SleepScreenRows
          label="When shut down"
          screen={s.shutdown_screen}
          image={s.shutdown_image}
          onScreen={(v) => patch({ shutdown_screen: v })}
          onImage={(v) => patch({ shutdown_image: v })}
        />
      </PanelSection>

      <PanelSection title="Advanced">
        <PanelSectionRow>
          <ToggleField
            label="Stay connected"
            description="Faster updates. Turn off to connect only when the picture changes."
            checked={s.keep_connected}
            onChange={(v) => patch({ keep_connected: v })}
          />
        </PanelSectionRow>
        <PanelSectionRow>
          <ToggleField
            label="Download missing cover art"
            description="Fetches public artwork from Steam's CDN when it isn't cached locally."
            checked={s.allow_steam_cdn}
            onChange={(v) => patch({ allow_steam_cdn: v })}
          />
        </PanelSectionRow>
        <PanelSectionRow>
          <Field label="Version" description={status.version} />
        </PanelSectionRow>
      </PanelSection>
    </>
  );
}

export default definePlugin(() => {
  const ignore = () => {}; // the service may not be running yet
  const unregister = relaySteamEvents(SteamClient, {
    game: (appId, running) => api.steamGame(appId, running).catch(ignore),
    suspend: () => api.steamPower("suspend").catch(ignore),
    resume: () => api.steamPower("resume").catch(ignore),
  });
  return {
    name: "E-Ink Faceplate",
    titleView: <div className={staticClasses.Title}>E-Ink Faceplate</div>,
    content: <Content />,
    icon: <MdTabletAndroid />,
    onDismount: unregister,
  };
});
