#!/usr/bin/env bash
# Builds client/build/tingyu for this machine with the toolchain from
# linux-setup.sh, and prints where the binary and its log live.
set -euo pipefail

cd "$(dirname "$0")/.."
tools_dir="${TINGYU_TOOLS:-${XDG_DATA_HOME:-$HOME/.local/share}/tingyu-tools}"

go_bin="$tools_dir/go/bin/go"
if [ ! -x "$go_bin" ]; then
	# A system Go of the right version is enough; go.mod enforces the floor.
	if command -v go >/dev/null 2>&1; then
		go_bin="$(command -v go)"
	else
		echo "没有找到 Go。先运行 ./scripts/linux-setup.sh。" >&2
		exit 1
	fi
fi

# CGO off is a MyGo premise: the GTK, WebKitGTK and libmpv libraries are
# loaded at run time through purego, so the binary needs no C toolchain.
mkdir -p build
CGO_ENABLED=0 "$go_bin" build -trimpath -o build/tingyu .
echo "已构建：$(pwd)/build/tingyu"
echo "启动：./build/tingyu    （日志在同目录的 tingyu.log）"
