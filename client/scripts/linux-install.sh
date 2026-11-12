#!/usr/bin/env bash
# Installs the client for this user: the binary in ~/.local/bin, a desktop
# entry and the icon in the XDG data directory, so it shows up in the launcher
# (Super + Space). No root and no system package; uninstall by deleting the
# three files this prints. pkgbuild/PKGBUILD is the system-wide package.
set -euo pipefail

cd "$(dirname "$0")/.."
data_dir="${XDG_DATA_HOME:-$HOME/.local/share}"
bin_dir="$HOME/.local/bin"

./scripts/linux-build.sh

entry="$(mktemp)"
trap 'rm -f "$entry"' EXIT
# The desktop entry ships with Exec=tingyu for the package; a user install
# points at the binary in the home directory.
sed "s|^Exec=tingyu\$|Exec=$bin_dir/tingyu|" ../pkgbuild/tingyu.desktop >"$entry"

install -Dm755 build/tingyu "$bin_dir/tingyu"
install -Dm644 "$entry" "$data_dir/applications/tingyu.desktop"
install -Dm644 ../pkgbuild/tingyu.svg "$data_dir/icons/hicolor/scalable/apps/tingyu.svg"

update-desktop-database "$data_dir/applications" 2>/dev/null || true
gtk-update-icon-cache -q -t -f "$data_dir/icons/hicolor" 2>/dev/null || true

echo "已安装："
echo "  程序    $bin_dir/tingyu"
echo "  启动项  $data_dir/applications/tingyu.desktop"
echo "  图标    $data_dir/icons/hicolor/scalable/apps/tingyu.svg"
echo
echo "现在可以从应用启动器（Super + Space）打开听屿，或直接运行 tingyu。"
echo "卸载：rm -f $bin_dir/tingyu $data_dir/applications/tingyu.desktop $data_dir/icons/hicolor/scalable/apps/tingyu.svg"
