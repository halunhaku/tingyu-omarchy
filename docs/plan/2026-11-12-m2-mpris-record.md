# 听屿 Omarchy 端 M2：MPRIS 与媒体键 · 实测与修复记录

日期：2026-11-12（Asia/Shanghai）
性质：**实测与修复记录**。只改 `client/internal/app/session.go`（与同目录测试）；未改界面、未动 `internal/player`、`internal/mpv` 等与平台无关的库层。分支 `cindy/inspiring-hamilton`，未 push、未动 main。
前置：[`2026-11-12-omarchy-migration-scope.md`](./2026-11-12-omarchy-migration-scope.md) 的 M2 行（MPRIS 实测、媒体键、关窗退出）。

---

## 0. 结论

| # | 检查项 | 结论 | 一句话证据 |
|---|---|---|---|
| 1 | 桌面会话总线上出现听屿 | **通过** | `playerctl -l` → `tingyu.instance128570` |
| 2 | 身份与桌面项 | **通过**（见 §6.6 的启动环境说明） | `Identity=tingyu`、`DesktopEntry=tingyu` |
| 3 | 标题 / 艺术家 / 专辑 | **通过** | `xesam:title=Spring`、`xesam:artist=['Vivaldi']`、`xesam:album=The Four Seasons` |
| 4 | `mpris:length` | **通过** | Spring `10031020`、Summer `7000000`、Winter `14000000`（µs，取曲库时长） |
| 5 | 封面 `mpris:artUrl` | **通过** | `file:///run/user/1000/tingyu/now-playing-…`，PNG 600×600、4621 字节，sha256 与内嵌封面完全相同 |
| 6 | 位置刷新 | **通过** | `playerctl position` 2 s 内 `7.020667 → 9.029499`（Δ2.009） |
| 7 | `Rate` | **通过**，恒为 1.0（不是 0） | `gdbus … Get … Rate` → `<1.0>` |
| 8 | 播放 / 暂停回传 | **通过** | `playerctl play-pause` → `Paused`，位置冻结 `9.114512 → 9.114512`；应用日志 `media: command pause` |
| 9 | 上一首 / 下一首 | **通过** | `next` 后标题、`mpris:trackid` 都变；应用日志 `media: command nextTrack` |
| 10 | 定位（SetPosition / Seek） | **通过** | `playerctl position 4` → 位置 4.38 s，日志 `media: command changePlaybackPosition pos=4.000`；`position 2+` → 7.000（被 mygo 夹到曲长） |
| 11 | 界面跟随 | **通过** | 同一 `player.Update` 同时驱动窗口与 MPRIS；截图见 §8（Spring 播放中、Winter 暂停 0:01，与同时的 `playerctl` 一致） |
| 12 | 播放结束的状态 | **修前：不一致**（`Paused`，且位置永远停在 13.746/14）；**修后：`Stopped`** | §3 与 §4.2、§4.4 |
| 13 | 换曲瞬间的位置 | **修前：旧曲位置**（新曲立即显示 4.843 s）；**修后：新曲从 0 开始** | §2.5 与 §4.1、§4.4 |
| 14 | 名字重复 | 单实例唯一；**双实例短名相同**，只有 `.instance<pid>` 区分（未改，见 §6.4） | `playerctl -a metadata` 两行都以 `tingyu` 开头 |
| 15 | 媒体键 | **未按真键盘**。全部控制经 `playerctl` 模拟；Omarchy 的真媒体键走另一条链（见 §5） | — |
| 16 | 关窗后 MPRIS 消失 | **通过** | 关窗 → 进程退出 → `playerctl -l` → `No players found` |

没有发现需要停下的回归：`CGO_ENABLED=0 go test -short ./...` 全绿（14 个包 `ok`），`gofmt -l` 干净。

---

## 1. 环境

