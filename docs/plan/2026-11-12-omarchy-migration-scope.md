# 听屿 Omarchy 迁移范围与里程碑

日期：2026-11-12（Asia/Shanghai）
性质：**范围与计划**。本文先记录已实测的可行性结论，再排里程碑；文中「已验证」只指本机实测过的部分，其余一律标为待验收。
基线：`tingyu-omarchy` 首个提交（`chore: start the Omarchy port from the Windows client baseline`），代码来自 `halunhaku/tingyu-windows` 的当前工作树（历史未带入）。
验证机：Omarchy 4.5.0-3，Hyprland 0.56.2（Wayland），x86_64，20 核 / 46 GB，显示器 DP-5 3840×2160 @ scale 1.6。

---

## 0. 结论速览

| 项 | 结论 |
|---|---|
| 技术可行性 | **已验证**：加 3 个平台文件即可编译；本机开窗、建库、加载系统 libmpv、MPRIS 就绪 |
| 代码复用率 | 界面、曲库、来源、元数据、歌词、AI 全部复用，**没有一处需要为 Linux 重写业务逻辑** |
| 平台层缺口 | 只差「数据目录 / 关窗退出 / 在文件管理器中显示」三个文件（本轮已补），其余是分发与桌面集成 |
| 播放内核 | 系统 `libmpv.so.2`（mpv 0.41.0-6）可加载，client API 2.5；PipeWire 输出尚未有出声证据 |
| 主要工作量 | 不在代码，而在**桌面集成与分发**：libmpv 自动发现、媒体键/MPRIS 实测、`.desktop` 与图标、PKGBUILD、Omarchy 主题跟随 |
| 风险最高 | Wayland 下 GTK3 文件选择器与二维码登录窗口、GPU 解码、托盘/指示器、Omarchy 主题与自绘界面的观感冲突 |

---

## 1. 本机实测（2026-11-12）

| # | 项 | 命令 / 做法 | 结果 |
|---|---|---|---|
| 1 | 工具链 | 下载 `go1.27.3.linux-amd64.tar.gz`，SHA-256 `65efbc80…9b40`，解压到 `~/.local/share/tingyu-tools/go` | `go version go1.27.3 linux/amd64` |
| 2 | 编译 | `CGO_ENABLED=0 go build ./...` | 起初 3 处 `undefined`，补齐平台文件后通过 |
| 3 | 测试 | `CGO_ENABLED=0 go test ./...` | 全部通过（无失败项） |
| 4 | 启动 | `MPV_LIB=/usr/lib/libmpv.so.2 TINGYU_DATA_DIR=/tmp/tingyu-test ./build/tingyu` | 开窗 1200×760，`startup ready` 298.8 ms |
| 5 | 曲库 | 同上 | `library: created … (schema version 2)`，0 曲目 |
| 6 | 播放内核 | 同上 | `playback: libmpv libmpv.so.2 client API 2.5` |
| 7 | 媒体会话 | 同上 | `session: mygo.NowPlaying ready`（MPRIS，未接播放） |
| 8 | 渲染 | `grim -g "65,42 1152x1296"` | 首次启动界面正常，中文、图标、1.6 缩放均正确（见 `docs/screenshots/omarchy/w0-onboarding.png`） |
| 9 | 关窗退出 | `pkill -x tingyu-linux` 前的进程行为 | 关窗即退出（本轮实现的口径，未在 GUI 里点关闭按钮复验） |
| 10 | 运行库 | `ldconfig -p` | gtk3 1:3.24.52、webkit2gtk-4.1 2.54.1、libsecret 0.21.8、mpv 1:0.41.0 均在系统里 |

启动日志存档：`docs/screenshots/omarchy/w0-startup.log`。

本轮补的三个平台文件：

| 文件 | 内容 | 依据 |
|---|---|---|
| `client/internal/library/datadir_linux.go` | `$XDG_DATA_HOME/com.halunhaku.tingyu`（未设则 `~/.local/share/…`） | XDG 基础目录规范；与 macOS/Windows 版同级 |
| `client/internal/app/closequit_linux.go` | 关最后一个窗口即退出 | 与 Windows 口径一致；Linux 上窗口就是应用 |
| `client/internal/app/reveal_linux.go` | `org.freedesktop.FileManager1.ShowItems`，失败回退 `xdg-open` | freedesktop 标准接口，Dolphin/Nautilus/Nemo/Thunar 均实现 |

工具脚本：`client/scripts/linux-setup.sh`（装 Go、校验 SHA-256、检查运行库）、`client/scripts/linux-build.sh`（`CGO_ENABLED=0` 构建到 `client/build/tingyu`）。

---

## 2. 还能复用什么

`client/` 的平台差异早已用独立文件隔离，Linux 只落在三处（已补齐）与 `internal/mpv/library_unix.go`（macOS/Linux 共用 dlopen 路径）。以下全部与平台无关，直接沿用：

- 曲库：SQLite schema v2、扫描、分组、搜索、收藏、歌单。
- 来源：本地文件夹、WebDAV、夸克网盘；凭据走 `mygo.Secrets`，Linux 上是 libsecret（Secret Service）。
- 元数据与歌词：QQ 音乐 / 网易云 / LRCLIB / iTunes 抓取，AI 识别。
- 界面：`internal/ui` 全部页面、快捷键、底部播放条。
- 字幕、繁简转换、标题解析等工具层。

