# lyricsync

`lyricsync` 把 go-musicfox 当前播放歌曲的网易云歌词实时输出给 Waybar。它参考
[go-musicfox 的歌词获取与 YRC 解析思路](https://github.com/go-musicfox/go-musicfox/blob/master/utils/netease/lyric.go)，
支持：

- MPRIS 自动读取歌曲 ID、播放状态和进度，无需读取 go-musicfox 内部文件；
- 网易云 `YRC` 逐字时间轴，传统 `LRC` 自动回退；
- 原文、翻译、罗马音可独立输出，由 Waybar 组合成双行显示；
- 左键在“翻译 / 罗马音”间即时切换；
- 播放中的词加粗、下划线，未播放部分半透明；
- 暂停后停止推进，切歌自动重新获取歌词。

## 构建与安装

需要 Go 1.23 或更新版本。首次构建需要下载一个轻量的 D-Bus 依赖。

```bash
CGO_ENABLED=0 go build -buildvcs=false -o lyricsync ./cmd/lyricsync
install -Dm755 lyricsync ~/.local/bin/lyricsync
```

也可以运行：

```bash
make build
make install PREFIX="$HOME/.local"
```

### Nix Flake

无需安装到系统即可运行：

```bash
nix run github:oukaromf/lyricsync
# 在当前源码目录中：
nix run .
```

构建、进入开发环境或安装到当前用户 profile：

```bash
nix build .
nix develop
nix profile install .
```

在其他 flake 中引入 overlay：

```nix
inputs.lyricsync.url = "github:oukaromf/lyricsync";
```

然后在 NixOS 或 Home Manager module 中：

```nix
{ inputs, pkgs, ... }: {
  # 在 NixOS / Home Manager 配置使用的 pkgs 上：
  nixpkgs.overlays = [ inputs.lyricsync.overlays.default ];

  # NixOS 使用这一项：
  environment.systemPackages = [ pkgs.lyricsync ];

  # Home Manager 则使用这一项：
  home.packages = [ pkgs.lyricsync ];
}
```

使用 Home Manager 管理 Waybar 时，可以直接引用 Nix store 中的可执行文件：

```nix
programs.waybar.settings.mainBar = {
  "group/lyrics" = {
    orientation = "vertical";
    modules = [ "custom/lyrics-secondary" "custom/lyrics-original" ];
  };
  "custom/lyrics-secondary" = {
    exec = "${pkgs.lyricsync}/bin/lyricsync -output secondary";
    on-click = "${pkgs.lyricsync}/bin/lyricsync toggle";
    return-type = "json";
    restart-interval = 2;
    exec-on-event = false;
    escape = false;
  };
  "custom/lyrics-original" = {
    exec = "${pkgs.lyricsync}/bin/lyricsync -output original";
    on-click = "${pkgs.lyricsync}/bin/lyricsync toggle";
    return-type = "json";
    restart-interval = 2;
    exec-on-event = false;
    escape = false;
  };
};
```

确保 go-musicfox 的 MPRIS 功能没有被关闭，并确认 `~/.local/bin` 在 Waybar
进程的 `PATH` 中。

## Waybar 配置

把 [waybar-module.jsonc](./examples/waybar-module.jsonc) 中的模块加入 Waybar
配置，并将 `group/lyrics` 放入 `modules-left`、`modules-center` 或
`modules-right`。然后把 [style.css](./examples/style.css) 追加到 Waybar 样式表。

示例模块的 `exec` 是一个持续运行的进程，每 80ms 输出一条 Waybar JSON；不要再
设置 `interval`。`restart-interval` 只用于进程意外退出后重启。歌词包含 Pango 标记，
所以需要保留 `escape: false`。`exec-on-event: false` 可避免点击时由 Waybar 重启这个
持续进程；程序会自行在下一帧读到切换后的模式。上下两行分别由独立模块输出，换行与
排版交给 Waybar 的垂直 group 处理，不需要增加整条栏的 `height`。

修改配置后重启 Waybar：

```bash
pkill -SIGUSR2 waybar
```

## 命令行

```text
lyricsync                         持续输出 go-musicfox 歌词
lyricsync -player auto            选择任意正在播放的 MPRIS 播放器
lyricsync -offset -200ms           把歌词提前 200ms
lyricsync toggle                  切换翻译 / 罗马音
lyricsync mode translation        固定为翻译
lyricsync mode romanization       固定为罗马音
lyricsync -output original        只输出原文（保留逐字高亮）
lyricsync -output translation     只输出翻译
lyricsync -output romanization    只输出罗马音
lyricsync -output secondary       按当前模式输出翻译或罗马音
lyricsync -id 524152942 -position 1m5s  调试指定歌曲和位置
lyricsync -once                   只输出一条 JSON
lyricsync -hide-when-inactive     未检测到 musicfox 时输出空文本，便于 Waybar 回退
```

默认只选择名称以 `musicfox` 开头的 MPRIS 实例。`-player auto` 也能用于其他播放器，
但它们必须在 `mpris:trackid` 或 `xesam:url` 中暴露网易云歌曲 ID，否则无法确定要获取
哪首歌词。

## 输出格式

程序逐行输出 Waybar 的 `return-type: json` 格式。默认的 `combined` 模式为兼容旧配置
保留内嵌换行；推荐为 Waybar 的两个垂直子模块分别使用 `secondary` 与 `original`：

```json
{"text":"<span size=\"small\" alpha=\"75%\">翻译</span>\n已播放<b><u>当前词</u></b><span alpha=\"55%\">未播放</span>","tooltip":"歌名\n歌手\n显示：翻译 · 左键切换","class":"playing","percentage":42}
```

`secondary` 在当前模式对应的副歌词缺失时会自动回退到另一种副歌词；明确选择
`translation` 或 `romanization` 时不会回退，缺失内容将输出为空。网易云接口可能因
网络、地区或版权策略无法返回个别歌曲歌词。

## 致谢与许可

歌词字段选择与 YRC 格式兼容逻辑参考了
[go-musicfox](https://github.com/go-musicfox/go-musicfox)。本项目采用 GPL-3.0-or-later。
