#!/usr/bin/env bash
# Installs the Go toolchain the client builds with, and checks the runtime
# libraries the app loads at start. Nothing is installed system-wide: the
# toolchain stays in its own directory and never joins PATH.
set -euo pipefail

GO_VERSION=1.27.3
GO_SHA256=65efbc80fdc1fa2cd2631cad5b481777a68e8725f34e89f50fc6ac714e379b40
tools_dir="${TINGYU_TOOLS:-${XDG_DATA_HOME:-$HOME/.local/share}/tingyu-tools}"

while [ $# -gt 0 ]; do
	case "$1" in
	--tools)
		tools_dir="$2"
		shift 2
		;;
	*)
		echo "用法：$0 [--tools 目录]" >&2
		exit 2
		;;
	esac
done

case "$(uname -m)" in
x86_64) go_arch=amd64 ;;
*)
	echo "目前只支持 x86_64：官方下载页需要对应的 SHA-256 校验值，请先补上。" >&2
	exit 1
	;;
esac

go_root="$tools_dir/go"
if [ -x "$go_root/bin/go" ] && "$go_root/bin/go" version | grep -q "go$GO_VERSION "; then
	echo "Go $GO_VERSION 已就绪：$go_root"
else
	mkdir -p "$tools_dir"
	tarball="$tools_dir/go$GO_VERSION.linux-$go_arch.tar.gz"
	if [ ! -f "$tarball" ]; then
		echo "下载 Go $GO_VERSION（约 68 MB）……"
		curl -fsSLo "$tarball" "https://go.dev/dl/go$GO_VERSION.linux-$go_arch.tar.gz"
	fi
	actual="$(sha256sum "$tarball" | cut -d' ' -f1)"
	if [ "$actual" != "$GO_SHA256" ]; then
		echo "校验失败：$tarball 的 SHA-256 是 $actual，期望 $GO_SHA256。" >&2
		exit 1
	fi
	rm -rf "$go_root"
	tar -xzf "$tarball" -C "$tools_dir"
	rm -f "$tarball"
	echo "Go $GO_VERSION 已解压到：$go_root"
fi

# The client asks the loader for these at run time, so a missing one only
# shows up when the window is about to open. Checking here keeps that out of
# the first launch.
missing=()
# Read the cache once: grep -q closing the pipe early would otherwise look
# like a missing library under pipefail.
ld_cache="$(ldconfig -p 2>/dev/null || true)"
for lib in libgtk-3.so.0 libgdk-3.so.0 libwebkit2gtk-4.1.so.0 libsecret-1.so.0 libmpv.so.2; do
	if ! grep -qE "^[[:space:]]*$lib " <<<"$ld_cache"; then
		missing+=("$lib")
	fi
done
if [ ${#missing[@]} -gt 0 ]; then
	echo
	echo "缺少运行库：${missing[*]}" >&2
	echo "Arch 上安装：sudo pacman -S gtk3 webkit2gtk-4.1 libsecret mpv" >&2
	exit 1
fi
echo "运行库齐全：gtk3、webkit2gtk-4.1、libsecret、libmpv"
echo
echo "下一步：./scripts/linux-build.sh"
