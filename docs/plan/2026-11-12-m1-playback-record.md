# 听屿 Omarchy 移植 · M1 播放内核与出声记录

日期：2026-11-12（Asia/Shanghai）
性质：**M1 验收记录**（播放内核与出声）。判据来源：[迁移范围 §3 M1](./2026-11-12-omarchy-migration-scope.md)。
本轮改动只落在验收装置 `client/internal/accept/`（3 处）；**`internal/mpv` 与 `internal/player` 一行未改**——所有失败项最后都定位到验收装置对 mpv 行为的假设，不是播放链路（§2 逐条给了判定依据）。
数据目录一律 `/tmp/tingyu-m1/data*`，本轮没有一条命令指向生产库 `~/.local/share/com.halunhaku.tingyu`（核对见 §7）。

---

## 0. 结论速览

客观证据全部来自本机实测；主观听感单列 §5，本轮**没有**人耳结论。

| # | M1 验收项 | 结论 | 一句话证据 |
|---|---|---|---|
| 1 | 出声走 PipeWire | **通过** | 9/9 格式 `AO: [pipewire] …`；PipeWire 侧 `Sink Input #2203 … node.name = "听屿"`、`Corked: no` |
| 2 | 采样率 | **通过** | 设备采样率跟随源：44.1k（mp3/m4a/aac/wav/ogg/aiff/alac）、48k（opus）、96k（96 kHz FLAC）；PipeWire 侧同一流为 `s16le 1ch 96000Hz` |
| 3 | underrun | **通过（0 次）** | 9 格式 `underruns=0`；M1 自有 4 个文件同样 0 |
| 4 | 音频设备重建次数 | **通过** | 单曲 `aoInits=1`；同专辑两首连播仍 `aoInits=1`、渲染波形 `silenceRuns=0` |
| 5 | 位置推进 | **通过** | 1 s 墙钟推进 0.92–0.99 s；14 s 音轨播到 10.01–10.06 s；应用日志 `pos` 从 0.51 递增到 13.78 |
| 6 | 播放到结尾 | **通过** | `mpv: end-file reason=0 err=0` → `processing=completed playing=false pos=1.73 dur=2.0`；不循环时点播放会重新加载并继续推进 |
| 7 | seek | **通过** | 9 格式 seek 到 50 %，恢复 **52–54 ms**（沿用仓库原有 pass 条件，未新造阈值） |
| 8 | 音量 | **通过** | 0.35 → 回读 0.35；0.8 → 回读 0.80 |
| 9 | hwdec | **不参与播放（实测 `hwdec=no`）** | 内核 `vo=null` + `audio-display=no`，全仓无 `hwdec` 选项；`hwdec-current` 不可用（无视频解码器）。独立 mpv 探针另证本机硬件解码可用（`vulkan-copy`），但不走这条路 |
| 10 | 主观听感 | **未验证** | 无自动化证据，需人耳，见 §5 |

附带结论：M2 验收 harness 在本机从 **22/25** 修到 **25/25**（连跑 3 次稳定），失败的 3 项全部是验收装置的问题（§2.1）。

---

## 1. 环境

| 项 | 值 |
|---|---|
| 机器 | x86_64，20 核 / 46 GB（i915 `0000:00:02.0` + amdgpu `0000:04:00.0`，`renderD128`/`renderD129`） |
| 系统 | Omarchy 4.5.0-3，`Linux 7.2.15-1-omarchy` |
| 会话 | Hyprland **0.56.2**（Wayland） |
| Go | `go1.27.3 linux/amd64`，`CGO_ENABLED=0` |
| libmpv | 系统 `libmpv.so → libmpv.so.2 → libmpv.so.2.5.0`（**mpv v0.41.0**），client API **2.5**，`purego` dlopen，无 `MPV_LIB` |
| 音频服务 | PipeWire（`/run/user/1000/pipewire-0` 与 `/run/user/1000/pulse/native` 均在） |
| ffmpeg | `n9.0.2`（只用于生成测试音） |

启动日志里的自证：`playback: libmpv libmpv.so.2 client API 2.5`。

---

## 2. 本轮改动与判定依据

### 2.1 验收装置（`client/internal/accept/`）