| 项 | 值 |
|---|---|
| 机器 / 系统 | Omarchy 4.5.0-3，Hyprland 0.56.2（Wayland），x86_64，DP-5 3840×2160 @ scale 1.6 |
| Go | go1.27.3 linux/amd64（`~/.local/share/tingyu-tools/go`），`CGO_ENABLED=0` |
| 播放内核 | 系统 `libmpv.so.2`，mpv 0.41.0，client API 2.5 |
| 媒体会话 | `mygo.NowPlaying`（Linux 上为 `org.mpris.MediaPlayer2`） |
| 构建 | `cd client && ./scripts/linux-build.sh` → `client/build/tingyu` |
| 曲库 | 全部 `/tmp` 临时目录（`TINGYU_DATA_DIR`），**未碰真实曲库** |

### 1.1 playerctl 不在本机

这台机器**没有装 `playerctl`**（也没有 `dbus-send` 之外现成的 MPRIS 客户端）。为了不改动系统，从 Arch 官方仓库下载二进制包并解包到 `/tmp`，不装进系统：

```console
$ curl -s https://archlinux.org/packages/extra/x86_64/playerctl/json/ | python3 -c "import json,sys; d=json.load(sys.stdin); print(d['pkgver']+'-'+str(d['pkgrel']), d['filename'], d.get('depends'))"
2.4.1-5 playerctl-2.4.1-5-x86_64.pkg.tar.zst ['glib2', 'glibc']

$ curl -sL -o /tmp/playerctl.pkg.tar.zst https://geo.mirror.pkgbuild.com/extra/os/x86_64/playerctl-2.4.1-5-x86_64.pkg.tar.zst
$ tar --zstd -xf /tmp/playerctl.pkg.tar.zst -C /tmp/playerctl-pkg
$ LD_LIBRARY_PATH=/tmp/playerctl-pkg/usr/lib /tmp/playerctl-pkg/usr/bin/playerctl --version
v2.4.1
```

下文所有 `playerctl` 都指 `/tmp/playerctl-pkg/usr/bin/playerctl`，并带 `LD_LIBRARY_PATH=/tmp/playerctl-pkg/usr/lib`。

### 1.2 测试媒体（ffmpeg 现造）

```console
$ cd /tmp/m2-mpris/media
$ ffmpeg -f lavfi -i "gradients=s=600x600:c0=#1f6feb:c1=#d29922:x0=0:y0=0:x1=600:y1=600:d=1" -frames:v 1 -update 1 cover.png
$ ffmpeg -f lavfi -i "sine=frequency=440:duration=10" -i cover.png -map 0:a -map 1:v \
    -c:a libmp3lame -b:a 192k -c:v copy -id3v2_version 3 \
    -metadata:s:v title="Album cover" -metadata:s:v comment="Cover (front)" \
    -metadata title="Spring" -metadata artist="Vivaldi" -metadata album="The Four Seasons" -metadata track="1" \
    "01 Spring.mp3"
$ ffmpeg -f lavfi -i "sine=frequency=523:duration=7" -i cover.png -map 0:a -map 1:v \
    -c:a flac -c:v copy -disposition:v attached_pic \
    -metadata title="Summer" -metadata artist="Vivaldi" -metadata album="The Four Seasons" -metadata track="2" \
    "02 Summer.flac"
$ ffmpeg -f lavfi -i "sine=frequency=659:duration=14" -c:a aac -b:a 192k \
    -metadata title="Winter" -metadata artist="Vivaldi" -metadata album="The Four Seasons" -metadata track="3" \
    "03 Winter.m4a"
```

| 文件 | 时长 | 内嵌封面 | 用途 |
|---|---|---|---|
| `01 Spring.mp3` | 10.031 s | 有（600×600 PNG） | 元数据 + 封面 + 队列下一首 |
| `02 Summer.flac` | 7.0 s | 有 | 换曲/快进被夹到曲长 |
| `03 Winter.m4a` | 14.0 s | **无** | 无封面时 `artUrl` 应消失 |

