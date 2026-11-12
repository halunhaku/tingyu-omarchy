# Windows W0 技术验证记录（2026-11-01）

基线：`a876512`（`docs: add macOS preview download`），分支 `windows-w0`，工作区
`G:\tingyu-windows-w0`。本文记录 Windows W0 的验证与必要实现；macOS 行为保持不变
（平台差异都走独立文件）。

范围参考（在共享主目录、未跟踪，因此不在本 worktree）：
`G:\tingyu-windows\docs\2026-11-01-windows-migration-scope.md`。

验证机：Windows 11 专业工作站版 10.0.26300（build 26300），Intel Core Ultra 7
265K，150% DPI，默认音频设备经 WASAPI 共享模式。所有命令行均在该机执行。

## 1. 结论

| # | 验收项 | 环境 | 预期 | 实际 | 证据 | 结论 |
|---|---|---|---|---|---|---|
| 1 | 构建 Windows/amd64 | Go 1.27.3，CGO=0 | `go build ./...` 通过 | 通过；另验证 darwin/arm64 仍可构建 | §4 | ✅ |
| 2 | 真实窗口显示 | 同上，150% DPI | 可见、标题「听屿」、不小于 900×600 | 可见；client 1778×1038（dpi 144，scale 1.5） | `window-report.json` / `window-before.png` | ✅ |
| 3 | 窗口交互 | 同上 | 真实点击能触发播放；切换页面画面变化 | UIA 点击「播放全部」→ 日志 `play: request` + `sound after 135.9 ms`；点击「搜索」后截图像素哈希改变 | `window-report.json` | ✅ |
| 4 | 关闭与进程退出 | 同上 | 关窗后进程退出、引擎/会话/库关闭 | `WindowPattern.Close` → 退出码 0；日志 `engine closed`、`now playing cleared`、`quit complete`；DB 可重命名（无锁） | `window-report.json` / `window.log` | ✅ |
| 5 | SQLite 事务 | MyGo sqlite 3.53.4 | 提交可见、失败回滚、只读拒写、关闭重开一致 | 提交 2 行；故意失败的事务未留下部分行（仍 2 行）；只读连接 INSERT 被拒；重开后 2 行仍在 | `w0accept.json` `sqlite` | ✅ |
| 6 | 库层只读/重开 | 同上，中文+空格目录 | 建库、写入、只读重开拒写、可写重开仍见数据 | 全部符合；playlist「听屿 W0 测试」持久化 | `w0accept.json` `library` | ✅ |
| 7 | 数据目录 | 同上 | 默认 `%AppData%\com.halunhaku.tingyu`；覆盖值精确生效 | 默认与覆盖均符合；中文与空格保留 | `w0accept.json` `datadir` | ✅ |
| 8 | 无权限目录 | NTFS 拒绝写 | 建库失败且给出原因 | `mkdir ...: Access is denied.`，退出码 1 | `noperm.json` | ✅ |
| 9 | libmpv 身份 | shinchiro 20261101 | 记录实际加载路径/x64/版本/来源/哈希/许可 | `G:\...\build\mpv\libmpv-2.dll`，amd64，mpv v0.41.0-1374，FFmpeg N-128264，sha256 `f402…8154`，GPL-2.0-or-later（来源 release 20261101） | `w0accept.json` `extra` | ✅ |
| 10 | 真实播放进度/结束/释放 | WASAPI | 播放推进、到结尾、Close 后卸载 DLL | 位置 1.71 s / 时长 2 s，`Completed`（`reachedEnd=true`），AO init 1、underrun 0；`moduleUnloaded=true`，goroutine 1→1 | `w0accept.json` `mpv-playback`、`w0accept.log` | ✅（出声见 §2 说明） |
| 11 | 可诊断失败 | 同上 | 缺文件/错架构/缺符号/不可信位置均有明确原因 | 四种场景均被拒，错误含解析后的绝对路径与原因（缺文件、`built for PE machine 0x14c`、`missing symbol mpv_create`） | `w0accept.json` `mpv-diagnostics` | ✅ |
| 12 | 测试 | Windows/amd64 | `go test ./...` 全绿 | 14 个有测试的包全部 `ok` | `go-test.log` | ✅ |
| 13 | 干净无 Go 机器 | 未执行 | 从零安装并可复现 | **未验证**（本机预先装好了 Go 1.27.3） | — | ⚠️ 未验证 |
| 14 | 可听感确认 | 人耳 | 能听到 440 Hz | **未验证/无法自动确认**：仅有 WASAPI 实际打开设备、音频时钟推进、0 underrun 的客观证据，没有人耳听感记录 | `w0accept.log` | ⚠️ 未验证 |
| 15 | Windows 10 / arm64 / 其他音频设备 | 未执行 | — | 未测 | — | ⚠️ 未验证 |