首轮 M2 全量结果是 **22/25**：`queue-200`、`loop-off`、`loop-one` 三项失败；同一条命令在 Lead 的机器上是 24/25（只有 `queue-200`）。重复实验后确认三项都不是播放链路的问题：

| 现象 | 实测根因 | 判定 |
|---|---|---|
| `queue-200`：`观察到的曲目数 201/401/600/800 != 200`，耗时 20 s → 60/80 s | ① mpv 的 `path` 先报 loadfile 给的**相对路径**、流打开后再报**绝对路径**，同一首被记成两次；② `player.New` 默认 `RepeatAll`，队列末尾会**自动回绕**，harness 等 `Completed` 时可能错过那个瞬时窗口，于是整轮又播一遍 | harness 假设不成立，不是 bug |
| `loop-one`：`pathChanged=true` | 同一首的 `firstPath` 取到了相对形式，循环后变成绝对形式，被判成“换了文件” | 同上 |
| `loop-off`：`replayed=false`，但 `filesReloaded=1` | 队列用 0.2 s 的 `queue/*.wav`；“重播”的可观测窗口只有约 200 ms，且末尾一串强制发布可能把控制器的快照通道（容量 8、满即丢）打满。改用 3 s 音轨后连跑 6 次全部通过（§4.6） | harness 时序假设不成立 |

改法（3 处，全部在验收装置里）：

| 文件 | 改动 | 为什么 |
|---|---|---|
| `internal/accept/session.go` | 新增 `normalizePath`（`filepath.Abs` 归一） | mpv 的 `path` 有相对/绝对两种形态；比较原始字符串会把一首记成两首 |
| `internal/accept/queue.go` | `runQueue` 显式 `SetRepeat(RepeatOff)`；观察按归一化路径去重；曲目下标允许“路径先行、`playlist-pos` 后到”的 1 步滞后；`loop-one` 用归一化路径比较；`loop-off` 末尾改用 3 s 音轨并要求位置推进 >300 ms | 四条都是上表实测出来的假设错误；`queue-200` 现在稳定在 200 个观察、19.9 s |
| `internal/accept/report.go`、`runner.go` | `hostInfo` 去掉硬编码 `"coreaudio"`，按 `runtime.GOOS` 给标签 | 首轮 `report.json` 里 `"audioOutput": "coreaudio"` 是 Linux 上产出的**假数据**，记录引用即失真 |

### 2.2 为什么没改产品代码

`internal/mpv`/`internal/player` 在这三条失败里都只是在做正确的事：

- 相对路径 → 绝对路径是 mpv 的属性行为，内核本来就靠 `playlist-pos` 兜底下标（`player.go` 的 `onProperty` 只在 URI 完全相等时才用 path 纠正下标）。应用里库里存的是绝对路径，这条路径根本不会出现。
- 队列末尾回绕是 `RepeatAll` 的**设计**（`controller.go:156` 注释写明沿用 Flutter 客户端默认值），harness 没关掉它而已。
- 0.2 s 音轨上“重播有没有被看到”是观测问题：直接轮询 `engine.Snapshot()` 时 4/4 都能看到 `Playing=true`（§4.6）。

因此按“只在证据显示有问题时改代码”的约束，产品代码保持不动。

---

## 3. 复现命令

```bash
cd client
./scripts/linux-setup.sh && ./scripts/linux-build.sh          # → build/tingyu
./scripts/gen-m2-media.sh                                     # → build/m2-media（214 文件，16 MB）
CGO_ENABLED=0 ~/.local/share/tingyu-tools/go/bin/go build -o build/accept ./cmd/accept

# M2 全量：本轮为 25/25
./build/accept -media build/m2-media -out build/m2-evidence/final
```

M1 自有素材（440 Hz 正弦 10 s ×3 + 2 s 短文件，全在 `/tmp`）：

```bash
M1=/tmp/tingyu-m1; mkdir -p "$M1"/{media/m1,data,logs}
tone="-f lavfi -i sine=frequency=440:sample_rate=48000:duration=10"
ffmpeg -hide_banner -loglevel error -y $tone -c:a libmp3lame -b:a 192k "$M1/media/m1/tone.mp3"
ffmpeg -hide_banner -loglevel error -y $tone -ar 96000 -c:a flac      "$M1/media/m1/tone.flac"
ffmpeg -hide_banner -loglevel error -y $tone -c:a aac -b:a 192k       "$M1/media/m1/tone.m4a"
ffmpeg -hide_banner -loglevel error -y -f lavfi \
  -i sine=frequency=440:sample_rate=48000:duration=2 -c:a flac        "$M1/media/m1/short.flac"
```