### 1.3 两条总线：为什么

本机同时有其他 agent 在跑同一个应用（实测出现过 3 个 `tingyu` 窗口、1 个别人的实例占着桌面会话总线）。在真实会话总线上做长测量时，别人的 `playerctl`（不带 `-p`）会打到**唯一注册了 MPRIS 名的那一个播放器**上——第一次测量就撞上了：应用日志里出现了我没发过的 `media: command pause pos=6.681`。

因此测量分两段：

| 段 | 总线 | 目的 |
|---|---|---|
| A 确定性测量 | `dbus-daemon --session` 私有总线（`/tmp/dbus-*`） | 长测量、换曲、播完，不受别人干扰 |
| B 桌面可见性 | 真实 `DBUS_SESSION_BUS_ADDRESS`（`unix:path=/run/user/1000/bus`） | `playerctl -l`/`status`/`metadata`/暂停/关窗退出，证明桌面确实看得见 |

两段的 MPRIS 实现、二进制、数据完全一样，区别只有总线地址。

### 1.4 应用启动参数

```console
$ TINGYU_ENRICH_AUTO=0 \            # 关掉联网补全，避免标题/封面中途被改写
  TINGYU_DATA_DIR=/tmp/m2-mpris/data4 \
  TINGYU_AUTO_SCAN_DIR=/tmp/m2-mpris/media \
  TINGYU_AUTO_PLAY_INDEX=0 \        # 用与双击同一入口开播（队列 = 第 0 首起的全部）
  TINGYU_STARTUP_LOG=/tmp/m2-mpris/run3/app.log \
  ./build/tingyu
```

- `TINGYU_AUTO_PLAY_INDEX=2` + `TINGYU_REPEAT=off` 用于「单曲队列播完」：入口只取第 2 首起的曲目，队列只有一首。
- 应用默认循环模式是 **RepeatAll**（`player/internal/controller.go` 的默认值），所以默认行为是队列循环，只有 `TINGYU_REPEAT=off` 才有真正的「播完」。
- `client/internal/accept` 是播放内核（`internal/mpv` + `internal/player`）的验收程序，不驱动应用窗口；驱动应用用的是仓库已有的验收开关（`TINGYU_AUTO_SCAN_DIR` / `TINGYU_AUTO_PLAY_INDEX` / `TINGYU_AUTO_PLAY_TITLE`，见 `internal/app/app.go`），它们走的是与「添加目录」「双击行」相同的入口。

---

## 2. 原始输出（A 段，真实桌面总线）

### 2.1 播放器出现与元数据

```console
$ playerctl -l
tingyu.instance128570

$ playerctl status
Playing

$ playerctl metadata
tingyu mpris:trackid             '/org/mygo/track/1'
tingyu xesam:title               Spring
tingyu xesam:artist              Vivaldi
tingyu xesam:album               The Four Seasons
tingyu mpris:length              10031020
tingyu mpris:artUrl              file:///run/user/1000/tingyu/now-playing-128570-2919f6a4152890cd
```

### 2.2 属性（gdbus，逐项）

```console
$ gdbus call --session --dest org.mpris.MediaPlayer2.tingyu.instance128570 \
    --object-path /org/mpris/MediaPlayer2 \
    --method org.freedesktop.DBus.Properties.Get org.mpris.MediaPlayer2.Player <PROP>
PlaybackStatus (<'Playing'>,)
Rate          (<1.0>,)
Position      (<int64 9052640>,)
Volume        (<1.0>,)
CanGoNext     (<true>,)
CanGoPrevious (<true>,)
CanPlay       (<true>,)
CanPause      (<true>,)
CanSeek       (<true>,)
CanControl    (<true>,)
MinimumRate   (<1.0>,)
MaximumRate   (<1.0>,)
```

根接口：