结论口径：**技术验证通过（上述 1–12）**；**不能称为“完整分发通过”**：干净机器、
安装包/签名/更新、Windows 10/arm64、真实听感、凭据相关能力都还没验证（见 §8）。

## 2. 关于“出声”的证据边界

- 客观证据：WASAPI 设备以 48 kHz 立体声 float 打开（`AO: [wasapi]`）、音频播放启动、
  mpv 位置从 0 推进到 1.71 s、`AO inits=1`、`underruns=0`、播放到达 EOF
  （`Completed`），随后 `mpv_terminate_destroy` 并卸载 DLL。
- 未验证：没有人耳试听记录，也无法从日志证明扬声器实际可闻。本文不声称“听到了声音”，
  只声称“音频引擎打开了真实设备并完整播放了测试音”。

## 3. 工具链与依赖（版本、来源、校验值）

| 组件 | 版本/提交 | 来源 | SHA-256 |
|---|---|---|---|
| Go | go1.27.3 windows/amd64 | <https://go.dev/dl/go1.27.3.windows-amd64.zip> | `F26081F438319757F8A1B46D7D0B2C5EC7583D4038E485E0AA725CFDE7885FFF` |
| MyGo | `v0.6.5-0.20261025230045-be164835449b` | `client/go.mod` | go.sum 锁定 |
| purego | `v0.11.1` | `client/go.mod` | go.sum 锁定 |
| libmpv | mpv v0.41.0-1374-g48fca90e8，FFmpeg N-128264，client API 2.6，PE amd64 | shinchiro/mpv-winbuild-cmake `20261101`，`mpv-dev-x86_64-20261101-git-48fca90e8c.7z` | 压缩包 `30D3F9FA121CE24638610A9AE52F8957743F9DEA416DD37DC4FF7E43589EF843`；DLL `F40293DC5F4012F3F7BCC79DE2B9BFB66309B87DBD33A47EE49EBFF716EB8154`（121,474,048 B） |
| MyGo SQLite | SQLite 3.53.4 | `egoist/mygo` `sqlite-3.53.4-1`，`mygo-sqlite3-windows-amd64.dll` | `A9FA4B1EAE82D8F9826D6B3CAC20E337EBB29B5E859EAB3D2852D65B323224F4`（1,472,512 B） |

- 许可：libmpv 为 **GPL-2.0-or-later**（mpv-winbuild-cmake 构建，资产内无 LICENSE 文件；
  按 mpv 默认 GPL 构建判定）；FFmpeg 组件含 LGPL/GPL 部分；SQLite 公有领域；MyGo MIT。
- 依赖：导入均为 Windows 系统 DLL（kernel32/user32/mfplat/d3d11/dxgi/winmm 等），
  FFmpeg 静态链接，无第三方运行期 DLL。
- 工具链位置（仓库外）：`G:\tingyu-windows-tools\go`、`G:\tingyu-windows-tools\7zr.exe`。
  进入 PATH 的只有每个命令临时设置的 `G:\tingyu-windows-tools\go\bin`。

## 4. 代码改动