应用本体（窗口、扫描、起播、自动退出，数据目录指向 `/tmp`）：

```bash
TINGYU_DATA_DIR=/tmp/tingyu-m1/data \
TINGYU_STARTUP_LOG=/tmp/tingyu-m1/logs/app-short.log \
TINGYU_AUTO_SCAN_DIR=/tmp/tingyu-m1/media/short-only \
TINGYU_AUTO_PLAY_TITLE=short TINGYU_REPEAT=off TINGYU_AUTO_QUIT_MS=7000 \
./build/tingyu
```

> `TINGYU_AUTO_PLAY_INDEX` 不等待扫描完成（`internal/app/app.go` 的 `acceptancePlay` 直接查库），首启会拿到空结果；用 `TINGYU_AUTO_PLAY_TITLE` 会先 `waitForScan()`。本轮两种都跑过，索引那条也能成，只是因为 9 文件的扫描只要 3 ms——属于抢跑成功，不是有保证的行为。

---

## 4. 客观证据

### 4.1 M2 全量：25/25

```
22:57:54.843 format mp3  played=true seek=true pos=7.03 dur=14.00 resume=52ms ao=1
22:58:05.053 format flac played=true seek=true pos=7.03 dur=14.00 resume=53ms ao=1
22:58:15.210 format m4a  played=true seek=true pos=7.03 dur=14.00 resume=53ms ao=1
22:58:25.362 format aac  played=true seek=true pos=7.05 dur=14.05 resume=53ms ao=1
22:58:35.527 format wav  played=true seek=true pos=7.03 dur=14.00 resume=53ms ao=1
22:58:45.679 format ogg  played=true seek=true pos=7.03 dur=14.00 resume=52ms ao=1
22:58:55.855 format opus played=true seek=true pos=7.03 dur=14.01 resume=54ms ao=1
22:59:06.032 format aiff played=true seek=true pos=7.03 dur=14.00 resume=53ms ao=1
22:59:16.188 format alac played=true seek=true pos=7.03 dur=14.00 resume=53ms ao=1
22:59:21.754 gapless: completed=true aoInits=1 files=2 rendered=6.000s silenceRuns=0
22:59:21.763 queue replace: entries=3 leftovers=0 first=track0003.wav
22:59:41.676 queue: observed=200 completed=true indexAtEnd=199 elapsed=19910ms
22:59:45.349 loop off: completed=true replayed=true advanced=true
22:59:46.034 loop all: wrapped=true seq=[0 1 2 0]
22:59:51.048 loop one: pathChanged=false completed=false playing=true
22:59:55.113 rate: 0.5×→0.54 1.8×→1.65
22:59:55.256 volume: 0.35→0.35 0.8→0.80
22:59:57.931 buffered remote: maxAhead=23.76s samples=79
25/25 passed
```
（原文：`/tmp/tingyu-m1/logs/accept-final.log`，报告：`build/m2-evidence/final/report.json`；全量整套连跑 3 次均为 25/25，最后一次就是本文引用的这份日志）

变基到 main `13ead9e`（其间合入了 MPRIS、主题、分发的其他工作）后整套重跑一次，同样 **25/25**，形态一致（seek 51–53 ms、gapless `aoInits=1`/`silenceRuns=0`、queue 200 个观察/19.9 s、`loop off advanced=true`）：`/tmp/tingyu-m1/logs/accept-rebased.log`。

### 4.2 AO 后端、采样率、underrun（9 格式）

`report.json` 里每个 format 的 `data.audioOutput` 就是 mpv 那条 `AO:` 原始行（前缀是 harness 回调加的时间戳字段）：

