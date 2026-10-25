# Development

## Layout

| Path | What |
| --- | --- |
| `backend/` | `monoinkd`, a static Go program: Bluetooth (BlueZ over D-Bus), rendering, scheduling, the local API. |
| `src/` | The Decky panel (TypeScript/React). |
| `tests/` | Panel tests (`pnpm test`, Node's built-in runner). |
| `main.py` | Decky's required Python entry point. Standard library only; it just supervises `monoinkd` and forwards panel requests. |
| `Containerfile` | The build environment: Go, Node, pnpm and zip at pinned versions. |
| `package.sh` | Tests and builds everything into `out/monoink.zip`, inside that environment. |
| `.github/` | CI and release workflows, Dependabot, issue templates (below). |

## Principles

These are requirements for every change; please check pull requests
against them when reviewing:

- No sudo, and no writes outside the plugin's own settings/data folders.
- Every file the core reads and every network request it makes is listed in
  the README, and anything that reveals something about the user is opt-in.
- The core never reads other apps' data or credentials; integrations are
  [providers](providers.md).
- The panel never assumes a `SteamClient` method exists. It is Steam's
  private API, changes between client releases, and Decky's typings don't
  reflect what a given user has, so type-checking can't catch it. Register
  through the guarded helper in `src/steamEvents.ts` and add a test with
  the method missing.
- `main.py` uses only the Python standard library; `monoinkd` stays
  `CGO_ENABLED=0`.

## Building and testing locally

Only podman (preinstalled on SteamOS) or docker is needed. `package.sh`
builds the image from `Containerfile` on first use and runs everything
inside it, so every contributor, CI and the release workflow use the same
toolchain:

```bash
./package.sh            # all tests and checks, then out/monoink.zip
./package.sh test       # tests and checks only
./package.sh previews   # re-render docs/images/screens after changing a screen
./package.sh shell      # a shell inside the build environment
./package.sh run pnpm install   # any command inside it
```

Build outputs are owned by your user, and tool caches are kept in `.cache/`
(gitignored) so repeat builds are fast. With Go 1.26+ and Node 22 already
installed, `./package.sh --native` skips the container.

### Screen previews

[docs/screens.md](screens.md) shows every screen in light and dark mode. The
images are rendered from fixed, fictional sample data
(`backend/internal/screens/sample.go`) by `./package.sh previews`, and the
output is deterministic. **If a pull request changes how any screen looks,
regenerate and commit the previews in the same pull request**; CI fails when
they're out of date, and reviewers see the visual change in the diff.

To change a toolchain version, edit `Containerfile` (and `packageManager` in
`package.json` for pnpm, `go` in `backend/go.mod` for Go).

Useful hardware checks (stop the plugin's connection first by turning
**Enabled** off in the panel, since only one program can connect):

```bash
backend/out/monoinkd probe                              # find and query the display
backend/out/monoinkd probe -test-pattern                # draw a checkerboard
backend/out/monoinkd probe -test-pattern -numbers -repeat 3 -idle 3s   # must end on "3"
```

## Continuous integration

| Workflow | When | What |
| --- | --- | --- |
| **CI** (`ci.yml`) | every push to `main` and every pull request | gofmt, `go vet`, Go tests with the race detector, a check that screen previews are up to date, TypeScript type-check, panel tests and build, `main.py` compile check, then a full package built with `Containerfile`. The installable zip is attached to the run as an artifact (*monoink-plugin-…*) for 14 days, so changes can be tried on a device before release. |
| **Release** (`release.yml`) | pushing a `v*` tag | Builds and tests with `Containerfile`, then publishes a GitHub release with `monoink-vX.Y.Z.zip`, its SHA-256, install instructions and generated notes. Tags with a suffix (`v0.2.0-beta1`) are marked pre-release; plain versions become the release that `releases/latest/download/monoink.zip` points to. |
| **Dependabot** (`dependabot.yml`) | weekly | Opens pull requests for updated npm packages, Go modules, GitHub Actions and build images (minor and patch updates grouped). CI runs on each one, so a green check means the update builds and passes the tests. |

Major updates that need coordinated changes (TypeScript, Rollup, Decky's
packages, Node) are ignored and taken deliberately. `@types/react` stays
pinned to the React version Steam ships. When Dependabot bumps the Go image,
keep `Containerfile` and `backend/Dockerfile` on the same version.

### Pull requests from contributors

CI runs on every pull request, including ones from forks (without access to
secrets), and attaches the built zip so the change can be tried on a device
before merging.

## Releasing

1. In a `release/vX.Y.Z` pull request, update `"version"` in `package.json`
   and add `docs/release-notes/vX.Y.Z.md` with the highlights (it becomes the
   top of the GitHub release, above the automatic list of merged pull
   requests). Merge it.
2. Tag the merged commit on `main` and push the tag:

   ```bash
   git tag v0.2.0
   git push origin v0.2.0
   ```

3. The Release workflow publishes the zip. Check the Releases page.
