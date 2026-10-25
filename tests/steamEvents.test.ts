// SteamClient differs between Steam client releases, and Decky's typings
// can't tell us which one a user has. These run the relay against the
// shapes we know of, and against ones missing or breaking anything, so the
// plugin keeps loading whatever Steam ships next.

import assert from "node:assert/strict";
import { test } from "node:test";

import { relaySteamEvents } from "../src/steamEvents.ts";

type Callback = (...args: unknown[]) => void;

// fakeSteam builds a SteamClient with only the given methods. Each one
// records its callback so a test can fire the event.
function fakeSteam(methods: Record<string, string[]>) {
  const callbacks: Record<string, Callback> = {};
  let unregistered = 0;
  const steam: Record<string, Record<string, (cb: Callback) => { unregister(): void }>> = {};
  for (const [ns, names] of Object.entries(methods)) {
    steam[ns] = {};
    for (const name of names) {
      steam[ns][name] = (cb) => {
        callbacks[`${ns}.${name}`] = cb;
        return { unregister: () => void unregistered++ };
      };
    }
  }
  return { steam, fire: (key: string, ...args: unknown[]) => callbacks[key](...args), callbacks, unregistered: () => unregistered };
}

function recorder() {
  const events: string[] = [];
  return {
    events,
    on: {
      game: (appId: number, running: boolean) => void events.push(`game ${appId} ${running}`),
      suspend: () => void events.push("suspend"),
      resume: () => void events.push("resume"),
    },
  };
}

test("relays every event with the System API", () => {
  const f = fakeSteam({
    GameSessions: ["RegisterForAppLifetimeNotifications"],
    System: ["RegisterForOnSuspendRequest", "RegisterForOnResumeFromSuspend"],
  });
  const r = recorder();
  const stop = relaySteamEvents(f.steam, r.on);
  f.fire("GameSessions.RegisterForAppLifetimeNotifications", { unAppID: 620, bRunning: true });
  f.fire("System.RegisterForOnSuspendRequest");
  f.fire("System.RegisterForOnResumeFromSuspend");
  assert.deepEqual(r.events, ["game 620 true", "suspend", "resume"]);
  stop();
  assert.equal(f.unregistered(), 3);
});

test("falls back to the User API when System's suspend events are gone", () => {
  const f = fakeSteam({
    GameSessions: ["RegisterForAppLifetimeNotifications"],
    System: [],
    User: ["RegisterForPrepareForSystemSuspendProgress", "RegisterForResumeSuspendedGamesProgress"],
  });
  const r = recorder();
  relaySteamEvents(f.steam, r.on);
  // progress is reported in steps; the service hears of the suspend once
  f.fire("User.RegisterForPrepareForSystemSuspendProgress", { state: 5 });
  f.fire("User.RegisterForPrepareForSystemSuspendProgress", { state: 1 });
  f.fire("User.RegisterForResumeSuspendedGamesProgress", { state: 1 });
  f.fire("User.RegisterForPrepareForSystemSuspendProgress", { state: 5 });
  assert.deepEqual(r.events, ["suspend", "resume", "suspend"]);
});

test("still relays games when no suspend events exist (v0.2.1 crash)", () => {
  const f = fakeSteam({ GameSessions: ["RegisterForAppLifetimeNotifications"], System: [] });
  const r = recorder();
  const stop = relaySteamEvents(f.steam, r.on);
  f.fire("GameSessions.RegisterForAppLifetimeNotifications", { unAppID: 620, bRunning: false });
  assert.deepEqual(r.events, ["game 620 false"]);
  stop();
});

test("survives an empty or missing SteamClient", () => {
  for (const steam of [{}, undefined, null, { System: null }]) {
    const stop = relaySteamEvents(steam, recorder().on);
    stop();
  }
});

test("survives registrations that throw or return nothing", () => {
  const f = fakeSteam({ GameSessions: ["RegisterForAppLifetimeNotifications"] });
  const steam = {
    ...f.steam,
    System: {
      RegisterForOnSuspendRequest: () => {
        throw new Error("not available");
      },
      RegisterForOnResumeFromSuspend: () => undefined,
    },
  };
  const warn = console.warn;
  console.warn = () => {};
  try {
    const r = recorder();
    const stop = relaySteamEvents(steam, r.on);
    f.fire("GameSessions.RegisterForAppLifetimeNotifications", { unAppID: 1, bRunning: true });
    assert.deepEqual(r.events, ["game 1 true"]);
    stop();
  } finally {
    console.warn = warn;
  }
});
