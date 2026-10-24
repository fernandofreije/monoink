import { callable } from "@decky/api";

// Mirrors backend/internal/settings.Settings (JSON field names).
export interface Settings {
  enabled: boolean;
  device_address: string;
  keep_connected: boolean;
  screens: string[];
  pinned_screen: string;
  rotate_minutes: number;
  clock_24h: boolean;
  dark_mode: boolean;
  week_start: "auto" | "monday" | "sunday" | "saturday";
  year_progress: boolean;
  weather: { latitude: number; longitude: number; place: string; imperial: boolean };
  allow_steam_cdn: boolean;
  game_while_playing: boolean;
  game_layout: "cover" | "timer";
  suspend_screen: SleepScreen;
  suspend_image: string;
  shutdown_screen: SleepScreen;
  shutdown_image: string;
  photo_dir: string;
  photo_minutes: number;
  photo_fill: boolean;
  providers: Record<string, { approved: boolean; name?: string }>;
  cards_preempt: boolean;
  web_ui: { enabled: boolean; port: number };
}

// What stays on the display while the system sleeps or is off; "" leaves it as it was.
export type SleepScreen = "" | "last_played" | "image" | "steam_logo";

export type ConnState = "off" | "searching" | "connecting" | "connected" | "standby" | "backoff";

export interface Connection {
  state: ConnState;
  reason?: string;
  address?: string;
  name?: string;
  info?: { Battery: number; Width: number; Height: number };
  sending: boolean;
  progress: number;
  last_frame?: string;
  retry_at?: string;
}

export interface Status {
  version: string;
  connection: Connection;
  screen: string;
  screens: { id: string; title: string }[];
  settings: Settings;
  week_start: { region: string; source: string; day: string };
}

export interface Candidate {
  address: string;
  name: string;
  rssi?: number;
  connected: boolean;
  battery?: number;
}

export interface Place {
  label: string;
  latitude: number;
  longitude: number;
}

export interface Provider {
  id: string;
  name: string;
  approved: boolean;
  pending: boolean;
  card?: { title: string; lines?: string[]; updated: string };
}

export interface Info {
  provider_socket: string;
  log_file: string;
  settings_dir: string;
  home: string;
  running: boolean;
}

type Reply<T> = { ok: true; status: number; data: T } | { ok: false; status: number; error: string };

const rawApi = callable<[method: string, path: string, body?: unknown], Reply<unknown>>("api");
export const preview = callable<[screen: string], string | null>("preview");
export const info = callable<[], Info>("info");

async function api<T>(method: "GET" | "POST" | "PATCH", path: string, body?: unknown): Promise<T> {
  const r = (await rawApi(method, path, body)) as Reply<T>;
  if (!r.ok) throw new Error(r.error);
  return r.data;
}

export const getStatus = () => api<Status>("GET", "/api/status");
export const patchSettings = (patch: Partial<Settings>) => api<Settings>("PATCH", "/api/settings", patch);
export const scan = () => api<Candidate[]>("POST", "/api/scan");
export const reconnect = () => api<Connection>("POST", "/api/reconnect");
export const refresh = () => api<{ ok: boolean }>("POST", "/api/refresh");
export const next = () => api<{ screen: string }>("POST", "/api/next");
export const searchPlaces = (query: string) => api<Place[]>("POST", "/api/weather/search", { query });
export const providers = () => api<Provider[]>("GET", "/api/providers");
export const setProvider = (id: string, approve: boolean) =>
  api<Provider[]>("POST", `/api/providers/${encodeURIComponent(id)}/${approve ? "approve" : "revoke"}`);
