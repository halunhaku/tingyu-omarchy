# 听屿 Omarchy 迁移 · M4「Omarchy 观感」实施与验收记录

日期：2026-11-12（Asia/Shanghai）
性质：**M4 的实施与验收记录**。任务口径见 [`2026-11-12-omarchy-migration-scope.md`](2026-11-12-omarchy-migration-scope.md) §3 M4：「主题跟随（读 colors.toml 的 accent，并处理主题目录被整体替换）」「圆角跟随 Hyprland decoration:rounding，0 时内部面板用直角」。
验证机：Omarchy 4.5.0-3，Hyprland 0.56.2（Wayland），x86_64，显示器 DP-5 3840×2160 @ scale 1.6；当前主题 `tokyo-night`（`mode = "dark"`，`accent = "#7aa2f7"`），`decoration:rounding` 0。
构建：Go 1.27.3 linux/amd64，`CGO_ENABLED=0`，MyGo `v0.6.5-0.20261025230045-be164835449b`。

---

## 0. 结论

| # | 验收项 | 结论 | 证据 |
|---|---|---|---|
| 1 | 界面跟随 Omarchy 主题的 accent | **通过** | 暗色：`#7aa2f7`（[`m4-theme-tokyo-night.png`](../screenshots/omarchy/m4-theme-tokyo-night.png)）；浅色：`#1e66f5`（[`m4-theme-catppuccin-latte.png`](../screenshots/omarchy/m4-theme-catppuccin-latte.png)） |
| 2 | 跟随主题的 `mode`（浅/深） | **通过** | 切到 catppuccin-latte（`mode = "light"`）后整套换浅色，无需改设置 |
| 3 | 圆角跟随 `decoration:rounding`，0 为直角 | **通过** | 0 时全直角（#1）；6 时按 6/8 收窄（[`m4-rounding-6.png`](../screenshots/omarchy/m4-rounding-6.png)） |
| 4 | 启动即正确，不闪一下设计色 | **通过** | 日志 `desktop: theme tokyo-night …` 早于 `startup window created`（§2.1） |
| 5 | 主题/圆角变化后自更新、无需重启 | **通过** | 轮询 2 s；两次切换与还原都由日志与截图确认（§2.2、§2.3） |
| 6 | 不改 `tokens.Light` / `tokens.Dark` 字面量 | **通过** | 两处字面量零改动，`TestTokensAreTheDesignScale` 仍绿 |
| 7 | 纯标准库、不引入 fsnotify | **通过** | 读取器只用 `os`/`path`/`strconv`/`encoding/json`/`os/exec`/`context`/`time` |
| 8 | 构建、测试、交叉编译 | **通过** | `go build ./...`、`go test -short ./...`、`GOOS=windows`/`GOOS=darwin go build ./...`、`go vet`、`gofmt -l` 全过（§2.4） |
| 9 | 不留环境改动 | **通过** | `theme.name` 回到 `tokyo-night`，`getoption` 回到 0；还原后帧与切换前逐字节相同（md5 `e49583e7…`） |
| 10 | 设置页说真话 + 「跟随桌面主题」开关 | **通过（追加，见 §5）** | 文案改为 Omarchy/Hyprland 口径；开关默认开，关掉后回到设计色与圆角 |

一句话：**accent、明暗、圆角三项都从桌面读，启动时读一次、之后 2 秒一轮，切换与还原都当场验证过；`~/.config` 与 Omarchy 状态没有留下任何改动。**

---

## 1. 改了什么

### 1.1 读桌面（`internal/ui/tokens`）

| 文件 | 内容 |
|---|---|
| `desktop.go` | `Desktop`（theme.name / mode / accent / rounding，带「读到了」标志）、`Corners`、`Accented`、`onAccent`（WCAG 对比度选黑白）、`Desktop.Equal` |
| `desktop_linux.go` | Linux 读取：`$XDG_STATE_HOME/omarchy/current`（未设或非绝对路径则 `~/.local/state/omarchy/current`）→ `theme.name` + `theme/colors.toml` 的 `mode`/`accent`；`hyprctl -j getoption decoration:rounding` 700 ms 超时 |
| `desktop_other.go` | `//go:build !linux`：`Read()` 返回零值，其他平台完全不受影响 |