```console
Identity      (<'tingyu'>,)
DesktopEntry  (<'tingyu'>,)
HasTrackList  (<false>,)
CanQuit       (<false>,)
CanRaise      (<false>,)
```

（`DesktopEntry` 这一行是在 `GIO_LAUNCHED_DESKTOP_FILE` 未设置时测的；见 §6.6。）

### 2.3 位置推进

```console
$ playerctl -p tingyu.instance104595 position
7.020667
$ sleep 2; playerctl -p tingyu.instance104595 position
9.029499        # Δ2.009 s
```

### 2.4 封面文件

```console
$ url=$(playerctl metadata --format '{{mpris:artUrl}}'); echo "$url"
file:///run/user/1000/tingyu/now-playing-104595-2919f6a4152890cd
$ ls -l /run/user/1000/tingyu/now-playing-104595-2919f6a4152890cd
-rw------- 1 halunhaku halunhaku 4621 11月12日 22:31 /run/user/1000/tingyu/now-playing-104595-2919f6a4152890cd
$ sha256sum /run/user/1000/tingyu/now-playing-104595-2919f6a4152890cd /tmp/m2-mpris/media/cover.png
7f7448e776d9e94e7084940e9aba25e71bc19271a87534127991902f8f14118c  /run/user/1000/tingyu/now-playing-104595-2919f6a4152890cd
7f7448e776d9e94e7084940e9aba25e71bc19271a87534127991902f8f14118c  /tmp/m2-mpris/media/cover.png
$ file /run/user/1000/tingyu/now-playing-104595-2919f6a4152890cd
PNG image data, 600 x 600, 8-bit/color RGBA, non-interlaced
```

切到无封面的 `03 Winter.m4a` 时 `mpris:artUrl` 消失，`/run/user/1000/tingyu/` 目录变空——与曲库一致。

### 2.5 播放 / 暂停 / 定位 / 换曲

```console
$ playerctl -p tingyu.instance104595 play-pause
$ playerctl -p tingyu.instance104595 status
Paused
$ p1=$(playerctl -p … position); sleep 1.5; p2=$(playerctl -p … position)
9.114512 -> 9.114512            # 暂停期间位置冻结

$ playerctl -p … play-pause; playerctl -p … status
Playing

$ playerctl -p … position 4; sleep 0.4; playerctl -p … position
4.383252
$ grep -n "media: command" /tmp/m2-mpris/run3/app.log | tail -4
556:media: command pause pos=9.092
562:media: command play pos=9.115
568:media: command changePlaybackPosition pos=4.000
570:media: command changePlaybackPosition pos=2.000
```

相对 Seek（另一轮）：`5.29 + 2 = 7.29` 被 mygo 夹到曲长 7 s，Summer 随即播完：

```console
$ playerctl -p … position 5; sleep 0.3; playerctl -p … position
5.288946
$ sleep 1.2; playerctl -p … position
6.496514
$ playerctl -p … position 2+; sleep 0.4; playerctl -p … position
0.404640                        # 已切到下一首
$ grep "changePlaybackPosition" /tmp/m2-mpris/run2/app.log | tail -2
619:media: command changePlaybackPosition pos=5.000
623:media: command changePlaybackPosition pos=7.000
```

换曲（修前，见 §4.1）：

```console
$ playerctl -p … metadata --format '{{xesam:title}} pos={{position}} dur={{mpris:length}}'
Winter pos=4845927 dur=14000000
$ playerctl -p … next
$ playerctl -p … metadata --format '{{xesam:title}} pos={{position}} dur={{mpris:length}}'
Spring pos=4843202 dur=10031020   # 新曲一开始就显示旧曲的 4.84 s
$ sleep 1
Spring pos=1139412 dur=10031020   # 下一次 500 ms 推送才纠正
```

`loop` / `shuffle` 没有实现（属库层，未改）：

```console
$ playerctl -p … loop
No player could handle this command
$ playerctl -p … shuffle
No player could handle this command
```

