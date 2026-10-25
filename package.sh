#!/usr/bin/env bash
# Build, test and package monoink. Only podman or docker is required: by
# default everything runs inside the image defined by ./Containerfile.
#
#   ./package.sh            test and build out/monoink.zip
#   ./package.sh test       run all tests and checks, no zip
#   ./package.sh previews   re-render docs/images/screens (commit the result)
#   ./package.sh shell      open a shell in the build environment
#   ./package.sh run CMD    run CMD in the build environment (e.g. pnpm install)
#   ./package.sh --native   build with locally installed Go 1.26+ and Node 22
#                           (this is what runs inside the container)
set -euo pipefail

cd "$(dirname "$0")"
VERSION=$(sed -n 's/.*"version": *"\([^"]*\)".*/\1/p' package.json | head -1)

native_test() {
	echo "==> backend checks"
	unformatted=$(cd backend && gofmt -l .)
	[ -z "$unformatted" ] || { echo "needs gofmt: $unformatted" >&2; exit 1; }
	make -C backend test
	echo "==> plugin checks"
	pnpm install --frozen-lockfile >/dev/null
	pnpm typecheck
	pnpm test
	python3 -c "import ast; ast.parse(open('main.py').read())" 2>/dev/null || { command -v python3 >/dev/null && { echo "main.py has a syntax error" >&2; exit 1; } || echo "(python3 not found; skipped main.py check)"; }
}

native_package() {
	native_test
	echo "==> build (monoink $VERSION)"
	make -C backend build VERSION="$VERSION"
	pnpm build
	echo "==> package"
	stage=out/stage/monoink
	rm -rf out/stage out/monoink.zip
	mkdir -p "$stage/dist" "$stage/bin"
	cp plugin.json package.json main.py LICENSE NOTICE README.md "$stage/"
	cp dist/index.js "$stage/dist/"
	cp backend/out/monoinkd "$stage/bin/"
	chmod 755 "$stage/bin/monoinkd"
	(cd out/stage && zip -qr ../monoink.zip monoink)
	rm -rf out/stage
	ls -l out/monoink.zip
}

in_container() {
	ENGINE=${ENGINE:-$(command -v podman || command -v docker || true)}
	[ -n "$ENGINE" ] || { echo "Install podman or docker, or use --native." >&2; exit 1; }
	"$ENGINE" build -q -t monoink-build -f Containerfile . >/dev/null
	# Run as the calling user so build outputs aren't owned by root, and keep
	# tool caches in ./.cache (gitignored) so repeat builds are fast.
	case "$(basename "$ENGINE")" in
	podman) user=(--userns=keep-id) ;;
	*) user=(--user "$(id -u):$(id -g)") ;;
	esac
	mkdir -p .cache
	tty=()
	[ -t 0 ] && tty=(-it)
	"$ENGINE" run --rm "${tty[@]}" "${user[@]}" -v "$PWD":/src:Z -w /src \
		-e HOME=/src/.cache/home -e GOCACHE=/src/.cache/go-build -e GOMODCACHE=/src/.cache/go-mod \
		-e npm_config_store_dir=/src/.cache/pnpm-store \
		monoink-build "$@"
}

case "${1:-}" in
--native) native_package ;;
--native-test) native_test ;;
test) in_container ./package.sh --native-test ;;
shell) in_container bash ;;
previews) in_container make -C backend previews ;;
run)
	shift
	in_container "$@"
	;;
"") in_container ./package.sh --native ;;
*)
	sed -n '2,12p' "$0" | sed 's/^# \{0,1\}//'
	exit 2
	;;
esac
