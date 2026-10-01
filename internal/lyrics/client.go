package lyrics

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const endpoint = "https://music.163.com/api/song/lyric"

type Client struct {
	http *http.Client
}

type lyricField struct {
	Lyric string `json:"lyric"`
}

type response struct {
	Code     int        `json:"code"`
	LRC      lyricField `json:"lrc"`
	TLyric   lyricField `json:"tlyric"`
	YRC      lyricField `json:"yrc"`
	YTLRC    lyricField `json:"ytlrc"`
	RomaLRC  lyricField `json:"romalrc"`
	YRomaLRC lyricField `json:"yromalrc"`
}

func NewClient(client *http.Client) *Client {
	if client == nil {
		client = &http.Client{Timeout: 12 * time.Second}
	}
	return &Client{http: client}
}

func (c *Client) Fetch(ctx context.Context, songID int64) (Data, error) {
	query := url.Values{
		"id": {strconv.FormatInt(songID, 10)}, "lv": {"-1"}, "kv": {"-1"},
		"tv": {"-1"}, "yv": {"-1"}, "ytv": {"-1"}, "yrv": {"-1"}, "rv": {"-1"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+query.Encode(), nil)
	if err != nil {
		return Data{}, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) lyricsync/1")
	req.Header.Set("Referer", "https://music.163.com/")
	resp, err := c.http.Do(req)
	if err != nil {
		return Data{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Data{}, fmt.Errorf("网易云接口返回 HTTP %s", resp.Status)
	}
	var payload response
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return Data{}, fmt.Errorf("解析网易云响应: %w", err)
	}
	if payload.Code != 200 {
		return Data{}, fmt.Errorf("网易云接口状态码 %d", payload.Code)
	}
	original := ParseLRC(payload.LRC.Lyric)
	// 和 go-musicfox 一样优先使用传统 tlyric：部分歌曲的 ytlrc 会缺行。
	// 时间轴存在小偏差时，渲染层会在 600ms 内取最近一行完成对齐。
	translatedRaw := payload.TLyric.Lyric
	if translatedRaw == "" {
		translatedRaw = payload.YTLRC.Lyric
	}
	romanRaw := payload.YRomaLRC.Lyric
	if romanRaw == "" {
		romanRaw = payload.RomaLRC.Lyric
	}
	data := Data{
		Original:   LinesFromLRC(original),
		YRC:        ParseYRC(payload.YRC.Lyric),
		Translated: ParseLRC(translatedRaw),
		Romanized:  ParseLRC(romanRaw),
	}
	if len(data.Original) == 0 && len(data.YRC) == 0 {
		return Data{}, fmt.Errorf("歌曲 %d 暂无歌词", songID)
	}
	return data, nil
}