### 2.6 关窗退出

```console
$ hyprctl clients -j | jq -r '.[] | select(.pid==128570) | .address'
0x55c60bdb3630
$ hyprctl dispatch 'hl.dsp.focus({ window = "address:0x55c60bdb3630" })'
$ hyprctl dispatch 'hl.dsp.window.close()'
$ sleep 2; ps -p 128570
<无输出，进程已退出>
$ playerctl -l
No players found
```

应用日志：

```
window: closed
window: last window closed, quitting
app: quitting
session: now playing cleared
playback: engine closed
app: quit complete
```

---

## 3. 播放结束（B 段，单曲队列 + `TINGYU_REPEAT=off`）

### 3.1 修前

`playerctl` 观察到：`Playing` →（14 s 后）`Paused`，位置停在 `13.746187` 不动；`gdbus` 的 `PlaybackStatus` 同样是 `Paused`。应用日志每 500 ms 重复同一条状态，没有上限：

```console
$ playerctl -p … status; playerctl -p … position
Paused
13.746187
$ sleep 2; playerctl -p … status; playerctl -p … position
Paused
13.746187
$ grep -c 'session: state' /tmp/m2-mpris/run2b/app.log
287                             # 其中播完后的几百行全是同一条 paused/completed 状态
```

### 3.2 修后

```console
$ playerctl -p … status
Stopped
$ playerctl -p … position
0.000000                        # 注意：这是 playerctl 对 Stopped 播放器的显示值
$ gdbus call … Get org.mpris.MediaPlayer2.Player Position
(<int64 13746101>,)             # 真正的 MPRIS Position 仍是 13.746101 s，与界面 0:14 一致
$ gdbus call … Get org.mpris.MediaPlayer2.Player PlaybackStatus
(<'Stopped'>,)
$ gdbus call … Get org.mpris.MediaPlayer2.Player CanGoNext
(<false>,)                      # 单曲队列、repeat off
$ gdbus call … Get org.mpris.MediaPlayer2.Player CanGoPrevious
(<false>,)
$ grep -c 'session: state' /tmp/m2-mpris/run4/app.log
41                              # 只剩真正变化的帧；processing=completed 只出现 1 次
$ grep -c 'processing=completed' /tmp/m2-mpris/run4/app.log
1
```

之后 `playerctl play`：从 0 重新开始（`Position` 0.498 s / 0.5 s 后），状态回 `Playing`。

默认 RepeatAll（三首队列）没有出现 `Stopped` 闪烁：以 50 ms 间隔从 Winter 轮回到 Spring，只观察到 `Playing Winter → Playing Spring`。

---

## 4. 修了什么（`client/internal/app/session.go`）

改动集中在 `publishSession` 与两个纯函数；没有动界面、没有动 `internal/player`/`internal/mpv`/`mygo`。

### 4.1 换曲瞬间的位置

**现象**：换曲时 `update.Snapshot.Position` 仍是上一首的播放时间（mpv 加载新文件前如此），而 `Duration` 已经是新曲的曲库时长。MPRIS 上就是「新曲一开始在旧曲的位置」，还被 mygo 夹到曲长。

**修法**：`sessionPosition`（新增）在曲目变化（`track.ID` 或队列下标变）时把位置归 0，并始终夹到 `[0, duration]`。

### 4.2 播完的状态

**现象**：队列播完仍是 `Paused`，位置停在末尾不动，MPRIS 客户端（`playerctl`、桌面媒体面板）永远不会显示「已结束」。

**修法**：`sessionState`（新增）把 `Processing == mpv.Completed` 且不在播放映射为 `mygo.PlaybackStopped`；其余不变。

### 4.3 静止状态的 500 ms 重复推送

**现象**：暂停/播完后每 500 ms 都 `SetPlayback` 一次同样的状态，mygo 每次都发 `PropertiesChanged`，日志也每 500 ms 一行（§3.1 的 287 行）。