| check | `AO:` 原始行 | aoInits | underruns | 播到 | seek 恢复 |
|---|---|---|---|---|---|
| format-mp3 | `mpv: info: cplayerAO: [pipewire] 44100Hz mono 1ch floatp` | 1 | 0 | 10.06 s | 52 ms |
| format-flac | `mpv: info: cplayerAO: [pipewire] 96000Hz mono 1ch s32` | 1 | 0 | 10.06 s | 53 ms |
| format-m4a | `mpv: info: cplayerAO: [pipewire] 44100Hz mono 1ch floatp` | 1 | 0 | 10.04 s | 53 ms |
| format-aac | `mpv: info: cplayerAO: [pipewire] 44100Hz mono 1ch floatp` | 1 | 0 | 10.01 s | 53 ms |
| format-wav | `mpv: info: cplayerAO: [pipewire] 44100Hz mono 1ch s16` | 1 | 0 | 10.03 s | 53 ms |
| format-ogg | `mpv: info: cplayerAO: [pipewire] 44100Hz stereo 2ch floatp` | 1 | 0 | 10.01 s | 52 ms |
| format-opus | `mpv: info: cplayerAO: [pipewire] 48000Hz mono 1ch floatp` | 1 | 0 | 10.05 s | 54 ms |
| format-aiff | `mpv: info: cplayerAO: [pipewire] 44100Hz mono 1ch s16` | 1 | 0 | 10.05 s | 53 ms |
| format-alac | `mpv: info: cplayerAO: [pipewire] 44100Hz mono 1ch s16p` | 1 | 0 | 10.01 s | 53 ms |

要点：**后端全是 pipewire**；**设备采样率等于源采样率**（没有强制重采样到 48 k）；**underrun 全 0**；每个文件**只初始化一次音频设备**。

### 4.3 M1 自有 4 个文件（属性直读）

用 `client/build/m1probe`（临时探针，`build/` 已 git-ignore，不入库）直接读 mpv 属性，不经日志文本。原文：`/tmp/tingyu-m1/logs/props.log`。

| 文件 | `audio-params` / `audio-out-params` | `hwdec` | `hwdec-current` | 1 s 墙钟推进 | 结尾 |
|---|---|---|---|---|---|
| tone.mp3（48 kHz 源） | `{"samplerate":48000,…,"format":"floatp"}` | `no` | 不可用 | 0.966 s | completed 9.75/10.00 |
| tone.flac（96 kHz 源） | `{"samplerate":96000,…,"format":"s16"}` | `no` | 不可用 | 0.966 s | completed 9.73/10.00 |
| tone.m4a（48 kHz 源） | `{"samplerate":48000,…,"format":"floatp"}` | `no` | 不可用 | 0.944 s | completed 9.75/10.00 |
| short.flac（2 s） | `{"samplerate":48000,…,"format":"s16"}` | `no` | 不可用 | 0.923 s | completed 1.73/2.00 |

四个文件的 `aoInits` 均为 1、`underruns` 均为 0、首帧到“在播”5–6 ms。`ao` 属性为空是因为 Linux 上 `ao_other.go` 故意不指定，交给 mpv 自选——真正的后端由 `AO:` 行给出（§4.2）。

### 4.4 PipeWire 服务端旁证

播放 96 kHz FLAC 时 `pactl list sink-inputs`（原文：`/tmp/tingyu-m1/logs/pactl-sink-inputs.txt`）：

```
Sink Input #2203
	Driver: PipeWire
	Sample Specification: s16le 1ch 96000Hz
	Channel Map: front-center
	Corked: no
	Mute: no
	Properties:
		node.name = "听屿"
		application.name = "听屿"
		node.rate = "1/96000"
		media.name = "tone.flac - mpv"
		stream.is-live = "true"
		media.class = "Stream/Output/Audio"
```

这比“mpv 说自己在播”更强：服务端确实有一条**未静音、未暂停、96 kHz** 的听屿音频流。

### 4.5 应用本体端到端

9 格式目录，`TINGYU_AUTO_PLAY_INDEX=0`、`TINGYU_AUTO_QUIT_MS=15000`（原文：`/tmp/tingyu-m1/logs/app-lead-cmd.log`）：