两条实现约束：

- **失败即零值，不报错**：状态目录不存在、`colors.toml` 读不到、accent 不是 `#rrggbb`、`hyprctl` 不在或超时，都只是「这一项没读到」，界面保持设计值。`colors.toml` 用逐行扫描而不是 TOML 库：文件是单层 `key = "值"`，只用两个键。
- **只信 JSON 里的 `int`**：`hyprctl` 的人类可读输出是拼给人看的句子，`-j` 的 `int` 才是数值；负数或解析失败按未知处理。

`Desktop` 的零值含义被写进类型注释：**「不知道」与「圆角 0」是两件事**，前者保留设计，后者把角收平；因此 `Rounding` 带 `HasRounding`、accent 带 `HasAccent`。

### 1.2 颜色：只换 accent 族

`(Palette) Accented(accent)` 换掉 7 个令牌，其余（画布、表面、文字、状态色）保持设计值——accent 是强调，不是第二套配色：

| 令牌 | 规则 |
|---|---|
| `Accent` / `Focus` | 桌面的 accent 原值 |
| `AccentHover` | accent 向白 10%（与设计一致：两套配色悬停都变亮） |
| `AccentPressed` | accent 向黑 12%（设计：按下变深） |
| `AccentSoft` / `Selected` | accent 套用该配色原有的 alpha（0.15 / 0.12），不另造强度 |
| `OnAccent` | 白或黑，取与 accent 的 WCAG 对比度更高者 |

最后一条是必需的：设计的 accent 在浅色里是深红（`#B01B2E`，配白字）、在深色里是浅粉（`#F26D7E`，配近黑字），桌面 accent 不能继承其中任一个。实测两组：`#7aa2f7`（相对亮度 0.40）选黑字，`#8839ef`（0.14）选白字。

### 1.3 圆角：按 compositor 的值缩放设计值

`Corners.Radius(r)`：

```
未知（非 Linux / hyprctl 没答） → r                      // 设计值
Rounding == 0                   → 0                      // 直角
0 < Rounding < RadiusSM(8)      → r × Rounding/8         // 按比例
Rounding >= 8                   → r                      // 设计值，不再放大
和 999（RadiusFull）            → 999                    // 药丸是形状，不是角
```

所有自绘圆角统一走 `Design.Radius(...)`，共 **91 处**（`components`/`pages`/`shell`）；内置控件的 `ui.Theme.Radius` 也在 `Palette.Apply` 里走同一函数，所以按钮、输入框与自绘面板的角是一致的。`RadiusFull` 只在一处判断里豁免，调用点不需要各自记规则。

写着具体像素的圆（播放键 42/21、舞台 52/26、状态点 6/3 与 4/2、2 px 进度柱）不参与缩放：这些都是「圆或药丸」这种形状，与 `RadiusFull` 同理，桌面收角时它们仍然是圆的。

### 1.4 明亮度：`mode` 接管「跟随系统」

`State.SystemDark(windowDark)`：桌面报了 `mode` 就用它，否则用窗口主题。设置里的「浅色 / 深色」仍然优先，`跟随系统` 才受它影响。这样 Omarchy 换浅色主题时客户端不用改设置就跟着变亮。

### 1.5 启动与轮询（`internal/app/desktop.go`）

- `prepare()` 末尾 `observeDesktop()`：**建窗口之前**读一次并 `SetDesktop`，第一帧就是 Omarchy 的配色。日志一行 `desktop: …`，便于验收时对时间线。
- `Main()` 在 `prepare` 之后启动 `watchDesktop`：2 s 一次 `tokens.Read()`，与上一轮 `Equal` 不同才 `win.Update(...)`（在主线程改状态并重绘）。
- 只有「桌面答过话」才起轮询（`Theme != "" || HasRounding`），macOS/Windows 上不会有一个空转的定时器。
- 主题切换是**整目录 rm -rf + mv**，文件级 inotify 会跟着旧 inode 失效；因此这里是重新读目录内容，不挂监听——与计划里「需要重新挂载监听」的结论相比，2 s 轮询两个小文件加一次 `hyprctl` 更简单，也没有跨目录重建的监听复杂度。