**修法**：只有位置真的变了（或元数据/语义变了）才推送；位置没变时不再重复。播放中仍是 500 ms 节流。

### 4.4 前后对照

| 场景 | 修前 | 修后 |
|---|---|---|
| Winter 播到 4.85 s 时 `next` | 立刻读到 `Spring pos=4843202`（旧曲位置） | 立刻读到 `Winter pos=4320`（新曲 ≈0） |
| 单曲队列播完 | `Paused`，位置 13.746 永不变 | `Stopped`，MPRIS Position 13.746101，`CanGoNext/Previous=false` |
| 播完后 | 每 500 ms 一行同样的状态，无上限（run2b 共 287 行） | `processing=completed` 只记 1 次（run4 共 41 行） |
| 暂停后位置 | 500 ms 重复推送同一位置 | 只推一次，位置冻结 |

新增测试（`internal/app/session_test.go`）：

- `TestSessionStateReportsTheEndAsStopped`：playing / paused / loading / completed / 最后一帧仍在播。
- `TestSessionPositionStartsANewTrackAtZero`：换曲归零、同曲保留、超长夹取、未知时长不夹、负值归零。

```console
$ CGO_ENABLED=0 go test ./internal/app/
ok  	github.com/halunhaku/tingyu/client/internal/app	0.016s
$ CGO_ENABLED=0 go test -short ./...   # 14 个包 ok，无 FAIL
```

---

## 5. 媒体键：这次测的是什么

**没有按真键盘的媒体键。** 所有播放控制都是 `playerctl` 发 MPRIS 方法调用模拟的（`Play`/`Pause`/`PlayPause`/`Next`/`Previous`/`SetPosition`/`Seek`），并用应用日志里的 `media: command <name> pos=…` 确认落到了播放器。

Omarchy 的真媒体键是另一条链（只读代码，未实测）：

| 环节 | 证据 |
|---|---|
| 键位 | `/usr/share/omarchy/default/hypr/bindings/media.lua`：`XF86AudioPlay/Pause → { ipc = "media.playPause" }`、`XF86AudioNext → media.next`、`XF86AudioPrev → media.previous` |
| 分发 | `/usr/share/omarchy/shell/shell.qml` 的 `runShortcut` 把 `media.*` 交给 `omarchy.media` 服务 |
| 服务 | `/usr/share/omarchy/shell/plugins/services/media/MediaModel.js`，自己读 MPRIS（`canGoNext`/`canGoPrevious`/`isPlaying`/元数据） |

也就是说：真媒体键 → Hyprland → omarchy-shell → MPRIS 方法，落到的是**同一个接口**。本记录证明的是该接口及其回传，**没有**证明物理按键、锁屏界面、Waybar/omarchy 面板上的显示。要补这一条，需要有键盘（或 `hyprctl dispatch` 注入 `XF86AudioPlay`，会走到 omarchy-shell）的人再测一次。

---

## 6. 未验证 / 局限 / 已知不一致（未在本轮修）