```
playback: libmpv libmpv.so.2 client API 2.5
scan: done in 3ms: 扫描完成：文件 9，入库 9（新增 9 / 更新 0 / 移除 0）
play: request "tone" (…/build/m2-media/formats/tone.aac) index=0
session: metadata id=… title="tone" artist="未知艺术家" artwork=false … duration=14.024852607s state=1
play: sound after 105.3 ms for "tone" (index=0)
session: state playing=true rate=1.00 pos=0.514 dur=14.0 processing=ready
…（逐 0.5 s 递增）…
session: state playing=true rate=1.00 pos=13.776 dur=14.0 processing=ready
mpv: end-file reason=0 err=0
mpv: state ready playing=true pos=0.00 dur=0.00 index=1 path=…/formats/tone.aiff
app: auto quit
session: now playing cleared
playback: engine closed
app: quit complete
window: closed
```

M1 自有素材（3 首 `tone.*`，`TINGYU_AUTO_PLAY_TITLE=tone`、`TINGYU_REPEAT=off`，原文 `app-m1-tone.log`）：`play: sound after 106.0 ms`，10 s 处 `end-file reason=0` 后自动切到 `index=1`，PipeWire 侧同一时刻 `node.rate = "1/96000"`。

变基后（main `13ead9e`）用同一条命令重跑：`play: sound after 181.8 ms`、14 s 处 `end-file reason=0`、`app: quit complete`（`/tmp/tingyu-m1/logs/app-lead-cmd-rebased.log`）。

> 应用本体的 mpv 日志级别是默认 `warn`，所以 `AO:`（info 级）**不会**出现在应用日志里；应用侧的出声结论来自“位置推进 + PipeWire 服务端流 + `sound after`”，后端结论来自 §4.2 的内核路径（同一份 `internal/mpv`，同一台机器）。

### 4.6 播放到结尾 / 不循环重播

单曲 2 s，`TINGYU_REPEAT=off`（原文：`/tmp/tingyu-m1/logs/app-short.log`）：

```
play: sound after 119.0 ms for "short" (index=0)
mpv: end-file reason=0 err=0
session: state playing=false rate=1.00 pos=1.734 dur=2.0 processing=completed
session: keeps awake false
session: now playing cleared
playback: engine closed
app: quit complete
```

内核侧 `loop-off`：`completed=true playingAtEnd=false replayed=true advanced=true`，`filesReloaded=1`（重播走的是重新加载，不是复用陈旧状态）。

变基后重跑同一用例：`end-file reason=0` → `processing=completed`（`/tmp/tingyu-m1/logs/app-short-rebased.log`）。该日志开头另有一条 `end-file reason=2`，是 `SetQueue` 的 stop 命令造成的 STOP，不是失败；`failure`/`error` 关键字为零。

排查记录：0.2 s 音轨重播的可观测窗口太短。用 `m1probe -mode replay` 经控制器轮询 12 次，有 5 次没看到 `Playing=true`（mpv 侧 `pause` 最终停在 `yes`，是重播的 0.2 s 已经播完、控制器又按“队列结束”暂停了一次）；同一段重播直接轮询 `engine.Snapshot()` 时 4/4 都能看到 `Playing=true`。也就是说重播本身是好的，是经控制器快照通道的**观测**不可靠——所以这项改成 3 s 音轨 + 位置推进双条件（§2.1）。

### 4.7 hwdec：实测值与其边界

内核属性（§4.3）：`hwdec = no`、`hwdec-current` **不可用**（没有视频解码器）。

原因在代码里：`internal/mpv/player.go` 的引擎选项是 `vo=null` + `audio-display=no`，全仓没有 `hwdec` 选项；封面由 Go 侧 `internal/images` 解码，不经过 mpv。**因此“hwdec=auto-safe 是否可用”对听屿没有影响，也没有“降级/黑屏”这个风险面。**

本机硬件解码能力另行实测（**独立 mpv 探针，不是听屿的路径**）：

```
$ mpv --no-config --hwdec=auto-safe --vo=null --frames=2 -v probe.mp4
[vd] Looking at hwdec h264-vaapi...
[vd] Looking at hwdec h264-vulkan-copy...
[vd] Using hardware decoding (vulkan-copy).
```

即：VA-API 驱动在位（`/usr/lib/dri/iHD_drv_video.so`、`radeonsi_drv_video.so`、`/usr/lib/libva.so.2`），mpv 最终选的是 `vulkan-copy`；但这条路径与听屿的音频播放无关。

### 4.8 seek / 音量 / 倍速