### 1.6 不动的东西

- `tokens.Light` / `tokens.Dark` 的字面量**一行没改**（`a1_test.go` 锁的 `#F4F4F1` / `#0E0E10` / `#B01B2E` / `#F26D7E` / 玻璃 alpha 全部原样）。
- 新增测试：`tokens/desktop_test.go`（accent 派生、圆角映射、`Apply`、`Desktop`）、`tokens/desktop_linux_test.go`（假状态目录、坏值、hex）、`model/desktop_test.go`（`SystemDark`）。

---

## 2. 证据

### 2.1 启动顺序（临时数据目录 `/tmp/m4-data`，3 首测试曲）

```
library: created /tmp/m4-data/library.sqlite (schema version 2)
desktop: theme tokyo-night (dark), accent #7aa2f7, rounding 0
window: 1200x760, minimum 900x600
startup window created    153.4ms since main
startup ready             204.5ms since main
```

`desktop:` 在 `window created` 之前，就是「先读再建窗口」的直接证据。

### 2.2 主题切换与还原

```
$ omarchy theme set catppuccin-latte
$ cat ~/.local/state/omarchy/current/theme.name     → catppuccin-latte
$ grep -E '^(mode|accent)' …/theme/colors.toml      → mode = "light"  accent = "#1e66f5"
desktop: theme catppuccin-latte (light), accent #1e66f5, rounding 0
…切回…
$ omarchy theme set tokyo-night
$ cat ~/.local/state/omarchy/current/theme.name     → tokyo-night
$ grep -E '^(mode|accent)' …/theme/colors.toml      → mode = "dark"  accent = "#7aa2f7"
desktop: theme tokyo-night (dark), accent #7aa2f7, rounding 0
```

两张截图之间没有重启进程：浅色那张是同一进程在切换后约 6 s 截的。

| 截图 | 内容 |
|---|---|
| [`m4-theme-tokyo-night.png`](../screenshots/omarchy/m4-theme-tokyo-night.png) | tokyo-night，深色，accent 蓝，圆角 0（全直角） |
| [`m4-theme-catppuccin-latte.png`](../screenshots/omarchy/m4-theme-catppuccin-latte.png) | catppuccin-latte，浅色，accent 蓝紫，圆角 0 |

### 2.3 圆角与还原

**计划里写的 `hyprctl keyword decoration:rounding 6` 在本机不成立**：Hyprland 0.56.2 的配置是 Lua 解析器，命令直接拒绝——

```
$ hyprctl keyword decoration:rounding 6
keyword can't work with non-legacy parsers. Use eval.
```

改用运行时 Lua 求值（只改内存，不写任何配置文件）：

```
$ hyprctl eval 'hl.config({ decoration = { rounding = 6 } })'   → ok
$ hyprctl -j getoption decoration:rounding                      → {"int": 6, "set": true}
desktop: theme tokyo-night (dark), accent #7aa2f7, rounding 6
$ hyprctl eval 'hl.config({ decoration = { rounding = 0 } })'   → ok
$ hyprctl -j getoption decoration:rounding                      → {"int": 0, "set": true}
desktop: theme tokyo-night (dark), accent #7aa2f7, rounding 0
```

| 截图 | 内容 |
|---|---|
| [`m4-rounding-6.png`](../screenshots/omarchy/m4-rounding-6.png) | rounding 6：卡片/按钮/封面/搜索框都收成 6/8 的角，药丸不变 |
| [`m4-rounding-0-restored.png`](../screenshots/omarchy/m4-rounding-0-restored.png) | 还原后与 #1 逐字节相同 |

