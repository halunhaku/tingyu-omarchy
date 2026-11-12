# 听屿 Omarchy 版

基于 [halunhaku/tingyu](https://github.com/halunhaku/tingyu) 的独立 Omarchy（Linux）版开发仓库。沿用 Go、MyGo 和 libmpv 的客户端，界面与 Windows 版同源。Windows 版说明见 [README-windows.md](README-windows.md)，macOS 原版见 [README-macos.md](README-macos.md)。

当前为开发预览版：已在 Omarchy（Hyprland 0.56.2，Wayland）本机完成构建、窗口显示、曲库初始化和 libmpv 加载，**尚未做播放、云盘、媒体键与分发的验收，也未发布安装包**。

![听屿 Omarchy 版首次启动的添加来源界面](docs/screenshots/omarchy/w0-onboarding.png)

## 已跑通

- Go 1.27.3、`CGO_ENABLED=0` 构建通过；`go test -short ./...` 全绿。
- Hyprland/Wayland 下开窗，从 `main` 到就绪约 190–300 ms，1.6 分数缩放输出下界面渲染正常。
- 数据目录按 XDG 落在 `~/.local/share/com.halunhaku.tingyu`（相对的 `XDG_DATA_HOME` 按规范忽略），可用 `TINGYU_DATA_DIR` 覆盖。
- 系统 libmpv 0.41 自动加载（`libmpv.so.2` → `.1` → `.0`，client API 2.5），不需要设 `MPV_LIB`。
- MPRIS 会话（`mygo.NowPlaying`）就绪；关窗后进程退出、引擎释放、MPRIS 清空已复验。
- 「显示所在文件夹」走 freedesktop `FileManager1`，失败回退 `xdg-open`。

## 从源码构建

需要 x86_64 的 Omarchy，以及网络访问 Go 模块与下载站点。脚本不向系统安装 Go，也不改 PATH；工具链默认放在 `~/.local/share/tingyu-tools`，可用 `--tools` 或 `TINGYU_TOOLS` 指定。

```bash
git clone https://github.com/halunhaku/tingyu-omarchy.git
cd tingyu-omarchy/client

# 下载并校验 Go 1.27.3，顺带检查运行库
./scripts/linux-setup.sh

# 构建 build/tingyu
./scripts/linux-build.sh

# 启动（会自动使用系统 libmpv）
./build/tingyu
```

运行库由 Omarchy 自带：`gtk3`、`webkit2gtk-4.1`、`libsecret`、`mpv`。

## 已知限制

- **窗口标识**：Wayland 的 app id 取二进制名（`tingyu`），`.desktop` 与图标尚未落地。
- **未验收**：真实出声、硬件解码、媒体键、MPRIS 控件、拖入目录、云盘登录、二维码登录。
- **性能门槛**：`internal/shell` 的 10000 行滚动 p95 闸门（16.7 ms）在本机实测 16.7–17.6 ms，临界失败；`go test -short` 跳过它，默认 `go test ./...` 仍然执行。
- **分发**：没有 PKGBUILD、`.desktop`、图标，也没有加入 Omarchy 的软件包源。

## 项目文档

- [迁移范围与里程碑](docs/plan/2026-11-12-omarchy-migration-scope.md)
- [设计说明](DESIGN.md)
- 上游 macOS 版：<https://github.com/halunhaku/tingyu>
