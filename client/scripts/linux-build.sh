#!/usr/bin/env bash
# Builds client/build/tingyu for this machine with the toolchain from
# linux-setup.sh, and prints how to run it with a log file.
set -euo pipefail

cd "$(dirname "$0")/.."
tools_dir="${TINGYU_TOOLS:-${XDG_DATA_HOME:-$HOME/.local/share}/tingyu-tools}"

while [ $# -gt 0 ]; do
	case "$1" in
	--tools)
		if [ $# -lt 2 ]; then
			echo "用法：$0 [--tools 目录]" >&2
			exit 2
		fi
		tools_dir="$2"
		shift 2
		;;
	*)
		echo "用法：$0 [--tools 目录]" >&2
		exit 2
		;;
	esac
done

go_bin="$tools_dir/go/bin/go"
if [ ! -x "$go_bin" ]; then
	# A system Go at or above the go.mod version is enough.
	if command -v go >/dev/null 2>&1; then
		go_bin="$(command -v go)"
		echo "使用系统 Go：$go_bin（$("$go_bin" version)）"
	else
		echo "没有找到 Go。先运行 ./scripts/linux-setup.sh，或用 --tools 指定工具目录。" >&2
		exit 1
	fi
fi

# CGO off is a MyGo premise: the GTK, WebKitGTK and libmpv libraries are
# loaded at run time through purego, so the binary needs no C toolchain.
mkdir -p build
CGO_ENABLED=0 "$go_bin" build -trimpath -o build/tingyu .
echo "已构建：$(pwd)/build/tingyu"
echo "启动：./build/tingyu"
echo "带日志启动：TINGYU_STARTUP_LOG=build/tingyu.log ./build/tingyu"