```
e49583e79f8c6ff41fdabcb3da89b279  m4-theme-tokyo-night.png
e49583e79f8c6ff41fdabcb3da89b279  m4-rounding-0-restored.png
```

两张 md5 相同 = 还原回到的帧与测试前完全一致（主题、配色、圆角、布局都没有残留差异）。

**没有留下配置改动**：`hyprctl eval` 只作用于运行中的 compositor，`~/.config/hypr/**` 与 Omarchy 主题目录都没有被编辑；还原后再跑一次 `getoption` 与 `theme.name` 均为初值 0 / `tokyo-night`。

### 2.4 闸门

| 命令 | 结果 |
|---|---|
| `CGO_ENABLED=0 go build ./...` | 通过 |
| `CGO_ENABLED=0 go test -short ./...` | 全部 ok（含新增 3 个测试文件） |
| `CGO_ENABLED=0 GOOS=windows go build ./...` | 通过 |
| `CGO_ENABLED=0 GOOS=darwin go build ./...` | 通过 |
| `go vet ./...` | 通过 |
| `gofmt -l .` | 无输出 |

窗口关闭走 `hyprctl dispatch 'hl.dsp.window.close()'`，进程日志 `session: now playing cleared` / `playback: engine closed` / `app: quit complete`，PID 自查已退出；只关了自己那一个窗口（其余同标题窗口属于别的 agent，未动）。

---

## 3. 回退路径

按顺序，代价从小到大：

1. **运行时关掉**（不改代码）：任何一项读不到就自动退回设计值。例如 `XDG_STATE_HOME=/tmp/empty ./tingyu` 让主题读不到，accent 回到设计的朱红/浅粉；`PATH` 里没有 `hyprctl` 则圆角回到设计值 8/16/22。
2. **撤掉接线**：`internal/app/desktop.go` + `Main`/`prepare` 的两处调用、`shell.View` 的 `SystemDark`/`Resolve` 参数。撤掉后 `tokens.Read()` 不再被调用，其余代码照常编译。
3. **整笔回退**：`git revert <commit>`。本笔新增 7 个 Go 文件（读取器、接线、3 个测试）与截图/记录，改动 25 个文件的调用点，没有数据/协议变更，回退不需要迁移。

---

## 4. 没验证 / 已知限制

| 项 | 说明 |
|---|---|
| 非 Omarchy 的 Hyprland | 只在本机（Omarchy 4.5 + Hyprland 0.56.2）验证；`hyprctl` 的 JSON 形状在别的版本上没抽过 |
| 圆角只试了 0 与 6 | 未逐一试 1–7 与 >8 的像素效果；比例公式本身有单测（`TestCornersFollowTheCompositor`） |
| 其他主题 | 只试了 tokyo-night 与 catppuccin-latte 两套；其余主题的 `mode`/`accent` 组合未逐一截图 |
| macOS / Windows 运行时 | 只做了交叉编译；`Read()` 在两个平台恒为零值，行为应与改动前一致，但没有真机跑过 |
| 轮询开销 | **已实测（§4.1）**：一次约 2.7 ms CPU（客户端 0.27 + `hyprctl` 2.4），每天 43,200 次 `hyprctl`、约 115 s CPU ≈ 单核 0.13%；整机空转对照看不出 |
| accent 的影响面 | 只到 accent 族与选中态；舞台的封面取色、侧栏封面氛围仍来自封面本身，不随桌面 accent 变化（有意为之，属封面氛围而不是主题色） |
| 高斯模糊 / 窗口描边 | compositor 侧的窗口圆角与模糊是 Hyprland 的事，本笔不接管；客户端只改自绘内容 |

### 4.1 轮询开销实测（2026-11-12）

方法（都在本机，跑完即删临时测试，不进仓库）：