与 Windows 版的人机交互差异只有一条要定：Windows 的 `closequit`/`reveal` 语义是否照搬。本轮选择了照搬。

---

## 3. 里程碑

估时为粗估，不构成承诺。

### M0 构建闸门（本轮已完成）

- 验收：`./scripts/linux-setup.sh && ./scripts/linux-build.sh` 在干净机器上出 `build/tingyu`；`go test ./...` 全绿。
- 状态：本机通过；干净机器未验证。

### M1 播放内核与出声

- libmpv 自动发现：无 `MPV_LIB` 时按 `libmpv.so.2` / `libmpv.so.1` 顺序 dlopen，错误信息里写清尝试过的路径。
- 音频输出：mpv 默认走 PipeWire；记录 `AO: [pipewire]`、采样率、underrun，并留一次人耳试听记录。
- 硬件解码：确认 `hwdec=auto-safe` 在本机可用（VA-API），不可用时明确降级而不是黑屏。
- gapless、seek、音量、播放到结尾的状态回报。
- 验收：本地导入 3 首以上（mp3/flac/m4a），能听到声音，进度、结尾、切歌正确；日志有 AO 证据。

### M2 桌面集成

- MPRIS 实测：`playerctl -l` 出现听屿，播放/暂停/上一首/下一首/进度同步；锁屏与媒体键可用。
- 关窗后进程退出、引擎释放、库文件可重命名（无残留锁）。
- 通知与封面（`mygo.Notification`）；托盘/指示器是否保留待定（Hyprland 下走 Waybar 指示器还是 appindicator）。
- 「在文件管理器中显示」实测 Dolphin。
- 验收：用键盘媒体键控制，`playerctl status` 与界面一致。

### M3 输入与对话框

- 本地目录选择走 GTK 文件选择器；确认 Wayland 下可用，必要时改走 `xdg-desktop-portal`。
- 拖入目录/文件是否支持，行为与 Windows 版对齐。
- 夸克二维码登录窗口：弹窗与二维码在 Wayland 下的尺寸、缩放、扫码流程。
- 验证码、WebDAV 凭据输入。

### M4 Omarchy 观感

- 主题跟随：读 `~/.local/state/omarchy/current/theme/colors.toml` 的 `accent`，并监听主题切换（符号链接会被整目录替换，需要重新挂载监听）。
- 圆角跟随 Hyprland `decoration:rounding`（`hyprctl getoption decoration:rounding`），默认 0 时窗口与内部面板用直角。
- 窗口装饰：Hyprland 无服务端装饰，确认自绘标题栏/边框与 Omarchy 其他应用一致；`app id` 与 `.desktop` 名一致，窗口规则才能命中。
- 快捷键表进 README，与 Omarchy「键盘优先」的习惯一致。
- 验收：切换主题与语言后重启观感正确；窗口无闪烁、无重复标题栏。

### M5 分发

- `pkgbuild/`：`PKGBUILD`、`tingyu.desktop`（`Name`/`Comment` 含中文、`StartupWMClass=tingyu`）、图标（沿用 `client/resources/icon.png`，另出 SVG）。
- `makepkg -fsi` 安装后出现在应用启动器（`Super + Space`），运行 `tingyu` 直接起窗口。
- 依赖写清：`gtk3 webkit2gtk-4.1 libsecret mpv`，音频由 mpv 自行选择。
- 后续可提 PR 到 `omacom/omarchy-pkgs`（`packages/<name>/`，`"source": "local"`），是否提交另行决定。
- 验收：干净机器 `makepkg -fsi` 后能从启动器打开并播放。

### M6 验收与回归

- 功能对照表：逐项对照 Windows 版「已实现」清单打勾，未通过项写原因。
- 性能：冷启动、曲库 5000 首滚动、内存占用，与 Windows 记录同口径对比。
- 回归：改了共享代码要同时跑通 `GOOS=windows` 与 `GOOS=darwin` 构建，避免为 Linux 破坏上游。

---

## 4. 风险与未知

| 风险 | 影响 | 现在能做什么 |
|---|---|---|
| GTK3 文件选择器在 Wayland 下的行为 | 选不了目录 = 无法入库 | M3 第一件事就验；退路是 portal |
| 二维码登录窗口（自绘弹窗）在 Wayland 的定位 | 扫码流程可能错位 | M3 实测；必要时改为页内展示 |
| 自绘界面的观感与 Omarchy 主题 | 像「外来的应用」 | M4 接管 accent 与圆角，先做颜色，再谈排版 |
| GPU 解码与 DMA-BUF | 高码率视频封面/大封面抖动 | M1 记录 `hwdec` 实测 |
| 媒体键在 Hyprland + Waybar 下的归属 | 键被别的 MPRIS 播放器接走 | M2 用 `playerctl` 验证并写进文档 |
| 无托盘 | Windows 版有托盘/菜单行为 | 明确「不做托盘」，关窗即退出 |
| 上游变动 | 与 tingyu-windows 分叉后各改各的 | 平台文件按目录隔离；共享代码改动保持最小 |

---

## 5. 明确不做

- 不做 Flatpak / AppImage：Omarchy 用 pacman，先做 PKGBUILD。
- 不做 X11 专属路径的适配（Omarchy 是 Wayland；XWayland 下能起窗口即可，不承诺）。
- 不为 Linux 重做界面：沿用 `internal/ui`，只接管主题色与圆角。
- 不带 Flutter 客户端（`app/`）进这个仓库的构建链，它只作为历史与设计参考。
