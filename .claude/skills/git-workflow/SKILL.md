---
name: git-workflow
description: Branching, Conventional Commit messages, PR titles/descriptions and rebase-only (linear history) merging for monoink. Use whenever creating a branch, committing, writing a PR, pushing, or syncing with main.
---

# Git workflow

History on `main` is **linear** and every commit is a **Conventional Commit**.
Merge commits (like the old `Merge pull request #N …`) are no longer allowed.

## 1. Branch

Start from an up-to-date `main`:

```bash
git fetch origin && git checkout main && git pull --ff-only
git checkout -b <type>/<short-kebab-description>   # e.g. fix/last-played-refresh
```

Branch prefixes match commit types: `feat/`, `fix/`, `docs/`, `refactor/`,
`test/`, `chore/`, `ci/`, `build/`, `perf/`.

## 2. Commit messages (Conventional Commits 1.0)

```
<type>(<scope>): <imperative subject, lowercase, no period, ≤72 chars>

<body: what changed and why, wrapped at 72. Mention user-visible behaviour.>

<footers: BREAKING CHANGE: …, Refs #N, Co-Authored-By: …>
```

**Types**
| type | use for | release bump |
| --- | --- | --- |
| `feat` | new user-visible capability | minor |
| `fix` | bug fix | patch |
| `perf` | faster / lighter, same behaviour | patch |
| `refactor` | code change, no behaviour change | none |
| `test` | tests only | none |
| `docs` | docs, README, release notes | none |
| `build` | `package.sh`, `Containerfile`, deps, Makefile | none |
| `ci` | `.github/workflows` | none |
| `chore` | anything else, incl. `chore(release): vX.Y.Z` | none |

`feat!:` / `fix!:` or a `BREAKING CHANGE:` footer → major bump (or minor while 0.x).

**Scopes** (optional, pick the area touched): `engine`, `screens`, `settings`,
`steam`, `power`, `ble`, `api`, `render`, `weather`, `panel` (src/),
`steam-events`, `backend`, `previews`, `readme`, `release`, `deps`.

**Examples** (rewrites of past commits):
- `fix(steam-events): don't fail to load when Steam lacks a SteamClient event`
- `fix(engine): switch "Now playing" to "Last played" as soon as a game closes`
- `feat(power): add configurable sleep and shutdown screens`
- `feat(screens): show year progress on the clock screen`
- `docs(readme): note this is a personal fork and list its changes`
- `chore(release): v0.2.2`

Rules:
- One logical change per commit; split unrelated work.
- Tests, docs and regenerated previews go in the same commit as the change they cover.
- End messages with the `Co-Authored-By` line the harness provides when Claude authored the commit.
- Check before committing: `git log --format=%s origin/main..HEAD` — every subject must match
  `^(feat|fix|perf|refactor|test|docs|build|ci|chore)(\([a-z0-9-]+\))?!?: .+`.

## 3. Keep the branch linear

Never `git merge main` into a branch. Rebase instead:

```bash
git fetch origin
git rebase origin/main            # resolve conflicts, `git rebase --continue`
git push --force-with-lease       # only for your own feature branch
```

Clean up fixup commits before review: `git commit --fixup <sha>` then
`GIT_SEQUENCE_EDITOR=: git rebase --autosquash origin/main` (non-interactive).

## 4. Pull request

- Target **fernandofreije/monoink:main**, never upstream v1k0d3n. Compare link:
  `https://github.com/fernandofreije/monoink/compare/main...<branch>`
- PR title = Conventional Commit subject of the change (used if squashed).
- Description: Summary per change, why, how it was tested (Go tests, `pnpm test`,
  on-device or not), and anything not verified. End with the harness attribution line.
- Only push / open PRs when the user asks. The user merges in the GitHub UI.

## 5. Merging

- Use **Rebase and merge** (keeps each conventional commit) or **Squash and merge**
  (PR title becomes the commit). Never "Create a merge commit".
- Local fast-forward only: `git checkout main && git pull --ff-only`. If that
  fails, `main` diverged — stop and ask, don't merge.
- After merge, delete the branch: `git branch -d <branch> && git push origin --delete <branch>`
  (skip the remote delete if GitHub auto-deleted it).

## GitHub settings that enforce this

Can't be changed from here (no `gh`); ask the user to confirm they're set:
- Settings → General → Pull Requests: untick **Allow merge commits**, tick
  **Allow rebase merging** (squash optional), tick **Automatically delete head branches**.
- Settings → Rules → Rulesets → branch ruleset on `main`: **Require linear history**
  (optionally **Require status checks**: CI). Leave the repo admin in the bypass list
  so the release commit can still be pushed directly to `main`.