1. **客户端自己**：临时测试用 `getrusage(RUSAGE_SELF)` 夹住 200 次 `tokens.Read()`。
2. **`hyprctl` 子进程**：`time -p` 跑 50 次 `hyprctl -j getoption decoration:rounding`，取 user+sys（子进程 CPU 不计入客户端自己的 rusage，只能单独量）。
3. **整机空转对照**：临时数据目录起应用，用 `/proc/<pid>/stat` 的 `utime+stime` 差值比「轮询开」与「轮询关」两轮（60 s 与 120 s 各一对）。关掉的方式是 `XDG_STATE_HOME` 指向空目录、`PATH` 里放一个立即退出的 `hyprctl` 桩：`Read()` 两项都没读到，poller 因此不启动（日志 `desktop: theme none (unstated), accent design, rounding unstated` 可以确认）。

| 项 | 实测 |
|---|---|
| 一次 `Read()`（客户端自己的 CPU） | **0.27 ms**（200 次共 53 ms：user 34 ms + sys 20 ms），200 次墙钟 0.48 s |
| 一次 `hyprctl` 子进程 | **2.4 ms CPU**（50 次共 0.12 s：user 0.06 + sys 0.06），墙钟与 CPU 同量级 |
| 一次轮询合计 | ≈ **2.7 ms CPU**，墙钟同样 ≈ 2.4 ms（两边重叠，不是相加） |
| 每天 `hyprctl` 次数 | **43,200**（2 s 一次；按客户端连续开一天算） |
| 每天 CPU | 客户端 11.5 s + `hyprctl` 104 s ≈ **115 s ≈ 单核 0.13%**（20 核机器上 0.007%） |
| 整机空转对照 | 60 s：开 173 ticks / 关 158 ticks；120 s：321 / 306。差值都是 15 ticks（≈ 0.25% 单核），与帧循环本身的抖动同量级、两轮不变，说明轮询在进程层面量不出来 |

结论：开销可以忽略，且主要花在 `hyprctl` 这个短命子进程上，不在客户端里；所以设置里只做功能开关（§5），没有为省 CPU 再加一个「降低轮询频率」。

---

## 5. 同日追加：设置页文案与「跟随桌面主题」

| 项 | 内容 |
|---|---|
| 文案 | 外观 → 主题模式的说明从「跟随系统时，随 macOS 的外观自动切换」改为「跟随系统时明暗取自 Omarchy 主题；强调色与圆角也跟随桌面，其它桌面用自带配色。」 |
| 开关 | 新增「跟随桌面主题」（默认开，`settings.json` 的 `followDesktop`）。关掉后 `State.Appearance()` 返回零值：强调色、明暗、圆角一起回到设计自带的值，等同于非 Omarchy 平台 |
| 兼容 | `followDesktop` 是本客户端新增的键；老 `settings.json` 没有它时按默认（开）读。Flutter 客户端读到不认识的键会怎样没有验证——最坏是那边保存设置时丢掉这个键，本客户端回到默认（开） |
| 测试 | `TestSettingsTolerantReadAndAtomicWrite`（缺键默认开 + 关掉后持久化）、`TestSystemDarkFollowsOmarchy`（关掉后明暗不再跟桌面、`Appearance()` 归零）、`TestSettingsTabsReachEverySectionAndResetAppearance`（外观区文本清单加入新行） |
| 中英 | 客户端这一页只有中文文案（仓库里没有英文 UI 串），所以只改一处，没有新增英文；`tools/designpreview` 里的同句是设计稿复刻，未改 |
| 截图 | [`m4-settings-follow-desktop-on.png`](../screenshots/omarchy/m4-settings-follow-desktop-on.png)（开：强调色是 Omarchy 的 `#7aa2f7`，圆角 0 时为直角）与 [`m4-settings-follow-desktop-off.png`](../screenshots/omarchy/m4-settings-follow-desktop-off.png)（关：回到设计朱红 `#F26D7E` 与设计圆角）。关的那张是先写 `settings.json` 的 `followDesktop:false` 再启动——验证的是「读设置 → 回到设计」这条真实路径；设置页开关本身的点击没在截图里驱动（本机没有可用的指针注入工具） |