| 文件 | 改动 |
|---|---|
| `internal/mpv/lib.go` | 加载前校验路径/文件/架构；符号解析失败返回错误而不是 panic；新增 `Lib.Close` |
| `internal/mpv/library_windows.go` | `LoadLibraryEx` + `LOAD_WITH_ALTERED_SEARCH_PATH`；PE machine 校验；只从 `MPV_LIB`（绝对化）或 exe 同目录/`mpv\` 查找，不回退 CWD/PATH |
| `internal/mpv/library_unix.go` | 同样的路径校验；保留 dlopen `RTLD_LAZY\|RTLD_GLOBAL` 与 macOS framework/pub-cache 查找 |
| `internal/mpv/ao_windows.go` / `ao_darwin.go` / `ao_other.go` | 修复硬编码 `ao=coreaudio`：Windows 用 `wasapi` |
| `internal/mpv/player.go` | 平台默认音频驱动；`Player.PropertyString` |
| `internal/library/datadir_windows.go` / `datadir_darwin.go` | 数据目录平台化（Windows `%AppData%\com.halunhaku.tingyu`） |
| `internal/library/library.go` | `sameFolder` 处理 Windows 大小写/`\\?\` 前缀；`~` 兼容 `~\` |
| `internal/library/schema.go` | `library.Create`：首次运行建库并写 `user_version=2`（`OpenWritable` 仍不隐式建文件） |
| `internal/app/app.go` | 去 darwin 标签；首启建库；`revealPath`；**Windows 关最后一个窗口即退出**（macOS 保持原行为） |
| `internal/app/closequit_windows.go` / `closequit_darwin.go` | 平台关闭语义 |
| `internal/app/session.go` | 去 darwin 标签；`CmdOrCtrl` 快捷键 |
| `internal/app/metadata.go` / `sources.go` / `quarkfolders.go` | 去 darwin 标签（代码本身平台无关） |
| `internal/app/reveal_windows.go` / `reveal_darwin.go` | `explorer /select` / `open -R` |
| `internal/app/session_test.go` / `quarkfolders_test.go` | 去 darwin 标签，纳入 Windows 测试 |
| `internal/library/library_test.go` | 平台化断言；新增 `TestCreateMakesCurrentSchema` |
| `cmd/w0accept/main.go` (windows-only) | 事务/只读/重开、库层只读、数据目录、DLL 诊断、播放到结尾与卸载、`-create-db`/`-wav`/`-print-paths` |
| `scripts/windows-setup.ps1` | 带 SHA-256 校验下载 Go/libmpv/SQLite |
| `scripts/windows-build.ps1` | 构建；`-Test`/`-Accept`/`-Run`/`-Window` |
| `scripts/windows-window-test.ps1` | UIA 驱动的真实窗口显示/交互/关闭/退出验收 |

## 5. 构建与验收命令

```powershell
cd G:\tingyu-windows-w0\client
powershell -ExecutionPolicy Bypass -File .\scripts\windows-setup.ps1
powershell -ExecutionPolicy Bypass -File .\scripts\windows-build.ps1 -Test -Accept -Window
```

分项：

```powershell
# 构建
$env:PATH = "G:\tingyu-windows-tools\go\bin;$env:PATH"
$env:GOOS="windows"; $env:GOARCH="amd64"; $env:CGO_ENABLED="0"
go build -trimpath -o build\tingyu.exe .
go build -trimpath -o build\w0accept.exe .\cmd\w0accept

# SQLite / 数据目录 / DLL 诊断 / 播放与释放
.\build\w0accept.exe -mpv .\build\mpv\libmpv-2.dll -dir .\build\w0-evidence\w0accept

# 真实窗口（UIA 点击、关窗退出）
powershell -ExecutionPolicy Bypass -File .\scripts\windows-window-test.ps1

# 无权限目录
$base = "G:\tingyu-windows-w0\client\build\w0-evidence\noperm"
icacls "$base\denied" /inheritance:r /grant:r "$($env:USERNAME):(OI)(CI)(RX)" /deny "$($env:USERNAME):(OI)(CI)(W)"
.\build\w0accept.exe -create-db "$base\denied\data\library.sqlite"   # 预期失败：Access is denied
```

产物：`build\tingyu.exe`（20,997,632 B）、`build\w0accept.exe`、`build\mpv\libmpv-2.dll`、
`build\mygo-sqlite3.dll`。运行脚本 `scripts\windows-build.ps1 -Run` 会建临时库、生成
测试音、扫描、播放并在 15 s 后退出。

## 6. 关键日志（已脱敏，`C:\Users\…` → `%USERPROFILE%`，worktree → `<worktree>`）

窗口与交互（`window.log`）：

```
startup ready             432.0ms since main (process start unavailable)
scan: done in 74ms: 扫描完成：文件 1，入库 1（新增 1 / 更新 0 / 移除 0）
play: request "测试 音频" (<worktree>\client\build\w0-window\媒体 目录\测试 音频.wav) index=0
play: sound after 135.9 ms for "测试 音频" (index=0)
window: last window closed, quitting
session: now playing cleared
playback: engine closed
app: quit complete
```

SQLite / 库层（`w0accept.json`）：

```
sqlite:  committed 2 rows, rolled back a failed transaction (2 rows),
         read them after close/reopen, and a read-only connection refused INSERT
library: created schema, wrote playlist "听屿 W0 测试", read it after a read-only
         reopen, refused a read-only write, and still saw one playlist writable
datadir: default=%USERPROFILE%\AppData\Roaming\com.halunhaku.tingyu;
         override=%USERPROFILE%\AppData\Local\Temp\听屿 W0 覆盖 数据