- seek：9 格式 seek 到 50 %，`seekResumeMs` 52–54 ms，恢复后位置 7.03–7.05 s（§4.1）。
- 音量：`SetVolume(0.35)` → 回读 0.35；`SetVolume(0.8)` → 回读 0.80。
- 倍速：0.5× 实测 0.54 倍推进、1.8× 实测 1.65 倍推进（仓库原有容差 ±35 %，通过）。
- 应用内拖动/音量按钮：无自动化入口，未驱动（§6）。

---

## 5. 主观听感

**本轮没有。** 我没有听觉，也没有让任何人试听，所以不写“听到了声音”。若需要人耳结论，请在应用本体播放 §3 的 440 Hz 文件后自行记录（听感、有无爆音/断续、耳机与扬声器是否都出声），再补进本节。

能替代“听到”的客观证据是：PipeWire 服务端有一条 `Corked: no` 的听屿音频流 + 设备采样率正确 + `underruns=0` + 位置按实时推进。这些都证明**音频数据被送到了音频服务器**，但不等价于“扬声器发出了正确的声音”。

---

## 6. 未验证项

| 项 | 为什么没验 | 建议 |
|---|---|---|
| 主观听感 | 见 §5 | 人耳试听一次即可闭环 |
| 非默认音频设备 | 本轮只跑默认 sink；未切耳机/HDMI/蓝牙，也未做热插拔 | `wpctl set-default` 后重跑 §3 的应用命令 |
| 独占/直通（高解析、DSD） | M1 范围外 | 后续单独评估 |
| 应用内 seek 与音量按钮 | 没有可从外部驱动的入口（`TINGYU_*` 无 seek/volume） | 内核级已验（§4.8）；按钮级靠 UI 手测 |
| gapless 在真实 PipeWire 上的波形 | harness 用 `ao=pcm` 渲染波形 + `AO: [` 计数，能证明“单一音频流未重建”，但不是在 PipeWire 上采样 | 需要时用 `pw-record` 抓流 |
| 1080p 视频/USB 声卡等长稳场景 | 听屿是纯音频客户端 | 不在 M1 范围 |
| 干净机器复验 M0 的 SONAME 探测 | 本轮是本机；干净机器仍待验 | M0 遗留项 |
| hwdec 对封面的影响 | 封面走 Go 解码，完全不经过 mpv | 不需要验，但应在文档里说明 |

---

## 7. 与文档/代码的关系

- 证据文件（全部不入库）：`/tmp/tingyu-m1/logs/`（`accept-final.log`、`props.log`、`pactl-sink-inputs.txt`、`app-lead-cmd.log`、`app-m1-tone.log`、`app-short.log`、`m1-media.sha256`）与 `client/build/m2-evidence/final/report.json`。
- 代码改动：仅 `client/internal/accept/`（§2.1）。`git diff` 未触及 `internal/mpv`、`internal/player`、`internal/app`。
- 质量门：`gofmt -l`（空）、`go vet ./...`、`go test -short ./...` 全绿、`GOOS=windows` / `GOOS=darwin` 交叉编译通过。
- 生产库未被本轮命令使用：四条应用命令的日志里数据目录全是 `/tmp/tingyu-m1/data*`（`library: created /tmp/…`）。核对时生产库 `~/.local/share/com.halunhaku.tingyu/library.sqlite` 为 **0 曲目**、0 来源、0 歌单，mtime `2026-11-12 22:49:44` 落在本轮时间窗内但**无法归属**——本机同时有其他 agent 在跑，本 worker 没有一条命令指向它，也不含任何 `/tmp/tingyu-m1` 或 `build/m2-media` 的曲目内容。
- 注意：[迁移范围 §3.1](./2026-11-12-omarchy-migration-scope.md) 里「24/25，`queue-200` 观测 201 次」是 harness 修复**之前**的结论，根因与修法见本文 §2.1；那一节要不要改指向本文，留给它的作者决定，本文不代改。
- 建议同步修订 [迁移范围 §3 M1](./2026-11-12-omarchy-migration-scope.md)：把“确认 `hwdec=auto-safe` 在本机可用（VA-API）”改成“确认 hwdec 是否参与播放路径；本移植 `vo=null`+`audio-display=no`，实测 `hwdec=no`，不参与”。
