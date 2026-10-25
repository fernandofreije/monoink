// Steam knows when a game starts or stops, and when it's about to
// suspend, before the service can tell from the system. Pass it on.
//
// SteamClient is Steam's private API: methods move or disappear between
// client releases while Decky's typings still list them, so the compiler
// can't catch it. Nothing here assumes a method exists; a missing one costs
// only that event, never the plugin (v0.2.1 failed to load on a client
// without System.RegisterForOnSuspendRequest).

export interface SteamHandlers {
  game(appId: number, running: boolean): void;
  suspend(): void;
  resume(): void;
}

interface Unregisterable {
  unregister(): void;
}

// Steam may report one suspend several times (the newer API reports each
// step of its progress); the service needs it once.
const SUSPEND_REPEAT_MS = 30_000;

// listen calls obj[method](callback) if it exists, and returns null if it
// doesn't or fails.
function listen(obj: any, method: string, callback: (...args: any[]) => void): Unregisterable | null {
  try {
    const register = obj?.[method];
    if (typeof register !== "function") return null;
    const sub = register.call(obj, callback);
    return typeof sub?.unregister === "function" ? sub : null;
  } catch (e) {
    console.warn(`E-Ink Faceplate: couldn't register for ${method}`, e);
    return null;
  }
}

export function relaySteamEvents(steam: any, on: SteamHandlers): () => void {
  let suspendedAt = 0;
  const suspend = () => {
    const now = Date.now();
    if (now - suspendedAt < SUSPEND_REPEAT_MS) return;
    suspendedAt = now;
    on.suspend();
  };
  const resume = () => {
    suspendedAt = 0;
    on.resume();
  };
  const subs = [
    listen(steam?.GameSessions, "RegisterForAppLifetimeNotifications", (n) => on.game(n.unAppID, n.bRunning)),
    listen(steam?.System, "RegisterForOnSuspendRequest", suspend) ??
      listen(steam?.User, "RegisterForPrepareForSystemSuspendProgress", suspend),
    listen(steam?.System, "RegisterForOnResumeFromSuspend", resume) ??
      listen(steam?.User, "RegisterForResumeSuspendedGamesProgress", resume),
  ];
  return () =>
    subs.forEach((s) => {
      try {
        s?.unregister();
      } catch {
        // already gone
      }
    });
}
