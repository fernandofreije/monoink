# monoink (personal fork)

Decky Loader plugin that drives the JSAUX E-Ink faceplate on a Steam Deck.
This repo is **fernandofreije/monoink**, a personal fork of
[v1k0d3n/monoink](https://github.com/v1k0d3n/monoink). Credit stays with
upstream (see the README fork note); never open PRs or push to upstream.

## Layout

- `backend/` — `monoinkd`, static Go daemon (`CGO_ENABLED=0`): BLE over BlueZ/D-Bus,
  rendering, scheduling, local API. Key packages: `engine` (screen precedence,
  redraws, sleep/wake), `screens`, `settings`, `steam`, `power` (logind suspend/shutdown).
- `src/` — Decky panel (TypeScript/React). `src/steamEvents.ts` is the only place
  that registers `SteamClient` events.
- `tests/` — panel tests, Node's built-in runner (`pnpm test`).
- `main.py` — Decky entry point, Python standard library only.
- `docs/` — user/dev docs; `docs/release-notes/vX.Y.Z.md`; `docs/images/screens` (generated previews).
- `.github/workflows/` — `ci.yml` (push to main + PRs), `release.yml` (on `v*` tag).

## Commands

```bash
corepack pnpm install --frozen-lockfile
corepack pnpm typecheck && corepack pnpm test && corepack pnpm build
./package.sh test        # everything, inside the Containerfile image (podman/docker)
./package.sh previews    # regenerate docs/images/screens after any screen change
cd backend && gofmt -l . && go vet ./... && go test ./...
```

Environment notes (WSL): `pnpm` is only available via `corepack pnpm`. Go is
not installed system-wide — download a toolchain into the session scratchpad
(`go.dev/dl/go1.26.x.linux-amd64.tar.gz`) and put it on `PATH` when backend
work needs it. `gh` is not installed; query GitHub with `curl` against the
public API, and tell the user what to check in the UI.

## Rules for every change

From `docs/development.md` (read it before larger changes):
- No sudo; no writes outside the plugin's own settings/data folders.
- Every file read / network request is listed in the docs; anything revealing about the user is opt-in. New system integrations (e.g. logind) go in `docs/privacy.md`.
- Never assume a `SteamClient` method exists — register through the guarded helper in `src/steamEvents.ts` and add a test with the method missing.
- If a screen's look changes, regenerate and commit `docs/images/screens` in the same PR (CI checks it).
- Prefer an end-to-end test (fake display, fake Steam) for behaviour bugs, like `backend/internal/engine/flow_test.go`.
- Update docs and the README fork-changes list when user-visible behaviour changes.

## Git workflow (mandatory)

- **Conventional Commits** for every commit and PR title: `type(scope): subject`.
  Use the `git-workflow` skill for types, scopes and examples.
- **Linear history.** Never create merge commits. Work on a branch
  (`feat/…`, `fix/…`, `docs/…`, `chore/…`), rebase it onto `origin/main`
  before pushing/merging, and merge PRs with **Rebase and merge** (or squash).
  Locally: `git pull --rebase`, `git merge --ff-only`.
- Never commit straight to `main`, except the release commit (see `release` skill).
- Don't push or open PRs without being asked; the user merges PRs in the GitHub UI.
- Force-push only your own feature branch, with `--force-with-lease`. Rewriting `main` needs explicit user approval.

## Releases

Use the `release` skill. Summary: bump `package.json`, add
`docs/release-notes/vX.Y.Z.md`, commit `chore(release): vX.Y.Z` on `main`,
tag `vX.Y.Z`, push `main` then the tag. The tag must match `package.json`.
Bug fixes are **patch** bumps even if the user says "minor release" — confirm
the number before tagging (v0.3.0 had to be withdrawn and redone as v0.2.2).
