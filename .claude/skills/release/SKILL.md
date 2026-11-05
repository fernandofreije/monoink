---
name: release
description: Cut a monoink release (version bump, release notes, tag, GitHub Release workflow). Use when the user asks to release, deploy, publish or tag a new version.
---

# Release

The **Release** workflow (`.github/workflows/release.yml`) runs only when a
`v*` tag is **created** on GitHub. It fails unless the tag equals
`v` + `package.json` `version`; `package.sh` also bakes that version into
`monoinkd`. So a release always needs a version-bump commit **and** a tag.

## 1. Pick the version (SemVer, confirm with the user)

Look at commits since the last tag: `git log --format=%s $(git describe --tags --abbrev=0)..origin/main`
- any `feat` → **minor** (0.2.2 → 0.3.0)
- only `fix` / `perf` → **patch** (0.2.2 → 0.2.3)
- `!` / `BREAKING CHANGE` → minor while on 0.x

If the user's word ("minor", "deploy a new version") disagrees with the commits,
say so and confirm the number before tagging. Past mistake: a fix-only release
was tagged v0.3.0 and had to be withdrawn and redone as v0.2.2.
Never use suffixes like `-fer1` unless a pre-release is wanted: suffixed tags are
pre-releases and `releases/latest/download/monoink.zip` skips them.

## 2. Prepare on main

```bash
git fetch origin --tags && git checkout main && git pull --ff-only
git status --short          # must be clean
```

- Set `"version"` in `package.json` (only file holding the version; `plugin.json` has none).
- Write `docs/release-notes/vX.Y.Z.md`, following the previous file's style:

```markdown
## What's new in vX.Y.Z

A fix release for this personal fork of
[v1k0d3n/monoink](https://github.com/v1k0d3n/monoink).

### Added / Changed / Fixed
- **User-facing headline.** One or two sentences on what it means on the Deck.

### Development
- Tests/CI/tooling changes, if any.
```

## 3. Commit, tag, push

```bash
git add package.json docs/release-notes/vX.Y.Z.md
git commit -m "chore(release): vX.Y.Z" -m "Co-Authored-By: …"
git tag vX.Y.Z
git push origin main        # first: the commit the tag points to
git push origin vX.Y.Z      # second, separately: this starts Release
```

The release commit goes straight to `main` (fast-forward, so history stays
linear). If `main` is protected without bypass, do it as a `chore/release-vX.Y.Z`
PR merged with rebase instead, then tag the resulting commit on `main`.

## 4. Verify (no `gh` here — use the public API)

```bash
curl -s 'https://api.github.com/repos/fernandofreije/monoink/actions/runs?per_page=5' \
  | python3 -c 'import json,sys;[print(r["name"],r["head_branch"],r["status"],r["conclusion"],r["html_url"]) for r in json.load(sys.stdin)["workflow_runs"]]'
curl -s https://api.github.com/repos/fernandofreije/monoink/releases/latest | python3 -c 'import json,sys;print(json.load(sys.stdin).get("tag_name"))'
```

Expect a **Release** run for `vX.Y.Z` (the **CI** run is for the `main` push).
Tell the user to install via Decky → Developer → Install Plugin from URL:
`https://github.com/fernandofreije/monoink/releases/latest/download/monoink.zip`

## Fixing a bad release

- **Release run never started:** Actions may be disabled on the fork (Actions tab /
  Release workflow "Enable"). Then re-create the tag: `git push origin --delete vX.Y.Z`
  and `git push origin vX.Y.Z` as two separate commands.
- **"Tag does not match package.json":** the tag predates the bump. Commit the bump,
  `git tag -f vX.Y.Z`, delete the remote tag, push `main`, push the tag.
- **"release already exists":** delete the release (not just the tag) in the UI first.
- **Withdrawing a wrong version:** needs explicit user approval since it rewrites
  `main`: `git reset --hard <previous>`, `git push --force-with-lease=main:<bad-sha> origin main`,
  `git push origin :refs/tags/vX.Y.Z`, `git tag -d vX.Y.Z`.
