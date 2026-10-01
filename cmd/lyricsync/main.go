package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/oukaromf/lyricsync/internal/app"
	"github.com/oukaromf/lyricsync/internal/lyrics"
	"github.com/oukaromf/lyricsync/internal/mpris"
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
		switch args[0] {
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
	id := fs.Int64("id", 0, "直接指定网易云歌曲 ID（调试用）")
	position := fs.Duration("position", 0, "配合 -id 指定播放位置（调试用）")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *interval < 20*time.Millisecond {
		return errors.New("-interval 不能小于 20ms")
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
		return writeOutput(engine.Output(track, state.ReadMode()))
	}

	source, err := mpris.New(*player, *poll)
	if err != nil {
		return fmt.Errorf("连接 MPRIS: %w", err)
	}
	defer source.Close()

	return stream(ctx, source, engine, *interval, *once, *hideWhenInactive)
}

func stream(ctx context.Context, source *mpris.Source, engine *app.Engine, interval time.Duration, once, hideWhenInactive bool) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	var lastKey string
	var nextFetch time.Time
	for {
		track, err := source.Track(ctx)
		if err != nil && !errors.Is(err, mpris.ErrNoPlayer) {
			log.Printf("lyricsync: MPRIS: %v", err)
		}
		key := fmt.Sprintf("%s:%d", track.Player, track.ID)
		shouldRetry := !nextFetch.IsZero() && time.Now().After(nextFetch)
		if track.ID > 0 && (key != lastKey || shouldRetry) {
			if err := engine.Update(ctx, track); err != nil {
				log.Printf("lyricsync: 获取歌曲 %d 歌词失败: %v", track.ID, err)
				nextFetch = time.Now().Add(30 * time.Second)
			} else {
				nextFetch = time.Time{}
			}
			lastKey = key
		}
		output := engine.Output(track, state.ReadMode())
		if hideWhenInactive && track.ID == 0 {
			output.Text = ""
			output.Tooltip = ""
		}
		if err := writeOutput(output); err != nil {
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
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	return enc.Encode(output)
}
