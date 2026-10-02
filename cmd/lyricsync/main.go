package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/oukaromf/lyricsync/internal/app"
	"github.com/oukaromf/lyricsync/internal/external"
	"github.com/oukaromf/lyricsync/internal/lyrics"
	"github.com/oukaromf/lyricsync/internal/mpris"
	"github.com/oukaromf/lyricsync/internal/native"
	"github.com/oukaromf/lyricsync/internal/state"
)

var version = "dev"

func main() {
	log.SetFlags(0)
	if err := run(os.Args[1:]); err != nil {
		log.Printf("lyricsync: %v", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) > 0 {
		if strings.HasPrefix(args[0], "chrome-extension://") {
			return runNative(args)
		}
		switch args[0] {
		case "native-host":
			return runNative(args[1:])
		case "install-native-host":
			return native.Install(args[1:], os.Stdout)
		case "toggle":
			mode, err := state.Toggle()
			if err == nil {
				fmt.Println(mode)
			}
			return err
		case "mode":
			if len(args) != 2 {
				return errors.New("用法: lyricsync mode translation|romanization")
			}
			mode, err := state.ParseMode(args[1])
			if err != nil {
				return err
			}
			return state.WriteMode(mode)
		case "version", "--version", "-version":
			fmt.Println(version)
			return nil
		}
	}

	fs := flag.NewFlagSet("lyricsync", flag.ContinueOnError)
	player := fs.String("player", "musicfox", "MPRIS 播放器名；auto 表示任意正在播放的播放器")
	interval := fs.Duration("interval", 80*time.Millisecond, "Waybar 输出刷新间隔")
	poll := fs.Duration("poll", 500*time.Millisecond, "MPRIS 查询间隔")
	offset := fs.Duration("offset", 0, "歌词时间偏移，例如 -200ms")
	once := fs.Bool("once", false, "只输出一次 JSON")
	hideWhenInactive := fs.Bool("hide-when-inactive", false, "未检测到目标播放器时输出空文本")
	externalSubtitles := fs.Bool("external-subtitles", false, "优先显示 CCTracker 网页字幕，失效时回退到 MPRIS 歌词")
	outputName := fs.String("output", "combined", "输出内容：combined、original、translation、romanization、secondary")
	id := fs.Int64("id", 0, "直接指定网易云歌曲 ID（调试用）")
	position := fs.Duration("position", 0, "配合 -id 指定播放位置（调试用）")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *interval < 20*time.Millisecond {
		return errors.New("-interval 不能小于 20ms")
	}
	outputPart, err := app.ParseOutputPart(*outputName)
	if err != nil {
		return err
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	client := lyrics.NewClient(nil)
	engine := app.NewEngine(client, *offset)

	if *id != 0 {
		track := mpris.Track{ID: *id, Position: *position, Status: mpris.Playing, Title: "网易云歌曲 " + strconv.FormatInt(*id, 10)}
		if err := engine.Update(ctx, track); err != nil {
			return err
		}
		return writeOutput(engine.Output(track, state.ReadMode(), outputPart))
	}

	source, err := mpris.New(*player, *poll)
	if err != nil {
		if !*externalSubtitles {
			return fmt.Errorf("连接 MPRIS: %w", err)
		}
		log.Printf("lyricsync: 连接 MPRIS: %v；继续接收外部字幕", err)
	} else {
		defer source.Close()
	}
	var captions *external.Reader
	if *externalSubtitles {
		captions = external.NewReader()
	}
	var playback trackSource
	if source != nil {
		playback = source
	}
	return stream(ctx, playback, engine, outputPart, *interval, *once, *hideWhenInactive, captions, os.Stdout)
}

type trackSource interface {
	Track(context.Context) (mpris.Track, error)
}

func stream(ctx context.Context, source trackSource, engine *app.Engine, outputPart app.OutputPart, interval time.Duration, once, hideWhenInactive bool, captions *external.Reader, writer io.Writer) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	var lastKey string
	var nextFetch time.Time
	// External subtitles must remain responsive while NetEase is slow/offline.
	results := make(chan error, 1)
	fetching := false
	for {
		if fetching {
			select {
			case err := <-results:
				fetching = false
				if err != nil {
					log.Printf("lyricsync: 获取歌词失败: %v", err)
					nextFetch = time.Now().Add(30 * time.Second)
				} else {
					nextFetch = time.Time{}
				}
			default:
			}
		}
		if captions != nil {
			if snapshot, ok := captions.Current(time.Now()); ok {
				if err := writeOutputTo(writer, app.ExternalOutput(snapshot, outputPart)); err != nil {
					if errors.Is(err, syscall.EPIPE) {
						return nil
					}
					return err
				}
				if once {
					return nil
				}
				select {
				case <-ctx.Done():
					return nil
				case <-ticker.C:
				}
				continue
			}
		}
		var track mpris.Track
		var err error
		if source != nil {
			track, err = source.Track(ctx)
		}
		if err != nil && !errors.Is(err, mpris.ErrNoPlayer) {
			log.Printf("lyricsync: MPRIS: %v", err)
		}
		key := fmt.Sprintf("%s:%d", track.Player, track.ID)
		shouldRetry := !nextFetch.IsZero() && time.Now().After(nextFetch)
		if track.ID > 0 && !fetching && (key != lastKey || shouldRetry) {
			if captions != nil && !once {
				fetching = true
				go func(track mpris.Track) { results <- engine.Update(ctx, track) }(track)
			} else if err := engine.Update(ctx, track); err != nil {
				log.Printf("lyricsync: 获取歌曲 %d 歌词失败: %v", track.ID, err)
				nextFetch = time.Now().Add(30 * time.Second)
			} else {
				nextFetch = time.Time{}
			}
			lastKey = key
		}
		output := engine.Output(track, state.ReadMode(), outputPart)
		if hideWhenInactive && track.ID == 0 {
			output.Text = ""
			output.Tooltip = ""
		}
		if err := writeOutputTo(writer, output); err != nil {
			if errors.Is(err, syscall.EPIPE) {
				return nil
			}
			return err
		}
		if once {
			return nil
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func writeOutput(output app.Output) error {
	return writeOutputTo(os.Stdout, output)
}

func writeOutputTo(writer io.Writer, output app.Output) error {
	enc := json.NewEncoder(writer)
	enc.SetEscapeHTML(false)
	return enc.Encode(output)
}

func runNative(args []string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return native.Serve(ctx, args, os.Stdin, os.Stdout)
}