```

libmpv（`w0accept.json` `extra`）：

```
library:         <worktree>\client\build\mpv\libmpv-2.dll
modulePath:      <worktree>\client\build\mpv\libmpv-2.dll
machine:         amd64
clientApi:       2.6
mpvVersion:      mpv v0.41.0-1374-g48fca90e8
ffmpegVersion:   N-128264-g078390418
sha256:          f40293dc5f4012f3f7bcc79de2b9bfb66309b87dbd33a47ee49ebff716eb8154
license:         GPL-2.0-or-later (mpv-winbuild-cmake build; the asset ships no LICENSE file)
source:          https://github.com/shinchiro/mpv-winbuild-cmake/releases/tag/20261101
audioOutput:     wasapi
positionMax:     1.71s of 2s   reachedEnd: true
aoInits: 1       underruns: 0   filesLoaded: 1
moduleUnloaded:  true           goroutines: 1 -> 1
```

DLL 可诊断失败（`mpv-diagnostics`）：缺文件 → `libmpv DLL not found at <abs>`；
改 machine 为 i386 的副本 → `built for PE machine 0x14c, this process is amd64`；
`System32\version.dll` → `missing symbol mpv_create`；裸相对名 → 解析为绝对路径后报缺失。
无可信位置静默加载：查找只接受 `MPV_LIB`（绝对化）或 exe 同目录/`mpv\`，不回退 CWD/PATH。

## 7. 测试

`go test ./...`（Windows/amd64）：14 个有测试的包全绿（`internal/library` 85 s、
`internal/shell` 20 s 等），日志 `go-test.log`。`go build ./...` 同时验证
`windows/amd64` 与 `darwin/arm64` 均可构建。仅调整了两处 macOS 专属断言
（`/private` 前缀、路径分隔符），未放宽行为。

## 8. 风险与未验证

1. **许可（阻断对外分发）**：libmpv 为 GPLv2+，仓库 MIT。分发前必须做 GPL 合规或改用
   LGPL 版 libmpv，并核对 FFmpeg 组件许可；未做法律审查。
2. **干净机器未实测**：本机已有 Go 1.27.3。`windows-setup.ps1` 的下载+校验流程在
   已有缓存/工具的上机执行过，但“全新无 Go 机器从零跑通”**未验证**，不能标完整分发通过。
3. **安装与签名**：无安装器、代码签名、更新、开始菜单/文件关联；没有用
   `go tool mygo build` 打包（该路径未验证），当前是 exe + 同目录 DLL。
4. **平台覆盖**：仅 Windows 11 amd64、单机、150% DPI、一个默认音频设备；Windows 10、
   arm64、独占模式、热插拔、无音频设备、多显示器未测。
5. **听感未验证**：见 §2；只有 WASAPI 打开设备、时钟推进、0 underrun、EOF 的客观证据。
6. **功能面**：WebDAV/夸克登录与同步、媒体键/SMTC 命令回传、电源事件、拖拽、长播稳定性
   未逐项验收；凭据能力不作为 W0 条件，后续单独做安全存储、分片、删除恢复验收。
7. **行为差异**：Windows 关窗即退出（本次新增）；macOS 仍保持关窗继续播放。数据目录
   使用 Roaming，漫游配置下可能偏大；首启自动建库为新增行为，需产品确认。
8. **CI**：仍只有 macOS；Windows 构建/测试未入 CI，工具链未固定进仓库。

## 9. 证据与产物（绝对路径）

证据（`build/` 被 `.gitignore` 忽略，未入库）：

- `G:\tingyu-windows-w0\client\build\w0-evidence\w0accept.json`、`w0accept.log`
- `G:\tingyu-windows-w0\client\build\w0-evidence\w0accept\run-<id>\`（SQLite 库、WAV）
- `G:\tingyu-windows-w0\client\build\w0-evidence\window\window-report.json` 及
  `window-before.png`、`window-before-nav.png`、`window-after-nav.png`
- `G:\tingyu-windows-w0\client\build\w0-window\window.log`
- `G:\tingyu-windows-w0\client\build\w0-evidence\noperm.json`
- `G:\tingyu-windows-w0\client\build\w0-evidence\app-integration.log`、`go-test.log`
- 脱敏副本：`G:\tingyu-windows-w0\client\build\w0-evidence\sanitized\`

可运行产物：

- `G:\tingyu-windows-w0\client\build\tingyu.exe`
- `G:\tingyu-windows-w0\client\build\w0accept.exe`
- `G:\tingyu-windows-w0\client\build\mpv\libmpv-2.dll`
- `G:\tingyu-windows-w0\client\build\mygo-sqlite3.dll`