| # | 项 | 情况 | 归属 |
|---|---|---|---|
| 6.1 | 真键盘媒体键、锁屏媒体控件、omarchy 媒体面板 | **未验证**（见 §5） | 需要人手动 |
| 6.2 | `Volume` 恒为 1.0 | MPRIS 的 `Volume` 写死 1.0，应用调音量（`playerctl` 读到的仍是 1.0）；`MinimumRate`/`MaximumRate` 同样写死 | mygo 的 Linux 后端 `internal/linux/nowplaying.go`；`NowPlayingInfo` 没有 Volume 字段，`session.go` 无法补 |
| 6.3 | `LoopStatus` / `Shuffle` 不支持 | `playerctl loop`/`shuffle` 返回 `No player could handle this command`。应用本身有循环与随机模式，但 MPRIS 不暴露 | 同上（库层） |
| 6.4 | 两个实例的短名相同 | `playerctl -l` 得到 `tingyu.instance136568`、`tingyu.instance136569`（唯一）；但 `playerctl -a metadata` 两行都以 `tingyu` 开头，`Identity` 也都是 `tingyu`。`-p tingyu` 能选到一个，`-p tingyu.instance<pid>` 才能精确选。应用没有单实例锁，同时开两份就会在桌面面板出现两个「听屿」 | 设计/打包（M5 单实例锁可解）；不是 `session.go` 能修的 |
| 6.5 | 换曲时旧封面文件被立即删除 | 切到无封面曲目时 `/run/user/1000/tingyu/now-playing-*` 立刻消失；正在显示旧封面的客户端可能读到 404 | mygo |
| 6.6 | `DesktopEntry` 取决于启动环境 | 从 Cindy 的 shell 启动时继承了 `GIO_LAUNCHED_DESKTOP_FILE=/home/halunhaku/.local/share/applications/com.xd.cindy.user.hb1f8e09b8c0be48f.desktop`，MPRIS 里就变成 `com.xd.cindy.user.hb1f8e09b8c0be48f`；`env -u GIO_LAUNCHED_DESKTOP_FILE` 启动时为 `tingyu`。装 `.desktop`（M5）后从启动器打开就是对的 | 环境；M5 一并闭环 |
| 6.7 | 相同元数据的连续曲目共用 `mpris:trackid` | mygo 的 trackid 计数器只在标题/艺术家/专辑/时长变化时自增；两首四项完全相同的曲目会共用一个 id。**未构造该用例实测** | mygo；`session.go` 无法补 |
| 6.8 | 换曲瞬间 `PlaybackStatus` 短暂为 `Paused` | 新曲刚加载、mpv 还没开始播的那一帧，状态是 `Paused`（约 0–200 ms），随后 `Playing`。这是当时的真实状态，未再包装 | 设计选择，未改 |
| 6.9 | 播完后的位置 | `Stopped` 时 MPRIS `Position` 仍是 13.746 s（与界面 0:14 一致），而 `playerctl position` 对 Stopped 播放器显示 0.000000。再按播放会从 0 开始 | 客户端显示差异，非不一致 |

---

## 7. 复现步骤（最短路径）

```console
# 1) 构建
cd client && ./scripts/linux-build.sh

# 2) 造媒体（§1.2）到 /tmp/m2-mpris/media

# 3) 拿 playerctl（§1.1），起应用（§1.4）
TINGYU_ENRICH_AUTO=0 TINGYU_DATA_DIR=/tmp/m2-mpris/data \
  TINGYU_AUTO_SCAN_DIR=/tmp/m2-mpris/media TINGYU_AUTO_PLAY_INDEX=0 \
  TINGYU_STARTUP_LOG=/tmp/m2-mpris/app.log ./build/tingyu &

# 4) 看
export LD_LIBRARY_PATH=/tmp/playerctl-pkg/usr/lib
P=/tmp/playerctl-pkg/usr/bin/playerctl
$P -l; $P status; $P metadata; $P position
$P play-pause; $P status; $P next; $P position 4; $P previous

# 5) 播完：另起一份、单曲队列、关循环
TINGYU_REPEAT=off TINGYU_AUTO_PLAY_INDEX=2 ... ./build/tingyu
# 等 14 s，$P status 应为 Stopped
```

---

## 8. 截图

- `docs/screenshots/omarchy/m2-mpris-playing.png`：Spring 播放中（0:05），封面与 MPRIS `mpris:artUrl` 一致。
- `docs/screenshots/omarchy/m2-mpris-paused.png`：Winter 暂停在 0:01，播放键变播放图标；同一时刻 `playerctl` 为 `Paused / Winter / 1.861`。

两张都是修前二进制截的（修的是 MPRIS 发布，不改界面）；后续复截图时桌面被别的 agent 的全屏窗口盖住，没有再取。
